package grpc_test

import (
	"context"
	"encoding/json"
	"testing"

	orderv1 "github.com/mozgovojnikita/delivery-tracker/gen/order/v1"
	pkgerrors "github.com/mozgovojnikita/delivery-tracker/pkg/errors"
	"github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/domain"
	grpchandler "github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/handler/grpc"
	"github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/repository"
	"github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/service"
)

// stubOrderRepoForBulk is a minimal stub that implements repository.OrderRepository
// with only GetByIDs wired; other methods are no-ops.
type stubOrderRepoForBulk struct {
	repository.OrderRepository // embed for non-implemented methods
	getByIDsFn func(ctx context.Context, ids []string) ([]*domain.Order, error)
}

func (s *stubOrderRepoForBulk) GetByIDs(ctx context.Context, ids []string) ([]*domain.Order, error) {
	if s.getByIDsFn != nil {
		return s.getByIDsFn(ctx, ids)
	}
	return nil, nil
}
func (s *stubOrderRepoForBulk) Create(ctx context.Context, input domain.CreateOrderInput) (*domain.Order, error) {
	return nil, nil
}
func (s *stubOrderRepoForBulk) GetByID(ctx context.Context, id string) (*domain.Order, error) {
	return nil, nil
}
func (s *stubOrderRepoForBulk) ListByUserID(ctx context.Context, userID string, status *domain.OrderStatus, page, pageSize int) ([]*domain.Order, int, error) {
	return nil, 0, nil
}
func (s *stubOrderRepoForBulk) UpdateStatus(ctx context.Context, id string, status domain.OrderStatus, cancelReason string) (*domain.Order, error) {
	return nil, nil
}

// stubUserRepo is a minimal stub implementing repository.UserRepository for tests.
type stubUserRepo struct {
	getByIDFn func(ctx context.Context, id string) (*domain.User, error)
}

func (s *stubUserRepo) Create(_ context.Context, _, _, _, _, _ string, _ domain.Role) (*domain.User, error) {
	return nil, pkgerrors.ErrNotFound
}
func (s *stubUserRepo) GetByEmail(_ context.Context, _ string) (*domain.User, error) {
	return nil, pkgerrors.ErrNotFound
}
func (s *stubUserRepo) GetByID(ctx context.Context, id string) (*domain.User, error) {
	if s.getByIDFn != nil {
		return s.getByIDFn(ctx, id)
	}
	return nil, pkgerrors.ErrNotFound
}

// makeItems builds JSON items bytes for test orders.
func makeItems(price float64, qty int32) json.RawMessage {
	b, _ := json.Marshal([]map[string]interface{}{
		{"name": "Item", "price": price, "quantity": qty},
	})
	return b
}

// makeNamedItems builds JSON items bytes for test orders with explicit name.
func makeNamedItems(name string, price float64, qty int32) json.RawMessage {
	b, _ := json.Marshal([]map[string]interface{}{
		{"name": name, "price": price, "quantity": qty},
	})
	return b
}

// TestGetOrdersByIDs_ReturnsSlimPreviews tests that the handler converts domain orders
// to OrderPreviewSlim with correct total_price, items_count, AND items slice (CRDR-07).
func TestGetOrdersByIDs_ReturnsSlimPreviews(t *testing.T) {
	repo := &stubOrderRepoForBulk{
		getByIDsFn: func(ctx context.Context, ids []string) ([]*domain.Order, error) {
			return []*domain.Order{
				{
					ID:              "o-1",
					DeliveryAddress: "Main St 1",
					Items:           makeNamedItems("Пицца", 100.0, 2), // total=200, count=2
				},
				{
					ID:              "o-2",
					DeliveryAddress: "Oak Ave 5",
					Items:           makeItems(50.0, 3), // total=150, count=3
				},
			}, nil
		},
	}

	svc := service.NewOrderService(repo, nil)
	h := grpchandler.NewOrderHandler(svc, nil)

	resp, err := h.GetOrdersByIDs(context.Background(), &orderv1.GetOrdersByIDsRequest{
		OrderIds: []string{"o-1", "o-2"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Orders) != 2 {
		t.Fatalf("expected 2 previews, got %d", len(resp.Orders))
	}

	// Verify first order
	p1 := resp.Orders[0]
	if p1.OrderId != "o-1" {
		t.Errorf("order[0].order_id: want o-1, got %q", p1.OrderId)
	}
	if p1.TotalPrice != 200.0 {
		t.Errorf("order[0].total_price: want 200.0, got %v", p1.TotalPrice)
	}
	if p1.ItemsCount != 2 {
		t.Errorf("order[0].items_count: want 2, got %d", p1.ItemsCount)
	}
	if p1.DeliveryAddress != "Main St 1" {
		t.Errorf("order[0].delivery_address: want 'Main St 1', got %q", p1.DeliveryAddress)
	}
	// CRDR-07: assert items populated
	if len(p1.GetItems()) == 0 {
		t.Fatal("order[0]: expected items to be populated (CRDR-07)")
	}
	item0 := p1.GetItems()[0]
	if item0.GetName() != "Пицца" {
		t.Errorf("order[0].items[0].name: want Пицца, got %q", item0.GetName())
	}
	if item0.GetQuantity() != 2 {
		t.Errorf("order[0].items[0].quantity: want 2, got %d", item0.GetQuantity())
	}
	if item0.GetPrice() != 100.0 {
		t.Errorf("order[0].items[0].price: want 100.0, got %v", item0.GetPrice())
	}

	// Verify second order
	p2 := resp.Orders[1]
	if p2.TotalPrice != 150.0 {
		t.Errorf("order[1].total_price: want 150.0, got %v", p2.TotalPrice)
	}
	if p2.ItemsCount != 3 {
		t.Errorf("order[1].items_count: want 3, got %d", p2.ItemsCount)
	}
}

// TestGetOrdersByIDs_EmptyRequest returns empty response without calling repo.
func TestGetOrdersByIDs_EmptyRequest(t *testing.T) {
	callCount := 0
	repo := &stubOrderRepoForBulk{
		getByIDsFn: func(ctx context.Context, ids []string) ([]*domain.Order, error) {
			callCount++
			return nil, nil
		},
	}

	svc := service.NewOrderService(repo, nil)
	h := grpchandler.NewOrderHandler(svc, nil)

	resp, err := h.GetOrdersByIDs(context.Background(), &orderv1.GetOrdersByIDsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Orders) != 0 {
		t.Errorf("expected empty orders, got %d", len(resp.Orders))
	}
	if callCount != 0 {
		t.Errorf("expected repo not called for empty request, called %d times", callCount)
	}
}

// TestGetOrdersByIDs_MaxLimitEnforced returns InvalidArgument for >100 IDs.
func TestGetOrdersByIDs_MaxLimitEnforced(t *testing.T) {
	repo := &stubOrderRepoForBulk{}
	svc := service.NewOrderService(repo, nil)
	h := grpchandler.NewOrderHandler(svc, nil)

	ids := make([]string, 101)
	for i := range ids {
		ids[i] = "id"
	}

	_, err := h.GetOrdersByIDs(context.Background(), &orderv1.GetOrdersByIDsRequest{OrderIds: ids})
	if err == nil {
		t.Fatal("expected error for >100 IDs")
	}
}

// TestGetUsersByIDs_ReturnsUserSlim verifies the handler returns UserSlim from UserRepository.GetByID (RATE-05).
func TestGetUsersByIDs_ReturnsUserSlim(t *testing.T) {
	const (
		wantID        = "user-courier-1"
		wantFirstName = "Иван"
		wantLastName  = "Иванов"
		wantRole      = "courier"
	)

	userRepo := &stubUserRepo{
		getByIDFn: func(_ context.Context, id string) (*domain.User, error) {
			if id == wantID {
				return &domain.User{
					ID:        wantID,
					FirstName: wantFirstName,
					LastName:  wantLastName,
					Role:      domain.RoleCourier,
				}, nil
			}
			return nil, pkgerrors.ErrNotFound
		},
	}

	svc := service.NewOrderService(&stubOrderRepoForBulk{}, nil)
	h := grpchandler.NewOrderHandler(svc, userRepo)

	resp, err := h.GetUsersByIDs(context.Background(), &orderv1.GetUsersByIDsRequest{
		UserIds: []string{wantID},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetUsers()) != 1 {
		t.Fatalf("expected 1 user, got %d", len(resp.GetUsers()))
	}
	u := resp.GetUsers()[0]
	if u.GetId() != wantID {
		t.Errorf("id: want %q, got %q", wantID, u.GetId())
	}
	if u.GetFirstName() != wantFirstName {
		t.Errorf("first_name: want %q, got %q", wantFirstName, u.GetFirstName())
	}
	if u.GetLastName() != wantLastName {
		t.Errorf("last_name: want %q, got %q", wantLastName, u.GetLastName())
	}
	if u.GetRole() != wantRole {
		t.Errorf("role: want %q, got %q", wantRole, u.GetRole())
	}
}

// TestGetUsersByIDs_EmptyRequest verifies empty user_ids returns empty response with no error.
func TestGetUsersByIDs_EmptyRequest(t *testing.T) {
	callCount := 0
	userRepo := &stubUserRepo{
		getByIDFn: func(_ context.Context, _ string) (*domain.User, error) {
			callCount++
			return nil, pkgerrors.ErrNotFound
		},
	}

	svc := service.NewOrderService(&stubOrderRepoForBulk{}, nil)
	h := grpchandler.NewOrderHandler(svc, userRepo)

	resp, err := h.GetUsersByIDs(context.Background(), &orderv1.GetUsersByIDsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetUsers()) != 0 {
		t.Errorf("expected empty users, got %d", len(resp.GetUsers()))
	}
	if callCount != 0 {
		t.Errorf("expected repo not called for empty request, called %d times", callCount)
	}
}
