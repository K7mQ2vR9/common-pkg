package v2

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/segmentio/kafka-go"
)

var errDLQNotConfigured = errors.New("Kafka DLQ is not configured")

func defaultDLQHandler(context.Context, kafka.Message, error) error {
	return errDLQNotConfigured
}

func newKafkaDLQHandler(topic string, publish func(context.Context, ...kafka.Message) error) DLQHandler {
	return func(ctx context.Context, msg kafka.Message, _ error) error {
		if topic == msg.Topic {
			return errors.New("Kafka DLQ topic must differ from the source topic")
		}

		headers := append([]kafka.Header(nil), msg.Headers...)
		headers = append(headers,
			kafka.Header{Key: "dlq-source-topic", Value: []byte(msg.Topic)},
			kafka.Header{Key: "dlq-source-partition", Value: []byte(strconv.Itoa(msg.Partition))},
			kafka.Header{Key: "dlq-source-offset", Value: []byte(strconv.FormatInt(msg.Offset, 10))},
		)
		// 目标主题由 Writer 设置，避免原消息的主题覆盖死信主题。
		if err := publish(ctx, kafka.Message{Key: msg.Key, Value: msg.Value, Headers: headers, Time: msg.Time}); err != nil {
			return fmt.Errorf("publish Kafka message to DLQ: %w", err)
		}
		return nil
	}
}
