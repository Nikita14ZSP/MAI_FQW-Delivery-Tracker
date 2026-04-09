package kafka

import (
	"context"
	"encoding/json"
	"log/slog"

	kafkago "github.com/segmentio/kafka-go"

	pkgkafka "github.com/mozgovojnikita/delivery-tracker/pkg/kafka"
	"github.com/mozgovojnikita/delivery-tracker/services/notification-service/internal/service"
)

// HandleDeliveryAssignedMessage returns a Kafka message handler function that
// processes delivery.assigned events and delegates to the notification service.
func HandleDeliveryAssignedMessage(svc *service.NotificationService) func(ctx context.Context, msg kafkago.Message) error {
	return func(ctx context.Context, msg kafkago.Message) error {
		var event pkgkafka.DeliveryAssignedEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			slog.Error("unmarshal delivery assigned event", "err", err)
			return err
		}
		slog.Info("processing delivery assigned event", "order_id", event.OrderID, "event_id", event.EventID)
		return svc.HandleDeliveryAssigned(ctx, event)
	}
}

// HandleOrderUpdatedMessage returns a Kafka message handler function that
// processes orders.updated events and delegates to the notification service.
func HandleOrderUpdatedMessage(svc *service.NotificationService) func(ctx context.Context, msg kafkago.Message) error {
	return func(ctx context.Context, msg kafkago.Message) error {
		var event pkgkafka.OrderUpdatedEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			slog.Error("unmarshal order updated event", "err", err)
			return err
		}
		slog.Info("processing order updated event", "order_id", event.OrderID, "new_status", event.NewStatus)
		return svc.HandleOrderUpdated(ctx, event)
	}
}
