package service

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"text/template"

	"github.com/mozgovojnikita/delivery-tracker/pkg/kafka"
	"github.com/mozgovojnikita/delivery-tracker/services/notification-service/internal/domain"
	"github.com/mozgovojnikita/delivery-tracker/services/notification-service/internal/repository"
)

// NotificationService handles notification business logic including template rendering
// and dispatching notifications via stdout logging (mock dispatch per D-09).
type NotificationService struct {
	repo repository.NotificationRepository
}

// NewNotificationService creates a new NotificationService.
func NewNotificationService(repo repository.NotificationRepository) *NotificationService {
	return &NotificationService{repo: repo}
}

// RenderTemplate parses the given template string and executes it with the provided vars.
// CRITICAL: map[string]string is converted to map[string]interface{} (D-10) so that
// Go text/template can access values via dot notation (e.g. {{.order_id}}).
func (s *NotificationService) RenderTemplate(tmplStr string, vars map[string]string) (string, error) {
	t, err := template.New("notification").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	// Convert map[string]string to map[string]interface{} — required by text/template (D-10).
	data := make(map[string]interface{}, len(vars))
	for k, v := range vars {
		data[k] = v
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

// SendNotification loads a template by name, renders it with vars, dispatches via slog.Info,
// and persists a notification record with status=sent (per D-08, D-09).
func (s *NotificationService) SendNotification(ctx context.Context, userID, channel, templateName string, vars map[string]string) (*domain.Notification, error) {
	// Load template from DB (D-08).
	tmpl, err := s.repo.GetTemplateByName(ctx, templateName)
	if err != nil {
		return nil, fmt.Errorf("get template %s: %w", templateName, err)
	}

	// Render body and subject.
	renderedBody, err := s.RenderTemplate(tmpl.Body, vars)
	if err != nil {
		return nil, fmt.Errorf("render body: %w", err)
	}
	renderedSubject, err := s.RenderTemplate(tmpl.Subject, vars)
	if err != nil {
		return nil, fmt.Errorf("render subject: %w", err)
	}

	// Mock dispatch via slog.Info (D-09).
	slog.Info("notification sent",
		"channel", channel,
		"user_id", userID,
		"template", templateName,
		"subject", renderedSubject,
		"message", renderedBody,
	)

	// Persist notification record with status=sent.
	created, err := s.repo.Create(ctx, &domain.Notification{
		UserID:       userID,
		Channel:      channel,
		TemplateName: templateName,
		TemplateVars: vars,
		Status:       domain.StatusSent,
	})
	if err != nil {
		return nil, fmt.Errorf("create notification record: %w", err)
	}

	return created, nil
}

// HandleDeliveryAssigned processes a DeliveryAssigned Kafka event.
// Sends "order_assigned" notification to the order owner (customer) AND
// "courier_assigned" notification to the assigned courier (D-04, D-07).
// Gracefully skips notification if UserID or CourierID is empty.
func (s *NotificationService) HandleDeliveryAssigned(ctx context.Context, event kafka.DeliveryAssignedEvent) error {
	vars := map[string]string{
		"order_id":    event.OrderID,
		"courier_id":  event.CourierID,
		"eta":         event.ETA,
		"delivery_id": event.DeliveryID,
	}

	// Notify customer (existing behavior)
	if event.UserID != "" {
		if _, err := s.SendNotification(ctx, event.UserID, "email", "order_assigned", vars); err != nil {
			return fmt.Errorf("handle delivery assigned (customer): %w", err)
		}
	}

	// Notify courier (D-04, D-07)
	if event.CourierID != "" {
		courierVars := map[string]string{
			"order_id":    event.OrderID,
			"delivery_id": event.DeliveryID,
			"eta":         event.ETA,
		}
		if _, err := s.SendNotification(ctx, event.CourierID, "email", "courier_assigned", courierVars); err != nil {
			return fmt.Errorf("handle delivery assigned (courier): %w", err)
		}
	}

	return nil
}

// HandleOrderUpdated processes an OrderUpdated Kafka event.
// Maps new_status to the appropriate notification template:
//   - confirmed  -> order_confirmed
//   - delivered  -> order_delivered
//   - cancelled  -> order_cancelled
//   - other      -> no notification (return nil)
func (s *NotificationService) HandleOrderUpdated(ctx context.Context, event kafka.OrderUpdatedEvent) error {
	templateName := ""
	switch event.NewStatus {
	case "confirmed":
		templateName = "order_confirmed"
	case "delivered":
		templateName = "order_delivered"
	case "cancelled":
		templateName = "order_cancelled"
	default:
		// Other statuses do not trigger notifications.
		return nil
	}

	vars := map[string]string{
		"order_id": event.OrderID,
	}
	_, err := s.SendNotification(ctx, event.UserID, "email", templateName, vars)
	if err != nil {
		return fmt.Errorf("handle order updated: %w", err)
	}
	return nil
}

// GetNotification retrieves a notification record by ID.
func (s *NotificationService) GetNotification(ctx context.Context, id string) (*domain.Notification, error) {
	n, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get notification: %w", err)
	}
	return n, nil
}
