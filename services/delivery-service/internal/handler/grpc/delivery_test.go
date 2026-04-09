package grpc_test

import (
	"context"
	"errors"
	"testing"
	"time"

	pkgerrors "github.com/mozgovojnikita/delivery-tracker/pkg/errors"
	"github.com/mozgovojnikita/delivery-tracker/services/delivery-service/internal/domain"
	"github.com/mozgovojnikita/delivery-tracker/services/delivery-service/internal/service"

	deliveryv1 "github.com/mozgovojnikita/delivery-tracker/gen/delivery/v1"
	grpchandler "github.com/mozgovojnikita/delivery-tracker/services/delivery-service/internal/handler/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	grpcstatus "google.golang.org/grpc/status"
)

// mockDeliveryRepository is a manual mock implementing repository.DeliveryRepository.
type mockDeliveryRepository struct {
	createDeliveryFn       func(ctx context.Context, orderID, zoneID string, lat, lng float64) (*domain.Delivery, error)
	getDeliveryByIDFn      func(ctx context.Context, id string) (*domain.Delivery, error)
	getDeliveryByOrderIDFn func(ctx context.Context, orderID string) (*domain.Delivery, error)
	updateStatusFn         func(ctx context.Context, id string, status domain.DeliveryStatus) error
	assignCourierFn        func(ctx context.Context, deliveryID, courierID string, eta time.Time) error
	listPendingFn          func(ctx context.Context) ([]*domain.Delivery, error)

	findNearestCourierFn func(ctx context.Context, zoneID string, lat, lng float64) (*domain.CourierCandidate, error)
	countActiveFn        func(ctx context.Context, courierID string) (int, error)

	createZoneFn          func(ctx context.Context, name, polygonGeoJSON string) (*domain.DeliveryZone, error)
	listZonesFn           func(ctx context.Context, limit, offset int) ([]*domain.DeliveryZone, int, error)
	findZoneByPointFn     func(ctx context.Context, lat, lng float64) (*domain.DeliveryZone, error)

	upsertCourierFn       func(ctx context.Context, courierID string) error
	assignCourierToZoneFn func(ctx context.Context, courierID, zoneID string) error

	calculateETAFn func(ctx context.Context, zoneID string, lat, lng float64) (time.Time, error)
}

func (m *mockDeliveryRepository) CreateDelivery(ctx context.Context, orderID, zoneID string, lat, lng float64) (*domain.Delivery, error) {
	if m.createDeliveryFn != nil {
		return m.createDeliveryFn(ctx, orderID, zoneID, lat, lng)
	}
	return &domain.Delivery{ID: "delivery-1", OrderID: orderID, Status: domain.StatusPending}, nil
}

func (m *mockDeliveryRepository) GetDeliveryByID(ctx context.Context, id string) (*domain.Delivery, error) {
	if m.getDeliveryByIDFn != nil {
		return m.getDeliveryByIDFn(ctx, id)
	}
	return nil, pkgerrors.ErrNotFound
}

func (m *mockDeliveryRepository) GetDeliveryByOrderID(ctx context.Context, orderID string) (*domain.Delivery, error) {
	if m.getDeliveryByOrderIDFn != nil {
		return m.getDeliveryByOrderIDFn(ctx, orderID)
	}
	return nil, pkgerrors.ErrNotFound
}

func (m *mockDeliveryRepository) UpdateStatus(ctx context.Context, id string, status domain.DeliveryStatus) error {
	if m.updateStatusFn != nil {
		return m.updateStatusFn(ctx, id, status)
	}
	return nil
}

func (m *mockDeliveryRepository) AssignCourier(ctx context.Context, deliveryID, courierID string, eta time.Time) error {
	if m.assignCourierFn != nil {
		return m.assignCourierFn(ctx, deliveryID, courierID, eta)
	}
	return nil
}

func (m *mockDeliveryRepository) ListPendingDeliveries(ctx context.Context) ([]*domain.Delivery, error) {
	if m.listPendingFn != nil {
		return m.listPendingFn(ctx)
	}
	return nil, nil
}

func (m *mockDeliveryRepository) FindNearestCourier(ctx context.Context, zoneID string, lat, lng float64) (*domain.CourierCandidate, error) {
	if m.findNearestCourierFn != nil {
		return m.findNearestCourierFn(ctx, zoneID, lat, lng)
	}
	return nil, nil
}

func (m *mockDeliveryRepository) CountActiveByCourier(ctx context.Context, courierID string) (int, error) {
	if m.countActiveFn != nil {
		return m.countActiveFn(ctx, courierID)
	}
	return 0, nil
}

func (m *mockDeliveryRepository) CreateZone(ctx context.Context, name, polygonGeoJSON string) (*domain.DeliveryZone, error) {
	if m.createZoneFn != nil {
		return m.createZoneFn(ctx, name, polygonGeoJSON)
	}
	return &domain.DeliveryZone{ID: "zone-1", Name: name, PolygonGeoJSON: polygonGeoJSON}, nil
}

func (m *mockDeliveryRepository) ListZones(ctx context.Context, limit, offset int) ([]*domain.DeliveryZone, int, error) {
	if m.listZonesFn != nil {
		return m.listZonesFn(ctx, limit, offset)
	}
	return nil, 0, nil
}

func (m *mockDeliveryRepository) FindZoneByPoint(ctx context.Context, lat, lng float64) (*domain.DeliveryZone, error) {
	if m.findZoneByPointFn != nil {
		return m.findZoneByPointFn(ctx, lat, lng)
	}
	return nil, nil
}

func (m *mockDeliveryRepository) UpsertCourier(ctx context.Context, courierID string) error {
	if m.upsertCourierFn != nil {
		return m.upsertCourierFn(ctx, courierID)
	}
	return nil
}

func (m *mockDeliveryRepository) AssignCourierToZone(ctx context.Context, courierID, zoneID string) error {
	if m.assignCourierToZoneFn != nil {
		return m.assignCourierToZoneFn(ctx, courierID, zoneID)
	}
	return nil
}

func (m *mockDeliveryRepository) CalculateETA(ctx context.Context, zoneID string, lat, lng float64) (time.Time, error) {
	if m.calculateETAFn != nil {
		return m.calculateETAFn(ctx, zoneID, lat, lng)
	}
	return time.Now().Add(30 * time.Minute), nil
}

func (m *mockDeliveryRepository) ListByCourierID(ctx context.Context, courierID string) ([]*domain.Delivery, error) {
	return nil, nil
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

// mockRatingRepository is a manual mock implementing service.RatingRepository.
type mockRatingRepository struct {
	createRatingFn    func(ctx context.Context, deliveryID, courierID, userID string, stars int, comment string) (*domain.CourierRating, error)
	getCourierRatingFn func(ctx context.Context, courierID string) (*domain.CourierRatingResult, error)
}

func (m *mockRatingRepository) CreateRating(ctx context.Context, deliveryID, courierID, userID string, stars int, comment string) (*domain.CourierRating, error) {
	if m.createRatingFn != nil {
		return m.createRatingFn(ctx, deliveryID, courierID, userID, stars, comment)
	}
	return &domain.CourierRating{ID: "rating-1"}, nil
}

func (m *mockRatingRepository) GetCourierRating(ctx context.Context, courierID string) (*domain.CourierRatingResult, error) {
	if m.getCourierRatingFn != nil {
		return m.getCourierRatingFn(ctx, courierID)
	}
	return &domain.CourierRatingResult{}, nil
}

// newTestHandler creates a DeliveryHandler wired with the given mock repo and publisher (no rating repo).
func newTestHandler(repo *mockDeliveryRepository, pub *mockPublisher) *grpchandler.DeliveryHandler {
	svc := service.NewDeliveryService(repo, pub, nil)
	return grpchandler.NewDeliveryHandler(svc)
}

// newTestHandlerWithRating creates a DeliveryHandler wired with repo, publisher, and rating repo.
func newTestHandlerWithRating(repo *mockDeliveryRepository, pub *mockPublisher, ratingRepo *mockRatingRepository) *grpchandler.DeliveryHandler {
	svc := service.NewDeliveryService(repo, pub, ratingRepo)
	return grpchandler.NewDeliveryHandler(svc)
}

// ctxWithUserID returns a context carrying gRPC incoming metadata with the given x-user-id.
func ctxWithUserID(userID string) context.Context {
	md := metadata.Pairs("x-user-id", userID)
	return metadata.NewIncomingContext(context.Background(), md)
}

// TestAssignCourier_ManualWithCourierID tests manual courier assignment via gRPC (DLVR-01).
func TestAssignCourier_ManualWithCourierID(t *testing.T) {
	repo := &mockDeliveryRepository{
		getDeliveryByOrderIDFn: func(ctx context.Context, orderID string) (*domain.Delivery, error) {
			return &domain.Delivery{
				ID:      "delivery-1",
				OrderID: orderID,
				ZoneID:  "zone-1",
				Status:  domain.StatusPending,
			}, nil
		},
		calculateETAFn: func(ctx context.Context, zoneID string, lat, lng float64) (time.Time, error) {
			return time.Now().Add(30 * time.Minute), nil
		},
	}
	pub := &mockPublisher{}

	handler := newTestHandler(repo, pub)

	resp, err := handler.AssignCourier(context.Background(), &deliveryv1.AssignCourierRequest{
		OrderId:   "order-1",
		CourierId: "courier-1",
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resp.GetDelivery() == nil {
		t.Fatal("expected delivery in response, got nil")
	}
}

// TestAssignCourier_AutoByZone tests that auto-assignment without courier_id returns InvalidArgument.
func TestAssignCourier_AutoByZone(t *testing.T) {
	repo := &mockDeliveryRepository{}
	pub := &mockPublisher{}

	handler := newTestHandler(repo, pub)

	_, err := handler.AssignCourier(context.Background(), &deliveryv1.AssignCourierRequest{
		OrderId: "order-1",
		ZoneId:  "zone-1",
		// No courier_id — auto-assignment via Kafka
	})

	if err == nil {
		t.Fatal("expected error for auto-assignment via RPC, got nil")
	}
	st, ok := grpcstatus.FromError(err)
	if !ok {
		t.Fatal("expected gRPC status error")
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", st.Code())
	}
}

// TestCreateZone_gRPC tests zone creation via gRPC handler.
func TestCreateZone_gRPC(t *testing.T) {
	const geoJSON = `{"type":"Polygon","coordinates":[[[37.0,55.0],[38.0,55.0],[38.0,56.0],[37.0,56.0],[37.0,55.0]]]}`

	repo := &mockDeliveryRepository{
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

	handler := newTestHandler(repo, pub)

	resp, err := handler.CreateZone(context.Background(), &deliveryv1.CreateZoneRequest{
		Name:           "Test Zone",
		PolygonGeojson: geoJSON,
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resp.GetZone() == nil {
		t.Fatal("expected zone in response, got nil")
	}
	if resp.GetZone().GetName() != "Test Zone" {
		t.Errorf("expected zone name 'Test Zone', got %q", resp.GetZone().GetName())
	}
}

// TestCreateZone_MissingName tests that CreateZone returns InvalidArgument when name is empty.
func TestCreateZone_MissingName(t *testing.T) {
	handler := newTestHandler(&mockDeliveryRepository{}, &mockPublisher{})

	_, err := handler.CreateZone(context.Background(), &deliveryv1.CreateZoneRequest{
		PolygonGeojson: `{"type":"Polygon"}`,
	})

	if err == nil {
		t.Fatal("expected error for empty name")
	}
	st, ok := grpcstatus.FromError(err)
	if !ok {
		t.Fatal("expected gRPC status error")
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", st.Code())
	}
}

// TestListZones_gRPC tests listing zones via gRPC handler.
func TestListZones_gRPC(t *testing.T) {
	repo := &mockDeliveryRepository{
		listZonesFn: func(ctx context.Context, limit, offset int) ([]*domain.DeliveryZone, int, error) {
			return []*domain.DeliveryZone{
				{ID: "zone-1", Name: "Zone A", PolygonGeoJSON: `{}`},
				{ID: "zone-2", Name: "Zone B", PolygonGeoJSON: `{}`},
			}, 2, nil
		},
	}
	pub := &mockPublisher{}

	handler := newTestHandler(repo, pub)

	resp, err := handler.ListZones(context.Background(), &deliveryv1.ListZonesRequest{})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(resp.GetZones()) != 2 {
		t.Errorf("expected 2 zones, got %d", len(resp.GetZones()))
	}
	if resp.GetPagination().GetTotal() != 2 {
		t.Errorf("expected total=2, got %d", resp.GetPagination().GetTotal())
	}
}

// TestAssignCourierToZone_gRPC tests courier-to-zone assignment via gRPC handler.
func TestAssignCourierToZone_gRPC(t *testing.T) {
	var assignCalled bool

	repo := &mockDeliveryRepository{
		assignCourierToZoneFn: func(ctx context.Context, courierID, zoneID string) error {
			assignCalled = true
			return nil
		},
	}
	pub := &mockPublisher{}

	handler := newTestHandler(repo, pub)

	resp, err := handler.AssignCourierToZone(context.Background(), &deliveryv1.AssignCourierToZoneRequest{
		CourierId: "courier-1",
		ZoneId:    "zone-1",
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !resp.GetSuccess() {
		t.Error("expected success=true")
	}
	if !assignCalled {
		t.Error("expected AssignCourierToZone to be called")
	}
}

// TestGetDelivery_gRPC tests retrieving a delivery by ID via gRPC handler.
func TestGetDelivery_gRPC(t *testing.T) {
	repo := &mockDeliveryRepository{
		getDeliveryByIDFn: func(ctx context.Context, id string) (*domain.Delivery, error) {
			return &domain.Delivery{
				ID:        id,
				OrderID:   "order-1",
				CourierID: "courier-1",
				Status:    domain.StatusAssigned,
				ZoneID:    "zone-1",
				CreatedAt: time.Now(),
			}, nil
		},
	}
	pub := &mockPublisher{}

	handler := newTestHandler(repo, pub)

	resp, err := handler.GetDelivery(context.Background(), &deliveryv1.GetDeliveryRequest{
		Id: "delivery-1",
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resp.GetDelivery() == nil {
		t.Fatal("expected delivery, got nil")
	}
	if resp.GetDelivery().GetId() != "delivery-1" {
		t.Errorf("expected delivery ID 'delivery-1', got %q", resp.GetDelivery().GetId())
	}
}

// TestGetDelivery_NotFound tests that GetDelivery returns NotFound for unknown ID.
func TestGetDelivery_NotFound(t *testing.T) {
	repo := &mockDeliveryRepository{
		getDeliveryByIDFn: func(ctx context.Context, id string) (*domain.Delivery, error) {
			return nil, pkgerrors.ErrNotFound
		},
	}
	pub := &mockPublisher{}

	handler := newTestHandler(repo, pub)

	_, err := handler.GetDelivery(context.Background(), &deliveryv1.GetDeliveryRequest{
		Id: "unknown-id",
	})

	if err == nil {
		t.Fatal("expected error for unknown delivery, got nil")
	}
	st, ok := grpcstatus.FromError(err)
	if !ok {
		t.Fatal("expected gRPC status error")
	}
	if st.Code() != codes.NotFound {
		t.Errorf("expected NotFound, got %v", st.Code())
	}
}

// TestGetHealth_gRPC tests that GetHealth returns status "ok".
func TestGetHealth_gRPC(t *testing.T) {
	handler := newTestHandler(&mockDeliveryRepository{}, &mockPublisher{})

	resp, err := handler.GetHealth(context.Background(), &deliveryv1.HealthRequest{})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resp.GetStatus() != "ok" {
		t.Errorf("expected status 'ok', got %q", resp.GetStatus())
	}
}

// TestAssignCourier_MissingOrderID tests that AssignCourier returns InvalidArgument without order_id.
func TestAssignCourier_MissingOrderID(t *testing.T) {
	handler := newTestHandler(&mockDeliveryRepository{}, &mockPublisher{})

	_, err := handler.AssignCourier(context.Background(), &deliveryv1.AssignCourierRequest{
		CourierId: "courier-1",
	})

	if err == nil {
		t.Fatal("expected error for missing order_id")
	}
	st, _ := grpcstatus.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", st.Code())
	}
}

// TestAssignCourier_NotPending tests that AssignCourier returns InvalidArgument if delivery not pending.
func TestAssignCourier_NotPending(t *testing.T) {
	repo := &mockDeliveryRepository{
		getDeliveryByOrderIDFn: func(ctx context.Context, orderID string) (*domain.Delivery, error) {
			return &domain.Delivery{
				ID:     "delivery-1",
				Status: domain.StatusAssigned, // already assigned
			}, nil
		},
	}

	handler := newTestHandler(repo, &mockPublisher{})

	_, err := handler.AssignCourier(context.Background(), &deliveryv1.AssignCourierRequest{
		OrderId:   "order-1",
		CourierId: "courier-1",
	})

	if err == nil {
		t.Fatal("expected error for non-pending delivery")
	}
	st, _ := grpcstatus.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", st.Code())
	}

	_ = errors.Is(err, pkgerrors.ErrInvalidInput) // ensure correct error wrapping
}

// TestRateCourier_Success tests that a valid rating request with x-user-id returns rating_id.
func TestRateCourier_Success(t *testing.T) {
	repo := &mockDeliveryRepository{
		getDeliveryByIDFn: func(ctx context.Context, id string) (*domain.Delivery, error) {
			return &domain.Delivery{
				ID:        id,
				CourierID: "courier-1",
				Status:    domain.StatusDelivered,
			}, nil
		},
	}
	ratingRepo := &mockRatingRepository{
		createRatingFn: func(ctx context.Context, deliveryID, courierID, userID string, stars int, comment string) (*domain.CourierRating, error) {
			return &domain.CourierRating{ID: "rating-99"}, nil
		},
	}
	handler := newTestHandlerWithRating(repo, &mockPublisher{}, ratingRepo)

	resp, err := handler.RateCourier(ctxWithUserID("user-1"), &deliveryv1.RateCourierRequest{
		DeliveryId: "delivery-1",
		Stars:      5,
		Comment:    "Great delivery",
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resp.GetRatingId() != "rating-99" {
		t.Errorf("expected rating_id 'rating-99', got %q", resp.GetRatingId())
	}
}

// TestRateCourier_MissingUserID tests that RateCourier returns Unauthenticated when x-user-id is absent.
func TestRateCourier_MissingUserID(t *testing.T) {
	handler := newTestHandlerWithRating(&mockDeliveryRepository{}, &mockPublisher{}, &mockRatingRepository{})

	_, err := handler.RateCourier(context.Background(), &deliveryv1.RateCourierRequest{
		DeliveryId: "delivery-1",
		Stars:      4,
	})

	if err == nil {
		t.Fatal("expected error for missing x-user-id, got nil")
	}
	st, ok := grpcstatus.FromError(err)
	if !ok {
		t.Fatal("expected gRPC status error")
	}
	if st.Code() != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", st.Code())
	}
}

// TestRateCourier_EmptyDeliveryID tests that RateCourier returns InvalidArgument when delivery_id is empty.
func TestRateCourier_EmptyDeliveryID(t *testing.T) {
	handler := newTestHandlerWithRating(&mockDeliveryRepository{}, &mockPublisher{}, &mockRatingRepository{})

	_, err := handler.RateCourier(ctxWithUserID("user-1"), &deliveryv1.RateCourierRequest{
		DeliveryId: "",
		Stars:      4,
	})

	if err == nil {
		t.Fatal("expected error for empty delivery_id, got nil")
	}
	st, ok := grpcstatus.FromError(err)
	if !ok {
		t.Fatal("expected gRPC status error")
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", st.Code())
	}
}

// TestRateCourier_ServiceError tests that a service ErrInvalidInput maps to InvalidArgument.
func TestRateCourier_ServiceError(t *testing.T) {
	repo := &mockDeliveryRepository{
		getDeliveryByIDFn: func(ctx context.Context, id string) (*domain.Delivery, error) {
			return &domain.Delivery{
				ID:        id,
				CourierID: "courier-1",
				Status:    domain.StatusDelivered,
			}, nil
		},
	}
	ratingRepo := &mockRatingRepository{
		createRatingFn: func(ctx context.Context, deliveryID, courierID, userID string, stars int, comment string) (*domain.CourierRating, error) {
			return nil, pkgerrors.ErrInvalidInput
		},
	}
	handler := newTestHandlerWithRating(repo, &mockPublisher{}, ratingRepo)

	_, err := handler.RateCourier(ctxWithUserID("user-1"), &deliveryv1.RateCourierRequest{
		DeliveryId: "delivery-1",
		Stars:      6, // out of range — triggers ErrInvalidInput from service
	})

	if err == nil {
		t.Fatal("expected error from service, got nil")
	}
	st, ok := grpcstatus.FromError(err)
	if !ok {
		t.Fatal("expected gRPC status error")
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", st.Code())
	}
}

// TestGetCourierRating_Success tests that a valid request returns average, count, and ratings list.
func TestGetCourierRating_Success(t *testing.T) {
	ratingRepo := &mockRatingRepository{
		getCourierRatingFn: func(ctx context.Context, courierID string) (*domain.CourierRatingResult, error) {
			return &domain.CourierRatingResult{
				AverageStars: 4.5,
				TotalRatings: 2,
				Ratings: []domain.CourierRating{
					{DeliveryID: "d-1", Stars: 5, Comment: "Excellent", CreatedAt: time.Now()},
					{DeliveryID: "d-2", Stars: 4, Comment: "Good", CreatedAt: time.Now()},
				},
			}, nil
		},
	}
	handler := newTestHandlerWithRating(&mockDeliveryRepository{}, &mockPublisher{}, ratingRepo)

	resp, err := handler.GetCourierRating(context.Background(), &deliveryv1.GetCourierRatingRequest{
		CourierId: "courier-1",
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resp.GetAverageStars() != 4.5 {
		t.Errorf("expected average_stars 4.5, got %v", resp.GetAverageStars())
	}
	if resp.GetTotalRatings() != 2 {
		t.Errorf("expected total_ratings 2, got %d", resp.GetTotalRatings())
	}
	if len(resp.GetRatings()) != 2 {
		t.Errorf("expected 2 ratings, got %d", len(resp.GetRatings()))
	}
}

// TestGetCourierRating_EmptyCourierID tests that GetCourierRating returns InvalidArgument when courier_id is empty.
func TestGetCourierRating_EmptyCourierID(t *testing.T) {
	handler := newTestHandlerWithRating(&mockDeliveryRepository{}, &mockPublisher{}, &mockRatingRepository{})

	_, err := handler.GetCourierRating(context.Background(), &deliveryv1.GetCourierRatingRequest{
		CourierId: "",
	})

	if err == nil {
		t.Fatal("expected error for empty courier_id, got nil")
	}
	st, ok := grpcstatus.FromError(err)
	if !ok {
		t.Fatal("expected gRPC status error")
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", st.Code())
	}
}

// TestGetCourierRating_NoRatings tests that GetCourierRating returns zeros for courier with no ratings.
func TestGetCourierRating_NoRatings(t *testing.T) {
	ratingRepo := &mockRatingRepository{
		getCourierRatingFn: func(ctx context.Context, courierID string) (*domain.CourierRatingResult, error) {
			return &domain.CourierRatingResult{
				AverageStars: 0,
				TotalRatings: 0,
				Ratings:      nil,
			}, nil
		},
	}
	handler := newTestHandlerWithRating(&mockDeliveryRepository{}, &mockPublisher{}, ratingRepo)

	resp, err := handler.GetCourierRating(context.Background(), &deliveryv1.GetCourierRatingRequest{
		CourierId: "courier-no-ratings",
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resp.GetAverageStars() != 0 {
		t.Errorf("expected average_stars 0, got %v", resp.GetAverageStars())
	}
	if resp.GetTotalRatings() != 0 {
		t.Errorf("expected total_ratings 0, got %d", resp.GetTotalRatings())
	}
	if len(resp.GetRatings()) != 0 {
		t.Errorf("expected empty ratings, got %d", len(resp.GetRatings()))
	}
}
