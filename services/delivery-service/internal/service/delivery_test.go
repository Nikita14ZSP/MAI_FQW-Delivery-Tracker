package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	orderv1 "github.com/mozgovojnikita/delivery-tracker/gen/order/v1"
	pkgerrors "github.com/mozgovojnikita/delivery-tracker/pkg/errors"
	"github.com/mozgovojnikita/delivery-tracker/pkg/kafka"
	"github.com/mozgovojnikita/delivery-tracker/services/delivery-service/internal/domain"
	"github.com/mozgovojnikita/delivery-tracker/services/delivery-service/internal/service"
)

// listAvailableForCourierFn type alias for readability.
type listAvailableForCourierFn func(ctx context.Context, courierID, cursor string, limit int32) ([]domain.AvailableOrderRow, string, error)

// mockDeliveryRepo is a manual mock implementing repository.DeliveryRepository.
type mockDeliveryRepo struct {
	createDeliveryFn       func(ctx context.Context, orderID, zoneID string, lat, lng float64) (*domain.Delivery, error)
	getDeliveryByIDFn      func(ctx context.Context, id string) (*domain.Delivery, error)
	getDeliveryByOrderIDFn func(ctx context.Context, orderID string) (*domain.Delivery, error)
	updateStatusFn         func(ctx context.Context, id string, status domain.DeliveryStatus) error
	assignCourierFn        func(ctx context.Context, deliveryID, courierID string, eta time.Time) error
	listPendingFn          func(ctx context.Context) ([]*domain.Delivery, error)

	findNearestCourierFn func(ctx context.Context, zoneID string, lat, lng float64) (*domain.CourierCandidate, error)
	countActiveFn        func(ctx context.Context, courierID string) (int, error)

	createZoneFn      func(ctx context.Context, name, polygonGeoJSON string) (*domain.DeliveryZone, error)
	listZonesFn       func(ctx context.Context, limit, offset int) ([]*domain.DeliveryZone, int, error)
	findZoneByPointFn func(ctx context.Context, lat, lng float64) (*domain.DeliveryZone, error)

	upsertCourierFn       func(ctx context.Context, courierID string) error
	assignCourierToZoneFn func(ctx context.Context, courierID, zoneID string) error

	calculateETAFn func(ctx context.Context, zoneID string, lat, lng float64) (time.Time, error)

	// BKND-01 methods
	updateCourierStatusFn                 func(ctx context.Context, courierID, newStatus string) (*domain.Courier, error)
	updateCourierStatusToOfflineGuardedFn func(ctx context.Context, courierID string) (*domain.Courier, bool, error)

	// BKND-02 methods
	listAvailableForCourierFn listAvailableForCourierFn

	// BKND-03 methods
	acceptByCourierFn func(ctx context.Context, deliveryID, courierID string, eta time.Time) (*domain.Delivery, bool, error)
	getCourierFn      func(ctx context.Context, courierID string) (*domain.Courier, error)
}

// mockOrderClient is a manual mock implementing service.OrderClient.
type mockOrderClient struct {
	getOrdersByIDsFn    func(ctx context.Context, in *orderv1.GetOrdersByIDsRequest) (*orderv1.GetOrdersByIDsResponse, error)
	updateOrderStatusFn func(ctx context.Context, in *orderv1.UpdateOrderStatusRequest) (*orderv1.UpdateOrderStatusResponse, error)
	getUsersByIDsFn     func(ctx context.Context, in *orderv1.GetUsersByIDsRequest) (*orderv1.GetUsersByIDsResponse, error)
}

func (m *mockOrderClient) GetOrdersByIDs(ctx context.Context, in *orderv1.GetOrdersByIDsRequest) (*orderv1.GetOrdersByIDsResponse, error) {
	if m.getOrdersByIDsFn != nil {
		return m.getOrdersByIDsFn(ctx, in)
	}
	return &orderv1.GetOrdersByIDsResponse{}, nil
}

func (m *mockOrderClient) UpdateOrderStatus(ctx context.Context, in *orderv1.UpdateOrderStatusRequest) (*orderv1.UpdateOrderStatusResponse, error) {
	if m.updateOrderStatusFn != nil {
		return m.updateOrderStatusFn(ctx, in)
	}
	return &orderv1.UpdateOrderStatusResponse{}, nil
}

func (m *mockOrderClient) GetUsersByIDs(ctx context.Context, in *orderv1.GetUsersByIDsRequest) (*orderv1.GetUsersByIDsResponse, error) {
	if m.getUsersByIDsFn != nil {
		return m.getUsersByIDsFn(ctx, in)
	}
	return &orderv1.GetUsersByIDsResponse{}, nil
}

func (m *mockDeliveryRepo) CreateDelivery(ctx context.Context, orderID, zoneID string, lat, lng float64) (*domain.Delivery, error) {
	if m.createDeliveryFn != nil {
		return m.createDeliveryFn(ctx, orderID, zoneID, lat, lng)
	}
	return &domain.Delivery{ID: "delivery-1", OrderID: orderID, ZoneID: zoneID, Status: domain.StatusPending}, nil
}

func (m *mockDeliveryRepo) GetDeliveryByID(ctx context.Context, id string) (*domain.Delivery, error) {
	if m.getDeliveryByIDFn != nil {
		return m.getDeliveryByIDFn(ctx, id)
	}
	return nil, pkgerrors.ErrNotFound
}

func (m *mockDeliveryRepo) GetDeliveryByOrderID(ctx context.Context, orderID string) (*domain.Delivery, error) {
	if m.getDeliveryByOrderIDFn != nil {
		return m.getDeliveryByOrderIDFn(ctx, orderID)
	}
	return nil, pkgerrors.ErrNotFound
}

func (m *mockDeliveryRepo) UpdateStatus(ctx context.Context, id string, status domain.DeliveryStatus) error {
	if m.updateStatusFn != nil {
		return m.updateStatusFn(ctx, id, status)
	}
	return nil
}

func (m *mockDeliveryRepo) AssignCourier(ctx context.Context, deliveryID, courierID string, eta time.Time) error {
	if m.assignCourierFn != nil {
		return m.assignCourierFn(ctx, deliveryID, courierID, eta)
	}
	return nil
}

func (m *mockDeliveryRepo) ListPendingDeliveries(ctx context.Context) ([]*domain.Delivery, error) {
	if m.listPendingFn != nil {
		return m.listPendingFn(ctx)
	}
	return nil, nil
}

func (m *mockDeliveryRepo) FindNearestCourier(ctx context.Context, zoneID string, lat, lng float64) (*domain.CourierCandidate, error) {
	if m.findNearestCourierFn != nil {
		return m.findNearestCourierFn(ctx, zoneID, lat, lng)
	}
	return nil, pkgerrors.ErrNotFound
}

func (m *mockDeliveryRepo) CountActiveByCourier(ctx context.Context, courierID string) (int, error) {
	if m.countActiveFn != nil {
		return m.countActiveFn(ctx, courierID)
	}
	return 0, nil
}

func (m *mockDeliveryRepo) CreateZone(ctx context.Context, name, polygonGeoJSON string) (*domain.DeliveryZone, error) {
	if m.createZoneFn != nil {
		return m.createZoneFn(ctx, name, polygonGeoJSON)
	}
	return &domain.DeliveryZone{ID: "zone-1", Name: name, PolygonGeoJSON: polygonGeoJSON}, nil
}

func (m *mockDeliveryRepo) ListZones(ctx context.Context, limit, offset int) ([]*domain.DeliveryZone, int, error) {
	if m.listZonesFn != nil {
		return m.listZonesFn(ctx, limit, offset)
	}
	return nil, 0, nil
}

func (m *mockDeliveryRepo) FindZoneByPoint(ctx context.Context, lat, lng float64) (*domain.DeliveryZone, error) {
	if m.findZoneByPointFn != nil {
		return m.findZoneByPointFn(ctx, lat, lng)
	}
	return nil, nil
}

func (m *mockDeliveryRepo) UpsertCourier(ctx context.Context, courierID string) error {
	if m.upsertCourierFn != nil {
		return m.upsertCourierFn(ctx, courierID)
	}
	return nil
}

func (m *mockDeliveryRepo) AssignCourierToZone(ctx context.Context, courierID, zoneID string) error {
	if m.assignCourierToZoneFn != nil {
		return m.assignCourierToZoneFn(ctx, courierID, zoneID)
	}
	return nil
}

func (m *mockDeliveryRepo) ListByCourierID(ctx context.Context, courierID string) ([]*domain.Delivery, error) {
	return nil, nil
}

func (m *mockDeliveryRepo) ListAvailableForCourier(ctx context.Context, courierID, cursor string, limit int32) ([]domain.AvailableOrderRow, string, error) {
	if m.listAvailableForCourierFn != nil {
		return m.listAvailableForCourierFn(ctx, courierID, cursor, limit)
	}
	return nil, "", nil
}

func (m *mockDeliveryRepo) AcceptByCourier(ctx context.Context, deliveryID, courierID string, eta time.Time) (*domain.Delivery, bool, error) {
	if m.acceptByCourierFn != nil {
		return m.acceptByCourierFn(ctx, deliveryID, courierID, eta)
	}
	return &domain.Delivery{ID: deliveryID, OrderID: "order-1", CourierID: courierID, Status: domain.StatusAssigned}, true, nil
}

func (m *mockDeliveryRepo) GetCourier(ctx context.Context, courierID string) (*domain.Courier, error) {
	if m.getCourierFn != nil {
		return m.getCourierFn(ctx, courierID)
	}
	return &domain.Courier{ID: courierID, Status: "available"}, nil
}

func (m *mockDeliveryRepo) CalculateETA(ctx context.Context, zoneID string, lat, lng float64) (time.Time, error) {
	if m.calculateETAFn != nil {
		return m.calculateETAFn(ctx, zoneID, lat, lng)
	}
	return time.Now().Add(30 * time.Minute), nil
}

func (m *mockDeliveryRepo) UpdateCourierStatus(ctx context.Context, courierID, newStatus string) (*domain.Courier, error) {
	if m.updateCourierStatusFn != nil {
		return m.updateCourierStatusFn(ctx, courierID, newStatus)
	}
	return &domain.Courier{ID: courierID, Status: newStatus}, nil
}

func (m *mockDeliveryRepo) UpdateCourierStatusToOfflineGuarded(ctx context.Context, courierID string) (*domain.Courier, bool, error) {
	if m.updateCourierStatusToOfflineGuardedFn != nil {
		return m.updateCourierStatusToOfflineGuardedFn(ctx, courierID)
	}
	return &domain.Courier{ID: courierID, Status: "offline"}, true, nil
}

// mockPublisher is a manual mock implementing kafka.EventPublisher.
type mockPublisher struct {
	publishFn func(ctx context.Context, topic, key string, value []byte) error
	closeFn   func() error
}

func (m *mockPublisher) Publish(ctx context.Context, topic, key string, value []byte) error {
	if m.publishFn != nil {
		return m.publishFn(ctx, topic, key, value)
	}
	return nil
}

func (m *mockPublisher) Close() error {
	if m.closeFn != nil {
		return m.closeFn()
	}
	return nil
}

// TestAutoAssignCourier_Success tests that a valid OrderCreatedEvent with a zone and available courier
// results in a delivery being created, assigned, and a DeliveryAssignedEvent published.
func TestAutoAssignCourier_Success(t *testing.T) {
	expectedZone := &domain.DeliveryZone{ID: "zone-1", Name: "Test Zone"}
	expectedCourier := &domain.CourierCandidate{CourierID: "courier-1", DistanceMeters: 500}
	expectedETA := time.Now().Add(30 * time.Minute)

	var publishedTopic string
	var publishedKey string

	repo := &mockDeliveryRepo{
		findZoneByPointFn: func(ctx context.Context, lat, lng float64) (*domain.DeliveryZone, error) {
			return expectedZone, nil
		},
		createDeliveryFn: func(ctx context.Context, orderID, zoneID string, lat, lng float64) (*domain.Delivery, error) {
			return &domain.Delivery{ID: "delivery-1", OrderID: orderID, ZoneID: zoneID, Status: domain.StatusPending}, nil
		},
		findNearestCourierFn: func(ctx context.Context, zoneID string, lat, lng float64) (*domain.CourierCandidate, error) {
			return expectedCourier, nil
		},
		calculateETAFn: func(ctx context.Context, zoneID string, lat, lng float64) (time.Time, error) {
			return expectedETA, nil
		},
		assignCourierFn: func(ctx context.Context, deliveryID, courierID string, eta time.Time) error {
			return nil
		},
	}
	pub := &mockPublisher{
		publishFn: func(ctx context.Context, topic, key string, value []byte) error {
			publishedTopic = topic
			publishedKey = key
			return nil
		},
	}

	svc := service.NewDeliveryService(repo, pub, nil, nil, true)
	err := svc.HandleOrderCreated(context.Background(), service.OrderCreatedInput{
		OrderID: "order-1",
		UserID:  "user-1",
		Lat:     55.75,
		Lng:     37.61,
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if publishedTopic == "" {
		t.Error("expected DeliveryAssignedEvent to be published")
	}
	if publishedKey == "" {
		t.Error("expected a publish key")
	}
	_ = publishedTopic
}

// TestHandleOrderCreated_AutoAssignDisabled_LeavesPending verifies the v2.0 manual-selection
// model: when DELIVERY_AUTO_ASSIGN is OFF (autoAssign=false), HandleOrderCreated creates the
// delivery but DOES NOT auto-assign — the delivery stays pending, no courier lookup, no
// assignment, no DeliveryAssignedEvent. v1.0 DLVR-02 behavior is preserved when the flag is
// ON (see TestAutoAssignCourier_Success above).
func TestHandleOrderCreated_AutoAssignDisabled_LeavesPending(t *testing.T) {
	zone := &domain.DeliveryZone{ID: "zone-1", Name: "Test Zone"}
	createdDelivery := &domain.Delivery{ID: "delivery-1", OrderID: "order-1", ZoneID: "zone-1", Status: domain.StatusPending}

	var findNearestCalls, assignCalls, publishCalls int

	repo := &mockDeliveryRepo{
		findZoneByPointFn: func(ctx context.Context, lat, lng float64) (*domain.DeliveryZone, error) {
			return zone, nil
		},
		createDeliveryFn: func(ctx context.Context, orderID, zoneID string, lat, lng float64) (*domain.Delivery, error) {
			return createdDelivery, nil
		},
		findNearestCourierFn: func(ctx context.Context, zoneID string, lat, lng float64) (*domain.CourierCandidate, error) {
			findNearestCalls++
			return &domain.CourierCandidate{CourierID: "courier-1", DistanceMeters: 500}, nil
		},
		assignCourierFn: func(ctx context.Context, deliveryID, courierID string, eta time.Time) error {
			assignCalls++
			return nil
		},
	}
	pub := &mockPublisher{
		publishFn: func(ctx context.Context, topic, key string, value []byte) error {
			publishCalls++
			return nil
		},
	}

	// autoAssign=false → v2.0 manual-selection product
	svc := service.NewDeliveryService(repo, pub, nil, nil, false)
	err := svc.HandleOrderCreated(context.Background(), service.OrderCreatedInput{
		OrderID: "order-1",
		UserID:  "user-1",
		Lat:     55.75,
		Lng:     37.61,
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if findNearestCalls != 0 {
		t.Errorf("autoAssign=false: FindNearestCourier should not be called, got %d", findNearestCalls)
	}
	if assignCalls != 0 {
		t.Errorf("autoAssign=false: AssignCourier should not be called, got %d", assignCalls)
	}
	if publishCalls != 0 {
		t.Errorf("autoAssign=false: no DeliveryAssignedEvent should be published, got %d", publishCalls)
	}
	if createdDelivery.Status != domain.StatusPending {
		t.Errorf("expected delivery to remain pending, got: %s", createdDelivery.Status)
	}
}

// TestAutoAssignCourier_NoCourier tests that when no courier is available the delivery stays pending (D-04).
func TestAutoAssignCourier_NoCourier(t *testing.T) {
	var assignCalled bool

	repo := &mockDeliveryRepo{
		findZoneByPointFn: func(ctx context.Context, lat, lng float64) (*domain.DeliveryZone, error) {
			return &domain.DeliveryZone{ID: "zone-1"}, nil
		},
		createDeliveryFn: func(ctx context.Context, orderID, zoneID string, lat, lng float64) (*domain.Delivery, error) {
			return &domain.Delivery{ID: "delivery-1", OrderID: orderID, ZoneID: zoneID, Status: domain.StatusPending}, nil
		},
		findNearestCourierFn: func(ctx context.Context, zoneID string, lat, lng float64) (*domain.CourierCandidate, error) {
			return nil, nil // no courier available
		},
		assignCourierFn: func(ctx context.Context, deliveryID, courierID string, eta time.Time) error {
			assignCalled = true
			return nil
		},
	}
	pub := &mockPublisher{}

	svc := service.NewDeliveryService(repo, pub, nil, nil, true)
	err := svc.HandleOrderCreated(context.Background(), service.OrderCreatedInput{
		OrderID: "order-1",
		UserID:  "user-1",
		Lat:     55.75,
		Lng:     37.61,
	})

	if err != nil {
		t.Fatalf("expected no error when no courier available, got: %v", err)
	}
	if assignCalled {
		t.Error("AssignCourier should NOT be called when no courier available")
	}
}

// TestAutoAssignCourier_NoZone tests that when point is not in any zone delivery is created pending without zone_id.
func TestAutoAssignCourier_NoZone(t *testing.T) {
	var capturedZoneID string

	repo := &mockDeliveryRepo{
		findZoneByPointFn: func(ctx context.Context, lat, lng float64) (*domain.DeliveryZone, error) {
			return nil, nil // not in any zone
		},
		createDeliveryFn: func(ctx context.Context, orderID, zoneID string, lat, lng float64) (*domain.Delivery, error) {
			capturedZoneID = zoneID
			return &domain.Delivery{ID: "delivery-1", OrderID: orderID, Status: domain.StatusPending}, nil
		},
	}
	pub := &mockPublisher{}

	svc := service.NewDeliveryService(repo, pub, nil, nil, true)
	err := svc.HandleOrderCreated(context.Background(), service.OrderCreatedInput{
		OrderID: "order-1",
		UserID:  "user-1",
		Lat:     0,
		Lng:     0,
	})

	if err != nil {
		t.Fatalf("expected no error when no zone found, got: %v", err)
	}
	if capturedZoneID != "" {
		t.Errorf("expected empty zoneID when no zone, got %q", capturedZoneID)
	}
}

// TestManualAssignCourier tests that admin can manually assign a courier to a pending delivery.
func TestManualAssignCourier(t *testing.T) {
	var publishedTopic string

	repo := &mockDeliveryRepo{
		getDeliveryByOrderIDFn: func(ctx context.Context, orderID string) (*domain.Delivery, error) {
			return &domain.Delivery{
				ID:      "delivery-1",
				OrderID: orderID,
				ZoneID:  "zone-1",
				Status:  domain.StatusPending,
			}, nil
		},
		upsertCourierFn: func(ctx context.Context, courierID string) error {
			return nil
		},
		calculateETAFn: func(ctx context.Context, zoneID string, lat, lng float64) (time.Time, error) {
			return time.Now().Add(30 * time.Minute), nil
		},
		assignCourierFn: func(ctx context.Context, deliveryID, courierID string, eta time.Time) error {
			return nil
		},
	}
	pub := &mockPublisher{
		publishFn: func(ctx context.Context, topic, key string, value []byte) error {
			publishedTopic = topic
			return nil
		},
	}

	svc := service.NewDeliveryService(repo, pub, nil, nil, true)
	delivery, err := svc.ManualAssignCourier(context.Background(), "order-1", "courier-1")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if delivery == nil {
		t.Fatal("expected delivery, got nil")
	}
	if publishedTopic == "" {
		t.Error("expected DeliveryAssignedEvent to be published")
	}
}

// TestManualAssignCourier_NotPending tests that manual assignment fails if delivery is not pending.
func TestManualAssignCourier_NotPending(t *testing.T) {
	repo := &mockDeliveryRepo{
		getDeliveryByOrderIDFn: func(ctx context.Context, orderID string) (*domain.Delivery, error) {
			return &domain.Delivery{
				ID:     "delivery-1",
				Status: domain.StatusAssigned, // already assigned
			}, nil
		},
	}
	pub := &mockPublisher{}

	svc := service.NewDeliveryService(repo, pub, nil, nil, true)
	_, err := svc.ManualAssignCourier(context.Background(), "order-1", "courier-1")

	if err == nil {
		t.Fatal("expected error when delivery is not pending")
	}
	if !errors.Is(err, pkgerrors.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got: %v", err)
	}
}

// TestCreateZone tests that a valid GeoJSON polygon creates a zone.
func TestCreateZone(t *testing.T) {
	const geoJSON = `{"type":"Polygon","coordinates":[[[37.0,55.0],[38.0,55.0],[38.0,56.0],[37.0,56.0],[37.0,55.0]]]}`

	repo := &mockDeliveryRepo{
		createZoneFn: func(ctx context.Context, name, polygonGeoJSON string) (*domain.DeliveryZone, error) {
			return &domain.DeliveryZone{
				ID:             "zone-1",
				Name:           name,
				PolygonGeoJSON: polygonGeoJSON,
				CreatedAt:      time.Now(),
			}, nil
		},
	}
	pub := &mockPublisher{}

	svc := service.NewDeliveryService(repo, pub, nil, nil, true)
	zone, err := svc.CreateZone(context.Background(), "Test Zone", geoJSON)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if zone == nil {
		t.Fatal("expected zone, got nil")
	}
	if zone.Name != "Test Zone" {
		t.Errorf("expected zone name 'Test Zone', got %q", zone.Name)
	}
}

// TestCreateZone_EmptyName tests that creating a zone with empty name fails.
func TestCreateZone_EmptyName(t *testing.T) {
	repo := &mockDeliveryRepo{}
	pub := &mockPublisher{}

	svc := service.NewDeliveryService(repo, pub, nil, nil, true)
	_, err := svc.CreateZone(context.Background(), "", `{"type":"Polygon"}`)

	if err == nil {
		t.Fatal("expected error for empty name")
	}
	if !errors.Is(err, pkgerrors.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got: %v", err)
	}
}

// TestAssignCourierToZone tests assigning a courier to a zone.
func TestAssignCourierToZone(t *testing.T) {
	var upsertCalled bool
	var assignCalled bool

	repo := &mockDeliveryRepo{
		upsertCourierFn: func(ctx context.Context, courierID string) error {
			upsertCalled = true
			return nil
		},
		assignCourierToZoneFn: func(ctx context.Context, courierID, zoneID string) error {
			assignCalled = true
			return nil
		},
	}
	pub := &mockPublisher{}

	svc := service.NewDeliveryService(repo, pub, nil, nil, true)
	err := svc.AssignCourierToZone(context.Background(), "courier-1", "zone-1")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !upsertCalled {
		t.Error("expected UpsertCourier to be called")
	}
	if !assignCalled {
		t.Error("expected AssignCourierToZone repo to be called")
	}
}

// TestCalculateETA tests that a valid zone+coords returns a future timestamp.
func TestCalculateETA(t *testing.T) {
	futureTime := time.Now().Add(45 * time.Minute)

	repo := &mockDeliveryRepo{
		calculateETAFn: func(ctx context.Context, zoneID string, lat, lng float64) (time.Time, error) {
			return futureTime, nil
		},
	}
	pub := &mockPublisher{}

	svc := service.NewDeliveryService(repo, pub, nil, nil, true)
	eta, err := svc.CalculateETA(context.Background(), "zone-1", 55.75, 37.61)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !eta.After(time.Now()) {
		t.Error("expected ETA to be in the future")
	}
}

// TestMaxActiveDeliveries tests that courier with 5 active deliveries is not assigned (D-05 via mock).
func TestMaxActiveDeliveries(t *testing.T) {
	// Mock: FindNearestCourier returns nil (the DB query enforces max 5 deliveries)
	var assignCalled bool

	repo := &mockDeliveryRepo{
		findZoneByPointFn: func(ctx context.Context, lat, lng float64) (*domain.DeliveryZone, error) {
			return &domain.DeliveryZone{ID: "zone-1"}, nil
		},
		createDeliveryFn: func(ctx context.Context, orderID, zoneID string, lat, lng float64) (*domain.Delivery, error) {
			return &domain.Delivery{ID: "delivery-1", OrderID: orderID, ZoneID: zoneID, Status: domain.StatusPending}, nil
		},
		findNearestCourierFn: func(ctx context.Context, zoneID string, lat, lng float64) (*domain.CourierCandidate, error) {
			// Simulates all couriers at max active deliveries - returns nil (no candidate)
			return nil, nil
		},
		assignCourierFn: func(ctx context.Context, deliveryID, courierID string, eta time.Time) error {
			assignCalled = true
			return nil
		},
	}
	pub := &mockPublisher{}

	svc := service.NewDeliveryService(repo, pub, nil, nil, true)
	err := svc.HandleOrderCreated(context.Background(), service.OrderCreatedInput{
		OrderID: "order-max",
		UserID:  "user-1",
		Lat:     55.75,
		Lng:     37.61,
	})

	if err != nil {
		t.Fatalf("expected no error when max active deliveries reached, got: %v", err)
	}
	if assignCalled {
		t.Error("AssignCourier should NOT be called when all couriers at max active deliveries")
	}
}

// ---------------------------------------------------------------------------
// BKND-01 tests — UpdateCourierStatus (Phase 06 plan 03)
// ---------------------------------------------------------------------------

// TestUpdateCourierStatus_AvailableTransition verifies that setting status=available
// calls repo.UpdateCourierStatus and returns the updated courier.
func TestUpdateCourierStatus_AvailableTransition(t *testing.T) {
	expectedCourier := &domain.Courier{ID: "courier-1", Status: "available"}
	var capturedStatus string

	repo := &mockDeliveryRepo{
		updateCourierStatusFn: func(ctx context.Context, courierID, newStatus string) (*domain.Courier, error) {
			capturedStatus = newStatus
			return expectedCourier, nil
		},
	}
	pub := &mockPublisher{}
	svc := service.NewDeliveryService(repo, pub, nil, nil, true)

	c, err := svc.UpdateCourierStatus(context.Background(), "courier-1", "available")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if c == nil {
		t.Fatal("expected courier, got nil")
	}
	if capturedStatus != "available" {
		t.Errorf("expected repo called with 'available', got %q", capturedStatus)
	}
}

// TestUpdateCourierStatus_RejectBusyFromCourier verifies that setting status=busy
// returns ErrInvalidStatusTransition (D-02: busy is system-only).
func TestUpdateCourierStatus_RejectBusyFromCourier(t *testing.T) {
	repo := &mockDeliveryRepo{}
	pub := &mockPublisher{}
	svc := service.NewDeliveryService(repo, pub, nil, nil, true)

	_, err := svc.UpdateCourierStatus(context.Background(), "courier-1", "busy")
	if err == nil {
		t.Fatal("expected error for status=busy, got nil")
	}
	if !errors.Is(err, domain.ErrInvalidStatusTransition) {
		t.Errorf("expected ErrInvalidStatusTransition, got: %v", err)
	}
}

// TestUpdateCourierStatus_RejectUnknownStatus verifies that an unknown status string
// returns ErrInvalidStatusTransition.
func TestUpdateCourierStatus_RejectUnknownStatus(t *testing.T) {
	repo := &mockDeliveryRepo{}
	pub := &mockPublisher{}
	svc := service.NewDeliveryService(repo, pub, nil, nil, true)

	_, err := svc.UpdateCourierStatus(context.Background(), "courier-1", "ondine")
	if err == nil {
		t.Fatal("expected error for unknown status, got nil")
	}
	if !errors.Is(err, domain.ErrInvalidStatusTransition) {
		t.Errorf("expected ErrInvalidStatusTransition, got: %v", err)
	}
}

// TestUpdateCourierStatus_OfflineWithActiveDelivery verifies that going offline
// when an active delivery exists returns ErrActiveDeliveryExists (D-03).
func TestUpdateCourierStatus_OfflineWithActiveDelivery(t *testing.T) {
	repo := &mockDeliveryRepo{
		updateCourierStatusToOfflineGuardedFn: func(ctx context.Context, courierID string) (*domain.Courier, bool, error) {
			return nil, false, nil // active delivery blocks
		},
	}
	pub := &mockPublisher{}
	svc := service.NewDeliveryService(repo, pub, nil, nil, true)

	_, err := svc.UpdateCourierStatus(context.Background(), "courier-1", "offline")
	if err == nil {
		t.Fatal("expected ErrActiveDeliveryExists, got nil")
	}
	if !errors.Is(err, domain.ErrActiveDeliveryExists) {
		t.Errorf("expected ErrActiveDeliveryExists, got: %v", err)
	}
}

// TestUpdateCourierStatus_OfflineSuccess verifies that going offline without
// active deliveries returns the updated courier.
func TestUpdateCourierStatus_OfflineSuccess(t *testing.T) {
	expectedCourier := &domain.Courier{ID: "courier-1", Status: "offline"}

	repo := &mockDeliveryRepo{
		updateCourierStatusToOfflineGuardedFn: func(ctx context.Context, courierID string) (*domain.Courier, bool, error) {
			return expectedCourier, true, nil
		},
	}
	pub := &mockPublisher{}
	svc := service.NewDeliveryService(repo, pub, nil, nil, true)

	c, err := svc.UpdateCourierStatus(context.Background(), "courier-1", "offline")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if c == nil {
		t.Fatal("expected courier, got nil")
	}
	if c.Status != "offline" {
		t.Errorf("expected status 'offline', got %q", c.Status)
	}
}

// ---------------------------------------------------------------------------
// BKND-02 tests — ListAvailableOrders (Phase 06 plan 04)
// ---------------------------------------------------------------------------

// TestListAvailableOrders_Success verifies that service merges repo rows with OrderClient enrichment.
func TestListAvailableOrders_Success(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	repoRows := []domain.AvailableOrderRow{
		{OrderID: "order-1", DeliveryID: "del-1", ZoneName: "Center", CreatedAt: now},
		{OrderID: "order-2", DeliveryID: "del-2", ZoneName: "Center", CreatedAt: now.Add(time.Second)},
	}

	repo := &mockDeliveryRepo{
		listAvailableForCourierFn: func(ctx context.Context, courierID, cursor string, limit int32) ([]domain.AvailableOrderRow, string, error) {
			return repoRows, "", nil
		},
	}
	pub := &mockPublisher{}
	oc := &mockOrderClient{
		getOrdersByIDsFn: func(ctx context.Context, in *orderv1.GetOrdersByIDsRequest) (*orderv1.GetOrdersByIDsResponse, error) {
			return &orderv1.GetOrdersByIDsResponse{
				Orders: []*orderv1.OrderPreviewSlim{
					{OrderId: "order-1", TotalPrice: 500.0, ItemsCount: 3, DeliveryAddress: "ул. Ленина, 1"},
					{OrderId: "order-2", TotalPrice: 200.0, ItemsCount: 1, DeliveryAddress: "ул. Мира, 5"},
				},
			}, nil
		},
	}

	svc := service.NewDeliveryService(repo, pub, nil, oc, true)
	result, nextCursor, err := svc.ListAvailableOrders(context.Background(), "courier-1", "", 20)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}
	if result[0].TotalPrice != 500.0 {
		t.Errorf("expected TotalPrice=500.0, got %v", result[0].TotalPrice)
	}
	if result[0].ItemsCount != 3 {
		t.Errorf("expected ItemsCount=3, got %v", result[0].ItemsCount)
	}
	if result[0].DeliveryAddress != "ул. Ленина, 1" {
		t.Errorf("expected DeliveryAddress set from order-service, got %q", result[0].DeliveryAddress)
	}
	if nextCursor != "" {
		t.Errorf("expected empty nextCursor for partial page, got %q", nextCursor)
	}
}

// TestListAvailableOrders_OrderServiceUnavailable verifies graceful degradation:
// rows still returned with zero TotalPrice/ItemsCount when order-service errors.
func TestListAvailableOrders_OrderServiceUnavailable(t *testing.T) {
	now := time.Now().UTC()
	repoRows := []domain.AvailableOrderRow{
		{OrderID: "order-1", DeliveryID: "del-1", ZoneName: "Center", CreatedAt: now},
	}

	repo := &mockDeliveryRepo{
		listAvailableForCourierFn: func(ctx context.Context, courierID, cursor string, limit int32) ([]domain.AvailableOrderRow, string, error) {
			return repoRows, "", nil
		},
	}
	pub := &mockPublisher{}
	oc := &mockOrderClient{
		getOrdersByIDsFn: func(ctx context.Context, in *orderv1.GetOrdersByIDsRequest) (*orderv1.GetOrdersByIDsResponse, error) {
			return nil, errors.New("connection refused")
		},
	}

	svc := service.NewDeliveryService(repo, pub, nil, oc, true)
	result, _, err := svc.ListAvailableOrders(context.Background(), "courier-1", "", 20)

	if err != nil {
		t.Fatalf("expected no error (graceful degradation), got: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 result even when order-service unavailable, got %d", len(result))
	}
	if result[0].TotalPrice != 0 {
		t.Errorf("expected TotalPrice=0 on degradation, got %v", result[0].TotalPrice)
	}
	if result[0].ItemsCount != 0 {
		t.Errorf("expected ItemsCount=0 on degradation, got %v", result[0].ItemsCount)
	}
}

// TestListAvailableOrders_EmptyRepoResult verifies that empty repo result short-circuits orderClient call.
func TestListAvailableOrders_EmptyRepoResult(t *testing.T) {
	var orderClientCalled bool

	repo := &mockDeliveryRepo{
		listAvailableForCourierFn: func(ctx context.Context, courierID, cursor string, limit int32) ([]domain.AvailableOrderRow, string, error) {
			return nil, "", nil
		},
	}
	pub := &mockPublisher{}
	oc := &mockOrderClient{
		getOrdersByIDsFn: func(ctx context.Context, in *orderv1.GetOrdersByIDsRequest) (*orderv1.GetOrdersByIDsResponse, error) {
			orderClientCalled = true
			return &orderv1.GetOrdersByIDsResponse{}, nil
		},
	}

	svc := service.NewDeliveryService(repo, pub, nil, oc, true)
	result, nextCursor, err := svc.ListAvailableOrders(context.Background(), "courier-1", "", 20)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d rows", len(result))
	}
	if nextCursor != "" {
		t.Errorf("expected empty nextCursor, got %q", nextCursor)
	}
	if orderClientCalled {
		t.Error("orderClient.GetOrdersByIDs should NOT be called when repo returns empty")
	}
}

// TestListAvailableOrders_CursorPropagated verifies that the repo's nextCursor is returned as-is.
func TestListAvailableOrders_CursorPropagated(t *testing.T) {
	expectedCursor := "dGVzdC1jdXJzb3I=" // base64 placeholder
	now := time.Now().UTC()
	repoRows := []domain.AvailableOrderRow{
		{OrderID: "order-1", DeliveryID: "del-1", ZoneName: "Center", CreatedAt: now},
	}

	repo := &mockDeliveryRepo{
		listAvailableForCourierFn: func(ctx context.Context, courierID, cursor string, limit int32) ([]domain.AvailableOrderRow, string, error) {
			return repoRows, expectedCursor, nil
		},
	}
	pub := &mockPublisher{}
	svc := service.NewDeliveryService(repo, pub, nil, nil, true)

	_, nextCursor, err := svc.ListAvailableOrders(context.Background(), "courier-1", "", 20)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if nextCursor != expectedCursor {
		t.Errorf("expected nextCursor=%q, got %q", expectedCursor, nextCursor)
	}
}

// ---------------------------------------------------------------------------
// BKND-03 tests — AcceptOrder (Phase 06 plan 05)
// ---------------------------------------------------------------------------

// makeAcceptOrderRepo builds a mockDeliveryRepo with happy-path defaults for AcceptOrder.
func makeAcceptOrderRepo() *mockDeliveryRepo {
	return &mockDeliveryRepo{
		getCourierFn: func(ctx context.Context, courierID string) (*domain.Courier, error) {
			return &domain.Courier{ID: courierID, Status: "available"}, nil
		},
		countActiveFn: func(ctx context.Context, courierID string) (int, error) {
			return 2, nil // below max
		},
		acceptByCourierFn: func(ctx context.Context, deliveryID, courierID string, eta time.Time) (*domain.Delivery, bool, error) {
			return &domain.Delivery{
				ID:        deliveryID,
				OrderID:   "order-1",
				CourierID: courierID,
				Status:    domain.StatusAssigned,
			}, true, nil
		},
		updateCourierStatusFn: func(ctx context.Context, courierID, newStatus string) (*domain.Courier, error) {
			return &domain.Courier{ID: courierID, Status: newStatus}, nil
		},
	}
}

// TestAcceptOrder_Success verifies the happy path: delivery returned, no error.
func TestAcceptOrder_Success(t *testing.T) {
	repo := makeAcceptOrderRepo()
	pub := &mockPublisher{}
	oc := &mockOrderClient{}

	svc := service.NewDeliveryService(repo, pub, nil, oc, true)
	delivery, err := svc.AcceptOrder(context.Background(), "courier-1", "delivery-1")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if delivery == nil {
		t.Fatal("expected delivery, got nil")
	}
	if delivery.CourierID != "courier-1" {
		t.Errorf("expected courier_id 'courier-1', got %q", delivery.CourierID)
	}
}

// TestAcceptOrder_PublishesEvent verifies publisher is called with TopicDeliveryAssigned
// and the event payload contains the correct DeliveryID / CourierID / OrderID.
func TestAcceptOrder_PublishesEvent(t *testing.T) {
	repo := makeAcceptOrderRepo()

	var capturedTopic string
	var capturedPayload []byte
	pub := &mockPublisher{
		publishFn: func(ctx context.Context, topic, key string, value []byte) error {
			capturedTopic = topic
			capturedPayload = value
			return nil
		},
	}
	oc := &mockOrderClient{}

	svc := service.NewDeliveryService(repo, pub, nil, oc, true)
	_, err := svc.AcceptOrder(context.Background(), "courier-1", "delivery-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedTopic != kafka.TopicDeliveryAssigned {
		t.Errorf("expected topic %q, got %q", kafka.TopicDeliveryAssigned, capturedTopic)
	}

	var event kafka.DeliveryAssignedEvent
	if err := json.Unmarshal(capturedPayload, &event); err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}
	if event.DeliveryID != "delivery-1" {
		t.Errorf("expected DeliveryID 'delivery-1', got %q", event.DeliveryID)
	}
	if event.CourierID != "courier-1" {
		t.Errorf("expected CourierID 'courier-1', got %q", event.CourierID)
	}
	if event.OrderID != "order-1" {
		t.Errorf("expected OrderID 'order-1', got %q", event.OrderID)
	}
}

// TestAcceptOrder_UpdatesOrderStatus verifies orderClient.UpdateOrderStatus is called
// with the delivery's order_id and status = ORDER_STATUS_CONFIRMED (D-08).
func TestAcceptOrder_UpdatesOrderStatus(t *testing.T) {
	repo := makeAcceptOrderRepo()
	pub := &mockPublisher{}

	var capturedReq *orderv1.UpdateOrderStatusRequest
	oc := &mockOrderClient{
		updateOrderStatusFn: func(ctx context.Context, in *orderv1.UpdateOrderStatusRequest) (*orderv1.UpdateOrderStatusResponse, error) {
			capturedReq = in
			return &orderv1.UpdateOrderStatusResponse{}, nil
		},
	}

	svc := service.NewDeliveryService(repo, pub, nil, oc, true)
	_, err := svc.AcceptOrder(context.Background(), "courier-1", "delivery-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedReq == nil {
		t.Fatal("expected UpdateOrderStatus to be called, got no call")
	}
	if capturedReq.Id != "order-1" {
		t.Errorf("expected order_id 'order-1', got %q", capturedReq.Id)
	}
	if capturedReq.Status != orderv1.OrderStatus_ORDER_STATUS_CONFIRMED {
		t.Errorf("expected ORDER_STATUS_CONFIRMED, got %v", capturedReq.Status)
	}
}

// TestAcceptOrder_AlreadyTaken verifies that when AcceptByCourier returns ok=false
// the service returns ErrAlreadyTaken.
func TestAcceptOrder_AlreadyTaken(t *testing.T) {
	repo := makeAcceptOrderRepo()
	repo.acceptByCourierFn = func(ctx context.Context, deliveryID, courierID string, eta time.Time) (*domain.Delivery, bool, error) {
		return nil, false, nil // contention
	}
	pub := &mockPublisher{}
	oc := &mockOrderClient{}

	svc := service.NewDeliveryService(repo, pub, nil, oc, true)
	_, err := svc.AcceptOrder(context.Background(), "courier-1", "delivery-1")
	if err == nil {
		t.Fatal("expected ErrAlreadyTaken, got nil")
	}
	if !errors.Is(err, domain.ErrAlreadyTaken) {
		t.Errorf("expected ErrAlreadyTaken, got: %v", err)
	}
}

// TestAcceptOrder_CourierOffline verifies that an offline courier gets ErrCourierOffline
// and AcceptByCourier is never called (guard fires first).
func TestAcceptOrder_CourierOffline(t *testing.T) {
	var acceptCalled bool
	repo := makeAcceptOrderRepo()
	repo.getCourierFn = func(ctx context.Context, courierID string) (*domain.Courier, error) {
		return &domain.Courier{ID: courierID, Status: "offline"}, nil
	}
	repo.acceptByCourierFn = func(ctx context.Context, deliveryID, courierID string, eta time.Time) (*domain.Delivery, bool, error) {
		acceptCalled = true
		return nil, false, nil
	}
	pub := &mockPublisher{}
	oc := &mockOrderClient{}

	svc := service.NewDeliveryService(repo, pub, nil, oc, true)
	_, err := svc.AcceptOrder(context.Background(), "courier-1", "delivery-1")
	if err == nil {
		t.Fatal("expected ErrCourierOffline, got nil")
	}
	if !errors.Is(err, domain.ErrCourierOffline) {
		t.Errorf("expected ErrCourierOffline, got: %v", err)
	}
	if acceptCalled {
		t.Error("AcceptByCourier should NOT be called when courier is offline")
	}
}

// TestAcceptOrder_MaxActive verifies that a courier with MaxActiveDeliveries active
// gets ErrMaxActiveReached and AcceptByCourier is never called.
func TestAcceptOrder_MaxActive(t *testing.T) {
	var acceptCalled bool
	repo := makeAcceptOrderRepo()
	repo.countActiveFn = func(ctx context.Context, courierID string) (int, error) {
		return domain.MaxActiveDeliveries, nil // at the limit
	}
	repo.acceptByCourierFn = func(ctx context.Context, deliveryID, courierID string, eta time.Time) (*domain.Delivery, bool, error) {
		acceptCalled = true
		return nil, false, nil
	}
	pub := &mockPublisher{}
	oc := &mockOrderClient{}

	svc := service.NewDeliveryService(repo, pub, nil, oc, true)
	_, err := svc.AcceptOrder(context.Background(), "courier-1", "delivery-1")
	if err == nil {
		t.Fatal("expected ErrMaxActiveReached, got nil")
	}
	if !errors.Is(err, domain.ErrMaxActiveReached) {
		t.Errorf("expected ErrMaxActiveReached, got: %v", err)
	}
	if acceptCalled {
		t.Error("AcceptByCourier should NOT be called when max active reached")
	}
}

// TestAcceptOrder_OrderServiceFailure verifies graceful degradation: if UpdateOrderStatus
// fails, AcceptOrder still returns success (delivery is committed; eventual consistency via Kafka).
func TestAcceptOrder_OrderServiceFailure(t *testing.T) {
	repo := makeAcceptOrderRepo()
	pub := &mockPublisher{}
	oc := &mockOrderClient{
		updateOrderStatusFn: func(ctx context.Context, in *orderv1.UpdateOrderStatusRequest) (*orderv1.UpdateOrderStatusResponse, error) {
			return nil, errors.New("order-service unavailable")
		},
	}

	svc := service.NewDeliveryService(repo, pub, nil, oc, true)
	delivery, err := svc.AcceptOrder(context.Background(), "courier-1", "delivery-1")
	if err != nil {
		t.Fatalf("expected success despite order-service failure, got: %v", err)
	}
	if delivery == nil {
		t.Fatal("expected delivery returned even when order-service fails")
	}
}

// TestAcceptOrder_KafkaPublishFailure verifies that a Kafka publish failure does not
// fail AcceptOrder — delivery is already committed (RESEARCH §Pattern 4 graceful publish).
func TestAcceptOrder_KafkaPublishFailure(t *testing.T) {
	repo := makeAcceptOrderRepo()
	pub := &mockPublisher{
		publishFn: func(ctx context.Context, topic, key string, value []byte) error {
			return errors.New("kafka broker unavailable")
		},
	}
	oc := &mockOrderClient{}

	svc := service.NewDeliveryService(repo, pub, nil, oc, true)
	delivery, err := svc.AcceptOrder(context.Background(), "courier-1", "delivery-1")
	if err != nil {
		t.Fatalf("expected success despite Kafka failure, got: %v", err)
	}
	if delivery == nil {
		t.Fatal("expected delivery returned even when Kafka publish fails")
	}
}

// TestRetryPendingDeliveries tests that deliveries with zone_id get retried.
func TestRetryPendingDeliveries(t *testing.T) {
	var assignCallCount int

	repo := &mockDeliveryRepo{
		listPendingFn: func(ctx context.Context) ([]*domain.Delivery, error) {
			return []*domain.Delivery{
				{ID: "d-1", OrderID: "o-1", ZoneID: "zone-1", Status: domain.StatusPending},
				{ID: "d-2", OrderID: "o-2", ZoneID: "", Status: domain.StatusPending}, // no zone - skip
			}, nil
		},
		findNearestCourierFn: func(ctx context.Context, zoneID string, lat, lng float64) (*domain.CourierCandidate, error) {
			return &domain.CourierCandidate{CourierID: "courier-1"}, nil
		},
		calculateETAFn: func(ctx context.Context, zoneID string, lat, lng float64) (time.Time, error) {
			return time.Now().Add(30 * time.Minute), nil
		},
		assignCourierFn: func(ctx context.Context, deliveryID, courierID string, eta time.Time) error {
			assignCallCount++
			return nil
		},
	}
	pub := &mockPublisher{}

	svc := service.NewDeliveryService(repo, pub, nil, nil, true)
	err := svc.RetryPendingDeliveries(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if assignCallCount != 1 {
		t.Errorf("expected 1 assignment (skipping no-zone delivery), got %d", assignCallCount)
	}
}

// ---------------------------------------------------------------------------
// Phase 13 Plan 02 — ListAvailableOrders items pass-through (CRDR-07)
// ---------------------------------------------------------------------------

// TestListAvailableOrders_ItemsPassThrough verifies that AvailableOrderEnriched carries
// the item list from order-service OrderPreviewSlim.Items (CRDR-07).
func TestListAvailableOrders_ItemsPassThrough(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	repoRows := []domain.AvailableOrderRow{
		{OrderID: "order-1", DeliveryID: "del-1", ZoneName: "Center", CreatedAt: now},
	}

	repo := &mockDeliveryRepo{
		listAvailableForCourierFn: func(ctx context.Context, courierID, cursor string, limit int32) ([]domain.AvailableOrderRow, string, error) {
			return repoRows, "", nil
		},
	}
	pub := &mockPublisher{}
	oc := &mockOrderClient{
		getOrdersByIDsFn: func(ctx context.Context, in *orderv1.GetOrdersByIDsRequest) (*orderv1.GetOrdersByIDsResponse, error) {
			return &orderv1.GetOrdersByIDsResponse{
				Orders: []*orderv1.OrderPreviewSlim{
					{
						OrderId:         "order-1",
						TotalPrice:      780.0,
						ItemsCount:      2,
						DeliveryAddress: "ул. Тест, 1",
						Items: []*orderv1.OrderItem{
							{Name: "Пицца", Quantity: 2, Price: 390.0},
						},
					},
				},
			}, nil
		},
	}

	svc := service.NewDeliveryService(repo, pub, nil, oc, true)
	result, _, err := svc.ListAvailableOrders(context.Background(), "courier-1", "", 20)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if len(result[0].Items) != 1 {
		t.Fatalf("expected 1 item in enriched order, got %d", len(result[0].Items))
	}
	if result[0].Items[0].Name != "Пицца" {
		t.Errorf("expected item name 'Пицца', got %q", result[0].Items[0].Name)
	}
	if result[0].Items[0].Quantity != 2 {
		t.Errorf("expected quantity 2, got %d", result[0].Items[0].Quantity)
	}
	if result[0].Items[0].Price != 390.0 {
		t.Errorf("expected price 390.0, got %v", result[0].Items[0].Price)
	}
}

// ---------------------------------------------------------------------------
// Phase 13 Plan 02 — GetCourierProfile service tests (RATE-05)
// ---------------------------------------------------------------------------

// TestGetCourierProfile_Success verifies aggregation of rating + name from order-service.
func TestGetCourierProfile_Success(t *testing.T) {
	repo := &mockDeliveryRepo{}
	pub := &mockPublisher{}
	ratingRepo := &mockRatingRepo{
		getCourierRatingFn: func(ctx context.Context, courierID string) (*domain.CourierRatingResult, error) {
			return &domain.CourierRatingResult{AverageStars: 4.5, TotalRatings: 12}, nil
		},
	}
	oc := &mockOrderClient{
		getUsersByIDsFn: func(ctx context.Context, in *orderv1.GetUsersByIDsRequest) (*orderv1.GetUsersByIDsResponse, error) {
			return &orderv1.GetUsersByIDsResponse{
				Users: []*orderv1.UserSlim{{FirstName: "Иван", LastName: "Иванов"}},
			}, nil
		},
	}

	svc := service.NewDeliveryService(repo, pub, ratingRepo, oc, true)
	profile, err := svc.GetCourierProfile(context.Background(), "courier-1")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if profile.FirstName != "Иван" {
		t.Errorf("expected FirstName 'Иван', got %q", profile.FirstName)
	}
	if profile.LastName != "Иванов" {
		t.Errorf("expected LastName 'Иванов', got %q", profile.LastName)
	}
	if profile.AverageStars != 4.5 {
		t.Errorf("expected AverageStars 4.5, got %v", profile.AverageStars)
	}
	if profile.TotalRatings != 12 {
		t.Errorf("expected TotalRatings 12, got %d", profile.TotalRatings)
	}
}

// TestGetCourierProfile_CourierNotFound verifies ErrNotFound when GetUsersByIDs returns empty users.
func TestGetCourierProfile_CourierNotFound(t *testing.T) {
	repo := &mockDeliveryRepo{}
	pub := &mockPublisher{}
	ratingRepo := &mockRatingRepo{
		getCourierRatingFn: func(ctx context.Context, courierID string) (*domain.CourierRatingResult, error) {
			return &domain.CourierRatingResult{AverageStars: 0, TotalRatings: 0}, nil
		},
	}
	oc := &mockOrderClient{
		getUsersByIDsFn: func(ctx context.Context, in *orderv1.GetUsersByIDsRequest) (*orderv1.GetUsersByIDsResponse, error) {
			return &orderv1.GetUsersByIDsResponse{Users: nil}, nil
		},
	}

	svc := service.NewDeliveryService(repo, pub, ratingRepo, oc, true)
	_, err := svc.GetCourierProfile(context.Background(), "courier-unknown")

	if err == nil {
		t.Fatal("expected error for unknown courier, got nil")
	}
	if !errors.Is(err, pkgerrors.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

// TestGetCourierProfile_NilOrderClient verifies ErrNotFound (no panic) when orderClient is nil (Pitfall 6).
func TestGetCourierProfile_NilOrderClient(t *testing.T) {
	repo := &mockDeliveryRepo{}
	pub := &mockPublisher{}
	ratingRepo := &mockRatingRepo{
		getCourierRatingFn: func(ctx context.Context, courierID string) (*domain.CourierRatingResult, error) {
			return &domain.CourierRatingResult{AverageStars: 0, TotalRatings: 0}, nil
		},
	}

	svc := service.NewDeliveryService(repo, pub, ratingRepo, nil, true) // nil orderClient
	_, err := svc.GetCourierProfile(context.Background(), "courier-1")

	if err == nil {
		t.Fatal("expected error when orderClient is nil, got nil")
	}
	if !errors.Is(err, pkgerrors.ErrNotFound) {
		t.Errorf("expected ErrNotFound when orderClient nil, got: %v", err)
	}
}
