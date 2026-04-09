package kafka

import (
	"context"
	"encoding/json"
	"log/slog"

	kafkago "github.com/segmentio/kafka-go"

	pkgkafka "github.com/mozgovojnikita/delivery-tracker/pkg/kafka"
	"github.com/mozgovojnikita/delivery-tracker/services/delivery-service/internal/service"
)

// HandleOrderCreatedMessage returns a Kafka message handler function that processes
// orders.created events by deserializing them and delegating to the delivery service.
func HandleOrderCreatedMessage(svc *service.DeliveryService) func(ctx context.Context, msg kafkago.Message) error {
	return func(ctx context.Context, msg kafkago.Message) error {
		var event pkgkafka.OrderCreatedEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			slog.Error("unmarshal order created event", "err", err)
			return err
		}
		slog.Info("processing order created event", "order_id", event.OrderID, "event_id", event.EventID)
		return svc.HandleOrderCreated(ctx, service.OrderCreatedInput{
			OrderID: event.OrderID,
			UserID:  event.UserID,
			Lat:     event.Lat,
			Lng:     event.Lng,
		})
	}
}

// HandleOrderUpdatedMessage returns a Kafka message handler that syncs delivery status
// when order status changes (e.g. picked_up, in_transit, delivered, cancelled).
func HandleOrderUpdatedMessage(svc *service.DeliveryService) func(ctx context.Context, msg kafkago.Message) error {
	return func(ctx context.Context, msg kafkago.Message) error {
		var event pkgkafka.OrderUpdatedEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			slog.Error("unmarshal order updated event", "err", err)
			return err
		}
		slog.Info("processing order updated event", "order_id", event.OrderID, "new_status", event.NewStatus)
		return svc.SyncDeliveryStatus(ctx, event.OrderID, event.NewStatus)
	}
}
