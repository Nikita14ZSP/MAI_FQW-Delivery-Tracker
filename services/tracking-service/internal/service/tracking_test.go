package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	deliveryv1 "github.com/mozgovojnikita/delivery-tracker/gen/delivery/v1"
	pkgkafka "github.com/mozgovojnikita/delivery-tracker/pkg/kafka"
	"github.com/mozgovojnikita/delivery-tracker/services/tracking-service/internal/domain"
	"github.com/mozgovojnikita/delivery-tracker/services/tracking-service/internal/service"
)

// --- Mock LocationRepository ---

type mockRepo struct {
	insertErr          error
	cacheErr           error
	getHistoryResult   []*domain.LocationPoint
	getHistoryErr      error
	getLastLocResult   *domain.LocationPoint
	getLastLocErr      error
	getCachedResult    *domain.CachedLocation
	getCachedErr       error
	insertCalled       bool
	cacheCalled        bool
}

func (m *mockRepo) InsertLocation(ctx context.Context, courierID, orderID string, lat, lng float64, recordedAt time.Time) error {
	m.insertCalled = true
	return m.insertErr
}

func (m *mockRepo) GetLocationHistory(ctx context.Context, orderID string, limit int) ([]*domain.LocationPoint, error) {
	return m.getHistoryResult, m.getHistoryErr
}

func (m *mockRepo) GetLastLocation(ctx context.Context, courierID string) (*domain.LocationPoint, error) {
	return m.getLastLocResult, m.getLastLocErr
}

func (m *mockRepo) CacheLastLocation(ctx context.Context, loc domain.CachedLocation) error {
	m.cacheCalled = true
	return m.cacheErr
}

func (m *mockRepo) GetCachedLocation(ctx context.Context, courierID string) (*domain.CachedLocation, error) {
	return m.getCachedResult, m.getCachedErr
}

// --- Mock Hub ---

type mockHub struct {
	publishErr     error
	lastOrderID    string
	lastPayload    []byte
	publishCalled  bool
}

func (m *mockHub) Publish(ctx context.Context, orderID string, payload []byte) error {
	m.publishCalled = true
	m.lastOrderID = orderID
	m.lastPayload = payload
	return m.publishErr
}

// --- Mock DeliveryClient ---

type mockDeliveryClient struct {
	deliveries []*deliveryv1.Delivery
	err        error
}

func (m *mockDeliveryClient) GetDeliveriesByCourier(ctx context.Context, courierID string) ([]*deliveryv1.Delivery, error) {
	return m.deliveries, m.err
}

// --- Tests ---

func TestUpdateLocation_CallsInsertAndCacheAndPublish(t *testing.T) {
	repo := &mockRepo{}
	hub := &mockHub{}
	client := &mockDeliveryClient{}
	svc := service.NewTrackingService(repo, hub, client)

	input := service.UpdateLocationInput{
		CourierID: "courier-1",
		OrderID:   "order-1",
		Lat:       55.0,
		Lng:       37.0,
	}
	if err := svc.UpdateLocation(context.Background(), input); err != nil {
		t.Fatalf("UpdateLocation returned error: %v", err)
	}

	if !repo.insertCalled {
		t.Error("expected InsertLocation to be called")
	}
	if !repo.cacheCalled {
		t.Error("expected CacheLastLocation to be called")
	}
	if !hub.publishCalled {
		t.Error("expected hub.Publish to be called")
	}
	if hub.lastOrderID != "order-1" {
		t.Errorf("expected orderID=order-1, got %s", hub.lastOrderID)
	}

	// Verify WSMessage format
	var msg domain.WSMessage
	if err := json.Unmarshal(hub.lastPayload, &msg); err != nil {
		t.Fatalf("invalid WSMessage JSON: %v", err)
	}
	if msg.Type != domain.MsgTypeLocationUpdate {
		t.Errorf("expected type=%s, got %s", domain.MsgTypeLocationUpdate, msg.Type)
	}
}

func TestUpdateLocation_CacheFailureIsNonFatal(t *testing.T) {
	repo := &mockRepo{cacheErr: errors.New("redis unavailable")}
	hub := &mockHub{}
	client := &mockDeliveryClient{}
	svc := service.NewTrackingService(repo, hub, client)

	input := service.UpdateLocationInput{
		CourierID: "courier-1",
		OrderID:   "order-1",
		Lat:       55.0,
		Lng:       37.0,
	}
	// Should not return error even when cache fails
	if err := svc.UpdateLocation(context.Background(), input); err != nil {
		t.Fatalf("UpdateLocation should not fail on cache error, got: %v", err)
	}
	if !hub.publishCalled {
		t.Error("expected Publish to be called even after cache failure")
	}
}

func TestGetLocationHistory_ReturnsFromRepo(t *testing.T) {
	expected := []*domain.LocationPoint{
		{ID: "loc-1", CourierID: "courier-1", OrderID: "order-1", Lat: 55.0, Lng: 37.0},
	}
	repo := &mockRepo{getHistoryResult: expected}
	hub := &mockHub{}
	client := &mockDeliveryClient{}
	svc := service.NewTrackingService(repo, hub, client)

	result, err := svc.GetLocationHistory(context.Background(), "order-1", 10)
	if err != nil {
		t.Fatalf("GetLocationHistory returned error: %v", err)
	}
	if len(result) != 1 || result[0].ID != "loc-1" {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestGetCourierLocation_ReturnsCachedLocation(t *testing.T) {
	cached := &domain.CachedLocation{
		CourierID: "courier-1",
		OrderID:   "order-1",
		Lat:       55.0,
		Lng:       37.0,
		UpdatedAt: time.Now().Format(time.RFC3339),
	}
	repo := &mockRepo{getCachedResult: cached}
	hub := &mockHub{}
	client := &mockDeliveryClient{}
	svc := service.NewTrackingService(repo, hub, client)

	cachedResult, fallbackResult, err := svc.GetCourierLocation(context.Background(), "courier-1")
	if err != nil {
		t.Fatalf("GetCourierLocation returned error: %v", err)
	}
	if cachedResult == nil {
		t.Fatal("expected cached result, got nil")
	}
	if cachedResult.CourierID != "courier-1" {
		t.Errorf("expected courierID=courier-1, got %s", cachedResult.CourierID)
	}
	if fallbackResult != nil {
		t.Error("expected nil fallback when cache hit")
	}
}

func TestGetCourierLocation_FallsBackToRepoOnCacheMiss(t *testing.T) {
	lastLoc := &domain.LocationPoint{
		ID:        "loc-1",
		CourierID: "courier-1",
		OrderID:   "order-1",
		Lat:       55.0,
		Lng:       37.0,
	}
	repo := &mockRepo{getCachedResult: nil, getCachedErr: nil, getLastLocResult: lastLoc}
	hub := &mockHub{}
	client := &mockDeliveryClient{}
	svc := service.NewTrackingService(repo, hub, client)

	cachedResult, fallbackResult, err := svc.GetCourierLocation(context.Background(), "courier-1")
	if err != nil {
		t.Fatalf("GetCourierLocation returned error: %v", err)
	}
	if cachedResult != nil {
		t.Error("expected nil cached result on cache miss")
	}
	if fallbackResult == nil {
		t.Fatal("expected fallback result from repo")
	}
	if fallbackResult.ID != "loc-1" {
		t.Errorf("expected loc-1, got %s", fallbackResult.ID)
	}
}

func TestBroadcastOrderCreated_PublishesCorrectMessage(t *testing.T) {
	repo := &mockRepo{}
	hub := &mockHub{}
	client := &mockDeliveryClient{}
	svc := service.NewTrackingService(repo, hub, client)

	event := pkgkafka.OrderCreatedEvent{
		EventID: "evt-1",
		OrderID: "order-1",
		UserID:  "user-1",
	}
	if err := svc.BroadcastOrderCreated(context.Background(), event); err != nil {
		t.Fatalf("BroadcastOrderCreated returned error: %v", err)
	}
	if !hub.publishCalled {
		t.Error("expected hub.Publish to be called")
	}

	var msg domain.WSMessage
	if err := json.Unmarshal(hub.lastPayload, &msg); err != nil {
		t.Fatalf("invalid WSMessage JSON: %v", err)
	}
	if msg.Type != domain.MsgTypeOrderCreated {
		t.Errorf("expected type=%s, got %s", domain.MsgTypeOrderCreated, msg.Type)
	}
}

func TestBroadcastDeliveryAssigned_PublishesCorrectMessage(t *testing.T) {
	repo := &mockRepo{}
	hub := &mockHub{}
	client := &mockDeliveryClient{}
	svc := service.NewTrackingService(repo, hub, client)

	event := pkgkafka.DeliveryAssignedEvent{
		EventID:   "evt-1",
		OrderID:   "order-1",
		CourierID: "courier-1",
	}
	if err := svc.BroadcastDeliveryAssigned(context.Background(), event); err != nil {
		t.Fatalf("BroadcastDeliveryAssigned returned error: %v", err)
	}
	if !hub.publishCalled {
		t.Error("expected hub.Publish to be called")
	}

	var msg domain.WSMessage
	if err := json.Unmarshal(hub.lastPayload, &msg); err != nil {
		t.Fatalf("invalid WSMessage JSON: %v", err)
	}
	if msg.Type != domain.MsgTypeDeliveryAssigned {
		t.Errorf("expected type=%s, got %s", domain.MsgTypeDeliveryAssigned, msg.Type)
	}
}

func TestGetActiveDeliveries_CallsDeliveryClient(t *testing.T) {
	deliveries := []*deliveryv1.Delivery{
		{Id: "del-1", OrderId: "order-1", CourierId: "courier-1"},
	}
	repo := &mockRepo{}
	hub := &mockHub{}
	client := &mockDeliveryClient{deliveries: deliveries}
	svc := service.NewTrackingService(repo, hub, client)

	result, err := svc.GetActiveDeliveries(context.Background(), "courier-1")
	if err != nil {
		t.Fatalf("GetActiveDeliveries returned error: %v", err)
	}
	if len(result) != 1 || result[0].Id != "del-1" {
		t.Errorf("unexpected result: %+v", result)
	}
}
