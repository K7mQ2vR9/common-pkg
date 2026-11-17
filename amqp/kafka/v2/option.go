package v2

import (
	"strings"
	"time"
)

type Option func(*Consumer)

func WithMaxRetry(n int) Option {
	return func(c *Consumer) {
		if n > 0 {
			c.maxRetry = n
		}
	}
}

func WithWorkerBuffer(n int) Option {
	return func(c *Consumer) {
		if n > 0 {
			c.workerBuffer = n
		}
	}
}

func WithBatchSize(n int) Option {
	return func(c *Consumer) {
		if n > 0 {
			c.batchSize = n
		}
	}
}

func WithFlushInterval(d time.Duration) Option {
	return func(c *Consumer) {
		if d > 0 {
			c.flushInterval = d
		}
	}
}

// WithShutdownTimeout 设置停止拉取后的缓冲处理时限，默认 10 秒。
func WithShutdownTimeout(d time.Duration) Option {
	return func(c *Consumer) {
		if d > 0 {
			c.shutdownTimeout = d
		}
	}
}

// WithDLQTopic 将失败消息同步投递到同一 Kafka 集群的独立主题。
func WithDLQTopic(topic string) Option {
	return func(c *Consumer) {
		c.dlqTopic = strings.TrimSpace(topic)
	}
}

// WithDLQHandler 优先于内置死信生产者。消息持久化成功后才应返回 nil。
func WithDLQHandler(h DLQHandler) Option {
	return func(c *Consumer) {
		if h != nil {
			c.dlqHandler = h
		}
	}
}
