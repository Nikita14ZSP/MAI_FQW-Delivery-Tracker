package kafka

import (
	"context"
	"log/slog"
	"strings"
	"time"

	kafkago "github.com/segmentio/kafka-go"
)

// NewConsumer creates a new Kafka reader (consumer) for the given topic and consumer group.
func NewConsumer(brokers, topic, groupID string) *kafkago.Reader {
	return kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:        strings.Split(brokers, ","),
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       1,
		MaxBytes:       10e6,
		CommitInterval: time.Second,
		StartOffset:    kafkago.FirstOffset,
	})
}

// RunConsumer runs a blocking loop that fetches messages and calls the handler.
// On handler success the message is committed. On context cancellation the loop exits.
func RunConsumer(ctx context.Context, reader *kafkago.Reader, handler func(ctx context.Context, msg kafkago.Message) error) {
	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("kafka fetch error", "err", err)
			continue
		}
		if err := handler(ctx, msg); err != nil {
			slog.Error("kafka handler error", "err", err, "topic", msg.Topic, "offset", msg.Offset)
		} else {
			if err := reader.CommitMessages(ctx, msg); err != nil {
				slog.Error("kafka commit error", "err", err)
			}
		}
	}
}
