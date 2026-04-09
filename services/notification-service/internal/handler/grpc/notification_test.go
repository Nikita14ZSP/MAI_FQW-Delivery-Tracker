package grpc_test

import (
	"context"
	"errors"
	"testing"
	"time"

	notificationv1 "github.com/mozgovojnikita/delivery-tracker/gen/notification/v1"
	grpchandler "github.com/mozgovojnikita/delivery-tracker/services/notification-service/internal/handler/grpc"
	"github.com/mozgovojnikita/delivery-tracker/services/notification-service/internal/domain"
)

// mockNotificationService is a test double for the notification service.
type mockNotificationService struct {
	sendNotificationFn func(ctx context.Context, userID, channel, templateName string, vars map[string]string) (*domain.Notification, error)
	getNotificationFn  func(ctx context.Context, id string) (*domain.Notification, error)
}

func (m *mockNotificationService) SendNotification(ctx context.Context, userID, channel, templateName string, vars map[string]string) (*domain.Notification, error) {
	return m.sendNotificationFn(ctx, userID, channel, templateName, vars)
}

func (m *mockNotificationService) GetNotification(ctx context.Context, id string) (*domain.Notification, error) {
	return m.getNotificationFn(ctx, id)
}

func TestSendNotification_gRPC(t *testing.T) {
	now := time.Now()
	expectedNotification := &domain.Notification{
		ID:           "notif-1",
		UserID:       "user-1",
		Channel:      "email",
		TemplateName: "order_assigned",
		TemplateVars: map[string]string{"order_id": "ord-1"},
		Status:       domain.StatusSent,
		CreatedAt:    now,
	}

	mock := &mockNotificationService{
		sendNotificationFn: func(ctx context.Context, userID, channel, templateName string, vars map[string]string) (*domain.Notification, error) {
			if userID != "user-1" {
				t.Errorf("expected userID=user-1, got %s", userID)
			}
			if channel != "email" {
				t.Errorf("expected channel=email, got %s", channel)
			}
			if templateName != "order_assigned" {
				t.Errorf("expected templateName=order_assigned, got %s", templateName)
			}
			return expectedNotification, nil
		},
	}

	handler := grpchandler.NewNotificationHandler(mock)
	req := &notificationv1.SendNotificationRequest{
		UserId:       "user-1",
		Channel:      "email",
		TemplateName: "order_assigned",
		TemplateVars: map[string]string{"order_id": "ord-1"},
	}

	resp, err := handler.SendNotification(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Notification == nil {
		t.Fatal("expected notification in response, got nil")
	}
	if resp.Notification.Id != "notif-1" {
		t.Errorf("expected id=notif-1, got %s", resp.Notification.Id)
	}
	if resp.Notification.Status != notificationv1.NotificationStatus_NOTIFICATION_STATUS_SENT {
		t.Errorf("expected status SENT, got %v", resp.Notification.Status)
	}
}

func TestSendNotification_gRPC_ServiceError(t *testing.T) {
	mock := &mockNotificationService{
		sendNotificationFn: func(ctx context.Context, userID, channel, templateName string, vars map[string]string) (*domain.Notification, error) {
			return nil, errors.New("service error")
		},
	}

	handler := grpchandler.NewNotificationHandler(mock)
	req := &notificationv1.SendNotificationRequest{
		UserId:       "user-1",
		Channel:      "email",
		TemplateName: "order_assigned",
	}

	_, err := handler.SendNotification(context.Background(), req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetNotification_gRPC(t *testing.T) {
	now := time.Now()
	expectedNotification := &domain.Notification{
		ID:           "notif-42",
		UserID:       "user-2",
		Channel:      "email",
		TemplateName: "order_confirmed",
		TemplateVars: map[string]string{},
		Status:       domain.StatusSent,
		CreatedAt:    now,
	}

	mock := &mockNotificationService{
		getNotificationFn: func(ctx context.Context, id string) (*domain.Notification, error) {
			if id != "notif-42" {
				t.Errorf("expected id=notif-42, got %s", id)
			}
			return expectedNotification, nil
		},
	}

	handler := grpchandler.NewNotificationHandler(mock)
	req := &notificationv1.GetNotificationRequest{Id: "notif-42"}

	resp, err := handler.GetNotification(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Notification == nil {
		t.Fatal("expected notification in response, got nil")
	}
	if resp.Notification.Id != "notif-42" {
		t.Errorf("expected id=notif-42, got %s", resp.Notification.Id)
	}
}

func TestGetNotification_gRPC_NotFound(t *testing.T) {
	mock := &mockNotificationService{
		getNotificationFn: func(ctx context.Context, id string) (*domain.Notification, error) {
			return nil, errors.New("not found")
		},
	}

	handler := grpchandler.NewNotificationHandler(mock)
	req := &notificationv1.GetNotificationRequest{Id: "missing"}

	_, err := handler.GetNotification(context.Background(), req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetHealth_gRPC(t *testing.T) {
	mock := &mockNotificationService{}
	handler := grpchandler.NewNotificationHandler(mock)

	resp, err := handler.GetHealth(context.Background(), &notificationv1.HealthRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("expected status=ok, got %s", resp.Status)
	}
}
