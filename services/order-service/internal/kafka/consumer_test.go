package kafka

import (
	"context"
	"encoding/json"
	"testing"

	kafkago "github.com/segmentio/kafka-go"

	pkgkafka "github.com/mozgovojnikita/delivery-tracker/pkg/kafka"
	"github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/domain"
)

// mockSyncer is a hand-rolled OrderStatusSyncer recording each (orderID,target) call.
type mockSyncer struct {
	calls []syncCall
	err   error
}

type syncCall struct {
	orderID string
	target  domain.OrderStatus
}

func (m *mockSyncer) SyncOrderToStatus(_ context.Context, orderID string, target domain.OrderStatus) error {
	m.calls = append(m.calls, syncCall{orderID: orderID, target: target})
	return m.err
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func TestHandleDeliveryAssignedMessage_ValidJSON_CallsSyncerWithAssigned(t *testing.T) {
	m := &mockSyncer{}
	h := HandleDeliveryAssignedMessage(m)

	payload := mustJSON(t, pkgkafka.DeliveryAssignedEvent{
		EventID: "ev-1", DeliveryID: "d-1", OrderID: "order-1", CourierID: "c-1",
	})
	if err := h(context.Background(), kafkago.Message{Value: payload}); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if len(m.calls) != 1 {
		t.Fatalf("expected 1 syncer call, got %d", len(m.calls))
	}
	if m.calls[0].orderID != "order-1" || m.calls[0].target != domain.StatusAssigned {
		t.Errorf("expected (order-1, assigned), got (%s, %s)", m.calls[0].orderID, m.calls[0].target)
	}
}

func TestHandleDeliveryAssignedMessage_MalformedJSON_ReturnsErr(t *testing.T) {
	m := &mockSyncer{}
	h := HandleDeliveryAssignedMessage(m)

	if err := h(context.Background(), kafkago.Message{Value: []byte("{not json")}); err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if len(m.calls) != 0 {
		t.Errorf("expected zero syncer calls on malformed JSON, got %d", len(m.calls))
	}
}

func TestHandleDeliveryAssignedMessage_EmptyOrderID_WarnsAndNoOp(t *testing.T) {
	m := &mockSyncer{}
	h := HandleDeliveryAssignedMessage(m)

	payload := mustJSON(t, pkgkafka.DeliveryAssignedEvent{EventID: "ev-1", OrderID: ""})
	if err := h(context.Background(), kafkago.Message{Value: payload}); err != nil {
		t.Fatalf("expected nil error for empty order_id, got: %v", err)
	}
	if len(m.calls) != 0 {
		t.Errorf("expected zero syncer calls for empty order_id, got %d", len(m.calls))
	}
}

func TestHandleDeliveryStatusMessage_Mapping(t *testing.T) {
	cases := []struct {
		newStatus string
		want      domain.OrderStatus
	}{
		{"assigned", domain.StatusAssigned},
		{"picked_up", domain.StatusPickedUp},
		{"in_transit", domain.StatusInTransit},
		{"delivered", domain.StatusDelivered},
		{"failed", domain.StatusFailed},
	}
	for _, tc := range cases {
		t.Run(tc.newStatus, func(t *testing.T) {
			m := &mockSyncer{}
			h := HandleDeliveryStatusMessage(m)
			payload := mustJSON(t, pkgkafka.DeliveryStatusEvent{
				EventID: "ev-1", OrderID: "order-1", NewStatus: tc.newStatus,
			})
			if err := h(context.Background(), kafkago.Message{Value: payload}); err != nil {
				t.Fatalf("expected nil error, got: %v", err)
			}
			if len(m.calls) != 1 {
				t.Fatalf("expected 1 syncer call, got %d", len(m.calls))
			}
			if m.calls[0].orderID != "order-1" || m.calls[0].target != tc.want {
				t.Errorf("expected (order-1, %s), got (%s, %s)", tc.want, m.calls[0].orderID, m.calls[0].target)
			}
		})
	}
}

func TestHandleDeliveryStatusMessage_UnknownStatus_WarnsAndNoOp(t *testing.T) {
	m := &mockSyncer{}
	h := HandleDeliveryStatusMessage(m)

	payload := mustJSON(t, pkgkafka.DeliveryStatusEvent{
		EventID: "ev-1", OrderID: "order-1", NewStatus: "bogus",
	})
	if err := h(context.Background(), kafkago.Message{Value: payload}); err != nil {
		t.Fatalf("expected nil error for unknown status (warn path), got: %v", err)
	}
	if len(m.calls) != 0 {
		t.Errorf("expected zero syncer calls for unknown status, got %d", len(m.calls))
	}
}

func TestHandleDeliveryStatusMessage_EmptyStatus_WarnsAndNoOp(t *testing.T) {
	m := &mockSyncer{}
	h := HandleDeliveryStatusMessage(m)

	payload := mustJSON(t, pkgkafka.DeliveryStatusEvent{
		EventID: "ev-1", OrderID: "order-1", NewStatus: "",
	})
	if err := h(context.Background(), kafkago.Message{Value: payload}); err != nil {
		t.Fatalf("expected nil error for empty status, got: %v", err)
	}
	if len(m.calls) != 0 {
		t.Errorf("expected zero syncer calls for empty status, got %d", len(m.calls))
	}
}

func TestHandleDeliveryStatusMessage_EmptyOrderID_WarnsAndNoOp(t *testing.T) {
	m := &mockSyncer{}
	h := HandleDeliveryStatusMessage(m)

	payload := mustJSON(t, pkgkafka.DeliveryStatusEvent{
		EventID: "ev-1", OrderID: "", NewStatus: "in_transit",
	})
	if err := h(context.Background(), kafkago.Message{Value: payload}); err != nil {
		t.Fatalf("expected nil error for empty order_id, got: %v", err)
	}
	if len(m.calls) != 0 {
		t.Errorf("expected zero syncer calls for empty order_id, got %d", len(m.calls))
	}
}

func TestHandleDeliveryStatusMessage_MalformedJSON_ReturnsErr(t *testing.T) {
	m := &mockSyncer{}
	h := HandleDeliveryStatusMessage(m)

	if err := h(context.Background(), kafkago.Message{Value: []byte("{not json")}); err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if len(m.calls) != 0 {
		t.Errorf("expected zero syncer calls on malformed JSON, got %d", len(m.calls))
	}
}
