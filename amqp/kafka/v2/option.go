package v2

import "time"

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

func WithDLQHandler(h DLQHandler) Option {
	return func(c *Consumer) {
		if h != nil {
			c.dlqHandler = h
		}
	}
}
