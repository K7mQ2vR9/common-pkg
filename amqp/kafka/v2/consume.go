package v2

import (
	"context"
	"errors"
	"github.com/doubunv/common-pkg/amqp/kafka/config"
	"github.com/segmentio/kafka-go"
	"github.com/zeromicro/go-zero/core/logc"
	"sync"
	"time"
)

type MessageHandler func(ctx context.Context, msg kafka.Message) error
type BatchMessageHandler func(ctx context.Context, msgs []kafka.Message) error
type DLQHandler func(ctx context.Context, msg kafka.Message, reason error) error

type Consumer struct {
	reader       *kafka.Reader
	workers      map[int]*partitionWorker
	mu           sync.Mutex
	wg           sync.WaitGroup
	maxRetry     int
	workerBuffer int
	batchSize    int
	//kafkaConf     kafka.ReaderConfig
	flushInterval time.Duration
	dlqHandler    DLQHandler
	closed        bool
}

func NewConsumer(conf config.CustomerConfig, opts ...Option) *Consumer {
	kafkaConf := kafka.ReaderConfig{
		Brokers:     conf.Brokers,
		GroupID:     conf.GroupID,
		Topic:       conf.Topic,
		StartOffset: conf.StartOffset,
	}
	c := &Consumer{
		reader:        kafka.NewReader(kafkaConf),
		workers:       make(map[int]*partitionWorker),
		maxRetry:      15,
		workerBuffer:  2000,
		batchSize:     1000,
		flushInterval: 3 * time.Second,
		dlqHandler:    defaultDLQHandler,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *Consumer) ConsumeMessagesWithContext(ctx context.Context, handler MessageHandler) error {
	// 兼容老接口：包装成 batch 处理
	return c.ConsumeBatchesWithContext(ctx, func(ctx context.Context, msgs []kafka.Message) error {
		for _, msg := range msgs {
			if err := handler(ctx, msg); err != nil {
				return err
			}
		}
		return nil
	}, handler)
}

// batchHandler: 优先使用的批量处理器
// singleHandler: 批量失败后的降级逐条处理器；可以传 nil，nil 时失败整批进 DLQ
func (c *Consumer) ConsumeBatchesWithContext(
	ctx context.Context,
	batchHandler BatchMessageHandler,
	singleHandler MessageHandler,
) error {
	defer c.Close()

	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				logc.Info(ctx, "kafka consumer stopped by context cancel")
				return nil
			}
			logc.Errorf(ctx, "fetch kafka message error: %v", err)
			time.Sleep(time.Second)
			continue
		}

		worker := c.getWorker(ctx, msg.Partition, batchHandler, singleHandler)

		select {
		case worker.ch <- msg:
		case <-ctx.Done():
			return nil
		}
	}
}

func (c *Consumer) getWorker(ctx context.Context, partition int, batchHandler BatchMessageHandler, singleHandler MessageHandler) *partitionWorker {
	c.mu.Lock()
	defer c.mu.Unlock()

	if w, ok := c.workers[partition]; ok {
		return w
	}

	w := newPartitionWorker(
		partition,
		c.reader,
		c.maxRetry,
		c.workerBuffer,
		c.batchSize,
		c.flushInterval,
		batchHandler,
		singleHandler,
		c.dlqHandler,
	)

	c.workers[partition] = w

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		w.run(ctx)
	}()

	return w
}

func (c *Consumer) Close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	for _, w := range c.workers {
		close(w.ch)
	}
	c.mu.Unlock()

	c.wg.Wait()

	if err := c.reader.Close(); err != nil {
	}
}
