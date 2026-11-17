package v2

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/zeromicro/go-zero/core/logc"
)

type partitionWorker struct {
	partition      int
	ch             chan kafka.Message
	commitMessages func(context.Context, ...kafka.Message) error
	maxRetry       int
	batchSize      int
	flushInterval  time.Duration
	batchHandler   BatchMessageHandler
	singleHandler  MessageHandler
	dlqHandler     DLQHandler

	batch     []kafka.Message
	processed int   // 批次中已处理、尚未确认提交的连续前缀。
	fallback  bool  // 批量处理已结束，剩余消息按顺序逐条处理。
	dlqReason error // 首条未处理消息已耗尽业务重试，后续只重试死信投递。
}

func newPartitionWorker(
	partition int,
	commitMessages func(context.Context, ...kafka.Message) error,
	maxRetry int,
	buffer int,
	batchSize int,
	flushInterval time.Duration,
	batchHandler BatchMessageHandler,
	singleHandler MessageHandler,
	dlqHandler DLQHandler,
) *partitionWorker {
	return &partitionWorker{
		partition:      partition,
		ch:             make(chan kafka.Message, buffer),
		commitMessages: commitMessages,
		maxRetry:       maxRetry,
		batchSize:      batchSize,
		flushInterval:  flushInterval,
		batchHandler:   batchHandler,
		singleHandler:  singleHandler,
		dlqHandler:     dlqHandler,
		batch:          make([]kafka.Message, 0, batchSize),
	}
}

func (w *partitionWorker) run(ctx context.Context, draining <-chan struct{}) {
	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()

	logc.Infof(ctx, "partition worker started, partition=%d", w.partition)
	input := (<-chan kafka.Message)(w.ch)
	blocked := false
	for ctx.Err() == nil && (input != nil || blocked) {
		messages := input
		var retryOnDrain <-chan struct{}
		if blocked {
			// 保留当前批次的边界，避免故障期间累积消息或将新消息混入逐条回退。
			messages = nil
			retryOnDrain = draining
		}
		select {
		case <-ctx.Done():
			return
		case <-retryOnDrain:
			draining = nil
			blocked = w.flushWithLog(ctx) != nil
		case <-ticker.C:
			blocked = w.flushWithLog(ctx) != nil
		case msg, ok := <-messages:
			if !ok {
				input = nil
				blocked = w.flushWithLog(ctx) != nil
				continue
			}
			w.batch = append(w.batch, msg)
			if len(w.batch) >= w.batchSize {
				blocked = w.flushWithLog(ctx) != nil
			}
		}
	}
}

func (w *partitionWorker) flushWithLog(ctx context.Context) error {
	err := w.flush(ctx)
	if err != nil {
		logc.Errorf(ctx, "Kafka batch is pending, partition=%d err=%v", w.partition, err)
	}
	return err
}

func (w *partitionWorker) flush(ctx context.Context) error {
	// 恢复时优先提交上次已处理的前缀，避免重复执行业务或写入死信。
	if err := w.commitProcessed(ctx); err != nil || len(w.batch) == 0 {
		return err
	}

	processingErr := w.processBatch(ctx)
	commitErr := w.commitProcessed(ctx)
	return errors.Join(processingErr, commitErr)
}

func (w *partitionWorker) processBatch(ctx context.Context) error {
	if !w.fallback && w.batchHandler != nil {
		batch := append([]kafka.Message(nil), w.batch...)
		err := w.retryHandler(ctx, func() error { return w.batchHandler(ctx, batch) })
		if err == nil {
			w.processed = len(w.batch)
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		logc.Errorf(ctx, "Kafka batch handler failed, falling back to single messages, partition=%d err=%v", w.partition, err)
	}
	w.fallback = true
	for w.processed < len(w.batch) {
		msg := w.batch[w.processed]
		if err := w.processSingle(ctx, msg); err != nil {
			return fmt.Errorf("Kafka message at offset %d is pending: %w", msg.Offset, err)
		}
		w.processed++
	}
	return nil
}

func (w *partitionWorker) processSingle(ctx context.Context, msg kafka.Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if w.dlqReason == nil {
		if w.singleHandler == nil {
			w.dlqReason = errors.New("Kafka batch failed and no single handler is configured")
		} else {
			err := w.retryHandler(ctx, func() error { return w.singleHandler(ctx, msg) })
			if err == nil {
				return nil
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			w.dlqReason = err
		}
	}

	if err := invokeHandler(ctx, func() error { return w.dlqHandler(ctx, msg, w.dlqReason) }); err != nil {
		return err
	}
	w.dlqReason = nil
	return nil
}

func (w *partitionWorker) commitProcessed(ctx context.Context) error {
	if w.processed == 0 {
		return nil
	}
	last := w.batch[w.processed-1]
	if err := w.commitLast(ctx, last, w.processed); err != nil {
		return fmt.Errorf("commit Kafka offset %d: %w", last.Offset, err)
	}

	// 只有 broker 确认后才移除已处理的前缀，未处理后缀和死信重试状态继续保留。
	remaining := copy(w.batch, w.batch[w.processed:])
	clear(w.batch[remaining:])
	w.batch = w.batch[:remaining]
	w.processed = 0
	if remaining == 0 {
		w.fallback = false
	}
	return nil
}

func (w *partitionWorker) commitLast(ctx context.Context, msg kafka.Message, batchCount int) error {
	var err error
	for i := 0; i < 3; i++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err = w.commitMessages(ctx, msg)
		if err == nil {
			logc.Debugf(ctx, "commit success, partition=%d offset=%d batch_count=%d", msg.Partition, msg.Offset, batchCount)
			return nil
		}
		if i < 2 && !sleepWithContext(ctx, time.Duration(i+1)*200*time.Millisecond) {
			return ctx.Err()
		}
	}
	return err
}

func (w *partitionWorker) retryHandler(ctx context.Context, handler func() error) error {
	var err error
	for i := 1; i <= w.maxRetry; i++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err = invokeHandler(ctx, handler)
		if err == nil {
			return nil
		}
		if i < w.maxRetry && !sleepWithContext(ctx, retryBackoff(i)) {
			return ctx.Err()
		}
	}
	return err
}

func invokeHandler(ctx context.Context, handler func() error) (err error) {
	defer func() {
		if recover() != nil {
			err = errors.New("Kafka handler panicked")
			logc.Errorf(ctx, "%s\n%s", err, debug.Stack())
		}
	}()
	return handler()
}

func retryBackoff(n int) time.Duration {
	d := time.Duration(n*n) * 200 * time.Millisecond
	if d > 10*time.Second {
		d = 10 * time.Second
	}
	return d
}

func sleepWithContext(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
