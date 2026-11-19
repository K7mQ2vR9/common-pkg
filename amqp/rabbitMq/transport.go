package rabbitmq

import (
	"context"
	"io"
	"net"
	"time"

	"github.com/streadway/amqp"
)

// brokerChannel 使用驱动的原生方法，供连接恢复与订阅统一管理。
type brokerChannel interface {
	QueueDeclare(string, bool, bool, bool, bool, amqp.Table) (amqp.Queue, error)
	ExchangeDeclare(string, string, bool, bool, bool, bool, amqp.Table) error
	QueueBind(string, string, string, bool, amqp.Table) error
	Confirm(bool) error
	NotifyPublish(chan amqp.Confirmation) chan amqp.Confirmation
	NotifyReturn(chan amqp.Return) chan amqp.Return
	NotifyClose(chan *amqp.Error) chan *amqp.Error
	NotifyCancel(chan string) chan string
	Qos(int, int, bool) error
	Consume(string, string, bool, bool, bool, bool, amqp.Table) (<-chan amqp.Delivery, error)
	Publish(string, string, bool, bool, amqp.Publishing) error
}

type brokerConnection interface {
	Channel() (brokerChannel, error)
	NotifyClose(chan *amqp.Error) chan *amqp.Error
}

type amqpConnection struct{ *amqp.Connection }

func (c amqpConnection) Channel() (brokerChannel, error) { return c.Connection.Channel() }

type dialFunc func(context.Context) (brokerConnection, io.Closer, error)

// deadlineConn 为 AMQP 写操作设置时限，关闭底层连接可中断握手及阻塞的 RPC。
type deadlineConn struct {
	net.Conn
	timeout time.Duration
}

func (c *deadlineConn) Write(data []byte) (int, error) {
	if err := c.SetWriteDeadline(time.Now().Add(c.timeout)); err != nil {
		return 0, err
	}
	return c.Conn.Write(data)
}

func amqpDialer(config settings) dialFunc {
	return func(ctx context.Context) (brokerConnection, io.Closer, error) {
		var raw net.Conn
		var stop func() bool
		conn, err := amqp.DialConfig(config.URL, amqp.Config{
			Heartbeat: config.heartbeat,
			Dial: func(network, addr string) (net.Conn, error) {
				connection, err := (&net.Dialer{Timeout: config.DialTimeout}).DialContext(ctx, network, addr)
				if err != nil {
					return nil, err
				}
				raw = &deadlineConn{Conn: connection, timeout: config.ConfirmTimeout}
				stop = context.AfterFunc(ctx, func() { _ = raw.Close() })
				deadline, _ := ctx.Deadline()
				if err := raw.SetDeadline(deadline); err != nil {
					_ = raw.Close()
					return nil, err
				}
				return raw, nil
			},
		})
		if stop != nil {
			stop()
		}
		if err != nil {
			if raw != nil {
				_ = raw.Close()
			}
			return nil, nil, &operationError{"connect", err}
		}
		return amqpConnection{conn}, raw, nil
	}
}
