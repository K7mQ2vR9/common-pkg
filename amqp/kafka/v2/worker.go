package v2

import (
	"context"
	"errors"
	"github.com/segmentio/kafka-go"
	"github.com/zeromicro/go-zero/core/logc"
	"runtime/debug"
	"time"
)

type partitionWorker struct {
	partition     int
	ch            chan kafka.Message
	reader        *kafka.Reader
	maxRetry      int
	batchSize     int
	flushInterval time.Duration
	batchHandler  BatchMessageHandler
	singleHandler MessageHandler
	dlqHandler    DLQHandler

	batch []kafka.Message
}

func newPartitionWorker(
	partition int,
	reader *kafka.Reader,
	maxRetry int,
	buffer int,
	batchSize int,
	flushInterval time.Duration,
	batchHandler BatchMessageHandler,
	singleHandler MessageHandler,
	dlqHandler DLQHandler,
) *partitionWorker {
	return &partitionWorker{
		partition:     partition,
		ch:            make(chan kafka.Message, buffer),
		reader:        reader,
		maxRetry:      maxRetry,
		batchSize:     batchSize,
		flushInterval: flushInterval,
		batchHandler:  batchHandler,
		singleHandler: singleHandler,
		dlqHandler:    dlqHandler,
		batch:         make([]kafka.Message, 0, batchSize),
	}
}

func (w *partitionWorker) run(ctx context.Context) {
	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()

	logc.Infof(ctx, "partition worker started, partition=%d", w.partition)

	for {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := w.flush(shutdownCtx); err != nil {
				logc.Errorf(ctx, "flush on ctx done failed, partition=%d err=%v", w.partition, err)
			}
			cancel()
			logc.Infof(ctx, "partition worker stopped by context, partition=%d", w.partition)
			return

		case <-ticker.C:
			if err := w.flush(ctx); err != nil {
				logc.Errorf(ctx, "flush by ticker failed, partition=%d err=%v", w.partition, err)
			}

		case msg, ok := <-w.ch:
			if !ok {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				if err := w.flush(shutdownCtx); err != nil {
					logc.Errorf(ctx, "flush on channel close failed, partition=%d err=%v", w.partition, err)
				}
				cancel()
				logc.Infof(ctx, "partition worker channel closed, partition=%d", w.partition)
				return
			}

			w.batch = append(w.batch, msg)
			if len(w.batch) >= w.batchSize {
				if err := w.flush(ctx); err != nil {
					logc.Errorf(ctx, "flush by batch size failed, partition=%d err=%v", w.partition, err)
				}
			}
		}
	}
}

func (w *partitionWorker) flush(ctx context.Context) error {
	if len(w.batch) == 0 {
		return nil
	}

	msgs := make([]kafka.Message, len(w.batch))
	copy(msgs, w.batch)
	needCommit, err := w.handleBatch(ctx, msgs)
	if err != nil {
		return err
	}
	w.batch = w.batch[:0]

	if needCommit {
		return w.commitLast(ctx, msgs[len(msgs)-1], len(msgs))
	}
	return nil
}

func (w *partitionWorker) handleBatch(ctx context.Context, msgs []kafka.Message) (needCommit bool, err error) {

	if w.batchHandler == nil {
		return false, errors.New("batchHandler is nil")
	}

	defer func() {
		if r := recover(); r != nil {
			err = errors.New("panic in batch handler")
			logc.Errorf(ctx, "panic recovered in batch handler, partition=%d panic=%v stack=%s", w.partition, r, debug.Stack())
		}
	}()

	for i := 1; i <= w.maxRetry; i++ {
		err = w.batchHandler(ctx, msgs)
		if err == nil {
			return true, nil
		}

		backoff := retryBackoff(i)
		logc.Errorf(ctx,
			"batch handle failed, partition=%d retry=%d/%d batch_size=%d err=%v backoff=%s",
			w.partition, i, w.maxRetry, len(msgs), err, backoff,
		)
		if !sleepWithContext(ctx, backoff) {
			return false, ctx.Err()
		}
	}

	logc.Errorf(ctx, "batch handle failed finally, fallback to single, partition=%d batch_size=%d last_err=%v", w.partition, len(msgs), err)
	var failed int
	for _, msg := range msgs {
		if singleErr := w.handleSingle(ctx, msg); singleErr != nil {
			failed++
			logc.Errorf(ctx, "single handle failed after fallback, partition=%d offset=%d err=%v", msg.Partition, msg.Offset, singleErr)
		}
	}
	//if failed > 0 {
	//	return false, errors.New("fallback single handle partially failed")
	//}
	return false, nil
}

func (w *partitionWorker) handleSingle(ctx context.Context, msg kafka.Message) (err error) {
	defer func() {
		if r := recover(); r != nil {
			panicErr := errors.New("panic in single handler")
			logc.Errorf(ctx, "panic recovered in single handler, partition=%d offset=%d panic=%v stack=%s", msg.Partition, msg.Offset, r, debug.Stack())
			if dlqErr := w.dlqHandler(ctx, msg, panicErr); dlqErr != nil {
				err = dlqErr
				return
			}
			err = w.commitLast(ctx, msg, 1)
		}
	}()

	if w.singleHandler == nil {
		if dlqErr := w.dlqHandler(ctx, msg, errors.New("batch failed and single handler is nil")); dlqErr != nil {
			return dlqErr
		}
		return w.commitLast(ctx, msg, 1)
	}

	for i := 1; i <= w.maxRetry; i++ {
		err = w.singleHandler(ctx, msg)
		if err == nil {
			return w.commitLast(ctx, msg, 1)
		}

		backoff := retryBackoff(i)
		logc.Errorf(ctx, "single handle failed, partition=%d offset=%d retry=%d/%d err=%v backoff=%s", msg.Partition, msg.Offset, i, w.maxRetry, err, backoff)

		if !sleepWithContext(ctx, backoff) {
			return ctx.Err()
		}
	}

	if dlqErr := w.dlqHandler(ctx, msg, err); dlqErr != nil {
		return dlqErr
	}
	return w.commitLast(ctx, msg, 1)
}

func (w *partitionWorker) commitLast(ctx context.Context, msg kafka.Message, batchCount int) error {
	var err error
	for i := 0; i < 3; i++ {
		err = w.reader.CommitMessages(ctx, msg)
		if err == nil {
			logc.Debugf(ctx, "commit success, partition=%d offset=%d batch_count=%d", msg.Partition, msg.Offset, batchCount)
			return nil
		}
		if !sleepWithContext(ctx, time.Duration(i+1)*200*time.Millisecond) {
			return ctx.Err()
		}
	}
	return err
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
