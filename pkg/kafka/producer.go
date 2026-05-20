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
			Addr:     kafkago.TCP(strings.Split(brokers, ",")...),
			Balancer: &kafkago.LeastBytes{},
			// RequireAll: leader waits for all in-sync replicas to acknowledge before returning success.
			// Trades a small latency cost for durability — events survive a broker failure mid-replication.
			// With our single-broker dev cluster this is effectively the same as RequireOne, but it future-proofs
			// the deployment for a multi-broker cluster without code changes.
			RequiredAcks: kafkago.RequireAll,
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
