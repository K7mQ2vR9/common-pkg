package rabbitmq

import (
	"context"
	"errors"
	"fmt"

	"github.com/streadway/amqp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// Publish 发布业务消息并返回实际 ID，nil 错误表示 broker 已确认且消息已路由。
// 分配 ID 后的任何失败都返回该 ID；结果不确定时重发须沿用同一 ID，并由业务实现幂等。
// 正文及 Headers 复制后发送，目标取 Config，消费专用字段不参与发布。
func (r *RabbitMQ) Publish(ctx context.Context, message Message) (string, error) {
	if ctx == nil {
		return message.ID, errors.New("rabbitmq publish context is required")
	}
	id, err := resolveMessageID(message.ID)
	if err != nil {
		return id, err
	}
	message.ID = id
	if err := ctx.Err(); err != nil {
		return id, err
	}
	r.mu.RLock()
	s, closed := r.session, r.closed
	r.mu.RUnlock()
	if closed {
		return id, ErrClosed
	}
	if s == nil {
		return id, ErrNotReady
	}
	wire, err := message.publishing(r.config.persistent)
	if err != nil {
		return id, err
	}
	ctx, err = messageTraceContext(ctx, wire.Headers)
	if err != nil {
		return id, err
	}
	ctx, span := otel.Tracer("base-pkg/rabbitmq").Start(ctx, "publish "+r.config.QueueName, trace.WithSpanKind(trace.SpanKindProducer))
	defer span.End()
	wire.Headers = messageTraceHeaders(ctx, wire.Headers)
	return id, r.publishOn(ctx, s, r.config.Exchange, r.config.RoutingKey, wire)
}

// PublishJSON 构造并发布 JSON 消息，未指定 ID 时自动生成；序列化失败不向 broker 发送。
// 分配后即使失败也返回消息 ID，底层错误支持 errors.Is/As。
func (r *RabbitMQ) PublishJSON(ctx context.Context, data any, options ...PublishOption) (string, error) {
	if ctx == nil {
		return "", errors.New("rabbitmq publish context is required")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	message, err := NewJSONMessage(data, options...)
	if err != nil {
		return message.ID, err
	}
	return r.Publish(ctx, message)
}

// PublishBatch 按输入顺序逐条等待确认，首次失败即停止，并返回实际 ID、已确认前缀数量及失败位置。
// 未尝试项保留原 ID，原 ID 为空时不分配；输入切片不修改，不自动重发部分成功的批次。
func (r *RabbitMQ) PublishBatch(ctx context.Context, messages []Message) (BatchResult, error) {
	result := BatchResult{MessageIDs: make([]string, len(messages)), FailedIndex: -1}
	for index, message := range messages {
		result.MessageIDs[index] = message.ID
	}
	if ctx == nil {
		return result, errors.New("rabbitmq publish context is required")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	for index, message := range messages {
		id, err := r.Publish(ctx, message)
		result.MessageIDs[index] = id
		if err != nil {
			result.FailedIndex = index
			return result, fmt.Errorf("rabbitmq batch message %d: %w", index, err)
		}
		result.ConfirmedCount++
	}
	return result, nil
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
