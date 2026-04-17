package service_test

import (
	"context"
	"testing"

	pkgerrors "github.com/mozgovojnikita/delivery-tracker/pkg/errors"
	"github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/domain"
	"github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/service"
)

// syncRepoHarness builds a mockOrderRepository whose GetByID returns an order
// reflecting a closure-mutated currentStatus, and whose UpdateStatus records the
// requested target and advances currentStatus so the forward walk progresses.
func syncRepoHarness(initial domain.OrderStatus) (*mockOrderRepository, *[]domain.OrderStatus) {
	current := initial
	var calls []domain.OrderStatus
	repo := &mockOrderRepository{
		getByIDFn: func(_ context.Context, id string) (*domain.Order, error) {
			return &domain.Order{ID: id, UserID: "user-1", Status: current}, nil
		},
		updateStatusFn: func(_ context.Context, id string, status domain.OrderStatus, _ string) (*domain.Order, error) {
			calls = append(calls, status)
			current = status
			return &domain.Order{ID: id, UserID: "user-1", Status: status}, nil
		},
	}
	return repo, &calls
}

func TestSyncOrderToStatus_CreatedToAssigned_WalksLegalChain(t *testing.T) {
	repo, calls := syncRepoHarness(domain.StatusCreated)
	svc := service.NewOrderService(repo, nil)

	if err := svc.SyncOrderToStatus(context.Background(), "order-1", domain.StatusAssigned); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	want := []domain.OrderStatus{domain.StatusConfirmed, domain.StatusAssigned}
	if len(*calls) != len(want) {
		t.Fatalf("expected %d updateStatus calls %v, got %d %v", len(want), want, len(*calls), *calls)
	}
	for i, w := range want {
		if (*calls)[i] != w {
			t.Errorf("step %d: expected %s, got %s", i, w, (*calls)[i])
		}
	}
}

func TestSyncOrderToStatus_CreatedToPickedUp_WalksThreeSteps(t *testing.T) {
	repo, calls := syncRepoHarness(domain.StatusCreated)
	svc := service.NewOrderService(repo, nil)

	if err := svc.SyncOrderToStatus(context.Background(), "order-1", domain.StatusPickedUp); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	want := []domain.OrderStatus{domain.StatusConfirmed, domain.StatusAssigned, domain.StatusPickedUp}
	if len(*calls) != len(want) {
		t.Fatalf("expected %d updateStatus calls %v, got %d %v", len(want), want, len(*calls), *calls)
	}
	for i, w := range want {
		if (*calls)[i] != w {
			t.Errorf("step %d: expected %s, got %s", i, w, (*calls)[i])
		}
	}
}

func TestSyncOrderToStatus_Idempotent_AlreadyAtTarget_NoOp(t *testing.T) {
	repo, calls := syncRepoHarness(domain.StatusAssigned)
	svc := service.NewOrderService(repo, nil)

	if err := svc.SyncOrderToStatus(context.Background(), "order-1", domain.StatusAssigned); err != nil {
		t.Fatalf("expected nil error on idempotent re-delivery, got: %v", err)
	}
	if len(*calls) != 0 {
		t.Errorf("expected zero updateStatus calls (idempotent), got %d %v", len(*calls), *calls)
	}
}

func TestSyncOrderToStatus_StaleOutOfOrder_NoRegression_NoOp(t *testing.T) {
	repo, calls := syncRepoHarness(domain.StatusInTransit)
	svc := service.NewOrderService(repo, nil)

	if err := svc.SyncOrderToStatus(context.Background(), "order-1", domain.StatusAssigned); err != nil {
		t.Fatalf("expected nil error on stale/out-of-order event, got: %v", err)
	}
	if len(*calls) != 0 {
		t.Errorf("expected zero updateStatus calls (no regression), got %d %v", len(*calls), *calls)
	}
}

func TestSyncOrderToStatus_TerminalState_NoOp(t *testing.T) {
	repo, calls := syncRepoHarness(domain.StatusCancelled)
	svc := service.NewOrderService(repo, nil)

	if err := svc.SyncOrderToStatus(context.Background(), "order-1", domain.StatusAssigned); err != nil {
		t.Fatalf("expected nil error on terminal-state order, got: %v", err)
	}
	if len(*calls) != 0 {
		t.Errorf("expected zero updateStatus calls (terminal), got %d %v", len(*calls), *calls)
	}
}

func TestSyncOrderToStatus_OrderNotFound_NonRetryableNoOp(t *testing.T) {
	var updateCalls int
	repo := &mockOrderRepository{
		getByIDFn: func(_ context.Context, _ string) (*domain.Order, error) {
			return nil, pkgerrors.ErrNotFound
		},
		updateStatusFn: func(_ context.Context, id string, status domain.OrderStatus, _ string) (*domain.Order, error) {
			updateCalls++
			return &domain.Order{ID: id, Status: status}, nil
		},
	}
	svc := service.NewOrderService(repo, nil)

	if err := svc.SyncOrderToStatus(context.Background(), "missing", domain.StatusAssigned); err != nil {
		t.Fatalf("expected nil error (non-retryable no-op) when order not found, got: %v", err)
	}
	if updateCalls != 0 {
		t.Errorf("expected zero updateStatus calls when order not found, got %d", updateCalls)
	}
}

func TestSyncOrderToStatus_OffChainTarget_NoOp(t *testing.T) {
	// failed is a valid OrderStatus but NOT on the linear syncForwardChain.
	repo, calls := syncRepoHarness(domain.StatusInTransit)
	svc := service.NewOrderService(repo, nil)

	if err := svc.SyncOrderToStatus(context.Background(), "order-1", domain.StatusFailed); err != nil {
		t.Fatalf("expected nil error for off-chain target, got: %v", err)
	}
	if len(*calls) != 0 {
		t.Errorf("expected zero updateStatus calls for off-chain target, got %d %v", len(*calls), *calls)
	}
}
