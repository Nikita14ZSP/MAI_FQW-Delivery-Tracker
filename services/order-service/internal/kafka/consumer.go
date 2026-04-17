package kafka

import (
	"context"
	"encoding/json"
	"log/slog"

	kafkago "github.com/segmentio/kafka-go"

	pkgkafka "github.com/mozgovojnikita/delivery-tracker/pkg/kafka"
	"github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/domain"
)

// OrderStatusSyncer is the read-side dependency of the delivery.* consumers.
// *service.OrderService satisfies it structurally via SyncOrderToStatus. The
// interface lives here (not in the service package) so package kafka does NOT
// import package service — service already imports this package (producer.go),
// so the reverse import would be a cycle.
type OrderStatusSyncer interface {
	SyncOrderToStatus(ctx context.Context, orderID string, target domain.OrderStatus) error
}

// deliveryStatusToOrderStatus maps a delivery-service NewStatus string onto the
// order-service domain status. Unknown/empty keys are absent => warn + no-op.
var deliveryStatusToOrderStatus = map[string]domain.OrderStatus{
	"assigned":   domain.StatusAssigned,
	"picked_up":  domain.StatusPickedUp,
	"in_transit": domain.StatusInTransit,
	"delivered":  domain.StatusDelivered,
	"failed":     domain.StatusFailed,
}

// HandleDeliveryAssignedMessage returns a Kafka handler for delivery.assigned.
// On a courier accepting a delivery, the order is advanced forward to 'assigned'
// through the legal state machine. Error semantics match delivery-service:
//   - malformed JSON => return err (not committed; redelivered)
//   - empty order_id => warn + nil (nothing to sync; do not wedge the consumer)
func HandleDeliveryAssignedMessage(svc OrderStatusSyncer) func(ctx context.Context, msg kafkago.Message) error {
	return func(ctx context.Context, msg kafkago.Message) error {
		var event pkgkafka.DeliveryAssignedEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			slog.ErrorContext(ctx, "unmarshal delivery assigned event", "err", err)
			return err
		}
		slog.InfoContext(ctx, "processing delivery assigned event",
			"order_id", event.OrderID, "event_id", event.EventID)
		if event.OrderID == "" {
			slog.WarnContext(ctx, "delivery assigned event missing order_id, skipping",
				"event_id", event.EventID)
			return nil
		}
		return svc.SyncOrderToStatus(ctx, event.OrderID, domain.StatusAssigned)
	}
}

// HandleDeliveryStatusMessage returns a Kafka handler for delivery.status.
// It maps the delivery's NewStatus to the order domain status and advances the
// order forward through legal transitions. Error semantics match delivery-service:
//   - malformed JSON          => return err (not committed; redelivered)
//   - unknown/empty NewStatus => warn + nil (non-fatal; commit offset, no wedge)
//   - empty order_id          => warn + nil
//
// Note: "failed" maps to domain.StatusFailed, which is intentionally OFF the
// linear syncForwardChain — SyncOrderToStatus no-ops it safely. Failure-terminal
// handling is out of this consumer's scope; the no-op is the documented behavior.
func HandleDeliveryStatusMessage(svc OrderStatusSyncer) func(ctx context.Context, msg kafkago.Message) error {
	return func(ctx context.Context, msg kafkago.Message) error {
		var event pkgkafka.DeliveryStatusEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			slog.ErrorContext(ctx, "unmarshal delivery status event", "err", err)
			return err
		}
		slog.InfoContext(ctx, "processing delivery status event",
			"order_id", event.OrderID, "new_status", event.NewStatus, "event_id", event.EventID)

		target, ok := deliveryStatusToOrderStatus[event.NewStatus]
		if !ok {
			slog.WarnContext(ctx, "unknown delivery status, skipping",
				"new_status", event.NewStatus, "order_id", event.OrderID)
			return nil
		}
		if event.OrderID == "" {
			slog.WarnContext(ctx, "delivery status event missing order_id, skipping",
				"event_id", event.EventID, "new_status", event.NewStatus)
			return nil
		}
		return svc.SyncOrderToStatus(ctx, event.OrderID, target)
	}
}
