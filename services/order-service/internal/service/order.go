package service

import (
	"context"
	"fmt"
	"log/slog"

	pkgerrors "github.com/mozgovojnikita/delivery-tracker/pkg/errors"
	pkgkafka "github.com/mozgovojnikita/delivery-tracker/pkg/kafka"
	"github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/domain"
	orderKafka "github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/kafka"
	"github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/repository"
)

// OrderService handles order lifecycle operations.
type OrderService struct {
	orderRepo repository.OrderRepository
	publisher pkgkafka.EventPublisher // may be nil (backward compat)
}

// NewOrderService creates a new OrderService.
func NewOrderService(orderRepo repository.OrderRepository, publisher pkgkafka.EventPublisher) *OrderService {
	return &OrderService{
		orderRepo: orderRepo,
		publisher: publisher,
	}
}

// CreateOrder creates a new order with status=created.
// Validates that required fields are present before delegating to the repository.
func (s *OrderService) CreateOrder(ctx context.Context, input domain.CreateOrderInput) (*domain.Order, error) {
	if input.UserID == "" {
		return nil, fmt.Errorf("user_id is required: %w", pkgerrors.ErrInvalidInput)
	}
	if input.DeliveryAddress == "" {
		return nil, fmt.Errorf("delivery_address is required: %w", pkgerrors.ErrInvalidInput)
	}

	return s.orderRepo.Create(ctx, input)
}

// GetOrder retrieves an order by ID.
func (s *OrderService) GetOrder(ctx context.Context, id string) (*domain.Order, error) {
	return s.orderRepo.GetByID(ctx, id)
}

// ListOrders retrieves a paginated list of orders for a user.
// Applies safe defaults: page>=1, 1<=pageSize<=100 (default 20).
func (s *OrderService) ListOrders(ctx context.Context, userID string, status *domain.OrderStatus, page, pageSize int) ([]*domain.Order, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return s.orderRepo.ListByUserID(ctx, userID, status, page, pageSize)
}

// UpdateStatus transitions an order to a new status, enforcing the state machine and role permissions.
func (s *OrderService) UpdateStatus(ctx context.Context, orderID string, newStatus domain.OrderStatus, role domain.Role) (*domain.Order, error) {
	order, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}

	oldStatus := order.Status

	if err := domain.ValidateTransition(order.Status, newStatus, role); err != nil {
		return nil, err
	}

	updated, err := s.orderRepo.UpdateStatus(ctx, orderID, newStatus, "")
	if err != nil {
		return nil, err
	}

	// Publish events after successful DB update. Failures are logged but do not fail the update.
	if s.publisher != nil {
		if err := orderKafka.PublishOrderUpdated(ctx, s.publisher, updated, string(oldStatus), string(newStatus)); err != nil {
			slog.Error("failed to publish order updated event", "err", err, "order_id", updated.ID)
		}

		if newStatus == domain.StatusConfirmed {
			if err := orderKafka.PublishOrderCreated(ctx, s.publisher, updated); err != nil {
				slog.Error("failed to publish order created event", "err", err, "order_id", updated.ID)
			}
		}
	}

	return updated, nil
}

// CancelOrder cancels an order, enforcing that the transition is valid for the given role.
// Only roles that can transition to "cancelled" from the current status are allowed.
func (s *OrderService) CancelOrder(ctx context.Context, orderID string, reason string, role domain.Role) (*domain.Order, error) {
	order, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}

	if err := domain.ValidateTransition(order.Status, domain.StatusCancelled, role); err != nil {
		return nil, err
	}

	return s.orderRepo.UpdateStatus(ctx, orderID, domain.StatusCancelled, reason)
}
