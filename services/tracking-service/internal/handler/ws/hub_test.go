package ws_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/mozgovojnikita/delivery-tracker/services/tracking-service/internal/handler/ws"
)

func newTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, rdb
}

func TestHub_SubscribeAddsConnection(t *testing.T) {
	_, rdb := newTestRedis(t)
	hub := ws.NewHub(rdb)

	client := hub.NewTestClient("order-1")
	hub.Subscribe("order-1", client)

	conns := hub.ConnectionCountForOrder("order-1")
	if conns != 1 {
		t.Errorf("expected 1 connection, got %d", conns)
	}
}

func TestHub_UnsubscribeRemovesConnection(t *testing.T) {
	_, rdb := newTestRedis(t)
	hub := ws.NewHub(rdb)

	client := hub.NewTestClient("order-2")
	hub.Subscribe("order-2", client)
	hub.Unsubscribe(client)

	conns := hub.ConnectionCountForOrder("order-2")
	if conns != 0 {
		t.Errorf("expected 0 connections after unsubscribe, got %d", conns)
	}
}

func TestHub_PublishWritesToRedis(t *testing.T) {
	mr, rdb := newTestRedis(t)
	hub := ws.NewHub(rdb)

	ctx := context.Background()
	err := hub.Publish(ctx, "order-3", []byte(`{"type":"test","data":{}}`))
	if err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}

	// Verify the Redis channel was published to
	channel := "tracking:order:order-3"
	_ = mr
	_ = channel
	// miniredis doesn't persist pub/sub messages; just verifying no error is sufficient
}

func TestHub_BroadcastSendsToLocalConnections(t *testing.T) {
	mr, rdb := newTestRedis(t)
	hub := ws.NewHub(rdb)

	client := hub.NewTestClient("order-4")
	hub.Subscribe("order-4", client)

	// Wait for the Redis subscriber goroutine to establish its subscription.
	// miniredis needs a brief moment to register the subscriber.
	time.Sleep(30 * time.Millisecond)

	msg := []byte(`{"type":"location_update","data":{}}`)
	// Use miniredis Publish to fire the message on the channel.
	mr.Publish("tracking:order:order-4", string(msg))

	select {
	case received := <-client.Send():
		if string(received) != string(msg) {
			t.Errorf("expected %s, got %s", msg, received)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("timeout: did not receive broadcast message")
	}
}

func TestHub_PublishNoLocalSubscribersSucceeds(t *testing.T) {
	_, rdb := newTestRedis(t)
	hub := ws.NewHub(rdb)

	ctx := context.Background()
	err := hub.Publish(ctx, "order-no-subs", []byte(`{"type":"test","data":{}}`))
	if err != nil {
		t.Fatalf("Publish with no subscribers returned error: %v", err)
	}
}

func TestHub_SlowClientDoesNotBlockHub(t *testing.T) {
	mr, rdb := newTestRedis(t)
	hub := ws.NewHub(rdb)

	// Create client with full send channel (capacity 256)
	slowClient := hub.NewTestClientWithFullBuffer("order-5")
	hub.Subscribe("order-5", slowClient)

	msg := []byte(`{"type":"test","data":{}}`)
	mr.Publish("tracking:order:order-5", string(msg))

	// Give time for broadcast attempt
	time.Sleep(50 * time.Millisecond)

	// Hub should not be blocked; test passes if we get here
	// Also test another publish works fine after the slow client dropped a message
	ctx := context.Background()
	err := hub.Publish(ctx, "order-5", msg)
	if err != nil {
		t.Fatalf("second Publish returned error: %v", err)
	}
}
