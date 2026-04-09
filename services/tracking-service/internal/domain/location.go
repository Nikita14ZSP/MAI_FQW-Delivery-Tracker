package domain

import (
	"encoding/json"
	"time"
)

// LocationPoint represents a recorded GPS position of a courier for an order.
type LocationPoint struct {
	ID         string    `json:"id"`
	CourierID  string    `json:"courier_id"`
	OrderID    string    `json:"order_id"`
	Lat        float64   `json:"lat"`
	Lng        float64   `json:"lng"`
	RecordedAt time.Time `json:"recorded_at"`
}

// CachedLocation is the Redis-cached last-known position of a courier.
// UpdatedAt is stored as RFC3339 string for easy JSON serialization.
type CachedLocation struct {
	CourierID string  `json:"courier_id"`
	OrderID   string  `json:"order_id"`
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	UpdatedAt string  `json:"updated_at"` // RFC3339
}

// WSMessage is the WebSocket message envelope per D-01.
type WSMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// WS message type constants.
const (
	MsgTypeLocationUpdate    = "location_update"
	MsgTypeOrderCreated      = "order_created"
	MsgTypeOrderStatusChange = "order_status_change"
	MsgTypeDeliveryAssigned  = "delivery_assigned"
	MsgTypeDeliveryStatus    = "delivery_status"
)

// LocationUpdateData is the data payload for location_update messages.
type LocationUpdateData struct {
	CourierID string  `json:"courier_id"`
	OrderID   string  `json:"order_id"`
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	Timestamp string  `json:"timestamp"` // RFC3339
}
