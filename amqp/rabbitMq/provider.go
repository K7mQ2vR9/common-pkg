package rabbitmq

import (
	"context"
	"errors"
	"time"

	"github.com/K7mQ2vR9/base-pkg/commonTool"
	"github.com/streadway/amqp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// Publish 发送一条消息，成功表示 broker 已确认且消息已路由。
// MessageId、ContentType、Timestamp 和 DeliveryMode 未填时使用默认值。
// 消息头中的 Request ID 与 Trace ID 沿用当前链路，冲突返回错误；独立发布缺少链路时生成一次。
func (r *RabbitMQ) Publish(ctx context.Context, message amqp.Publishing) error {
	if ctx == nil {
		return errors.New("rabbitmq publish context is required")
	}
	ctx, err := messageTraceContext(ctx, message.Headers)
	if err != nil {
		return err
	}
	ctx, span := otel.Tracer("base-pkg/rabbitmq").Start(ctx, "publish "+r.config.QueueName, trace.WithSpanKind(trace.SpanKindProducer))
	defer span.End()
	message.Headers = messageTraceHeaders(ctx, message.Headers)
	if message.MessageId == "" {
		id, err := commonTool.NewUUidV7()
		if err != nil {
			return &operationError{"generate message ID", err}
		}
		message.MessageId = id
	}
	if message.ContentType == "" {
		message.ContentType = "application/json"
	}
	if message.Timestamp.IsZero() {
		message.Timestamp = time.Now().UTC()
	}
	if message.DeliveryMode == 0 {
		message.DeliveryMode = amqp.Transient
		if r.config.persistent {
			message.DeliveryMode = amqp.Persistent
		}
	}
	r.mu.RLock()
	s, closed := r.session, r.closed
	r.mu.RUnlock()
	if closed {
		return ErrClosed
	}
	if s == nil {
		return ErrNotReady
	}
	return r.publishOn(ctx, s, r.config.Exchange, r.config.RoutingKey, message)
}

// PublishBatch 按顺序等待各条发布确认，部分成功后返回错误时由调用方按 MessageId 幂等重试。
func (r *RabbitMQ) PublishBatch(ctx context.Context, messages []amqp.Publishing) error {
	for _, message := range messages {
		if err := r.Publish(ctx, message); err != nil {
			return err
		}
	}
	return nil
}

func (r *RabbitMQ) publishOn(ctx context.Context, s *session, exchange, key string, message amqp.Publishing) error {
	if err := message.Headers.Validate(); err != nil {
		return &operationError{"validate message headers", err}
	}
	ctx, cancel := context.WithTimeout(ctx, r.config.ConfirmTimeout)
	defer cancel()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.done:
		return ErrNotReady
	case s.publishLock <- struct{}{}:
	}
	defer func() { <-s.publishLock }()
	if err := ctx.Err(); err != nil {
		return err
	}
	// 发布时限到达后关闭当前连接，中断驱动中阻塞的网络写入或确认等待。
	stop := context.AfterFunc(ctx, func() { r.invalidate(s) })
	defer stop()
	tag := s.nextTag
	if err := s.publisher.Publish(exchange, key, true, false, message); err != nil {
		r.invalidate(s)
		return errors.Join(ErrPublishUnknown, ctx.Err(), &operationError{"publish", err})
	}
	s.nextTag++
	returned := false
	for {
		select {
		case _, ok := <-s.returns:
			if !ok {
				r.invalidate(s)
				return ErrPublishUnknown
			}
			returned = true
		case confirmation, ok := <-s.confirms:
			if !ok || confirmation.DeliveryTag != tag {
				r.invalidate(s)
				return ErrPublishUnknown
			}
			// broker 先发送 basic.return，再发送对应的发布确认。
			select {
			case _, ok := <-s.returns:
				if !ok {
					r.invalidate(s)
					return ErrPublishUnknown
				}
				returned = true
			default:
			}
			if !confirmation.Ack {
				return ErrPublishRejected
			}
			if returned {
				return ErrUnroutable
			}
			return nil
		case <-ctx.Done():
			r.invalidate(s)
			return errors.Join(ErrPublishUnknown, ctx.Err())
		case <-s.done:
			return errors.Join(ErrPublishUnknown, ctx.Err())
		case <-r.ctx.Done():
			r.invalidate(s)
			return errors.Join(ErrPublishUnknown, ErrClosed)
		}
	}
}
