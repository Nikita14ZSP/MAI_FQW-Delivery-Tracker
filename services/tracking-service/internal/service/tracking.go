package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	deliveryv1 "github.com/mozgovojnikita/delivery-tracker/gen/delivery/v1"
	pkgkafka "github.com/mozgovojnikita/delivery-tracker/pkg/kafka"
	"github.com/mozgovojnikita/delivery-tracker/services/tracking-service/internal/domain"
	"github.com/mozgovojnikita/delivery-tracker/services/tracking-service/internal/repository"
)

// Hub is the interface for publishing WebSocket messages via Redis Pub/Sub.
// Implemented by ws.Hub in plan 04-02.
type Hub interface {
	Publish(ctx context.Context, orderID string, payload []byte) error
}

// DeliveryClient is the interface for gRPC calls to the Delivery Service.
// Implemented by a gRPC client wrapper in plan 04-02.
type DeliveryClient interface {
	GetDeliveriesByCourier(ctx context.Context, courierID string) ([]*deliveryv1.Delivery, error)
}

// UpdateLocationInput holds the data needed to record a courier's GPS position.
type UpdateLocationInput struct {
	CourierID string
	OrderID   string
	Lat       float64
	Lng       float64
}

// TrackingServiceIface defines all methods needed by gRPC and WebSocket handlers.
type TrackingServiceIface interface {
	UpdateLocation(ctx context.Context, input UpdateLocationInput) error
	GetLocationHistory(ctx context.Context, orderID string, limit int) ([]*domain.LocationPoint, error)
	GetCourierLocation(ctx context.Context, courierID string) (*domain.CachedLocation, *domain.LocationPoint, error)
	BroadcastOrderCreated(ctx context.Context, event pkgkafka.OrderCreatedEvent) error
	BroadcastOrderUpdated(ctx context.Context, event pkgkafka.OrderUpdatedEvent) error
	BroadcastDeliveryAssigned(ctx context.Context, event pkgkafka.DeliveryAssignedEvent) error
	BroadcastDeliveryStatus(ctx context.Context, event pkgkafka.DeliveryStatusEvent) error
	GetActiveDeliveries(ctx context.Context, courierID string) ([]*deliveryv1.Delivery, error)
}

// TrackingService implements TrackingServiceIface.
type TrackingService struct {
	repo           repository.LocationRepository
	hub            Hub
	deliveryClient DeliveryClient
}

// NewTrackingService creates a new TrackingService.
func NewTrackingService(repo repository.LocationRepository, hub Hub, deliveryClient DeliveryClient) *TrackingService {
	return &TrackingService{
		repo:           repo,
		hub:            hub,
		deliveryClient: deliveryClient,
	}
}

// UpdateLocation records the courier's GPS position, updates the Redis cache,
// and broadcasts a location_update message via the hub.
// Cache failure is non-fatal (logged as warning) per D-05.
func (s *TrackingService) UpdateLocation(ctx context.Context, input UpdateLocationInput) error {
	now := time.Now().UTC()

	// 1. Persist to PostGIS.
	if err := s.repo.InsertLocation(ctx, input.CourierID, input.OrderID, input.Lat, input.Lng, now); err != nil {
		return fmt.Errorf("insert location: %w", err)
	}

	// 2. Update Redis cache — non-fatal on failure.
	cached := domain.CachedLocation{
		CourierID: input.CourierID,
		OrderID:   input.OrderID,
		Lat:       input.Lat,
		Lng:       input.Lng,
		UpdatedAt: now.Format(time.RFC3339),
	}
	if err := s.repo.CacheLastLocation(ctx, cached); err != nil {
		slog.Warn("failed to cache courier location", "courier_id", input.CourierID, "err", err)
	}

	// 3. Broadcast location_update via Redis Pub/Sub.
	data := domain.LocationUpdateData{
		CourierID: input.CourierID,
		OrderID:   input.OrderID,
		Lat:       input.Lat,
		Lng:       input.Lng,
		Timestamp: now.Format(time.RFC3339),
	}
	payload, err := marshalWSMessage(domain.MsgTypeLocationUpdate, data)
	if err != nil {
		return fmt.Errorf("marshal location_update message: %w", err)
	}
	if err := s.hub.Publish(ctx, input.OrderID, payload); err != nil {
		return fmt.Errorf("publish location_update: %w", err)
	}
	return nil
}

// GetLocationHistory returns the last `limit` location points for an order.
func (s *TrackingService) GetLocationHistory(ctx context.Context, orderID string, limit int) ([]*domain.LocationPoint, error) {
	points, err := s.repo.GetLocationHistory(ctx, orderID, limit)
	if err != nil {
		return nil, fmt.Errorf("get location history: %w", err)
	}
	return points, nil
}

// GetCourierLocation returns the courier's last known location.
// First tries Redis cache; falls back to PostGIS DB on cache miss.
// Returns (cachedLoc, nil, nil) on cache hit, (nil, dbLoc, nil) on cache miss.
func (s *TrackingService) GetCourierLocation(ctx context.Context, courierID string) (*domain.CachedLocation, *domain.LocationPoint, error) {
	cached, err := s.repo.GetCachedLocation(ctx, courierID)
	if err != nil {
		slog.Warn("redis cache error, falling back to DB", "courier_id", courierID, "err", err)
	}
	if cached != nil {
		return cached, nil, nil
	}

	// Cache miss or error — fall back to DB.
	lastLoc, err := s.repo.GetLastLocation(ctx, courierID)
	if err != nil {
		return nil, nil, fmt.Errorf("get last location from db: %w", err)
	}
	return nil, lastLoc, nil
}

// BroadcastOrderCreated publishes an order_created WSMessage to all order subscribers.
func (s *TrackingService) BroadcastOrderCreated(ctx context.Context, event pkgkafka.OrderCreatedEvent) error {
	payload, err := marshalWSMessage(domain.MsgTypeOrderCreated, event)
	if err != nil {
		return fmt.Errorf("marshal order_created message: %w", err)
	}
	if err := s.hub.Publish(ctx, event.OrderID, payload); err != nil {
		return fmt.Errorf("publish order_created: %w", err)
	}
	return nil
}

// BroadcastOrderUpdated publishes an order_status_change WSMessage.
func (s *TrackingService) BroadcastOrderUpdated(ctx context.Context, event pkgkafka.OrderUpdatedEvent) error {
	payload, err := marshalWSMessage(domain.MsgTypeOrderStatusChange, event)
	if err != nil {
		return fmt.Errorf("marshal order_status_change message: %w", err)
	}
	if err := s.hub.Publish(ctx, event.OrderID, payload); err != nil {
		return fmt.Errorf("publish order_status_change: %w", err)
	}
	return nil
}

// BroadcastDeliveryAssigned publishes a delivery_assigned WSMessage.
func (s *TrackingService) BroadcastDeliveryAssigned(ctx context.Context, event pkgkafka.DeliveryAssignedEvent) error {
	payload, err := marshalWSMessage(domain.MsgTypeDeliveryAssigned, event)
	if err != nil {
		return fmt.Errorf("marshal delivery_assigned message: %w", err)
	}
	if err := s.hub.Publish(ctx, event.OrderID, payload); err != nil {
		return fmt.Errorf("publish delivery_assigned: %w", err)
	}
	return nil
}

// BroadcastDeliveryStatus publishes a delivery_status WSMessage.
func (s *TrackingService) BroadcastDeliveryStatus(ctx context.Context, event pkgkafka.DeliveryStatusEvent) error {
	payload, err := marshalWSMessage(domain.MsgTypeDeliveryStatus, event)
	if err != nil {
		return fmt.Errorf("marshal delivery_status message: %w", err)
	}
	if err := s.hub.Publish(ctx, event.OrderID, payload); err != nil {
		return fmt.Errorf("publish delivery_status: %w", err)
	}
	return nil
}

// GetActiveDeliveries retrieves all active deliveries for a courier via gRPC.
func (s *TrackingService) GetActiveDeliveries(ctx context.Context, courierID string) ([]*deliveryv1.Delivery, error) {
	deliveries, err := s.deliveryClient.GetDeliveriesByCourier(ctx, courierID)
	if err != nil {
		return nil, fmt.Errorf("get deliveries by courier: %w", err)
	}
	return deliveries, nil
}

// marshalWSMessage encodes a WSMessage envelope with the given type and data payload.
func marshalWSMessage(msgType string, data any) ([]byte, error) {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal data: %w", err)
	}
	msg := domain.WSMessage{
		Type: msgType,
		Data: dataJSON,
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal WSMessage: %w", err)
	}
	return payload, nil
}
