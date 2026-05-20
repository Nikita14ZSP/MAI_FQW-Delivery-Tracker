package ws

import (
	"context"
	"log/slog"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

const channelPrefix = "tracking:order:"

// Client represents a single WebSocket connection.
type Client struct {
	conn    *websocket.Conn
	send    chan []byte
	orderID string
	hub     *Hub
}

// Send returns the client's send channel (used in tests and write pump).
func (c *Client) Send() <-chan []byte {
	return c.send
}

// Hub manages WebSocket connections grouped by order ID, with Redis Pub/Sub fan-out.
type Hub struct {
	// connections: orderID -> set of *Client
	connections map[string]map[*Client]bool
	mu          sync.RWMutex
	rdb         *redis.Client
	// Track Redis subscriptions per order to avoid duplicates
	subs   map[string]context.CancelFunc
	subsMu sync.Mutex
}

// NewHub creates a new Hub backed by the given Redis client.
func NewHub(rdb *redis.Client) *Hub {
	return &Hub{
		connections: make(map[string]map[*Client]bool),
		rdb:         rdb,
		subs:        make(map[string]context.CancelFunc),
	}
}

// Subscribe registers a client for an order's WebSocket channel.
// If this is the first subscriber for the order, it starts a Redis Pub/Sub listener.
func (h *Hub) Subscribe(orderID string, client *Client) {
	h.mu.Lock()
	if h.connections[orderID] == nil {
		h.connections[orderID] = make(map[*Client]bool)
	}
	h.connections[orderID][client] = true
	isFirst := len(h.connections[orderID]) == 1
	h.mu.Unlock()

	if isFirst {
		ctx, cancel := context.WithCancel(context.Background())
		h.subsMu.Lock()
		h.subs[orderID] = cancel
		h.subsMu.Unlock()
		go h.listenRedis(ctx, orderID)
	}
}

// Unsubscribe removes a client from an order's connection set and closes its send channel.
// If this was the last client for the order, the Redis listener goroutine is stopped.
func (h *Hub) Unsubscribe(client *Client) {
	orderID := client.orderID

	h.mu.Lock()
	if conns, ok := h.connections[orderID]; ok {
		delete(conns, client)
		if len(conns) == 0 {
			delete(h.connections, orderID)
		}
	}
	empty := len(h.connections[orderID]) == 0
	h.mu.Unlock()

	// Close the send channel to signal the write pump to stop.
	close(client.send)

	if empty {
		h.subsMu.Lock()
		if cancel, ok := h.subs[orderID]; ok {
			cancel()
			delete(h.subs, orderID)
		}
		h.subsMu.Unlock()
	}
}

// Publish writes a message to the Redis Pub/Sub channel for the given order.
// This publishes to Redis regardless of local subscribers — other instances may have subscribers.
func (h *Hub) Publish(ctx context.Context, orderID string, payload []byte) error {
	return h.rdb.Publish(ctx, channelPrefix+orderID, payload).Err()
}

// listenRedis subscribes to the Redis channel for an order and broadcasts received messages
// to all local WebSocket connections for that order.
func (h *Hub) listenRedis(ctx context.Context, orderID string) {
	sub := h.rdb.Subscribe(ctx, channelPrefix+orderID)
	defer func() {
		if err := sub.Close(); err != nil {
			slog.Warn("redis subscriber close error", "order_id", orderID, "err", err)
		}
	}()

	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			h.broadcast(orderID, []byte(msg.Payload))
		}
	}
}

// broadcast sends data to all local WebSocket connections for the given order.
// Uses non-blocking send to avoid blocking if a client is slow — drops message for slow clients.
// Snapshots the client set under the read lock to avoid a concurrent map iteration / Unsubscribe race.
func (h *Hub) broadcast(orderID string, data []byte) {
	h.mu.RLock()
	conns := h.connections[orderID]
	clients := make([]*Client, 0, len(conns))
	for c := range conns {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	for _, client := range clients {
		select {
		case client.send <- data:
		default:
			slog.Warn("dropping message for slow client", "order_id", orderID)
		}
	}
}

// ConnectionCountForOrder returns the number of active connections for an order (test helper).
func (h *Hub) ConnectionCountForOrder(orderID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.connections[orderID])
}

// NewTestClient creates a Client for use in tests (no real WebSocket connection).
func (h *Hub) NewTestClient(orderID string) *Client {
	return &Client{
		conn:    nil,
		send:    make(chan []byte, 256),
		orderID: orderID,
		hub:     h,
	}
}

// NewTestClientWithFullBuffer creates a Client with a full send channel for slow-client testing.
func (h *Hub) NewTestClientWithFullBuffer(orderID string) *Client {
	ch := make(chan []byte, 256)
	// Fill the buffer completely so sends are non-blocking dropped.
	for i := 0; i < 256; i++ {
		ch <- []byte(`{}`)
	}
	return &Client{
		conn:    nil,
		send:    ch,
		orderID: orderID,
		hub:     h,
	}
}
