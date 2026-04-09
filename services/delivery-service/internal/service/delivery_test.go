package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	pkgerrors "github.com/mozgovojnikita/delivery-tracker/pkg/errors"
	"github.com/mozgovojnikita/delivery-tracker/services/delivery-service/internal/domain"
	"github.com/mozgovojnikita/delivery-tracker/services/delivery-service/internal/service"
)

// mockDeliveryRepo is a manual mock implementing repository.DeliveryRepository.
type mockDeliveryRepo struct {
	createDeliveryFn      func(ctx context.Context, orderID, zoneID string, lat, lng float64) (*domain.Delivery, error)
	getDeliveryByIDFn     func(ctx context.Context, id string) (*domain.Delivery, error)
	getDeliveryByOrderIDFn func(ctx context.Context, orderID string) (*domain.Delivery, error)
	updateStatusFn        func(ctx context.Context, id string, status domain.DeliveryStatus) error
	assignCourierFn       func(ctx context.Context, deliveryID, courierID string, eta time.Time) error
	listPendingFn         func(ctx context.Context) ([]*domain.Delivery, error)

	findNearestCourierFn func(ctx context.Context, zoneID string, lat, lng float64) (*domain.CourierCandidate, error)
	countActiveFn        func(ctx context.Context, courierID string) (int, error)

	createZoneFn          func(ctx context.Context, name, polygonGeoJSON string) (*domain.DeliveryZone, error)
	listZonesFn           func(ctx context.Context, limit, offset int) ([]*domain.DeliveryZone, int, error)
	findZoneByPointFn     func(ctx context.Context, lat, lng float64) (*domain.DeliveryZone, error)

	upsertCourierFn       func(ctx context.Context, courierID string) error
	assignCourierToZoneFn func(ctx context.Context, courierID, zoneID string) error

	calculateETAFn func(ctx context.Context, zoneID string, lat, lng float64) (time.Time, error)
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

func (m *mockDeliveryRepo) CalculateETA(ctx context.Context, zoneID string, lat, lng float64) (time.Time, error) {
	if m.calculateETAFn != nil {
		return m.calculateETAFn(ctx, zoneID, lat, lng)
	}
	return time.Now().Add(30 * time.Minute), nil
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

	svc := service.NewDeliveryService(repo, pub, nil)
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

	svc := service.NewDeliveryService(repo, pub, nil)
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

	svc := service.NewDeliveryService(repo, pub, nil)
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

	svc := service.NewDeliveryService(repo, pub, nil)
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

	svc := service.NewDeliveryService(repo, pub, nil)
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

	svc := service.NewDeliveryService(repo, pub, nil)
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

	svc := service.NewDeliveryService(repo, pub, nil)
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

	svc := service.NewDeliveryService(repo, pub, nil)
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

	svc := service.NewDeliveryService(repo, pub, nil)
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

	svc := service.NewDeliveryService(repo, pub, nil)
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

	svc := service.NewDeliveryService(repo, pub, nil)
	err := svc.RetryPendingDeliveries(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if assignCallCount != 1 {
		t.Errorf("expected 1 assignment (skipping no-zone delivery), got %d", assignCallCount)
	}
}
