package kafka_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	kafkago "github.com/segmentio/kafka-go"

	pkgkafka "github.com/mozgovojnikita/delivery-tracker/pkg/kafka"
	trackingkafka "github.com/mozgovojnikita/delivery-tracker/services/tracking-service/internal/kafka"
)

// mockTrackingService implements TrackingServiceIface for testing.
type mockTrackingService struct {
	orderCreatedCalled      bool
	orderCreatedEvent       pkgkafka.OrderCreatedEvent
	orderUpdatedCalled      bool
	orderUpdatedEvent       pkgkafka.OrderUpdatedEvent
	deliveryAssignedCalled  bool
	deliveryAssignedEvent   pkgkafka.DeliveryAssignedEvent
	deliveryStatusCalled    bool
	deliveryStatusEvent     pkgkafka.DeliveryStatusEvent
	returnErr               error
}

func (m *mockTrackingService) BroadcastOrderCreated(ctx context.Context, event pkgkafka.OrderCreatedEvent) error {
	m.orderCreatedCalled = true
	m.orderCreatedEvent = event
	return m.returnErr
}

func (m *mockTrackingService) BroadcastOrderUpdated(ctx context.Context, event pkgkafka.OrderUpdatedEvent) error {
	m.orderUpdatedCalled = true
	m.orderUpdatedEvent = event
	return m.returnErr
}

func (m *mockTrackingService) BroadcastDeliveryAssigned(ctx context.Context, event pkgkafka.DeliveryAssignedEvent) error {
	m.deliveryAssignedCalled = true
	m.deliveryAssignedEvent = event
	return m.returnErr
}

func (m *mockTrackingService) BroadcastDeliveryStatus(ctx context.Context, event pkgkafka.DeliveryStatusEvent) error {
	m.deliveryStatusCalled = true
	m.deliveryStatusEvent = event
	return m.returnErr
}

func TestHandleOrderCreatedMessage(t *testing.T) {
	svc := &mockTrackingService{}
	handler := trackingkafka.HandleOrderCreatedMessage(svc)

	event := pkgkafka.OrderCreatedEvent{
		EventID: "evt-1", OrderID: "order-1", UserID: "user-1",
		Address: "Moscow", Lat: 55.75, Lng: 37.62, CreatedAt: "2026-01-01T00:00:00Z",
	}
	data, _ := json.Marshal(event)
	msg := kafkago.Message{Value: data}

	if err := handler(context.Background(), msg); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !svc.orderCreatedCalled {
		t.Error("BroadcastOrderCreated was not called")
	}
	if svc.orderCreatedEvent.OrderID != "order-1" {
		t.Errorf("expected order_id order-1, got %s", svc.orderCreatedEvent.OrderID)
	}
}

func TestHandleOrderUpdatedMessage(t *testing.T) {
	svc := &mockTrackingService{}
	handler := trackingkafka.HandleOrderUpdatedMessage(svc)

	event := pkgkafka.OrderUpdatedEvent{
		EventID: "evt-2", OrderID: "order-2", UserID: "user-2",
		OldStatus: "pending", NewStatus: "confirmed", UpdatedAt: "2026-01-01T00:00:00Z",
	}
	data, _ := json.Marshal(event)
	msg := kafkago.Message{Value: data}

	if err := handler(context.Background(), msg); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !svc.orderUpdatedCalled {
		t.Error("BroadcastOrderUpdated was not called")
	}
	if svc.orderUpdatedEvent.NewStatus != "confirmed" {
		t.Errorf("expected new_status confirmed, got %s", svc.orderUpdatedEvent.NewStatus)
	}
}

func TestHandleDeliveryAssignedMessage(t *testing.T) {
	svc := &mockTrackingService{}
	handler := trackingkafka.HandleDeliveryAssignedMessage(svc)

	event := pkgkafka.DeliveryAssignedEvent{
		EventID: "evt-3", DeliveryID: "del-1", OrderID: "order-3",
		UserID: "user-3", CourierID: "courier-1", ZoneID: "zone-1",
		ETA: "2026-01-01T01:00:00Z", AssignedAt: "2026-01-01T00:00:00Z",
	}
	data, _ := json.Marshal(event)
	msg := kafkago.Message{Value: data}

	if err := handler(context.Background(), msg); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !svc.deliveryAssignedCalled {
		t.Error("BroadcastDeliveryAssigned was not called")
	}
	if svc.deliveryAssignedEvent.CourierID != "courier-1" {
		t.Errorf("expected courier_id courier-1, got %s", svc.deliveryAssignedEvent.CourierID)
	}
}

func TestHandleDeliveryStatusMessage(t *testing.T) {
	svc := &mockTrackingService{}
	handler := trackingkafka.HandleDeliveryStatusMessage(svc)

	event := pkgkafka.DeliveryStatusEvent{
		EventID: "evt-4", DeliveryID: "del-2", OrderID: "order-4",
		CourierID: "courier-2", OldStatus: "assigned", NewStatus: "in_transit",
		UpdatedAt: "2026-01-01T00:00:00Z",
	}
	data, _ := json.Marshal(event)
	msg := kafkago.Message{Value: data}

	if err := handler(context.Background(), msg); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !svc.deliveryStatusCalled {
		t.Error("BroadcastDeliveryStatus was not called")
	}
	if svc.deliveryStatusEvent.NewStatus != "in_transit" {
		t.Errorf("expected new_status in_transit, got %s", svc.deliveryStatusEvent.NewStatus)
	}
}

func TestHandleInvalidJSON_ReturnsError(t *testing.T) {
	svc := &mockTrackingService{}

	handlers := []func(ctx context.Context, msg kafkago.Message) error{
		trackingkafka.HandleOrderCreatedMessage(svc),
		trackingkafka.HandleOrderUpdatedMessage(svc),
		trackingkafka.HandleDeliveryAssignedMessage(svc),
		trackingkafka.HandleDeliveryStatusMessage(svc),
	}

	for i, handler := range handlers {
		msg := kafkago.Message{Value: []byte(`not-valid-json`)}
		err := handler(context.Background(), msg)
		if err == nil {
			t.Errorf("handler[%d]: expected error for invalid JSON, got nil", i)
		}
		if !errors.Is(err, err) { // always true — just confirm non-nil
			t.Errorf("handler[%d]: unexpected error type", i)
		}
	}
}
