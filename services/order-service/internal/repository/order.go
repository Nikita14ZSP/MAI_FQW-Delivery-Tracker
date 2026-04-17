package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	pkgerrors "github.com/mozgovojnikita/delivery-tracker/pkg/errors"
	"github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/domain"
)

// OrderRepository defines the data access interface for orders.
type OrderRepository interface {
	Create(ctx context.Context, input domain.CreateOrderInput) (*domain.Order, error)
	GetByID(ctx context.Context, id string) (*domain.Order, error)
	GetByIDs(ctx context.Context, ids []string) ([]*domain.Order, error)
	ListByUserID(ctx context.Context, userID string, status *domain.OrderStatus, page, pageSize int) ([]*domain.Order, int, error)
	UpdateStatus(ctx context.Context, id string, status domain.OrderStatus, cancelReason string) (*domain.Order, error)
}

type orderRepository struct {
	pool *pgxpool.Pool
}

// NewOrderRepository creates a new OrderRepository backed by a pgx connection pool.
func NewOrderRepository(pool *pgxpool.Pool) OrderRepository {
	return &orderRepository{pool: pool}
}

// Create inserts a new order with status 'created' and returns the created record.
func (r *orderRepository) Create(ctx context.Context, input domain.CreateOrderInput) (*domain.Order, error) {
	const query = `
		INSERT INTO orders (user_id, delivery_address, delivery_lat, delivery_lng, items, contact_phone, payment_method, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'created')
		RETURNING id, user_id, status, delivery_address, COALESCE(delivery_lat, 0), COALESCE(delivery_lng, 0), items, contact_phone, payment_method, cancel_reason, created_at, updated_at`

	o := &domain.Order{}
	var cancelReason *string
	err := r.pool.QueryRow(ctx, query,
		input.UserID, input.DeliveryAddress, input.DeliveryLat, input.DeliveryLng, input.Items,
		input.ContactPhone, input.PaymentMethod,
	).Scan(
		&o.ID, &o.UserID, &o.Status, &o.DeliveryAddress, &o.DeliveryLat, &o.DeliveryLng, &o.Items,
		&o.ContactPhone, &o.PaymentMethod, &cancelReason,
		&o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}
	if cancelReason != nil {
		o.CancelReason = *cancelReason
	}
	return o, nil
}

// GetByID retrieves an order by its UUID.
// Returns ErrNotFound if no order exists with that ID.
func (r *orderRepository) GetByID(ctx context.Context, id string) (*domain.Order, error) {
	const query = `
		SELECT id, user_id, status, delivery_address, COALESCE(delivery_lat, 0), COALESCE(delivery_lng, 0), items, contact_phone, payment_method, cancel_reason, created_at, updated_at
		FROM orders
		WHERE id = $1`

	o := &domain.Order{}
	var cancelReason *string
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&o.ID, &o.UserID, &o.Status, &o.DeliveryAddress, &o.DeliveryLat, &o.DeliveryLng, &o.Items,
		&o.ContactPhone, &o.PaymentMethod, &cancelReason,
		&o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("order %s: %w", id, pkgerrors.ErrNotFound)
		}
		return nil, fmt.Errorf("get order by id: %w", err)
	}
	if cancelReason != nil {
		o.CancelReason = *cancelReason
	}
	return o, nil
}

// GetByIDs retrieves multiple orders by their UUIDs in a single query.
// Returns only the orders that exist; missing IDs are silently ignored.
// Used by delivery-service's GetOrdersByIDs bulk-fetch RPC (BKND-02).
func (r *orderRepository) GetByIDs(ctx context.Context, ids []string) ([]*domain.Order, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	const query = `
		SELECT id, user_id, status, delivery_address, COALESCE(delivery_lat, 0), COALESCE(delivery_lng, 0), items, contact_phone, payment_method, cancel_reason, created_at, updated_at
		FROM orders
		WHERE id = ANY($1::uuid[])`

	rows, err := r.pool.Query(ctx, query, ids)
	if err != nil {
		return nil, fmt.Errorf("get orders by ids: %w", err)
	}
	defer rows.Close()

	var orders []*domain.Order
	for rows.Next() {
		o := &domain.Order{}
		var cancelReason *string
		if err := rows.Scan(
			&o.ID, &o.UserID, &o.Status, &o.DeliveryAddress, &o.DeliveryLat, &o.DeliveryLng, &o.Items,
			&o.ContactPhone, &o.PaymentMethod, &cancelReason,
			&o.CreatedAt, &o.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}
		if cancelReason != nil {
			o.CancelReason = *cancelReason
		}
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate orders by ids: %w", err)
	}
	return orders, nil
}

// ListByUserID retrieves a paginated list of orders for a user.
// If status is non-nil, only orders with that status are returned.
// Returns the orders, the total count, and any error.
func (r *orderRepository) ListByUserID(ctx context.Context, userID string, status *domain.OrderStatus, page, pageSize int) ([]*domain.Order, int, error) {
	offset := (page - 1) * pageSize

	baseWhere := "WHERE user_id = $1"
	args := []interface{}{userID}

	if status != nil {
		baseWhere += " AND status = $2"
		args = append(args, string(*status))
	}

	// Count query.
	countQuery := "SELECT COUNT(*) FROM orders " + baseWhere
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count orders: %w", err)
	}

	// Data query with pagination.
	limitParam := fmt.Sprintf("$%d", len(args)+1)
	offsetParam := fmt.Sprintf("$%d", len(args)+2)
	dataQuery := fmt.Sprintf(`
		SELECT id, user_id, status, delivery_address, COALESCE(delivery_lat, 0), COALESCE(delivery_lng, 0), items, contact_phone, payment_method, cancel_reason, created_at, updated_at
		FROM orders
		%s
		ORDER BY created_at DESC
		LIMIT %s OFFSET %s`, baseWhere, limitParam, offsetParam)
	args = append(args, pageSize, offset)

	rows, err := r.pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()

	var orders []*domain.Order
	for rows.Next() {
		o := &domain.Order{}
		var cancelReason *string
		if err := rows.Scan(
			&o.ID, &o.UserID, &o.Status, &o.DeliveryAddress, &o.DeliveryLat, &o.DeliveryLng, &o.Items,
			&o.ContactPhone, &o.PaymentMethod, &cancelReason,
			&o.CreatedAt, &o.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan order: %w", err)
		}
		if cancelReason != nil {
			o.CancelReason = *cancelReason
		}
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate orders: %w", err)
	}

	return orders, total, nil
}

// UpdateStatus updates an order's status and optional cancel reason.
// Returns ErrNotFound if no order exists with that ID.
func (r *orderRepository) UpdateStatus(ctx context.Context, id string, status domain.OrderStatus, cancelReason string) (*domain.Order, error) {
	const query = `
		UPDATE orders
		SET status = $1, cancel_reason = $2, updated_at = NOW()
		WHERE id = $3
		RETURNING id, user_id, status, delivery_address, COALESCE(delivery_lat, 0), COALESCE(delivery_lng, 0), items, contact_phone, payment_method, cancel_reason, created_at, updated_at`

	o := &domain.Order{}
	var cr *string
	err := r.pool.QueryRow(ctx, query, string(status), cancelReason, id).Scan(
		&o.ID, &o.UserID, &o.Status, &o.DeliveryAddress, &o.DeliveryLat, &o.DeliveryLng, &o.Items,
		&o.ContactPhone, &o.PaymentMethod, &cr,
		&o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("order %s: %w", id, pkgerrors.ErrNotFound)
		}
		return nil, fmt.Errorf("update order status: %w", err)
	}
	if cr != nil {
		o.CancelReason = *cr
	}
	return o, nil
}
