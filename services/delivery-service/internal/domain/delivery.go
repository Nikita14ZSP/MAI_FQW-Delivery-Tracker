package domain

import (
	"errors"
	"fmt"
	"time"

	pkgerrors "github.com/mozgovojnikita/delivery-tracker/pkg/errors"
)

// Typed sentinel errors for courier status and delivery acceptance flows.
// Mapped to gRPC codes.Aborted (→ HTTP 409) in toGRPCError.
var (
	ErrInvalidStatusTransition = errors.New("invalid_status_transition")
	ErrActiveDeliveryExists    = errors.New("active_delivery_exists")
	ErrAlreadyTaken            = errors.New("already_taken")        // for plan 06-04
	ErrMaxActiveReached        = errors.New("max_active_reached")   // for plan 06-04
	ErrCourierOffline          = errors.New("courier_offline")      // for plan 06-04
)

// DeliveryStatus represents the current state of a delivery in the lifecycle.
type DeliveryStatus string

const (
	StatusPending   DeliveryStatus = "pending"    // initial, no courier found
	StatusAssigned  DeliveryStatus = "assigned"   // courier assigned
	StatusPickedUp  DeliveryStatus = "picked_up"
	StatusInTransit DeliveryStatus = "in_transit"
	StatusDelivered DeliveryStatus = "delivered"
	StatusFailed    DeliveryStatus = "failed"

	// MaxActiveDeliveries is the maximum number of active deliveries a courier can have (D-05).
	MaxActiveDeliveries = 5
)

// allowedTransitions defines the valid state transitions for the delivery state machine.
var allowedTransitions = map[DeliveryStatus][]DeliveryStatus{
	StatusPending:   {StatusAssigned},
	StatusAssigned:  {StatusPickedUp, StatusFailed},
	StatusPickedUp:  {StatusInTransit},
	StatusInTransit: {StatusDelivered, StatusFailed},
	// Terminal states: delivered, failed have no outgoing transitions.
	StatusDelivered: {},
	StatusFailed:    {},
}

// ValidateTransition checks whether transitioning from to next status is allowed.
// Returns ErrInvalidInput if the transition is not in the state machine.
func ValidateTransition(from, to DeliveryStatus) error {
	allowed, exists := allowedTransitions[from]
	if !exists || !containsStatus(allowed, to) {
		return fmt.Errorf("transition %s -> %s: %w", from, to, pkgerrors.ErrInvalidInput)
	}
	return nil
}

// containsStatus returns true if target is in the statuses slice.
func containsStatus(statuses []DeliveryStatus, target DeliveryStatus) bool {
	for _, s := range statuses {
		if s == target {
			return true
		}
	}
	return false
}

// Delivery represents a delivery assignment linking an order to a courier.
type Delivery struct {
	ID                string
	OrderID           string
	CourierID         string
	Status            DeliveryStatus
	ZoneID            string
	EstimatedDelivery time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// DeliveryZone represents a geographic zone for deliveries.
type DeliveryZone struct {
	ID             string
	Name           string
	PolygonGeoJSON string // GeoJSON polygon text
	CreatedAt      time.Time
}

// Courier represents a courier in the delivery system.
type Courier struct {
	ID        string
	Status    string // "available", "busy", "offline"
	UpdatedAt time.Time
}

// CourierCandidate holds information about a potential courier for assignment.
type CourierCandidate struct {
	CourierID      string
	DistanceMeters float64
}

// AvailableOrderRow is the repository-level result row for ListAvailableForCourier.
// DeliveryAddress and TotalPrice/ItemsCount are filled by service layer via cross-service OrderClient.
type AvailableOrderRow struct {
	OrderID    string
	DeliveryID string
	ZoneName   string
	CreatedAt  time.Time
}

// OrderItemEnriched is a lightweight item summary within AvailableOrderEnriched (CRDR-07).
type OrderItemEnriched struct {
	Name     string
	Quantity int32
	Price    float64
}

// AvailableOrderEnriched is the service-level result for ListAvailableOrders,
// combining repository data with enrichment from order-service.
type AvailableOrderEnriched struct {
	OrderID         string
	DeliveryID      string
	DeliveryAddress string
	ZoneName        string
	TotalPrice      float64
	ItemsCount      int32
	CreatedAt       time.Time
	Items           []OrderItemEnriched // CRDR-07: order composition before accept
}
