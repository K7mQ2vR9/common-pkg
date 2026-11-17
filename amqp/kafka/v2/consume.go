package v2

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/K7mQ2vR9/base-pkg/amqp/kafka/config"
	"github.com/segmentio/kafka-go"
	"github.com/zeromicro/go-zero/core/logc"
)

type MessageHandler func(ctx context.Context, msg kafka.Message) error
type BatchMessageHandler func(ctx context.Context, msgs []kafka.Message) error
type DLQHandler func(ctx context.Context, msg kafka.Message, reason error) error

type consumerReader interface {
	Config() kafka.ReaderConfig
	FetchMessage(context.Context) (kafka.Message, error)
	CommitMessages(context.Context, ...kafka.Message) error
	Close() error
}

type Consumer struct {
	reader          consumerReader
	workers         map[int]*partitionWorker
	workersDone     sync.WaitGroup
	maxRetry        int
	workerBuffer    int
	batchSize       int
	flushInterval   time.Duration
	shutdownTimeout time.Duration
	dlqHandler      DLQHandler
	dlqTopic        string
	dlqWriter       *kafka.Writer

	mu        sync.Mutex
	stopFetch context.CancelFunc
	closed    bool
	draining  chan struct{}
	done      chan struct{}
}

func NewConsumer(conf config.CustomerConfig, opts ...Option) *Consumer {
	kafkaConf := kafka.ReaderConfig{
		Brokers:     conf.Brokers,
		GroupID:     conf.GroupID,
		Topic:       conf.Topic,
		StartOffset: conf.StartOffset,
	}
	c := &Consumer{
		reader:          kafka.NewReader(kafkaConf),
		workers:         make(map[int]*partitionWorker),
		maxRetry:        15,
		workerBuffer:    2000,
		batchSize:       1000,
		flushInterval:   3 * time.Second,
		shutdownTimeout: 10 * time.Second,
		draining:        make(chan struct{}),
		done:            make(chan struct{}),
	}

	for _, opt := range opts {
		opt(c)
	}
	if c.dlqHandler == nil && c.dlqTopic != "" {
		c.dlqWriter = &kafka.Writer{
			Addr:         kafka.TCP(conf.Brokers...),
			Topic:        c.dlqTopic,
			RequiredAcks: kafka.RequireAll,
			MaxAttempts:  3,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
		}
		c.dlqHandler = newKafkaDLQHandler(c.dlqTopic, c.dlqWriter.WriteMessages)
	}
	if c.dlqHandler == nil {
		c.dlqHandler = defaultDLQHandler
	}

	return c
}

func (c *Consumer) ConsumeMessagesWithContext(ctx context.Context, handler MessageHandler) error {
	return c.ConsumeBatchesWithContext(ctx, nil, handler)
}

// ConsumeBatchesWithContext 优先批量处理，失败后逐条处理。批量处理器可能重试，应支持幂等。
// singleHandler 为空时，批量失败后的消息逐条进入死信处理。
// 调用方取消时停止拉取，worker 在收尾时限内处理已接收的消息。回调需响应传入的上下文。
func (c *Consumer) ConsumeBatchesWithContext(
	ctx context.Context,
	batchHandler BatchMessageHandler,
	singleHandler MessageHandler,
) (err error) {
	c.mu.Lock()
	if c.closed || c.stopFetch != nil {
		c.mu.Unlock()
		return errors.New("Kafka consumer is already running or closed")
	}
	fetchCtx, cancelFetch := context.WithCancel(ctx)
	c.stopFetch = cancelFetch
	c.mu.Unlock()

	workerCtx, cancelWorkers := context.WithCancel(context.WithoutCancel(ctx))
	defer func() {
		cancelFetch()
		err = errors.Join(err, c.finish(cancelWorkers))
	}()

	if batchHandler == nil && singleHandler == nil {
		return errors.New("at least one Kafka message handler is required")
	}
	readerConfig := c.reader.Config()
	if readerConfig.GroupID == "" {
		return errors.New("Kafka consumer group ID is required to commit offsets")
	}
	if c.dlqWriter != nil && c.dlqTopic == readerConfig.Topic {
		return errors.New("Kafka DLQ topic must differ from the source topic")
	}

	for fetchCtx.Err() == nil {
		msg, err := c.reader.FetchMessage(fetchCtx)
		if err != nil {
			if errors.Is(err, io.EOF) || fetchCtx.Err() != nil {
				break
			}
			logc.Errorf(ctx, "fetch kafka message error: %v", err)
			if !sleepWithContext(fetchCtx, time.Second) {
				break
			}
			continue
		}

		if fetchCtx.Err() != nil {
			break
		}
		worker := c.getWorker(workerCtx, msg.Partition, batchHandler, singleHandler)

		select {
		case worker.ch <- msg:
		case <-fetchCtx.Done():
		}
	}
	if errors.Is(fetchCtx.Err(), context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	return nil
}

func (c *Consumer) getWorker(ctx context.Context, partition int, batchHandler BatchMessageHandler, singleHandler MessageHandler) *partitionWorker {
	if w, ok := c.workers[partition]; ok {
		return w
	}

	w := newPartitionWorker(
		partition,
		c.reader.CommitMessages,
		c.maxRetry,
		c.workerBuffer,
		c.batchSize,
		c.flushInterval,
		batchHandler,
		singleHandler,
		c.dlqHandler,
	)

	c.workers[partition] = w

	c.workersDone.Add(1)
	go func() {
		defer c.workersDone.Done()
		w.run(ctx, c.draining)
	}()

	return w
}

// Close 请求停止拉取并等待收尾，由 handler 之外的调用方调用。
func (c *Consumer) Close() {
	c.mu.Lock()
	if c.closed || c.stopFetch != nil {
		stopFetch := c.stopFetch
		c.mu.Unlock()
		if stopFetch != nil {
			stopFetch()
		}
		<-c.done
		return
	}
	c.closed = true
	c.mu.Unlock()

	if err := c.finish(nil); err != nil {
		logc.Errorf(context.Background(), "close Kafka consumer: %v", err)
	}
}

func (c *Consumer) finish(cancelWorkers context.CancelFunc) error {
	defer close(c.done)
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()

	var closeErr error
	if cancelWorkers != nil {
		defer cancelWorkers()
		timer := time.AfterFunc(c.shutdownTimeout, cancelWorkers)
		// 只有消费循环退出后才关闭投递队列，worker 可在 Reader 和 DLQ Writer 仍打开时排空缓冲。
		close(c.draining)
		for _, worker := range c.workers {
			close(worker.ch)
		}
		c.workersDone.Wait()
		if !timer.Stop() {
			closeErr = fmt.Errorf("drain Kafka consumer: %w", context.DeadlineExceeded)
		}
	}
	if err := c.reader.Close(); err != nil {
		closeErr = errors.Join(closeErr, fmt.Errorf("close Kafka reader: %w", err))
	}
	if c.dlqWriter != nil {
		if err := c.dlqWriter.Close(); err != nil {
			closeErr = errors.Join(closeErr, fmt.Errorf("close Kafka DLQ writer: %w", err))
		}
	}
	return closeErr
}
