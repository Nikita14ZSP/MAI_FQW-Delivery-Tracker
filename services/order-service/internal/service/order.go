package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	deliveryv1 "github.com/mozgovojnikita/delivery-tracker/gen/delivery/v1"
	pkgerrors "github.com/mozgovojnikita/delivery-tracker/pkg/errors"
	pkgkafka "github.com/mozgovojnikita/delivery-tracker/pkg/kafka"
	"github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/domain"
	orderKafka "github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/kafka"
	"github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/repository"
)

// OrderService handles order lifecycle operations.
type OrderService struct {
	orderRepo      repository.OrderRepository
	publisher      pkgkafka.EventPublisher // may be nil (backward compat)
	deliveryClient DeliveryClient          // may be nil; if nil, GetOrder skips delivery lookup (BKND-04)
}

// NewOrderService creates a new OrderService.
func NewOrderService(orderRepo repository.OrderRepository, publisher pkgkafka.EventPublisher) *OrderService {
	return &OrderService{
		orderRepo: orderRepo,
		publisher: publisher,
	}
}

// WithDeliveryClient injects a delivery-service gRPC client used by GetOrder
// to populate Order.DeliveryID (BKND-04). nil-safe; when omitted, GetOrder
// returns Order with empty DeliveryID.
func (s *OrderService) WithDeliveryClient(c DeliveryClient) *OrderService {
	s.deliveryClient = c
	return s
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

	order, err := s.orderRepo.Create(ctx, input)
	if err != nil {
		return nil, err
	}

	// Publish orders.created after successful persistence. This drives the
	// order -> delivery -> tracking event chain (delivery-service creates a
	// pending delivery on this event). Failures are logged but do NOT fail
	// order creation — Kafka being down must not break order placement.
	if s.publisher != nil {
		if err := orderKafka.PublishOrderCreated(ctx, s.publisher, order); err != nil {
			slog.Error("failed to publish order created event", "err", err, "order_id", order.ID)
		}
	}

	return order, nil
}

// GetOrder retrieves an order by ID.
// When a DeliveryClient is configured, also populates Order.DeliveryID via cross-service
// lookup (BKND-04 D-10). Graceful degradation per RESEARCH §Anti-Patterns:
// - delivery not found → DeliveryID stays empty (D-10 graceful empty)
// - delivery-service unavailable → DeliveryID stays empty + warn log (NOT a 500)
func (s *OrderService) GetOrder(ctx context.Context, id string) (*domain.Order, error) {
	order, err := s.orderRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if s.deliveryClient != nil {
		resp, dErr := s.deliveryClient.GetDeliveryByOrderID(ctx, &deliveryv1.GetDeliveryByOrderIDRequest{OrderId: id})
		switch {
		case dErr == nil && resp != nil && resp.GetDelivery() != nil:
			order.DeliveryID = resp.GetDelivery().GetId()
		case dErr != nil && errors.Is(dErr, pkgerrors.ErrNotFound):
			// No delivery yet — leave DeliveryID empty (D-10 graceful empty).
		case dErr != nil:
			// codes.NotFound / codes.Unavailable / network errors — graceful degradation.
			slog.WarnContext(ctx, "delivery-service unavailable, returning order without delivery_id",
				"error", dErr, "order_id", id)
		}
	}

	return order, nil
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

	// Publish orders.updated after successful DB update. Failures are logged but
	// do not fail the update. orders.created is NOT republished here — it fires
	// exclusively at order creation (CreateOrder); republishing on a confirmed
	// transition would double-create a delivery in delivery-service.
	if s.publisher != nil {
		if err := orderKafka.PublishOrderUpdated(ctx, s.publisher, updated, string(oldStatus), string(newStatus)); err != nil {
			slog.Error("failed to publish order updated event", "err", err, "order_id", updated.ID)
		}
	}

	return updated, nil
}

// syncForwardChain is the LINEAR happy-path ordering used by SyncOrderToStatus to
// translate a target status into a forward ordinal. Branch/terminal statuses
// (cancelled, failed, returned) are deliberately NOT on this chain: an event
// targeting them is treated as off-chain and produces a safe no-op.
var syncForwardChain = []domain.OrderStatus{
	domain.StatusCreated,
	domain.StatusConfirmed,
	domain.StatusAssigned,
	domain.StatusPickedUp,
	domain.StatusInTransit,
	domain.StatusDelivered,
}

// syncOrdinal returns the forward index of s on syncForwardChain and whether s
// is on the chain at all.
func syncOrdinal(s domain.OrderStatus) (int, bool) {
	for i, cs := range syncForwardChain {
		if cs == s {
			return i, true
		}
	}
	return -1, false
}

// SyncOrderToStatus advances an order FORWARD along the legal state machine until
// it reaches target. It is the read side of the delivery->order status sync:
// delivery-service publishes delivery.assigned / delivery.status, and this method
// brings the order status in line with delivery reality.
//
// It is idempotent and safe under Kafka at-least-once + out-of-order delivery:
//   - order not found            => nil  (non-retryable; do not wedge the consumer)
//   - other repo error           => wrapped error (retryable; offset NOT committed)
//   - current == target          => nil  (already synced)
//   - ordinal(current) >= ordinal(target) => nil (stale/out-of-order; no regression)
//   - current is off-chain (cancelled/failed/returned/delivered terminal) => nil
//   - target is off-chain / unknown ordinal => nil
//
// Otherwise it walks ONE legal edge at a time (created->confirmed->assigned->
// picked_up->in_transit->delivered) via the existing UpdateStatus using
// domain.RoleAdmin (bypasses only the role map; allowedTransitions is still
// satisfied because each call moves exactly one legal edge). If any step errors,
// the error is wrapped and returned so the Kafka offset is not committed and the
// event is retried.
func (s *OrderService) SyncOrderToStatus(ctx context.Context, orderID string, target domain.OrderStatus) error {
	order, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, pkgerrors.ErrNotFound) {
			slog.InfoContext(ctx, "sync skipped: order not found (non-retryable no-op)",
				"order_id", orderID, "target", target)
			return nil
		}
		return fmt.Errorf("sync order to status: load order %s: %w", orderID, err)
	}

	slog.InfoContext(ctx, "syncing order to status", "order_id", orderID,
		"current", order.Status, "target", target)

	targetOrd, targetOK := syncOrdinal(target)
	if !targetOK {
		slog.InfoContext(ctx, "sync no-op: target not on linear chain",
			"order_id", orderID, "current", order.Status, "target", target)
		return nil
	}

	currentOrd, currentOK := syncOrdinal(order.Status)
	if !currentOK {
		// Branch/terminal state (cancelled/failed/returned) — terminal for sync.
		slog.InfoContext(ctx, "sync no-op: order in terminal/branch state",
			"order_id", orderID, "current", order.Status, "target", target)
		return nil
	}

	if currentOrd >= targetOrd {
		slog.InfoContext(ctx, "sync no-op: order at or beyond target (idempotent/stale)",
			"order_id", orderID, "current", order.Status, "target", target)
		return nil
	}

	// Walk one legal edge per iteration until current reaches target.
	for ord := currentOrd; ord < targetOrd; ord++ {
		next := syncForwardChain[ord+1]
		if _, err := s.UpdateStatus(ctx, orderID, next, domain.RoleAdmin); err != nil {
			return fmt.Errorf("sync order to status: step %s->%s for %s: %w",
				syncForwardChain[ord], next, orderID, err)
		}
	}

	slog.InfoContext(ctx, "order synced to status", "order_id", orderID, "target", target)
	return nil
}

// GetOrdersByIDs returns slim previews for multiple orders by ID (BKND-02 bulk enrichment).
// Used by delivery-service to enrich ListAvailableOrders with total_price + items_count.
func (s *OrderService) GetOrdersByIDs(ctx context.Context, ids []string) ([]*domain.Order, error) {
	return s.orderRepo.GetByIDs(ctx, ids)
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
