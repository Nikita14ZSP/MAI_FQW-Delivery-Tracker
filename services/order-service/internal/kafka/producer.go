package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	pkgkafka "github.com/mozgovojnikita/delivery-tracker/pkg/kafka"
	"github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/domain"
)

// PublishOrderCreated publishes an OrderCreatedEvent to the orders.created topic.
func PublishOrderCreated(ctx context.Context, pub pkgkafka.EventPublisher, order *domain.Order) error {
	event := pkgkafka.OrderCreatedEvent{
		EventID:   uuid.New().String(),
		OrderID:   order.ID,
		UserID:    order.UserID,
		Address:   order.DeliveryAddress,
		Lat:       order.DeliveryLat,
		Lng:       order.DeliveryLng,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal order created event: %w", err)
	}
	return pub.Publish(ctx, pkgkafka.TopicOrdersCreated, order.ID, data)
}

// PublishOrderUpdated publishes an OrderUpdatedEvent to the orders.updated topic.
func PublishOrderUpdated(ctx context.Context, pub pkgkafka.EventPublisher, order *domain.Order, oldStatus, newStatus string) error {
	event := pkgkafka.OrderUpdatedEvent{
		EventID:   uuid.New().String(),
		OrderID:   order.ID,
		UserID:    order.UserID,
		OldStatus: oldStatus,
		NewStatus: newStatus,
		UpdatedAt: time.Now().Format(time.RFC3339),
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal order updated event: %w", err)
	}
	return pub.Publish(ctx, pkgkafka.TopicOrdersUpdated, order.ID, data)
}
