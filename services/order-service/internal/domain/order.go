package domain

import (
	"encoding/json"
	"fmt"
	"time"

	pkgerrors "github.com/mozgovojnikita/delivery-tracker/pkg/errors"
)

// OrderStatus represents the current state of an order in the delivery lifecycle.
type OrderStatus string

const (
	StatusCreated   OrderStatus = "created"
	StatusConfirmed OrderStatus = "confirmed"
	StatusAssigned  OrderStatus = "assigned"
	StatusPickedUp  OrderStatus = "picked_up"
	StatusInTransit OrderStatus = "in_transit"
	StatusDelivered OrderStatus = "delivered"
	StatusCancelled OrderStatus = "cancelled"
	StatusFailed    OrderStatus = "failed"
	StatusReturned  OrderStatus = "returned"
)

// Order represents a delivery order.
type Order struct {
	ID              string
	UserID          string
	Status          OrderStatus
	DeliveryAddress string
	DeliveryLat     float64
	DeliveryLng     float64
	Items           json.RawMessage
	ContactPhone    string
	PaymentMethod   string
	CancelReason    string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// CreateOrderInput holds the input data for creating a new order.
type CreateOrderInput struct {
	UserID          string
	DeliveryAddress string
	DeliveryLat     float64
	DeliveryLng     float64
	Items           json.RawMessage
	ContactPhone    string
	PaymentMethod   string
}

// allowedTransitions defines the valid state transitions for the order state machine.
var allowedTransitions = map[OrderStatus][]OrderStatus{
	StatusCreated:   {StatusConfirmed, StatusCancelled},
	StatusConfirmed: {StatusAssigned, StatusCancelled},
	StatusAssigned:  {StatusPickedUp, StatusCancelled},
	StatusPickedUp:  {StatusInTransit},
	StatusInTransit: {StatusDelivered, StatusFailed},
	StatusFailed:    {StatusReturned},
	// Terminal states: delivered, cancelled, returned have no outgoing transitions.
	StatusDelivered: {},
	StatusCancelled: {},
	StatusReturned:  {},
}

// roleTransitionPermissions defines which transitions each role is allowed to perform.
// Admin role is handled separately (allowed to perform any valid transition).
var roleTransitionPermissions = map[Role]map[OrderStatus][]OrderStatus{
	RoleUser: {
		StatusCreated:   {StatusCancelled},
		StatusConfirmed: {StatusCancelled},
		StatusAssigned:  {StatusCancelled},
	},
	RoleCourier: {
		StatusAssigned:  {StatusPickedUp},
		StatusPickedUp:  {StatusInTransit},
		StatusInTransit: {StatusDelivered, StatusFailed},
		StatusFailed:    {StatusReturned},
	},
}

// ValidateTransition checks whether transitioning from current to next status is allowed
// for the given role. Returns ErrInvalidInput if the transition is not in the state machine,
// ErrForbidden if the role is not permitted to perform the transition.
func ValidateTransition(current, next OrderStatus, role Role) error {
	// Check if transition is structurally valid in the state machine.
	allowed, exists := allowedTransitions[current]
	if !exists || !containsStatus(allowed, next) {
		return fmt.Errorf("transition %s -> %s: %w", current, next, pkgerrors.ErrInvalidInput)
	}

	// Admin can perform any valid transition.
	if role == RoleAdmin {
		return nil
	}

	// Check role-based permission.
	roleAllowed, roleExists := roleTransitionPermissions[role]
	if !roleExists {
		return fmt.Errorf("role %s cannot transition %s -> %s: %w", role, current, next, pkgerrors.ErrForbidden)
	}
	permittedTargets, hasFrom := roleAllowed[current]
	if !hasFrom || !containsStatus(permittedTargets, next) {
		return fmt.Errorf("role %s cannot transition %s -> %s: %w", role, current, next, pkgerrors.ErrForbidden)
	}

	return nil
}

// containsStatus returns true if target is in the statuses slice.
func containsStatus(statuses []OrderStatus, target OrderStatus) bool {
	for _, s := range statuses {
		if s == target {
			return true
		}
	}
	return false
}
