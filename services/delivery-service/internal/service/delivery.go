package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/mozgovojnikita/delivery-tracker/pkg/kafka"
	pkgerrors "github.com/mozgovojnikita/delivery-tracker/pkg/errors"
	"github.com/mozgovojnikita/delivery-tracker/services/delivery-service/internal/domain"
	"github.com/mozgovojnikita/delivery-tracker/services/delivery-service/internal/repository"
)

// OrderCreatedInput holds the fields from a kafka.OrderCreatedEvent needed by the service.
// This avoids a direct kafka package dependency in test assertions.
type OrderCreatedInput struct {
	OrderID string
	UserID  string
	Lat     float64
	Lng     float64
}

// DeliveryService handles delivery lifecycle operations.
type DeliveryService struct {
	repo       repository.DeliveryRepository
	publisher  kafka.EventPublisher
	ratingRepo RatingRepository
}

// NewDeliveryService creates a new DeliveryService with the given repository, event publisher, and optional rating repository.
func NewDeliveryService(repo repository.DeliveryRepository, publisher kafka.EventPublisher, ratingRepo RatingRepository) *DeliveryService {
	return &DeliveryService{
		repo:       repo,
		publisher:  publisher,
		ratingRepo: ratingRepo,
	}
}

// HandleOrderCreated processes an OrderCreatedEvent:
// finds zone, creates delivery, then auto-assigns courier if available.
func (s *DeliveryService) HandleOrderCreated(ctx context.Context, input OrderCreatedInput) error {
	// 1. Find zone by coordinates.
	zone, err := s.repo.FindZoneByPoint(ctx, input.Lat, input.Lng)
	if err != nil {
		return fmt.Errorf("find zone by point: %w", err)
	}

	zoneID := ""
	if zone != nil {
		zoneID = zone.ID
	}

	// 2. Create delivery record.
	delivery, err := s.repo.CreateDelivery(ctx, input.OrderID, zoneID, input.Lat, input.Lng)
	if err != nil {
		return fmt.Errorf("create delivery: %w", err)
	}

	// 3. If no zone found, leave delivery as pending (D-04).
	if zone == nil {
		return nil
	}

	// 4. Attempt to auto-assign courier.
	return s.assignCourierToDelivery(ctx, delivery, input.Lat, input.Lng, input.UserID)
}

// assignCourierToDelivery attempts to find the nearest available courier and assign them.
// If no courier is available, the delivery remains pending (D-04).
func (s *DeliveryService) assignCourierToDelivery(ctx context.Context, delivery *domain.Delivery, lat, lng float64, userID string) error {
	// 1. Find nearest available courier.
	courier, err := s.repo.FindNearestCourier(ctx, delivery.ZoneID, lat, lng)
	if err != nil {
		return fmt.Errorf("find nearest courier: %w", err)
	}
	// No courier available - leave delivery as pending.
	if courier == nil {
		return nil
	}

	// 2. Calculate ETA.
	eta, err := s.repo.CalculateETA(ctx, delivery.ZoneID, lat, lng)
	if err != nil {
		return fmt.Errorf("calculate eta: %w", err)
	}

	// 3. Assign courier in database.
	if err := s.repo.AssignCourier(ctx, delivery.ID, courier.CourierID, eta); err != nil {
		return fmt.Errorf("assign courier: %w", err)
	}

	// 4. Publish DeliveryAssignedEvent.
	event := kafka.DeliveryAssignedEvent{
		EventID:    uuid.New().String(),
		DeliveryID: delivery.ID,
		OrderID:    delivery.OrderID,
		UserID:     userID,
		CourierID:  courier.CourierID,
		ZoneID:     delivery.ZoneID,
		ETA:        eta.Format(time.RFC3339),
		AssignedAt: time.Now().UTC().Format(time.RFC3339),
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal delivery assigned event: %w", err)
	}
	if err := s.publisher.Publish(ctx, kafka.TopicDeliveryAssigned, delivery.OrderID, payload); err != nil {
		return fmt.Errorf("publish delivery assigned event: %w", err)
	}
	return nil
}

// ManualAssignCourier allows admin to manually assign a courier to an order (DLVR-01).
// The delivery must be in pending status.
func (s *DeliveryService) ManualAssignCourier(ctx context.Context, orderID, courierID string) (*domain.Delivery, error) {
	// 1. Get delivery by order ID.
	delivery, err := s.repo.GetDeliveryByOrderID(ctx, orderID)
	if err != nil {
		return nil, err
	}

	// 2. Validate delivery is in pending status.
	if delivery.Status != domain.StatusPending {
		return nil, fmt.Errorf("delivery for order %s is not pending (status: %s): %w", orderID, delivery.Status, pkgerrors.ErrInvalidInput)
	}

	// 3. Upsert courier (ensure exists).
	if err := s.repo.UpsertCourier(ctx, courierID); err != nil {
		return nil, fmt.Errorf("upsert courier: %w", err)
	}

	// 4. Calculate ETA (use zero coords since we do manual assignment).
	var eta time.Time
	if delivery.ZoneID != "" {
		eta, err = s.repo.CalculateETA(ctx, delivery.ZoneID, 0, 0)
		if err != nil {
			// Non-fatal: use a default ETA of 60 minutes if calculation fails.
			eta = time.Now().Add(60 * time.Minute)
		}
	} else {
		eta = time.Now().Add(60 * time.Minute)
	}

	// 5. Assign courier in database.
	if err := s.repo.AssignCourier(ctx, delivery.ID, courierID, eta); err != nil {
		return nil, fmt.Errorf("assign courier: %w", err)
	}
	delivery.CourierID = courierID
	delivery.Status = domain.StatusAssigned
	delivery.EstimatedDelivery = eta

	// 6. Publish DeliveryAssignedEvent.
	event := kafka.DeliveryAssignedEvent{
		EventID:    uuid.New().String(),
		DeliveryID: delivery.ID,
		OrderID:    delivery.OrderID,
		UserID:     "", // not available in manual assignment context
		CourierID:  courierID,
		ZoneID:     delivery.ZoneID,
		ETA:        eta.Format(time.RFC3339),
		AssignedAt: time.Now().UTC().Format(time.RFC3339),
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("marshal delivery assigned event: %w", err)
	}
	if err := s.publisher.Publish(ctx, kafka.TopicDeliveryAssigned, delivery.OrderID, payload); err != nil {
		return nil, fmt.Errorf("publish delivery assigned event: %w", err)
	}
	return delivery, nil
}

// CreateZone creates a new delivery zone from a GeoJSON polygon (DLVR-03).
func (s *DeliveryService) CreateZone(ctx context.Context, name, polygonGeoJSON string) (*domain.DeliveryZone, error) {
	if name == "" {
		return nil, fmt.Errorf("zone name is required: %w", pkgerrors.ErrInvalidInput)
	}
	if polygonGeoJSON == "" {
		return nil, fmt.Errorf("polygon_geojson is required: %w", pkgerrors.ErrInvalidInput)
	}
	return s.repo.CreateZone(ctx, name, polygonGeoJSON)
}

// ListZones returns a paginated list of delivery zones (DLVR-03).
// Sanitizes page and pageSize to safe defaults.
func (s *DeliveryService) ListZones(ctx context.Context, page, pageSize int) ([]*domain.DeliveryZone, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.ListZones(ctx, pageSize, offset)
}

// AssignCourierToZone assigns a courier to a delivery zone (DLVR-04).
func (s *DeliveryService) AssignCourierToZone(ctx context.Context, courierID, zoneID string) error {
	if err := s.repo.UpsertCourier(ctx, courierID); err != nil {
		return fmt.Errorf("upsert courier: %w", err)
	}
	return s.repo.AssignCourierToZone(ctx, courierID, zoneID)
}

// RetryPendingDeliveries attempts to assign couriers to pending deliveries (D-04).
// Skips deliveries without a zone_id (cannot be auto-assigned).
func (s *DeliveryService) RetryPendingDeliveries(ctx context.Context) error {
	pending, err := s.repo.ListPendingDeliveries(ctx)
	if err != nil {
		return fmt.Errorf("list pending deliveries: %w", err)
	}
	for _, delivery := range pending {
		if delivery.ZoneID == "" {
			continue // cannot auto-assign without zone
		}
		if err := s.assignCourierToDelivery(ctx, delivery, 0, 0, ""); err != nil {
			// Log but don't abort retry loop for individual failures.
			continue
		}
	}
	return nil
}

// SyncDeliveryStatus updates the delivery status to match the order status.
// Maps order statuses to delivery statuses (e.g. picked_up, in_transit, delivered, cancelled).
func (s *DeliveryService) SyncDeliveryStatus(ctx context.Context, orderID, orderStatus string) error {
	statusMap := map[string]domain.DeliveryStatus{
		"picked_up":  domain.StatusPickedUp,
		"in_transit": domain.StatusInTransit,
		"delivered":  domain.StatusDelivered,
		"cancelled":  domain.StatusFailed,
		"failed":     domain.StatusFailed,
	}
	deliveryStatus, ok := statusMap[orderStatus]
	if !ok {
		return nil // irrelevant status, ignore
	}
	delivery, err := s.repo.GetDeliveryByOrderID(ctx, orderID)
	if err != nil {
		return fmt.Errorf("get delivery by order: %w", err)
	}
	if err := domain.ValidateTransition(delivery.Status, deliveryStatus); err != nil {
		return nil // invalid transition, skip silently
	}
	return s.repo.UpdateStatus(ctx, delivery.ID, deliveryStatus)
}

// GetDelivery retrieves a delivery by its ID.
func (s *DeliveryService) GetDelivery(ctx context.Context, id string) (*domain.Delivery, error) {
	return s.repo.GetDeliveryByID(ctx, id)
}

// CalculateETA returns the estimated delivery time for the given zone and coordinates.
func (s *DeliveryService) CalculateETA(ctx context.Context, zoneID string, lat, lng float64) (time.Time, error) {
	return s.repo.CalculateETA(ctx, zoneID, lat, lng)
}

// ListDeliveriesByCourier returns all active deliveries for a courier (TRAK-05).
func (s *DeliveryService) ListDeliveriesByCourier(ctx context.Context, courierID string) ([]*domain.Delivery, error) {
	if courierID == "" {
		return nil, fmt.Errorf("courier_id is required: %w", pkgerrors.ErrInvalidInput)
	}
	return s.repo.ListByCourierID(ctx, courierID)
}
