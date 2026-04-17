package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	orderv1 "github.com/mozgovojnikita/delivery-tracker/gen/order/v1"
	pkgerrors "github.com/mozgovojnikita/delivery-tracker/pkg/errors"
	"github.com/mozgovojnikita/delivery-tracker/pkg/kafka"
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
	repo        repository.DeliveryRepository
	publisher   kafka.EventPublisher
	ratingRepo  RatingRepository
	orderClient OrderClient // nilable — if nil, ListAvailableOrders returns zero-valued enrichment
	autoAssign  bool        // v2.0 manual-selection model: v1.0 DLVR-02 auto-assign gated; default OFF (manual accept only)
}

// NewDeliveryService creates a new DeliveryService with the given repository, event publisher,
// optional rating repository, optional order-service client, and the v1.0 DLVR-02 auto-assign
// feature flag (OFF for v2.0 manual-courier-selection product; ON to preserve legacy auto-assign
// behavior — controlled by env DELIVERY_AUTO_ASSIGN, see cmd/main.go).
func NewDeliveryService(repo repository.DeliveryRepository, publisher kafka.EventPublisher, ratingRepo RatingRepository, orderClient OrderClient, autoAssign bool) *DeliveryService {
	return &DeliveryService{
		repo:        repo,
		publisher:   publisher,
		ratingRepo:  ratingRepo,
		orderClient: orderClient,
		autoAssign:  autoAssign,
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
//
// v2.0 manual-selection model: when autoAssign is OFF (default), this is a no-op — the
// delivery stays pending and a courier must accept it manually via AcceptOrder (BKND-03).
// v1.0 DLVR-02 behavior is preserved when autoAssign is ON (env DELIVERY_AUTO_ASSIGN=true).
// This single guard covers both call sites: HandleOrderCreated and RetryPendingDeliveries.
func (s *DeliveryService) assignCourierToDelivery(ctx context.Context, delivery *domain.Delivery, lat, lng float64, userID string) error {
	if !s.autoAssign {
		return nil
	}
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

// AcceptOrder implements BKND-03: courier self-assigns a pending delivery.
// Guards: courier must be online (D-09), active count < MaxActiveDeliveries (D-09).
// Atomic claim via repo.AcceptByCourier (conditional UPDATE, D-07).
// Side effects on success: courier→busy (D-02/D-08), order→confirmed (D-08), Kafka publish (RESEARCH §Pattern 4).
func (s *DeliveryService) AcceptOrder(ctx context.Context, courierID, deliveryID string) (*domain.Delivery, error) {
	// Guard 1: courier must not be offline (D-09).
	courier, err := s.repo.GetCourier(ctx, courierID)
	if err != nil {
		return nil, fmt.Errorf("get courier: %w", err)
	}
	if courier.Status == "offline" {
		return nil, domain.ErrCourierOffline
	}

	// Guard 2: max active deliveries (D-09).
	active, err := s.repo.CountActiveByCourier(ctx, courierID)
	if err != nil {
		return nil, fmt.Errorf("count active: %w", err)
	}
	if active >= domain.MaxActiveDeliveries {
		return nil, domain.ErrMaxActiveReached
	}

	// Default ETA: now + 30 minutes.
	eta := time.Now().Add(30 * time.Minute)

	// Atomic claim (D-07): conditional UPDATE..RETURNING — no explicit locks.
	delivery, ok, err := s.repo.AcceptByCourier(ctx, deliveryID, courierID, eta)
	if err != nil {
		return nil, fmt.Errorf("accept by courier: %w", err)
	}
	if !ok {
		return nil, domain.ErrAlreadyTaken
	}

	// Side effect 1: set courier status = 'busy' (D-02 system-managed, D-08).
	if _, err := s.repo.UpdateCourierStatus(ctx, courierID, "busy"); err != nil {
		slog.WarnContext(ctx, "set courier busy failed (delivery already committed)",
			"error", err, "courier_id", courierID)
	}

	// Side effect 2: notify order-service → order.status = 'confirmed' (D-08 cross-service).
	// Graceful: if RPC fails, log and continue — delivery is committed; eventual consistency via Kafka.
	if s.orderClient != nil {
		if _, err := s.orderClient.UpdateOrderStatus(ctx, &orderv1.UpdateOrderStatusRequest{
			Id:     delivery.OrderID,
			Status: orderv1.OrderStatus_ORDER_STATUS_CONFIRMED,
		}); err != nil {
			slog.WarnContext(ctx, "update order status failed (manual reconciliation may be needed)",
				"error", err, "order_id", delivery.OrderID)
		}
	}

	// Kafka publish-after-commit (D-08 + RESEARCH §Pattern 4 — REUSE TopicDeliveryAssigned, NOT a new topic).
	event := kafka.DeliveryAssignedEvent{
		EventID:    uuid.New().String(),
		DeliveryID: delivery.ID,
		OrderID:    delivery.OrderID,
		CourierID:  courierID,
		ZoneID:     delivery.ZoneID,
		ETA:        eta.Format(time.RFC3339),
		AssignedAt: time.Now().UTC().Format(time.RFC3339),
	}
	payload, err := json.Marshal(event)
	if err != nil {
		slog.WarnContext(ctx, "marshal delivery assigned event failed", "error", err, "delivery_id", delivery.ID)
	} else if err := s.publisher.Publish(ctx, kafka.TopicDeliveryAssigned, delivery.OrderID, payload); err != nil {
		slog.WarnContext(ctx, "publish delivery.assigned failed", "error", err, "delivery_id", delivery.ID)
	}

	return delivery, nil
}

// ListAvailableOrders returns FIFO-ordered, zone-scoped, paginated deliveries available
// for the courier to accept (BKND-02). Each row is enriched with total_price and items_count
// from order-service via OrderClient. If order-service is unavailable, graceful degradation
// returns rows with zero-valued financial fields (D-06).
func (s *DeliveryService) ListAvailableOrders(ctx context.Context, courierID, cursor string, limit int32) ([]domain.AvailableOrderEnriched, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20 // D-05 default
	}

	rows, nextCursor, err := s.repo.ListAvailableForCourier(ctx, courierID, cursor, limit)
	if err != nil {
		return nil, "", fmt.Errorf("list available: %w", err)
	}
	if len(rows) == 0 {
		return nil, "", nil
	}

	// Bulk enrichment via order-service.
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.OrderID
	}

	var previewsByID map[string]*orderv1.OrderPreviewSlim
	if s.orderClient != nil {
		resp, oErr := s.orderClient.GetOrdersByIDs(ctx, &orderv1.GetOrdersByIDsRequest{OrderIds: ids})
		if oErr != nil {
			// Graceful degradation per RESEARCH Anti-Patterns: warn + continue with zero values.
			slog.WarnContext(ctx, "order-service unavailable, returning preview without total_price/items_count",
				"error", oErr, "courier_id", courierID)
		} else {
			previewsByID = make(map[string]*orderv1.OrderPreviewSlim, len(resp.GetOrders()))
			for _, p := range resp.GetOrders() {
				previewsByID[p.GetOrderId()] = p
			}
		}
	}

	// Merge repo data + order-service enrichment.
	out := make([]domain.AvailableOrderEnriched, 0, len(rows))
	for _, r := range rows {
		enriched := domain.AvailableOrderEnriched{
			OrderID:    r.OrderID,
			DeliveryID: r.DeliveryID,
			ZoneName:   r.ZoneName,
			CreatedAt:  r.CreatedAt,
		}
		if p, ok := previewsByID[r.OrderID]; ok {
			enriched.TotalPrice = p.GetTotalPrice()
			enriched.ItemsCount = p.GetItemsCount()
			enriched.DeliveryAddress = p.GetDeliveryAddress()
			for _, item := range p.GetItems() {
				enriched.Items = append(enriched.Items, domain.OrderItemEnriched{
					Name:     item.GetName(),
					Quantity: item.GetQuantity(),
					Price:    item.GetPrice(),
				})
			}
		}
		out = append(out, enriched)
	}
	return out, nextCursor, nil
}

// GetDeliveryByOrderID returns the delivery associated with the given order, if any (BKND-04).
// Returns pkgerrors.ErrNotFound when no delivery exists for the order.
// Public wrapper over repo.GetDeliveryByOrderID for cross-service consumption via gRPC.
func (s *DeliveryService) GetDeliveryByOrderID(ctx context.Context, orderID string) (*domain.Delivery, error) {
	return s.repo.GetDeliveryByOrderID(ctx, orderID)
}

// CourierProfileResult aggregates courier name (from order-service) and rating (own repo) (RATE-05).
type CourierProfileResult struct {
	FirstName    string
	LastName     string
	AverageStars float64
	TotalRatings int
}

// GetCourierProfile aggregates rating (own repo) + name (order-service) for public profile (RATE-05).
// Returns ErrNotFound when courier not found or orderClient is nil (Pitfall 6: graceful degradation).
func (s *DeliveryService) GetCourierProfile(ctx context.Context, courierID string) (*CourierProfileResult, error) {
	rating, err := s.ratingRepo.GetCourierRating(ctx, courierID)
	if err != nil {
		return nil, err
	}
	if s.orderClient == nil {
		return nil, fmt.Errorf("courier %s: %w", courierID, pkgerrors.ErrNotFound)
	}
	resp, err := s.orderClient.GetUsersByIDs(ctx, &orderv1.GetUsersByIDsRequest{UserIds: []string{courierID}})
	if err != nil || len(resp.GetUsers()) == 0 {
		return nil, fmt.Errorf("courier %s: %w", courierID, pkgerrors.ErrNotFound)
	}
	u := resp.GetUsers()[0]
	return &CourierProfileResult{
		FirstName:    u.GetFirstName(),
		LastName:     u.GetLastName(),
		AverageStars: rating.AverageStars,
		TotalRatings: rating.TotalRatings,
	}, nil
}

// UpdateCourierStatus updates a courier's own status (BKND-01).
// D-02: couriers may only set "available" or "offline"; "busy" is system-managed.
// D-03: offline transition is guarded — blocked if an active delivery exists.
func (s *DeliveryService) UpdateCourierStatus(ctx context.Context, courierID, newStatus string) (*domain.Courier, error) {
	// D-02: courier may only set "available" or "offline"; "busy" is system-managed.
	if newStatus != "available" && newStatus != "offline" {
		return nil, fmt.Errorf("status=%q: %w", newStatus, domain.ErrInvalidStatusTransition)
	}

	if newStatus == "offline" {
		// D-03: atomic offline guard — block if active delivery exists.
		c, ok, err := s.repo.UpdateCourierStatusToOfflineGuarded(ctx, courierID)
		if err != nil {
			return nil, fmt.Errorf("offline guard: %w", err)
		}
		if !ok {
			return nil, domain.ErrActiveDeliveryExists
		}
		return c, nil
	}

	// newStatus == "available"
	return s.repo.UpdateCourierStatus(ctx, courierID, newStatus)
}
