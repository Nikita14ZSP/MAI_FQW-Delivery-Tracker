package ws

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 54 * time.Second // (pongWait * 9) / 10
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// CheckOrigin always returns true — the API Gateway handles auth.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ServeWS returns an HTTP handler that upgrades the connection to WebSocket
// and subscribes the client to the specified order's real-time updates.
func ServeWS(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orderID := chi.URLParam(r, "order_id")
		if orderID == "" {
			http.Error(w, "missing order_id", http.StatusBadRequest)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			slog.Error("websocket upgrade failed", "order_id", orderID, "err", err)
			return
		}

		client := &Client{
			conn:    conn,
			send:    make(chan []byte, 256),
			orderID: orderID,
			hub:     hub,
		}

		hub.Subscribe(orderID, client)

		// writePump owns the connection writer; readPump detects disconnect.
		go client.writePump()
		go client.readPump()
	}
}

// writePump is the sole goroutine writing to the WebSocket connection (single-writer pattern).
// It reads from client.send and writes to the WebSocket, sending pings periodically.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				slog.Warn("set write deadline error", "order_id", c.orderID, "err", err)
				return
			}
			if !ok {
				// Channel closed — send close frame and exit.
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				slog.Warn("write message error", "order_id", c.orderID, "err", err)
				return
			}

		case <-ticker.C:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				slog.Warn("set write deadline error (ping)", "order_id", c.orderID, "err", err)
				return
			}
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				slog.Warn("ping error", "order_id", c.orderID, "err", err)
				return
			}
		}
	}
}

// readPump reads incoming messages (client doesn't send data, just pong frames).
// Disconnect detection happens here via read errors.
func (c *Client) readPump() {
	defer func() {
		c.hub.Unsubscribe(c)
	}()

	c.conn.SetReadLimit(maxMessageSize)
	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		slog.Warn("set read deadline error", "order_id", c.orderID, "err", err)
		return
	}
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Info("websocket closed unexpectedly", "order_id", c.orderID, "err", err)
			}
			return
		}
	}
}
