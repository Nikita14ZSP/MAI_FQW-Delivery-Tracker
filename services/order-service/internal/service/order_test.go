package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	deliveryv1 "github.com/mozgovojnikita/delivery-tracker/gen/delivery/v1"
	pkgerrors "github.com/mozgovojnikita/delivery-tracker/pkg/errors"
	pkgkafka "github.com/mozgovojnikita/delivery-tracker/pkg/kafka"
	"github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/domain"
	"github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/service"
)

// mockOrderRepository is a manual mock implementing repository.OrderRepository.
type mockOrderRepository struct {
	createFn       func(ctx context.Context, input domain.CreateOrderInput) (*domain.Order, error)
	getByIDFn      func(ctx context.Context, id string) (*domain.Order, error)
	getByIDsFn     func(ctx context.Context, ids []string) ([]*domain.Order, error)
	listByUserIDFn func(ctx context.Context, userID string, status *domain.OrderStatus, page, pageSize int) ([]*domain.Order, int, error)
	updateStatusFn func(ctx context.Context, id string, status domain.OrderStatus, cancelReason string) (*domain.Order, error)
}

func (m *mockOrderRepository) Create(ctx context.Context, input domain.CreateOrderInput) (*domain.Order, error) {
	if m.createFn != nil {
		return m.createFn(ctx, input)
	}
	return nil, nil
}

func (m *mockOrderRepository) GetByID(ctx context.Context, id string) (*domain.Order, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, pkgerrors.ErrNotFound
}

func (m *mockOrderRepository) GetByIDs(ctx context.Context, ids []string) ([]*domain.Order, error) {
	if m.getByIDsFn != nil {
		return m.getByIDsFn(ctx, ids)
	}
	return nil, nil
}

func (m *mockOrderRepository) ListByUserID(ctx context.Context, userID string, status *domain.OrderStatus, page, pageSize int) ([]*domain.Order, int, error) {
	if m.listByUserIDFn != nil {
		return m.listByUserIDFn(ctx, userID, status, page, pageSize)
	}
	return nil, 0, nil
}

func (m *mockOrderRepository) UpdateStatus(ctx context.Context, id string, status domain.OrderStatus, cancelReason string) (*domain.Order, error) {
	if m.updateStatusFn != nil {
		return m.updateStatusFn(ctx, id, status, cancelReason)
	}
	return nil, nil
}

// mockPublisher is a manual mock implementing pkgkafka.EventPublisher.
type mockPublisher struct {
	publishFn    func(ctx context.Context, topic string, key string, value []byte) error
	publishCalls []publishCall
	err          error
}

type publishCall struct {
	topic string
	key   string
}

func (m *mockPublisher) Publish(ctx context.Context, topic string, key string, value []byte) error {
	m.publishCalls = append(m.publishCalls, publishCall{topic: topic, key: key})
	if m.publishFn != nil {
		return m.publishFn(ctx, topic, key, value)
	}
	return m.err
}

func (m *mockPublisher) Close() error {
	return nil
}

// Ensure mockPublisher implements EventPublisher.
var _ pkgkafka.EventPublisher = (*mockPublisher)(nil)

func TestOrderService_CreateOrder_Success(t *testing.T) {
	repo := &mockOrderRepository{
		createFn: func(ctx context.Context, input domain.CreateOrderInput) (*domain.Order, error) {
			return &domain.Order{
				ID:              "order-1",
				UserID:          input.UserID,
				Status:          domain.StatusCreated,
				DeliveryAddress: input.DeliveryAddress,
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
			}, nil
		},
	}
	svc := service.NewOrderService(repo, nil)

	order, err := svc.CreateOrder(context.Background(), domain.CreateOrderInput{
		UserID:          "user-1",
		DeliveryAddress: "123 Main St",
		ContactPhone:    "+7-999-000-0000",
		PaymentMethod:   "card",
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if order == nil {
		t.Fatal("expected order, got nil")
	}
	if order.Status != domain.StatusCreated {
		t.Errorf("expected status created, got %s", order.Status)
	}
}

func TestOrderService_CreateOrder_PublishesOrderCreated(t *testing.T) {
	repo := &mockOrderRepository{
		createFn: func(ctx context.Context, input domain.CreateOrderInput) (*domain.Order, error) {
			return &domain.Order{
				ID:              "order-1",
				UserID:          input.UserID,
				Status:          domain.StatusCreated,
				DeliveryAddress: input.DeliveryAddress,
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
			}, nil
		},
	}
	pub := &mockPublisher{}
	svc := service.NewOrderService(repo, pub)

	order, err := svc.CreateOrder(context.Background(), domain.CreateOrderInput{
		UserID:          "user-1",
		DeliveryAddress: "123 Main St",
		ContactPhone:    "+7-999-000-0000",
		PaymentMethod:   "card",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if order == nil {
		t.Fatal("expected order, got nil")
	}

	// Exactly one publish call, to orders.created, keyed by order ID.
	if len(pub.publishCalls) != 1 {
		t.Fatalf("expected exactly 1 publish call, got %d", len(pub.publishCalls))
	}

	topicsPublished := make(map[string]bool)
	for _, call := range pub.publishCalls {
		topicsPublished[call.topic] = true
	}
	if !topicsPublished[pkgkafka.TopicOrdersCreated] {
		t.Errorf("expected publish to %s, but it was not called", pkgkafka.TopicOrdersCreated)
	}
	if topicsPublished[pkgkafka.TopicOrdersUpdated] {
		t.Errorf("did not expect publish to %s on order creation", pkgkafka.TopicOrdersUpdated)
	}
	if pub.publishCalls[0].key != order.ID {
		t.Errorf("expected publish key to be order ID %q, got %q", order.ID, pub.publishCalls[0].key)
	}
}

func TestOrderService_CreateOrder_PublishFailureDoesNotFail(t *testing.T) {
	repo := &mockOrderRepository{
		createFn: func(ctx context.Context, input domain.CreateOrderInput) (*domain.Order, error) {
			return &domain.Order{
				ID:              "order-1",
				UserID:          input.UserID,
				Status:          domain.StatusCreated,
				DeliveryAddress: input.DeliveryAddress,
			}, nil
		},
	}
	// Publisher that always returns an error (Kafka unavailable).
	pub := &mockPublisher{err: errors.New("kafka unavailable")}
	svc := service.NewOrderService(repo, pub)

	order, err := svc.CreateOrder(context.Background(), domain.CreateOrderInput{
		UserID:          "user-1",
		DeliveryAddress: "123 Main St",
	})
	if err != nil {
		t.Fatalf("publish failure should not fail CreateOrder, got: %v", err)
	}
	if order == nil {
		t.Fatal("expected order to be returned even when publish fails")
	}
}

func TestOrderService_CreateOrder_NilPublisherOK(t *testing.T) {
	repo := &mockOrderRepository{
		createFn: func(ctx context.Context, input domain.CreateOrderInput) (*domain.Order, error) {
			return &domain.Order{
				ID:              "order-1",
				UserID:          input.UserID,
				Status:          domain.StatusCreated,
				DeliveryAddress: input.DeliveryAddress,
			}, nil
		},
	}
	// nil publisher must not panic (nil-guard).
	svc := service.NewOrderService(repo, nil)

	order, err := svc.CreateOrder(context.Background(), domain.CreateOrderInput{
		UserID:          "user-1",
		DeliveryAddress: "123 Main St",
	})
	if err != nil {
		t.Fatalf("expected no error with nil publisher, got: %v", err)
	}
	if order == nil {
		t.Fatal("expected order, got nil")
	}
}

func TestOrderService_GetOrder_Success(t *testing.T) {
	expected := &domain.Order{
		ID:     "order-1",
		UserID: "user-1",
		Status: domain.StatusCreated,
	}
	repo := &mockOrderRepository{
		getByIDFn: func(ctx context.Context, id string) (*domain.Order, error) {
			return expected, nil
		},
	}
	svc := service.NewOrderService(repo, nil)

	order, err := svc.GetOrder(context.Background(), "order-1")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if order.ID != "order-1" {
		t.Errorf("expected order ID order-1, got %s", order.ID)
	}
}

func TestOrderService_GetOrder_NotFound(t *testing.T) {
	repo := &mockOrderRepository{
		getByIDFn: func(ctx context.Context, id string) (*domain.Order, error) {
			return nil, pkgerrors.ErrNotFound
		},
	}
	svc := service.NewOrderService(repo, nil)

	_, err := svc.GetOrder(context.Background(), "nonexistent")
	if !errors.Is(err, pkgerrors.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestOrderService_ListOrders_WithFilter(t *testing.T) {
	var capturedStatus *domain.OrderStatus
	var capturedPage, capturedPageSize int

	status := domain.StatusCreated
	repo := &mockOrderRepository{
		listByUserIDFn: func(ctx context.Context, userID string, s *domain.OrderStatus, page, pageSize int) ([]*domain.Order, int, error) {
			capturedStatus = s
			capturedPage = page
			capturedPageSize = pageSize
			return []*domain.Order{}, 0, nil
		},
	}
	svc := service.NewOrderService(repo, nil)

	_, _, err := svc.ListOrders(context.Background(), "user-1", &status, 1, 10)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if capturedStatus == nil || *capturedStatus != domain.StatusCreated {
		t.Error("expected status filter to be passed to repository")
	}
	if capturedPage != 1 || capturedPageSize != 10 {
		t.Errorf("expected page=1 pageSize=10, got page=%d pageSize=%d", capturedPage, capturedPageSize)
	}
}

func TestOrderService_CancelOrder_FromCreated(t *testing.T) {
	repo := &mockOrderRepository{
		getByIDFn: func(ctx context.Context, id string) (*domain.Order, error) {
			return &domain.Order{
				ID:     id,
				UserID: "user-1",
				Status: domain.StatusCreated,
			}, nil
		},
		updateStatusFn: func(ctx context.Context, id string, status domain.OrderStatus, cancelReason string) (*domain.Order, error) {
			return &domain.Order{
				ID:           id,
				Status:       status,
				CancelReason: cancelReason,
			}, nil
		},
	}
	svc := service.NewOrderService(repo, nil)

	order, err := svc.CancelOrder(context.Background(), "order-1", "customer request", domain.RoleUser)
	if err != nil {
		t.Fatalf("expected no error cancelling from created status, got: %v", err)
	}
	if order.Status != domain.StatusCancelled {
		t.Errorf("expected status cancelled, got %s", order.Status)
	}
}

func TestOrderService_CancelOrder_FromInTransit(t *testing.T) {
	repo := &mockOrderRepository{
		getByIDFn: func(ctx context.Context, id string) (*domain.Order, error) {
			return &domain.Order{
				ID:     id,
				UserID: "user-1",
				Status: domain.StatusInTransit,
			}, nil
		},
	}
	svc := service.NewOrderService(repo, nil)

	_, err := svc.CancelOrder(context.Background(), "order-1", "cancel reason", domain.RoleUser)
	if err == nil {
		t.Fatal("expected error when cancelling from in_transit status")
	}
	// Should fail with either ErrInvalidInput or ErrForbidden (state machine violation)
	if !errors.Is(err, pkgerrors.ErrInvalidInput) && !errors.Is(err, pkgerrors.ErrForbidden) {
		t.Errorf("expected ErrInvalidInput or ErrForbidden, got: %v", err)
	}
}

func TestUpdateStatus_ConfirmedPublishesOnlyOrderUpdated(t *testing.T) {
	repo := &mockOrderRepository{
		getByIDFn: func(ctx context.Context, id string) (*domain.Order, error) {
			return &domain.Order{
				ID:     id,
				UserID: "user-1",
				Status: domain.StatusCreated,
			}, nil
		},
		updateStatusFn: func(ctx context.Context, id string, status domain.OrderStatus, cancelReason string) (*domain.Order, error) {
			return &domain.Order{
				ID:     id,
				UserID: "user-1",
				Status: status,
			}, nil
		},
	}
	pub := &mockPublisher{}
	svc := service.NewOrderService(repo, pub)

	_, err := svc.UpdateStatus(context.Background(), "order-1", domain.StatusConfirmed, domain.RoleAdmin)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// A created->confirmed transition must publish ONLY orders.updated.
	// orders.created fires exclusively at order creation (CreateOrder), never on
	// a status transition — republishing it here would double-create a delivery.
	if len(pub.publishCalls) == 0 {
		t.Fatal("expected at least one publish call for orders.updated")
	}

	topicsPublished := make(map[string]bool)
	for _, call := range pub.publishCalls {
		topicsPublished[call.topic] = true
	}

	if !topicsPublished[pkgkafka.TopicOrdersUpdated] {
		t.Errorf("expected publish to %s, but it was not called", pkgkafka.TopicOrdersUpdated)
	}
	if topicsPublished[pkgkafka.TopicOrdersCreated] {
		t.Errorf("did not expect publish to %s on a confirmed transition", pkgkafka.TopicOrdersCreated)
	}
}

func TestUpdateStatus_PublishesOrderUpdatedOnEveryTransition(t *testing.T) {
	repo := &mockOrderRepository{
		getByIDFn: func(ctx context.Context, id string) (*domain.Order, error) {
			return &domain.Order{
				ID:     id,
				UserID: "user-1",
				Status: domain.StatusConfirmed,
			}, nil
		},
		updateStatusFn: func(ctx context.Context, id string, status domain.OrderStatus, cancelReason string) (*domain.Order, error) {
			return &domain.Order{
				ID:     id,
				UserID: "user-1",
				Status: status,
			}, nil
		},
	}
	pub := &mockPublisher{}
	svc := service.NewOrderService(repo, pub)

	// Transition from confirmed -> assigned (not confirmed, so only orders.updated should fire)
	_, err := svc.UpdateStatus(context.Background(), "order-1", domain.StatusAssigned, domain.RoleAdmin)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(pub.publishCalls) == 0 {
		t.Fatal("expected at least one publish call for orders.updated")
	}

	updatedPublished := false
	for _, call := range pub.publishCalls {
		if call.topic == pkgkafka.TopicOrdersUpdated {
			updatedPublished = true
		}
	}
	if !updatedPublished {
		t.Errorf("expected publish to %s on any status transition", pkgkafka.TopicOrdersUpdated)
	}

	// orders.created should NOT be published for non-confirmed transitions
	for _, call := range pub.publishCalls {
		if call.topic == pkgkafka.TopicOrdersCreated {
			t.Errorf("did not expect publish to %s for non-confirmed transition", pkgkafka.TopicOrdersCreated)
		}
	}
}

// ---------------------------------------------------------------------------
// Wave 0 stubs — BKND-04 (Phase 06 backend gaps)
// ---------------------------------------------------------------------------

// stubDeliveryClient is a hand-rolled mock of service.DeliveryClient for order-service tests.
type stubDeliveryClient struct {
	resp *deliveryv1.GetDeliveryByOrderIDResponse
	err  error
}

func (s *stubDeliveryClient) GetDeliveryByOrderID(ctx context.Context, in *deliveryv1.GetDeliveryByOrderIDRequest) (*deliveryv1.GetDeliveryByOrderIDResponse, error) {
	return s.resp, s.err
}

// Ensure stubDeliveryClient implements service.DeliveryClient.
var _ service.DeliveryClient = (*stubDeliveryClient)(nil)

// TestGetOrder_IncludesDeliveryID verifies that when DeliveryClient.GetDeliveryByOrderID returns
// a delivery, the order's DeliveryID field is populated from the delivery's ID.
// BKND-04 D-10.
func TestGetOrder_IncludesDeliveryID(t *testing.T) {
	repo := &mockOrderRepository{
		getByIDFn: func(ctx context.Context, id string) (*domain.Order, error) {
			return &domain.Order{ID: id, UserID: "user-1", Status: domain.StatusCreated}, nil
		},
	}
	dc := &stubDeliveryClient{
		resp: &deliveryv1.GetDeliveryByOrderIDResponse{
			Delivery: &deliveryv1.Delivery{Id: "delivery-xyz"},
		},
	}
	svc := service.NewOrderService(repo, nil).WithDeliveryClient(dc)

	order, err := svc.GetOrder(context.Background(), "order-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order.DeliveryID != "delivery-xyz" {
		t.Errorf("expected DeliveryID=delivery-xyz, got %q", order.DeliveryID)
	}
}

// TestGetOrder_GracefulDegradation verifies that when DeliveryClient returns ErrNotFound,
// the order's DeliveryID stays empty and no error is returned (D-10 graceful empty).
func TestGetOrder_GracefulDegradation(t *testing.T) {
	repo := &mockOrderRepository{
		getByIDFn: func(ctx context.Context, id string) (*domain.Order, error) {
			return &domain.Order{ID: id, UserID: "user-1", Status: domain.StatusCreated}, nil
		},
	}
	dc := &stubDeliveryClient{
		err: pkgerrors.ErrNotFound,
	}
	svc := service.NewOrderService(repo, nil).WithDeliveryClient(dc)

	order, err := svc.GetOrder(context.Background(), "order-1")
	if err != nil {
		t.Fatalf("expected no error (graceful empty), got: %v", err)
	}
	if order.DeliveryID != "" {
		t.Errorf("expected empty DeliveryID, got %q", order.DeliveryID)
	}
}

// TestGetOrder_DeliveryServiceUnavailable verifies that when DeliveryClient returns
// codes.Unavailable (delivery-service down), the order's DeliveryID stays empty
// and no error is returned (graceful degradation).
func TestGetOrder_DeliveryServiceUnavailable(t *testing.T) {
	repo := &mockOrderRepository{
		getByIDFn: func(ctx context.Context, id string) (*domain.Order, error) {
			return &domain.Order{ID: id, UserID: "user-1", Status: domain.StatusCreated}, nil
		},
	}
	dc := &stubDeliveryClient{
		err: grpcstatus.Error(grpccodes.Unavailable, "connection refused"),
	}
	svc := service.NewOrderService(repo, nil).WithDeliveryClient(dc)

	order, err := svc.GetOrder(context.Background(), "order-1")
	if err != nil {
		t.Fatalf("expected no error (graceful degradation), got: %v", err)
	}
	if order.DeliveryID != "" {
		t.Errorf("expected empty DeliveryID on unavailable, got %q", order.DeliveryID)
	}
}

func TestUpdateStatus_PublishFailureDoesNotFailUpdate(t *testing.T) {
	repo := &mockOrderRepository{
		getByIDFn: func(ctx context.Context, id string) (*domain.Order, error) {
			return &domain.Order{
				ID:     id,
				UserID: "user-1",
				Status: domain.StatusCreated,
			}, nil
		},
		updateStatusFn: func(ctx context.Context, id string, status domain.OrderStatus, cancelReason string) (*domain.Order, error) {
			return &domain.Order{
				ID:     id,
				UserID: "user-1",
				Status: status,
			}, nil
		},
	}
	// Publisher that always returns an error
	pub := &mockPublisher{err: errors.New("kafka unavailable")}
	svc := service.NewOrderService(repo, pub)

	order, err := svc.UpdateStatus(context.Background(), "order-1", domain.StatusConfirmed, domain.RoleAdmin)
	if err != nil {
		t.Fatalf("publish failure should not fail UpdateStatus, got: %v", err)
	}
	if order == nil {
		t.Fatal("expected order to be returned even when publish fails")
	}
	if order.Status != domain.StatusConfirmed {
		t.Errorf("expected status confirmed, got %s", order.Status)
	}
}
