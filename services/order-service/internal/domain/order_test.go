package domain_test

import (
	stderrors "errors"
	"testing"

	pkgerrors "github.com/mozgovojnikita/delivery-tracker/pkg/errors"
	"github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/domain"
)

// TestValidateTransition_ValidForward: created->confirmed returns nil
func TestValidateTransition_ValidForward(t *testing.T) {
	err := domain.ValidateTransition(domain.StatusCreated, domain.StatusConfirmed, domain.RoleAdmin)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

// TestValidateTransition_InvalidBackward: delivered->created returns ErrInvalidInput
func TestValidateTransition_InvalidBackward(t *testing.T) {
	err := domain.ValidateTransition(domain.StatusDelivered, domain.StatusCreated, domain.RoleAdmin)
	if !stderrors.Is(err, pkgerrors.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

// TestValidateTransition_TerminalState: delivered->anything returns ErrInvalidInput
func TestValidateTransition_TerminalState(t *testing.T) {
	err := domain.ValidateTransition(domain.StatusDelivered, domain.StatusConfirmed, domain.RoleAdmin)
	if !stderrors.Is(err, pkgerrors.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for terminal state, got %v", err)
	}
}

// TestValidateTransition_RoleUser_CanCancel: user + created->cancelled returns nil
func TestValidateTransition_RoleUser_CanCancel(t *testing.T) {
	err := domain.ValidateTransition(domain.StatusCreated, domain.StatusCancelled, domain.RoleUser)
	if err != nil {
		t.Errorf("expected nil for user cancel, got %v", err)
	}
}

// TestValidateTransition_RoleUser_CannotConfirm: user + created->confirmed returns ErrForbidden
func TestValidateTransition_RoleUser_CannotConfirm(t *testing.T) {
	err := domain.ValidateTransition(domain.StatusCreated, domain.StatusConfirmed, domain.RoleUser)
	if !stderrors.Is(err, pkgerrors.ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

// TestValidateTransition_RoleCourier_CanPickUp: courier + assigned->picked_up returns nil
func TestValidateTransition_RoleCourier_CanPickUp(t *testing.T) {
	err := domain.ValidateTransition(domain.StatusAssigned, domain.StatusPickedUp, domain.RoleCourier)
	if err != nil {
		t.Errorf("expected nil for courier pickup, got %v", err)
	}
}

// TestValidateTransition_RoleCourier_CannotCancel: courier + created->cancelled returns ErrForbidden
func TestValidateTransition_RoleCourier_CannotCancel(t *testing.T) {
	err := domain.ValidateTransition(domain.StatusCreated, domain.StatusCancelled, domain.RoleCourier)
	if !stderrors.Is(err, pkgerrors.ErrForbidden) {
		t.Errorf("expected ErrForbidden for courier cancel, got %v", err)
	}
}

// TestValidateTransition_RoleAdmin_CanDoAnything: admin + any valid transition returns nil
func TestValidateTransition_RoleAdmin_CanDoAnything(t *testing.T) {
	validTransitions := []struct {
		from domain.OrderStatus
		to   domain.OrderStatus
	}{
		{domain.StatusCreated, domain.StatusConfirmed},
		{domain.StatusCreated, domain.StatusCancelled},
		{domain.StatusConfirmed, domain.StatusAssigned},
		{domain.StatusConfirmed, domain.StatusCancelled},
		{domain.StatusAssigned, domain.StatusPickedUp},
		{domain.StatusAssigned, domain.StatusCancelled},
		{domain.StatusPickedUp, domain.StatusInTransit},
		{domain.StatusInTransit, domain.StatusDelivered},
		{domain.StatusInTransit, domain.StatusFailed},
	}
	for _, tc := range validTransitions {
		err := domain.ValidateTransition(tc.from, tc.to, domain.RoleAdmin)
		if err != nil {
			t.Errorf("admin: expected nil for %s->%s, got %v", tc.from, tc.to, err)
		}
	}
}

// TestHashPassword: hashing produces bcrypt output starting with "$2a$"
func TestHashPassword(t *testing.T) {
	hash, err := domain.HashPassword("testpassword123")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if len(hash) < 4 || hash[:4] != "$2a$" {
		t.Errorf("expected bcrypt hash starting with $2a$, got: %s", hash)
	}
}

// TestCheckPassword: correct password returns nil, wrong password returns error
func TestCheckPassword(t *testing.T) {
	hash, err := domain.HashPassword("correctpassword")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if err := domain.CheckPassword(hash, "correctpassword"); err != nil {
		t.Errorf("expected nil for correct password, got %v", err)
	}

	if err := domain.CheckPassword(hash, "wrongpassword"); err == nil {
		t.Error("expected error for wrong password, got nil")
	}
}
