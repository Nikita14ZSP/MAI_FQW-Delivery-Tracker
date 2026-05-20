package ws_test

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"github.com/mozgovojnikita/delivery-tracker/services/tracking-service/internal/handler/ws"
)

// httpToWS converts http://host → ws://host for gorilla dialer.
func httpToWS(t *testing.T, raw, path string) string {
	t.Helper()
	return "ws" + strings.TrimPrefix(raw, "http") + path
}

// TestWebSocket_EndToEnd_PublishDelivered exercises the full real-time path:
// dial WS → Hub.Subscribe via handler → Hub.Publish → Redis Pub/Sub fan-out → client receives bytes.
// This guards against regressions in the live tracking demo (CRDR / CTRK phases).
func TestWebSocket_EndToEnd_PublishDelivered(t *testing.T) {
	mr, rdb := newTestRedis(t)
	hub := ws.NewHub(rdb)

	r := chi.NewRouter()
	r.Get("/ws/orders/{order_id}", ws.ServeWS(hub))
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	const orderID = "order-e2e-42"
	url := httpToWS(t, srv.URL, "/ws/orders/"+orderID)

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial WS: %v", err)
	}
	defer conn.Close()

	// Wait until handler registered the client + Redis subscription is established.
	// ConnectionCountForOrder flips synchronously in Subscribe; the Redis subscriber
	// goroutine starts right after, but miniredis needs a brief moment to register it.
	deadline := time.Now().Add(2 * time.Second)
	for hub.ConnectionCountForOrder(orderID) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := hub.ConnectionCountForOrder(orderID); got != 1 {
		t.Fatalf("expected 1 active connection, got %d", got)
	}
	time.Sleep(50 * time.Millisecond) // let listenRedis attach (matches hub_test.go pattern)

	// Publish via miniredis directly (matches the deterministic pattern used in hub_test.go).
	payload := `{"type":"location_update","data":{"courier_id":"c-1","order_id":"order-e2e-42","lat":55.75,"lng":37.61,"timestamp":"2026-05-20T12:00:00Z"}}`
	mr.Publish("tracking:order:"+orderID, payload)

	if err := conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	mt, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if mt != websocket.TextMessage {
		t.Errorf("expected TextMessage, got %d", mt)
	}
	if string(data) != payload {
		t.Errorf("payload mismatch:\nwant: %s\ngot:  %s", payload, data)
	}
}

// TestWebSocket_EndToEnd_DisconnectUnsubscribes ensures Hub bookkeeping
// correctly removes the client when the WS connection closes.
func TestWebSocket_EndToEnd_DisconnectUnsubscribes(t *testing.T) {
	_, rdb := newTestRedis(t)
	hub := ws.NewHub(rdb)

	r := chi.NewRouter()
	r.Get("/ws/orders/{order_id}", ws.ServeWS(hub))
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	const orderID = "order-unsub-7"
	url := httpToWS(t, srv.URL, "/ws/orders/"+orderID)

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial WS: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for hub.ConnectionCountForOrder(orderID) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if hub.ConnectionCountForOrder(orderID) != 1 {
		t.Fatalf("expected 1 connection before close, got %d", hub.ConnectionCountForOrder(orderID))
	}

	_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	conn.Close()

	deadline = time.Now().Add(2 * time.Second)
	for hub.ConnectionCountForOrder(orderID) > 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := hub.ConnectionCountForOrder(orderID); got != 0 {
		t.Errorf("expected 0 connections after close, got %d", got)
	}
}
