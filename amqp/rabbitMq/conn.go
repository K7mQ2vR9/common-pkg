package rabbitmq

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/streadway/amqp"
	"github.com/zeromicro/go-zero/core/logc"
)

var (
	// ErrClosed 表示客户端已关闭，不再接受发布或订阅。
	ErrClosed = errors.New("rabbitmq client is closed")
	// ErrNotReady 表示连接正在恢复，调用方可稍后重试。
	ErrNotReady = errors.New("rabbitmq connection is not ready")
	// ErrPublishUnknown 表示无法确定 broker 是否已接收，重发须沿用 Message.ID，消费须按业务标识幂等。
	ErrPublishUnknown = errors.New("rabbitmq publish result is unknown")
	// ErrUnroutable 表示 mandatory 发布未匹配任何队列。
	ErrUnroutable = errors.New("rabbitmq message was not routed")
	// ErrPublishRejected 表示 broker 明确拒绝此次发布。
	ErrPublishRejected = errors.New("rabbitmq broker rejected the publish")
)

// Handler 处理独立的业务消息快照，ACK/NACK 由客户端管理。
// ctx 带有 HandlerTimeout 截止时间，连接失效或关闭时也会取消。
// 生命周期仍有效且限时内返回 nil 才表示成功；超过时限即使返回 nil 也按超时失败处理。
// 生命周期取消后不确认消息，业务 I/O 须使用传入的 ctx。
// 回调返回后 ctx 随即取消并释放，不用于回调之外的后台任务。
// 普通错误默认重试，Permanent 直接归档；标准 JSON 格式/类型/目标错误默认归档，可用 Retryable 覆盖。
type Handler func(ctx context.Context, message Message) error

type subscription struct {
	handler Handler
	ready   chan error
	started bool
}

type session struct {
	conn             brokerConnection
	transport        io.Closer
	publisher        brokerChannel
	connectionClosed <-chan *amqp.Error
	publisherClosed  <-chan *amqp.Error
	confirms         <-chan amqp.Confirmation
	returns          <-chan amqp.Return
	publishLock      chan struct{}
	nextTag          uint64
	done             chan struct{}
	once             sync.Once
}

func (s *session) close() {
	s.once.Do(func() { close(s.done); _ = s.transport.Close() })
}

// RabbitMQ 管理连接、发布确认和订阅恢复，队列参数在初始化后只读。
type RabbitMQ struct {
	config     settings
	dial       dialFunc
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.RWMutex
	session    *session
	sub        *subscription
	closed     bool
	changed    chan struct{}
	wg         sync.WaitGroup
	closeOnce  sync.Once
	closeErr   error
	httpClient *http.Client
}

// NewRabbitMQ 在 ctx 时限内初始化，成功后由 Close 管理客户端生命周期。
func NewRabbitMQ(ctx context.Context, config Config) (*RabbitMQ, error) {
	if ctx == nil {
		return nil, errors.New("rabbitmq startup context is required")
	}
	settings, err := config.settings()
	if err != nil {
		return nil, err
	}
	return newRabbitMQ(ctx, settings, amqpDialer(settings))
}

func newRabbitMQ(ctx context.Context, config settings, dial dialFunc) (*RabbitMQ, error) {
	lifetime, cancel := context.WithCancel(context.Background())
	r := &RabbitMQ{config: config, dial: dial, ctx: lifetime, cancel: cancel, changed: make(chan struct{}, 1),
		httpClient: &http.Client{Timeout: config.Telegram.Timeout}}
	s, err := r.openSession(ctx)
	if err != nil {
		cancel()
		return nil, err
	}
	r.session = s
	r.wg.Add(1)
	go r.run(s)
	return r, nil
}

func (r *RabbitMQ) openSession(parent context.Context) (result *session, err error) {
	ctx, cancel := context.WithTimeout(parent, r.config.DialTimeout)
	defer cancel()
	conn, transport, err := r.dial(ctx)
	if err != nil {
		if transport != nil {
			_ = transport.Close()
		}
		return nil, err
	}
	s := &session{conn: conn, transport: transport, publishLock: make(chan struct{}, 1), nextTag: 1, done: make(chan struct{})}
	stop := context.AfterFunc(ctx, s.close)
	defer stop()
	defer func() {
		if err != nil {
			s.close()
		}
	}()
	s.connectionClosed = conn.NotifyClose(make(chan *amqp.Error, 1))
	s.publisher, err = conn.Channel()
	if err != nil {
		return nil, &operationError{"open publisher channel", err}
	}
	s.publisherClosed = s.publisher.NotifyClose(make(chan *amqp.Error, 1))
	if err = r.declare(s.publisher); err != nil {
		return nil, err
	}
	if err = s.publisher.Confirm(false); err != nil {
		return nil, &operationError{"enable publisher confirms", err}
	}
	s.confirms = s.publisher.NotifyPublish(make(chan amqp.Confirmation, 1))
	s.returns = s.publisher.NotifyReturn(make(chan amqp.Return, 1))
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return s, nil
}

func (r *RabbitMQ) declare(ch brokerChannel) error {
	c := r.config
	if c.Exchange != "" {
		if err := ch.ExchangeDeclare(c.Exchange, c.ExchangeType, c.exchangeDurable, false, false, false, nil); err != nil {
			return &operationError{"declare exchange", err}
		}
	}
	if _, err := ch.QueueDeclare(c.QueueName, c.queueDurable, c.QueueAutoDelete, c.QueueExclusive, false, nil); err != nil {
		return &operationError{"declare business queue", err}
	}
	if c.Exchange != "" {
		if err := ch.QueueBind(c.QueueName, c.RoutingKey, c.Exchange, false, nil); err != nil {
			return &operationError{"bind business queue", err}
		}
	}
	// 辅助队列通过默认交换机按队列名定向投递，失败归档与通知分别等待发布确认。
	for _, name := range []string{c.RetryQueue, c.FailedQueue, c.NotifyQueue} {
		if _, err := ch.QueueDeclare(name, true, false, false, false, nil); err != nil {
			return &operationError{"declare auxiliary queue", err}
		}
	}
	return nil
}

func (r *RabbitMQ) invalidate(s *session) {
	r.mu.Lock()
	if r.session == s {
		r.session = nil
	}
	r.mu.Unlock()
	s.close()
}

func (r *RabbitMQ) run(s *session) {
	defer r.wg.Done()
	for r.ctx.Err() == nil {
		if s == nil {
			var err error
			s, err = r.openSession(r.ctx)
			if err != nil {
				logc.Errorf(r.ctx, "RabbitMQ reconnect failed: %v", err)
				if !wait(r.ctx, r.config.ReconnectInterval) {
					return
				}
				continue
			}
			r.mu.Lock()
			if r.closed {
				r.mu.Unlock()
				s.close()
				return
			}
			r.session = s
			r.mu.Unlock()
		}
		err := r.serveSession(s)
		if r.ctx.Err() != nil {
			return
		}
		if err != nil {
			logc.Errorf(r.ctx, "RabbitMQ session interrupted: %v", err)
		}
		r.invalidate(s)
		s = nil
		if !wait(r.ctx, r.config.ReconnectInterval) {
			return
		}
	}
}

func (r *RabbitMQ) serveSession(s *session) error {
	ctx, cancel := context.WithCancel(r.ctx)
	defer cancel()
	failures := make(chan error, 1)
	subscribed := false
	for {
		r.mu.RLock()
		sub := r.sub
		r.mu.RUnlock()
		if sub != nil && !subscribed {
			err := r.startConsumers(ctx, s, sub, failures)
			if !sub.started {
				if err != nil {
					r.mu.Lock()
					if r.sub == sub {
						r.sub = nil
					}
					r.mu.Unlock()
				} else {
					sub.started = true
				}
				sub.ready <- err
			}
			if err != nil {
				return err
			}
			subscribed = true
		}
		select {
		case <-r.ctx.Done():
			return nil
		case <-s.done:
			return ErrNotReady
		case <-s.connectionClosed:
			return ErrNotReady
		case <-s.publisherClosed:
			return ErrNotReady
		case err := <-failures:
			return err
		case <-r.changed:
		}
	}
}

// Close 停止重连和消费，等待处理协程收尾后关闭连接，可重复或并发调用。
// 回调应及时响应 ctx 取消，超过 ShutdownTimeout 时关闭连接并返回超时错误。
func (r *RabbitMQ) Close() error {
	r.closeOnce.Do(func() {
		r.mu.Lock()
		r.closed = true
		r.mu.Unlock()
		r.cancel()
		finished := make(chan struct{})
		go func() { r.wg.Wait(); close(finished) }()
		timer := time.NewTimer(r.config.ShutdownTimeout)
		defer timer.Stop()
		select {
		case <-finished:
		case <-timer.C:
			r.closeErr = context.DeadlineExceeded
		}
		r.mu.Lock()
		s := r.session
		r.session = nil
		r.mu.Unlock()
		if s != nil {
			s.close()
		}
	})
	return r.closeErr
}

func wait(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
