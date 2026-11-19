package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/K7mQ2vR9/base-pkg/commonTool"
	"github.com/K7mQ2vR9/base-pkg/headInfo"
	"github.com/streadway/amqp"
	"github.com/zeromicro/go-zero/core/logc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

const (
	retryCountHeader = "x-base-retry-count"
	retryAtHeader    = "x-base-retry-at"
	errorHeader      = "x-base-error"
)

// Consume 注册单个业务处理器并等待订阅成功，重连后自动恢复订阅。
// Handler 的 ctx 在连接失效或关闭时取消，手动确认模式按单条消息确认。
func (r *RabbitMQ) Consume(handler Handler) error {
	if handler == nil {
		return errors.New("rabbitmq handler is required")
	}
	sub := &subscription{handler: handler, ready: make(chan error, 1)}
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return ErrClosed
	}
	if r.sub != nil {
		r.mu.Unlock()
		return errors.New("rabbitmq consumer already registered")
	}
	r.sub = sub
	r.mu.Unlock()
	select {
	case r.changed <- struct{}{}:
	default:
	}
	select {
	case err := <-sub.ready:
		return err
	case <-r.ctx.Done():
		return ErrClosed
	}
}

func (r *RabbitMQ) startConsumers(ctx context.Context, s *session, sub *subscription, failures chan<- error) error {
	type consumer struct {
		queue             string
		workers, prefetch int
		autoAck           bool
		process           func(context.Context, amqp.Delivery) error
	}
	consumers := []consumer{
		{r.config.QueueName, r.config.workerNum, r.config.prefetch, r.config.AutoAck, func(ctx context.Context, d amqp.Delivery) error { return r.processBusiness(ctx, s, sub.handler, d) }},
		{r.config.RetryQueue, 1, 1, false, func(ctx context.Context, d amqp.Delivery) error { return r.processRetry(ctx, s, d) }},
	}
	if r.config.Telegram.Enabled {
		consumers = append(consumers, consumer{r.config.NotifyQueue, 1, 1, false, r.processNotification})
	}
	type stream struct {
		consumer
		deliveries <-chan amqp.Delivery
		closed     <-chan *amqp.Error
		canceled   <-chan string
	}
	streams := make([]stream, 0, len(consumers))
	for _, config := range consumers {
		ch, err := s.conn.Channel()
		if err != nil {
			return &operationError{"open consumer channel", err}
		}
		closed := ch.NotifyClose(make(chan *amqp.Error, 1))
		canceled := ch.NotifyCancel(make(chan string, 1))
		if err := ch.Qos(config.prefetch, 0, false); err != nil {
			return &operationError{"set consumer prefetch", err}
		}
		deliveries, err := ch.Consume(config.queue, "", config.autoAck, false, false, false, nil)
		if err != nil {
			return &operationError{"subscribe", err}
		}
		streams = append(streams, stream{config, deliveries, closed, canceled})
	}
	for _, stream := range streams {
		stream := stream
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			select {
			case <-ctx.Done():
				return
			case <-stream.closed:
			case <-stream.canceled:
			}
			select {
			case failures <- ErrNotReady:
			default:
			}
		}()
		for i := 0; i < stream.workers; i++ {
			r.wg.Add(1)
			go func() {
				defer r.wg.Done()
				for {
					select {
					case <-ctx.Done():
						return
					case delivery, ok := <-stream.deliveries:
						if !ok {
							select {
							case failures <- ErrNotReady:
							default:
							}
							return
						}
						if ctx.Err() != nil {
							return
						}
						if err := stream.process(ctx, delivery); err != nil {
							if ctx.Err() != nil {
								return
							}
							logc.Errorf(ctx, "RabbitMQ processing failed: %v", err)
							if !stream.autoAck {
								if err := delivery.Nack(false, true); err != nil {
									r.invalidate(s)
									return
								}
							}
							if !wait(ctx, r.config.ReconnectInterval) {
								return
							}
						}
					}
				}
			}()
		}
	}
	return nil
}

func (r *RabbitMQ) processBusiness(ctx context.Context, s *session, handler Handler, d amqp.Delivery) error {
	traced, traceErr := messageTraceContext(ctx, d.Headers)
	if traceErr != nil {
		if !errors.Is(traceErr, headInfo.ErrTraceInvalid) && !errors.Is(traceErr, headInfo.ErrTraceConflict) {
			return traceErr
		}
		// 错误链路消息不进入业务；使用本地诊断上下文归档，原始错误头留在归档中供排查。
		fallback, err := headInfo.EnsureTrace(ctx)
		if err != nil {
			return err
		}
		return r.archive(fallback, s, d, traceErr, r.config.AutoAck)
	}
	ctx, span := otel.Tracer("base-pkg/rabbitmq").Start(traced, "consume "+r.config.QueueName, trace.WithSpanKind(trace.SpanKindConsumer))
	defer span.End()
	// 后续重试、失败归档和 TG 通知都保留同一个统一 ID，以当前消费 Span 作为下一次接收的父级。
	d.Headers = messageTraceHeaders(ctx, d.Headers)
	// 外部生产者未设置 ID 时在第一次处理生成，后续重试和通知沿用该 ID。
	if d.MessageId == "" {
		id, err := commonTool.NewUUidV7()
		if err != nil {
			return &operationError{"generate message ID", err}
		}
		d.MessageId = id
	}
	count, err := retryCount(d.Headers[retryCountHeader])
	if err != nil {
		return r.archive(ctx, s, d, err, r.config.AutoAck)
	}
	handlerErr := invokeHandler(ctx, handler, d.Body)
	if handlerErr == nil {
		return r.ack(s, d, r.config.AutoAck)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	message := publishingFromDelivery(d)
	message.Headers[errorHeader] = errorSummary(handlerErr)
	if count >= r.config.maxRetries {
		return r.archive(ctx, s, d, handlerErr, r.config.AutoAck)
	}
	message.Headers[retryCountHeader] = int32(count + 1)
	message.Headers[retryAtHeader] = time.Now().Add(r.config.RetryDelay).UnixMilli()
	if err := r.publishOn(ctx, s, "", r.config.RetryQueue, message); err != nil {
		return err
	}
	return r.ack(s, d, r.config.AutoAck)
}

func invokeHandler(ctx context.Context, handler Handler, body []byte) (err error) {
	defer func() {
		if value := recover(); value != nil {
			err = fmt.Errorf("message handler panicked: %v", value)
		}
	}()
	return handler(ctx, body)
}

func (r *RabbitMQ) archive(ctx context.Context, s *session, d amqp.Delivery, cause error, autoAck bool) error {
	message := publishingFromDelivery(d)
	message.Headers[errorHeader] = errorSummary(cause)
	delete(message.Headers, retryAtHeader)
	if err := r.publishOn(ctx, s, r.config.failedExchange, "", message); err != nil {
		return err
	}
	return r.ack(s, d, autoAck)
}

// processRetry 在独立消费者中等待重试时间，确认重投到原业务队列后再确认重试消息。
// 使用默认交换机定向返回业务队列，避免再次向原交换机的其他订阅者广播。
func (r *RabbitMQ) processRetry(ctx context.Context, s *session, d amqp.Delivery) error {
	traced, err := messageTraceContext(ctx, d.Headers)
	if err != nil {
		return r.archive(ctx, s, d, err, false)
	}
	ctx, span := otel.Tracer("base-pkg/rabbitmq").Start(traced, "retry "+r.config.QueueName, trace.WithSpanKind(trace.SpanKindConsumer))
	defer span.End()
	d.Headers = messageTraceHeaders(ctx, d.Headers)
	due, ok := d.Headers[retryAtHeader].(int64)
	if !ok {
		return r.archive(ctx, s, d, errors.New("invalid retry schedule"), false)
	}
	if delay := time.Until(time.UnixMilli(due)); delay > 0 && !wait(ctx, delay) {
		return ctx.Err()
	}
	message := publishingFromDelivery(d)
	delete(message.Headers, retryAtHeader)
	if err := r.publishOn(ctx, s, "", r.config.QueueName, message); err != nil {
		return err
	}
	return r.ack(s, d, false)
}

func (r *RabbitMQ) ack(s *session, d amqp.Delivery, autoAck bool) error {
	if autoAck {
		return nil
	}
	if err := d.Ack(false); err != nil {
		r.invalidate(s)
		return &operationError{"acknowledge", err}
	}
	return nil
}

func publishingFromDelivery(d amqp.Delivery) amqp.Publishing {
	headers := make(amqp.Table, len(d.Headers)+2)
	for key, value := range d.Headers {
		headers[key] = value
	}
	// 内部转存强制持久化，保留正文和消息 ID；不沿用原 TTL 及与原生产连接身份绑定的 UserId。
	return amqp.Publishing{
		Headers: headers, ContentType: d.ContentType, ContentEncoding: d.ContentEncoding, DeliveryMode: amqp.Persistent,
		Priority: d.Priority, CorrelationId: d.CorrelationId, ReplyTo: d.ReplyTo, MessageId: d.MessageId,
		Timestamp: d.Timestamp, Type: d.Type, AppId: d.AppId, Body: d.Body,
	}
}

// errorSummary 限制错误头为 1024 字节，避免超长业务错误使 AMQP 消息头无法发送。
func errorSummary(err error) string {
	text := err.Error()
	if len(text) <= 1024 {
		return text
	}
	return text[:previousRuneBoundary(text, 1024)]
}

func previousRuneBoundary(text string, limit int) int {
	for limit > 0 && text[limit]&0xc0 == 0x80 {
		limit--
	}
	return limit
}

func retryCount(value any) (int, error) {
	var count int64
	switch value := value.(type) {
	case nil:
		return 0, nil
	case int:
		count = int64(value)
	case int32:
		count = int64(value)
	case int64:
		count = value
	case int16:
		count = int64(value)
	case byte:
		count = int64(value)
	case string:
		parsed, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			return 0, errors.New("invalid retry count")
		}
		count = parsed
	default:
		return 0, errors.New("invalid retry count")
	}
	if count < 0 || count > math.MaxInt32 {
		return 0, errors.New("invalid retry count")
	}
	return int(count), nil
}
