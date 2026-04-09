package kafka

import (
	"context"
	"strings"

	kafkago "github.com/segmentio/kafka-go"
)

// EventPublisher defines the interface for publishing events to Kafka topics.
type EventPublisher interface {
	Publish(ctx context.Context, topic string, key string, value []byte) error
	Close() error
}

type kafkaProducer struct {
	writer *kafkago.Writer
}

// NewProducer creates a new Kafka producer connecting to the given comma-separated brokers.
func NewProducer(brokers string) EventPublisher {
	return &kafkaProducer{
		writer: &kafkago.Writer{
			Addr:         kafkago.TCP(strings.Split(brokers, ",")...),
			Balancer:     &kafkago.LeastBytes{},
			RequiredAcks: kafkago.RequireOne,
			Async:        false,
		},
	}
}

// Publish sends a message with the given key and value to the specified topic.
func (p *kafkaProducer) Publish(ctx context.Context, topic, key string, value []byte) error {
	return p.writer.WriteMessages(ctx, kafkago.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: value,
	})
}

// Close closes the underlying Kafka writer.
func (p *kafkaProducer) Close() error {
	return p.writer.Close()
}
