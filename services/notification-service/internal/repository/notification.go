package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	pkgerrors "github.com/mozgovojnikita/delivery-tracker/pkg/errors"
	"github.com/mozgovojnikita/delivery-tracker/services/notification-service/internal/domain"
)

// NotificationRepository defines data access operations for notifications and templates.
type NotificationRepository interface {
	Create(ctx context.Context, n *domain.Notification) (*domain.Notification, error)
	GetByID(ctx context.Context, id string) (*domain.Notification, error)
	UpdateStatus(ctx context.Context, id string, status domain.NotificationStatus) error
	GetTemplateByName(ctx context.Context, name string) (*domain.Template, error)
}

type notificationRepository struct {
	pool *pgxpool.Pool
}

// NewNotificationRepository creates a new NotificationRepository backed by a pgx connection pool.
func NewNotificationRepository(pool *pgxpool.Pool) NotificationRepository {
	return &notificationRepository{pool: pool}
}

// Create inserts a new notification record and returns the created record.
// template_vars is stored as JSONB.
func (r *notificationRepository) Create(ctx context.Context, n *domain.Notification) (*domain.Notification, error) {
	const query = `
		INSERT INTO notifications (user_id, channel, template_name, template_vars, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, channel, template_name, template_vars, status, created_at`

	varsJSON, err := json.Marshal(n.TemplateVars)
	if err != nil {
		return nil, fmt.Errorf("marshal template_vars: %w", err)
	}

	created := &domain.Notification{}
	var rawVars []byte
	err = r.pool.QueryRow(ctx, query,
		n.UserID, n.Channel, n.TemplateName, varsJSON, string(n.Status),
	).Scan(
		&created.ID, &created.UserID, &created.Channel,
		&created.TemplateName, &rawVars, &created.Status, &created.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create notification: %w", err)
	}

	if err := json.Unmarshal(rawVars, &created.TemplateVars); err != nil {
		return nil, fmt.Errorf("unmarshal template_vars: %w", err)
	}

	return created, nil
}

// GetByID retrieves a notification by its UUID.
// Returns ErrNotFound if no notification exists with that ID.
func (r *notificationRepository) GetByID(ctx context.Context, id string) (*domain.Notification, error) {
	const query = `
		SELECT id, user_id, channel, template_name, template_vars, status, created_at
		FROM notifications WHERE id = $1`

	n := &domain.Notification{}
	var rawVars []byte
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&n.ID, &n.UserID, &n.Channel,
		&n.TemplateName, &rawVars, &n.Status, &n.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("notification %s: %w", id, pkgerrors.ErrNotFound)
		}
		return nil, fmt.Errorf("get notification by id: %w", err)
	}

	if err := json.Unmarshal(rawVars, &n.TemplateVars); err != nil {
		return nil, fmt.Errorf("unmarshal template_vars: %w", err)
	}

	return n, nil
}

// UpdateStatus updates the status of a notification by ID.
func (r *notificationRepository) UpdateStatus(ctx context.Context, id string, status domain.NotificationStatus) error {
	const query = `UPDATE notifications SET status = $2 WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id, string(status))
	if err != nil {
		return fmt.Errorf("update notification status: %w", err)
	}
	return nil
}

// GetTemplateByName retrieves a notification template by name.
// Returns ErrNotFound if no template exists with that name.
func (r *notificationRepository) GetTemplateByName(ctx context.Context, name string) (*domain.Template, error) {
	const query = `
		SELECT id, name, channel, subject, body, created_at
		FROM notification_templates WHERE name = $1`

	t := &domain.Template{}
	err := r.pool.QueryRow(ctx, query, name).Scan(
		&t.ID, &t.Name, &t.Channel, &t.Subject, &t.Body, &t.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("template %s: %w", name, pkgerrors.ErrNotFound)
		}
		return nil, fmt.Errorf("get template by name: %w", err)
	}

	return t, nil
}
