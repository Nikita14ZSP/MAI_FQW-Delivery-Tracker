package kafka

import (
	"context"
	"encoding/json"
	"log/slog"

	kafkago "github.com/segmentio/kafka-go"

	pkgkafka "github.com/mozgovojnikita/delivery-tracker/pkg/kafka"
)

// TrackingServiceIface is extracted for testability (same pattern as Phase 3 NotificationServiceIface).
// It defines only the Broadcast* methods needed by Kafka handlers.
type TrackingServiceIface interface {
	BroadcastOrderCreated(ctx context.Context, event pkgkafka.OrderCreatedEvent) error
	BroadcastOrderUpdated(ctx context.Context, event pkgkafka.OrderUpdatedEvent) error
	BroadcastDeliveryAssigned(ctx context.Context, event pkgkafka.DeliveryAssignedEvent) error
	BroadcastDeliveryStatus(ctx context.Context, event pkgkafka.DeliveryStatusEvent) error
}

// HandleOrderCreatedMessage returns a Kafka handler closure that processes orders.created events.
// It deserializes the message and delegates to the tracking service for WebSocket broadcast.
func HandleOrderCreatedMessage(svc TrackingServiceIface) func(ctx context.Context, msg kafkago.Message) error {
	return func(ctx context.Context, msg kafkago.Message) error {
		var event pkgkafka.OrderCreatedEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			slog.Error("unmarshal order_created event", "err", err)
			return err
		}
		slog.Info("tracking: broadcasting order_created", "order_id", event.OrderID, "event_id", event.EventID)
		return svc.BroadcastOrderCreated(ctx, event)
	}
}

// HandleOrderUpdatedMessage returns a Kafka handler closure that processes orders.updated events.
// It deserializes the message and delegates to the tracking service for WebSocket broadcast.
func HandleOrderUpdatedMessage(svc TrackingServiceIface) func(ctx context.Context, msg kafkago.Message) error {
	return func(ctx context.Context, msg kafkago.Message) error {
		var event pkgkafka.OrderUpdatedEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			slog.Error("unmarshal order_updated event", "err", err)
			return err
		}
		slog.Info("tracking: broadcasting order_updated", "order_id", event.OrderID, "event_id", event.EventID)
		return svc.BroadcastOrderUpdated(ctx, event)
	}
}

// HandleDeliveryAssignedMessage returns a Kafka handler closure that processes delivery.assigned events.
// It deserializes the message and delegates to the tracking service for WebSocket broadcast.
func HandleDeliveryAssignedMessage(svc TrackingServiceIface) func(ctx context.Context, msg kafkago.Message) error {
	return func(ctx context.Context, msg kafkago.Message) error {
		var event pkgkafka.DeliveryAssignedEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			slog.Error("unmarshal delivery_assigned event", "err", err)
			return err
		}
		slog.Info("tracking: broadcasting delivery_assigned", "order_id", event.OrderID, "event_id", event.EventID)
		return svc.BroadcastDeliveryAssigned(ctx, event)
	}
}

// HandleDeliveryStatusMessage returns a Kafka handler closure that processes delivery.status events.
// It deserializes the message and delegates to the tracking service for WebSocket broadcast.
func HandleDeliveryStatusMessage(svc TrackingServiceIface) func(ctx context.Context, msg kafkago.Message) error {
	return func(ctx context.Context, msg kafkago.Message) error {
		var event pkgkafka.DeliveryStatusEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			slog.Error("unmarshal delivery_status event", "err", err)
			return err
		}
		slog.Info("tracking: broadcasting delivery_status", "order_id", event.OrderID, "event_id", event.EventID)
		return svc.BroadcastDeliveryStatus(ctx, event)
	}
}
