package v2

import (
	"github.com/segmentio/kafka-go"
	"github.com/zeromicro/go-zero/core/logc"
)

import (
	"context"
)

func defaultDLQHandler(ctx context.Context, msg kafka.Message, reason error) error {
	// 这里可以换成真正的 DLQ producer
	logc.Errorf(
		ctx,
		"[DLQ] topic=%s partition=%d offset=%d err=%v value=%s",
		msg.Topic,
		msg.Partition,
		msg.Offset,
		reason,
		string(msg.Value),
	)
	return nil
}
