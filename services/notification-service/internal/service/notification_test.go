package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	pkgerrors "github.com/mozgovojnikita/delivery-tracker/pkg/errors"
	"github.com/mozgovojnikita/delivery-tracker/pkg/kafka"
	"github.com/mozgovojnikita/delivery-tracker/services/notification-service/internal/domain"
	"github.com/mozgovojnikita/delivery-tracker/services/notification-service/internal/service"
)

// mockNotificationRepo is a manual mock implementing repository.NotificationRepository.
type mockNotificationRepo struct {
	getTemplateByNameFn func(ctx context.Context, name string) (*domain.Template, error)
	createFn            func(ctx context.Context, n *domain.Notification) (*domain.Notification, error)
	getByIDFn           func(ctx context.Context, id string) (*domain.Notification, error)
	updateStatusFn      func(ctx context.Context, id string, status domain.NotificationStatus) error
}

func (m *mockNotificationRepo) GetTemplateByName(ctx context.Context, name string) (*domain.Template, error) {
	if m.getTemplateByNameFn != nil {
		return m.getTemplateByNameFn(ctx, name)
	}
	return nil, pkgerrors.ErrNotFound
}

func (m *mockNotificationRepo) Create(ctx context.Context, n *domain.Notification) (*domain.Notification, error) {
	if m.createFn != nil {
		return m.createFn(ctx, n)
	}
	created := *n
	created.ID = "notif-1"
	created.CreatedAt = time.Now()
	return &created, nil
}

func (m *mockNotificationRepo) GetByID(ctx context.Context, id string) (*domain.Notification, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, pkgerrors.ErrNotFound
}

func (m *mockNotificationRepo) UpdateStatus(ctx context.Context, id string, status domain.NotificationStatus) error {
	if m.updateStatusFn != nil {
		return m.updateStatusFn(ctx, id, status)
	}
	return nil
}

// TestRenderTemplate_Success verifies that template variables are substituted correctly.
func TestRenderTemplate_Success(t *testing.T) {
	svc := service.NewNotificationService(&mockNotificationRepo{})

	result, err := svc.RenderTemplate("Hello {{.name}}", map[string]string{"name": "John"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result != "Hello John" {
		t.Errorf("expected 'Hello John', got: %q", result)
	}
}

// TestRenderTemplate_MissingVar verifies that missing vars render without panicking.
func TestRenderTemplate_MissingVar(t *testing.T) {
	svc := service.NewNotificationService(&mockNotificationRepo{})

	result, err := svc.RenderTemplate("Hello {{.name}}", map[string]string{})
	if err != nil {
		t.Fatalf("expected no error for missing var, got: %v", err)
	}
	// Go text/template renders missing keys as "<no value>" by default
	_ = result // Result is valid (no panic), content checked separately
}

// TestSendNotification verifies template loading, rendering, dispatch, and record creation.
func TestSendNotification(t *testing.T) {
	var capturedNotif *domain.Notification

	repo := &mockNotificationRepo{
		getTemplateByNameFn: func(ctx context.Context, name string) (*domain.Template, error) {
			return &domain.Template{
				ID:      "tmpl-1",
				Name:    name,
				Channel: "email",
				Subject: "Order {{.order_id}} confirmed",
				Body:    "Your order {{.order_id}} is confirmed.",
			}, nil
		},
		createFn: func(ctx context.Context, n *domain.Notification) (*domain.Notification, error) {
			capturedNotif = n
			created := *n
			created.ID = "notif-1"
			created.CreatedAt = time.Now()
			return &created, nil
		},
	}

	svc := service.NewNotificationService(repo)
	vars := map[string]string{"order_id": "ord-123"}

	notif, err := svc.SendNotification(context.Background(), "user-1", "email", "order_confirmed", vars)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if notif == nil {
		t.Fatal("expected notification, got nil")
	}
	if capturedNotif == nil {
		t.Fatal("expected Create to be called")
	}
	if capturedNotif.Status != domain.StatusSent {
		t.Errorf("expected status sent, got: %s", capturedNotif.Status)
	}
	if capturedNotif.UserID != "user-1" {
		t.Errorf("expected user_id user-1, got: %s", capturedNotif.UserID)
	}
}

// TestHandleDeliveryAssigned verifies that a DeliveryAssignedEvent triggers both
// order_assigned (customer) and courier_assigned (courier) template notifications.
func TestHandleDeliveryAssigned(t *testing.T) {
	var capturedTemplateNames []string

	repo := &mockNotificationRepo{
		getTemplateByNameFn: func(ctx context.Context, name string) (*domain.Template, error) {
			capturedTemplateNames = append(capturedTemplateNames, name)
			return &domain.Template{
				ID:      "tmpl-1",
				Name:    name,
				Channel: "email",
				Subject: "Subject for " + name,
				Body:    "Body for " + name + " order {{.order_id}}",
			}, nil
		},
	}

	svc := service.NewNotificationService(repo)
	event := kafka.DeliveryAssignedEvent{
		EventID:    "evt-1",
		DeliveryID: "del-1",
		OrderID:    "ord-1",
		UserID:     "user-1",
		CourierID:  "courier-1",
		ZoneID:     "zone-1",
		ETA:        "2026-03-28T18:00:00Z",
		AssignedAt: "2026-03-28T17:00:00Z",
	}

	err := svc.HandleDeliveryAssigned(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(capturedTemplateNames) != 2 {
		t.Fatalf("expected 2 template lookups, got %d: %v", len(capturedTemplateNames), capturedTemplateNames)
	}
	if capturedTemplateNames[0] != "order_assigned" {
		t.Errorf("expected first template 'order_assigned', got: %q", capturedTemplateNames[0])
	}
	if capturedTemplateNames[1] != "courier_assigned" {
		t.Errorf("expected second template 'courier_assigned', got: %q", capturedTemplateNames[1])
	}
}

// TestHandleDeliveryAssigned_CourierNotification verifies that the second call uses
// courier_assigned template with courier-specific vars when CourierID is set.
func TestHandleDeliveryAssigned_CourierNotification(t *testing.T) {
	var capturedTemplateNames []string

	repo := &mockNotificationRepo{
		getTemplateByNameFn: func(ctx context.Context, name string) (*domain.Template, error) {
			capturedTemplateNames = append(capturedTemplateNames, name)
			return &domain.Template{
				Name:    name,
				Channel: "email",
				Subject: "Subject for " + name,
				Body:    "Body for " + name + " order {{.order_id}} delivery {{.delivery_id}} eta {{.eta}}",
			}, nil
		},
	}

	svc := service.NewNotificationService(repo)
	event := kafka.DeliveryAssignedEvent{
		OrderID:    "ord-2",
		DeliveryID: "del-2",
		UserID:     "user-2",
		CourierID:  "c1",
		ETA:        "2026-03-28T18:00:00Z",
	}

	err := svc.HandleDeliveryAssigned(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(capturedTemplateNames) < 2 {
		t.Fatalf("expected at least 2 template lookups, got %d", len(capturedTemplateNames))
	}
	if capturedTemplateNames[1] != "courier_assigned" {
		t.Errorf("expected second template 'courier_assigned', got: %q", capturedTemplateNames[1])
	}
}

// TestHandleDeliveryAssigned_EmptyCourierID verifies that when CourierID is empty,
// only the customer notification is sent (courier notification skipped).
func TestHandleDeliveryAssigned_EmptyCourierID(t *testing.T) {
	var capturedTemplateNames []string
	repo := &mockNotificationRepo{
		getTemplateByNameFn: func(ctx context.Context, name string) (*domain.Template, error) {
			capturedTemplateNames = append(capturedTemplateNames, name)
			return &domain.Template{Name: name, Channel: "email", Subject: "s", Body: "b"}, nil
		},
	}
	svc := service.NewNotificationService(repo)
	event := kafka.DeliveryAssignedEvent{
		OrderID:   "ord-1",
		UserID:    "user-1",
		CourierID: "", // empty — manual assignment without courier context
	}
	err := svc.HandleDeliveryAssigned(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(capturedTemplateNames) != 1 {
		t.Fatalf("expected 1 template lookup (customer only), got %d", len(capturedTemplateNames))
	}
	if capturedTemplateNames[0] != "order_assigned" {
		t.Errorf("expected 'order_assigned', got: %q", capturedTemplateNames[0])
	}
}

// TestHandleDeliveryAssigned_EmptyUserID verifies that when UserID is empty,
// only the courier notification is sent (customer notification skipped).
func TestHandleDeliveryAssigned_EmptyUserID(t *testing.T) {
	var capturedTemplateNames []string
	repo := &mockNotificationRepo{
		getTemplateByNameFn: func(ctx context.Context, name string) (*domain.Template, error) {
			capturedTemplateNames = append(capturedTemplateNames, name)
			return &domain.Template{Name: name, Channel: "email", Subject: "s", Body: "b"}, nil
		},
	}
	svc := service.NewNotificationService(repo)
	event := kafka.DeliveryAssignedEvent{
		OrderID:   "ord-1",
		UserID:    "",         // empty — no customer context
		CourierID: "courier-1",
	}
	err := svc.HandleDeliveryAssigned(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(capturedTemplateNames) != 1 {
		t.Fatalf("expected 1 template lookup (courier only), got %d", len(capturedTemplateNames))
	}
	if capturedTemplateNames[0] != "courier_assigned" {
		t.Errorf("expected 'courier_assigned', got: %q", capturedTemplateNames[0])
	}
}

// TestHandleOrderUpdated_Confirmed verifies confirmed status maps to order_confirmed template.
func TestHandleOrderUpdated_Confirmed(t *testing.T) {
	var capturedTemplateName string

	repo := &mockNotificationRepo{
		getTemplateByNameFn: func(ctx context.Context, name string) (*domain.Template, error) {
			capturedTemplateName = name
			return &domain.Template{
				Name:    name,
				Channel: "email",
				Subject: "Order {{.order_id}} confirmed",
				Body:    "Your order {{.order_id}} is confirmed.",
			}, nil
		},
	}

	svc := service.NewNotificationService(repo)
	event := kafka.OrderUpdatedEvent{
		EventID:   "evt-2",
		OrderID:   "ord-2",
		UserID:    "user-2",
		OldStatus: "created",
		NewStatus: "confirmed",
		UpdatedAt: "2026-03-28T17:00:00Z",
	}

	err := svc.HandleOrderUpdated(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if capturedTemplateName != "order_confirmed" {
		t.Errorf("expected template 'order_confirmed', got: %q", capturedTemplateName)
	}
}

// TestHandleOrderUpdated_Delivered verifies delivered status maps to order_delivered template.
func TestHandleOrderUpdated_Delivered(t *testing.T) {
	var capturedTemplateName string

	repo := &mockNotificationRepo{
		getTemplateByNameFn: func(ctx context.Context, name string) (*domain.Template, error) {
			capturedTemplateName = name
			return &domain.Template{
				Name:    name,
				Channel: "email",
				Subject: "Order {{.order_id}} delivered",
				Body:    "Your order {{.order_id}} has been delivered.",
			}, nil
		},
	}

	svc := service.NewNotificationService(repo)
	event := kafka.OrderUpdatedEvent{
		OrderID:   "ord-3",
		UserID:    "user-3",
		NewStatus: "delivered",
	}

	err := svc.HandleOrderUpdated(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if capturedTemplateName != "order_delivered" {
		t.Errorf("expected template 'order_delivered', got: %q", capturedTemplateName)
	}
}

// TestHandleOrderUpdated_Cancelled verifies cancelled status maps to order_cancelled template.
func TestHandleOrderUpdated_Cancelled(t *testing.T) {
	var capturedTemplateName string

	repo := &mockNotificationRepo{
		getTemplateByNameFn: func(ctx context.Context, name string) (*domain.Template, error) {
			capturedTemplateName = name
			return &domain.Template{
				Name:    name,
				Channel: "email",
				Subject: "Order {{.order_id}} cancelled",
				Body:    "Your order {{.order_id}} has been cancelled.",
			}, nil
		},
	}

	svc := service.NewNotificationService(repo)
	event := kafka.OrderUpdatedEvent{
		OrderID:   "ord-4",
		UserID:    "user-4",
		NewStatus: "cancelled",
	}

	err := svc.HandleOrderUpdated(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if capturedTemplateName != "order_cancelled" {
		t.Errorf("expected template 'order_cancelled', got: %q", capturedTemplateName)
	}
}

// TestHandleOrderUpdated_OtherStatus verifies that non-notification statuses are skipped.
func TestHandleOrderUpdated_OtherStatus(t *testing.T) {
	repo := &mockNotificationRepo{
		getTemplateByNameFn: func(ctx context.Context, name string) (*domain.Template, error) {
			// Should NOT be called for in_transit status
			t.Errorf("unexpected template lookup for status that should be skipped")
			return nil, pkgerrors.ErrNotFound
		},
	}

	svc := service.NewNotificationService(repo)
	event := kafka.OrderUpdatedEvent{
		OrderID:   "ord-5",
		UserID:    "user-5",
		NewStatus: "in_transit",
	}

	err := svc.HandleOrderUpdated(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error for skipped status, got: %v", err)
	}
}

// TestSendNotification_TemplateNotFound verifies ErrNotFound propagation.
func TestSendNotification_TemplateNotFound(t *testing.T) {
	repo := &mockNotificationRepo{
		getTemplateByNameFn: func(ctx context.Context, name string) (*domain.Template, error) {
			return nil, pkgerrors.ErrNotFound
		},
	}

	svc := service.NewNotificationService(repo)
	_, err := svc.SendNotification(context.Background(), "user-1", "email", "nonexistent_template", nil)
	if err == nil {
		t.Fatal("expected error for missing template, got nil")
	}
	if !errors.Is(err, pkgerrors.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}
