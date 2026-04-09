package kafka

// Topic constants
const (
	TopicOrdersCreated    = "orders.created"
	TopicOrdersUpdated    = "orders.updated"
	TopicDeliveryAssigned = "delivery.assigned"
	TopicDeliveryStatus   = "delivery.status"
)

type OrderCreatedEvent struct {
	EventID   string  `json:"event_id"`
	OrderID   string  `json:"order_id"`
	UserID    string  `json:"user_id"`
	Address   string  `json:"address"`
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	CreatedAt string  `json:"created_at"` // RFC3339
}

type OrderUpdatedEvent struct {
	EventID   string `json:"event_id"`
	OrderID   string `json:"order_id"`
	UserID    string `json:"user_id"`
	OldStatus string `json:"old_status"`
	NewStatus string `json:"new_status"`
	UpdatedAt string `json:"updated_at"` // RFC3339
}

type DeliveryAssignedEvent struct {
	EventID    string `json:"event_id"`
	DeliveryID string `json:"delivery_id"`
	OrderID    string `json:"order_id"`
	UserID     string `json:"user_id"`
	CourierID  string `json:"courier_id"`
	ZoneID     string `json:"zone_id"`
	ETA        string `json:"eta"`        // RFC3339
	AssignedAt string `json:"assigned_at"` // RFC3339
}

type DeliveryStatusEvent struct {
	EventID    string `json:"event_id"`
	DeliveryID string `json:"delivery_id"`
	OrderID    string `json:"order_id"`
	CourierID  string `json:"courier_id"`
	OldStatus  string `json:"old_status"`
	NewStatus  string `json:"new_status"`
	UpdatedAt  string `json:"updated_at"` // RFC3339
}
