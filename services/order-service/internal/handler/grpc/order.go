package grpc

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonv1 "github.com/mozgovojnikita/delivery-tracker/gen/common/v1"
	orderv1 "github.com/mozgovojnikita/delivery-tracker/gen/order/v1"
	pkgerrors "github.com/mozgovojnikita/delivery-tracker/pkg/errors"
	"github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/domain"
	"github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/service"
)

// OrderHandler implements orderv1.OrderServiceServer.
type OrderHandler struct {
	orderv1.UnimplementedOrderServiceServer
	svc *service.OrderService
}

// NewOrderHandler creates a new gRPC order handler.
func NewOrderHandler(svc *service.OrderService) *OrderHandler {
	return &OrderHandler{svc: svc}
}

// CreateOrder creates a new delivery order.
// User ID is extracted from gRPC metadata header "x-user-id" set by the API Gateway.
func (h *OrderHandler) CreateOrder(ctx context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
	userID := userIDFromMD(ctx)
	if userID == "" {
		return nil, status.Error(codes.Unauthenticated, "missing x-user-id metadata")
	}

	var items json.RawMessage
	if len(req.GetItems()) > 0 {
		var err error
		items, err = protoItemsToJSON(req.GetItems())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid items: %v", err)
		}
	}

	var lat, lng float64
	if coords := req.GetDeliveryCoordinates(); coords != nil {
		lat = coords.GetLatitude()
		lng = coords.GetLongitude()
	}

	input := domain.CreateOrderInput{
		UserID:          userID,
		DeliveryAddress: req.GetDeliveryAddress(),
		DeliveryLat:     lat,
		DeliveryLng:     lng,
		Items:           items,
		ContactPhone:    req.GetContactPhone(),
		PaymentMethod:   req.GetPaymentMethod(),
	}

	order, err := h.svc.CreateOrder(ctx, input)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &orderv1.CreateOrderResponse{Order: toProtoOrder(order)}, nil
}

// GetOrder retrieves an order by ID.
func (h *OrderHandler) GetOrder(ctx context.Context, req *orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error) {
	order, err := h.svc.GetOrder(ctx, req.GetId())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &orderv1.GetOrderResponse{Order: toProtoOrder(order)}, nil
}

// ListOrders retrieves a paginated list of orders for the authenticated user.
func (h *OrderHandler) ListOrders(ctx context.Context, req *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error) {
	userID := userIDFromMD(ctx)
	if userID == "" {
		return nil, status.Error(codes.Unauthenticated, "missing x-user-id metadata")
	}

	var statusPtr *domain.OrderStatus
	if req.GetStatus() != orderv1.OrderStatus_ORDER_STATUS_UNSPECIFIED {
		ds := protoStatusToDomain(req.GetStatus())
		statusPtr = &ds
	}

	page := int32(1)
	pageSize := int32(20)
	if p := req.GetPagination(); p != nil {
		if p.GetPage() > 0 {
			page = p.GetPage()
		}
		if p.GetPageSize() > 0 {
			pageSize = p.GetPageSize()
		}
	}

	orders, total, err := h.svc.ListOrders(ctx, userID, statusPtr, int(page), int(pageSize))
	if err != nil {
		return nil, toGRPCError(err)
	}

	protoOrders := make([]*orderv1.Order, 0, len(orders))
	for _, o := range orders {
		protoOrders = append(protoOrders, toProtoOrder(o))
	}

	return &orderv1.ListOrdersResponse{
		Orders: protoOrders,
		Pagination: &commonv1.PaginationResponse{
			Total:    int32(total),
			Page:     page,
			PageSize: pageSize,
		},
	}, nil
}

// UpdateOrderStatus transitions an order to a new status.
// Role is extracted from gRPC metadata header "x-user-role".
func (h *OrderHandler) UpdateOrderStatus(ctx context.Context, req *orderv1.UpdateOrderStatusRequest) (*orderv1.UpdateOrderStatusResponse, error) {
	role := domain.Role(userRoleFromMD(ctx))
	if role == "" {
		role = domain.RoleUser
	}

	domainStatus := protoStatusToDomain(req.GetStatus())

	order, err := h.svc.UpdateStatus(ctx, req.GetId(), domainStatus, role)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &orderv1.UpdateOrderStatusResponse{Order: toProtoOrder(order)}, nil
}

// CancelOrder cancels an existing order.
// Role is extracted from gRPC metadata header "x-user-role".
func (h *OrderHandler) CancelOrder(ctx context.Context, req *orderv1.CancelOrderRequest) (*orderv1.CancelOrderResponse, error) {
	role := domain.Role(userRoleFromMD(ctx))
	if role == "" {
		role = domain.RoleUser
	}

	order, err := h.svc.CancelOrder(ctx, req.GetId(), req.GetReason(), role)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &orderv1.CancelOrderResponse{Order: toProtoOrder(order)}, nil
}

// GetHealth returns a health status response.
func (h *OrderHandler) GetHealth(ctx context.Context, req *orderv1.HealthRequest) (*orderv1.HealthResponse, error) {
	return &orderv1.HealthResponse{Status: "ok"}, nil
}

// toProtoOrder converts a domain.Order to an orderv1.Order proto message.
func toProtoOrder(o *domain.Order) *orderv1.Order {
	if o == nil {
		return nil
	}

	var items []*orderv1.OrderItem
	if len(o.Items) > 0 {
		type itemJSON struct {
			Name     string  `json:"name"`
			Quantity int32   `json:"quantity"`
			Price    float64 `json:"price"`
		}
		var raw []itemJSON
		if err := json.Unmarshal(o.Items, &raw); err == nil {
			for _, it := range raw {
				items = append(items, &orderv1.OrderItem{
					Name:     it.Name,
					Quantity: it.Quantity,
					Price:    it.Price,
				})
			}
		}
	}

	return &orderv1.Order{
		Id:              o.ID,
		UserId:          o.UserID,
		Status:          domainStatusToProto(o.Status),
		DeliveryAddress: o.DeliveryAddress,
		DeliveryCoordinates: &commonv1.Coordinates{
			Latitude:  o.DeliveryLat,
			Longitude: o.DeliveryLng,
		},
		Items:         items,
		ContactPhone:  o.ContactPhone,
		PaymentMethod: o.PaymentMethod,
		CreatedAt:     timestamppb.New(o.CreatedAt),
		UpdatedAt:     timestamppb.New(o.UpdatedAt),
	}
}

// toGRPCError maps domain sentinel errors to gRPC status codes.
func toGRPCError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case isErr(err, pkgerrors.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case isErr(err, pkgerrors.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case isErr(err, pkgerrors.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	case isErr(err, pkgerrors.ErrUnauthorized):
		return status.Error(codes.Unauthenticated, err.Error())
	case isErr(err, pkgerrors.ErrForbidden):
		return status.Error(codes.PermissionDenied, err.Error())
	default:
		return status.Error(codes.Internal, "internal error")
	}
}

// isErr checks if err wraps target using errors.Is semantics.
func isErr(err, target error) bool {
	unwrapped := err
	for unwrapped != nil {
		if unwrapped == target {
			return true
		}
		type unwrapper interface{ Unwrap() error }
		if u, ok := unwrapped.(unwrapper); ok {
			unwrapped = u.Unwrap()
		} else {
			break
		}
	}
	return false
}

// userIDFromMD extracts the "x-user-id" value from gRPC incoming metadata.
func userIDFromMD(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	vals := md.Get("x-user-id")
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

// userRoleFromMD extracts the "x-user-role" value from gRPC incoming metadata.
func userRoleFromMD(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	vals := md.Get("x-user-role")
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

// domainStatusToProto maps a domain.OrderStatus string to its proto enum value.
func domainStatusToProto(s domain.OrderStatus) orderv1.OrderStatus {
	m := map[domain.OrderStatus]orderv1.OrderStatus{
		domain.StatusCreated:   orderv1.OrderStatus_ORDER_STATUS_CREATED,
		domain.StatusConfirmed: orderv1.OrderStatus_ORDER_STATUS_CONFIRMED,
		domain.StatusAssigned:  orderv1.OrderStatus_ORDER_STATUS_ASSIGNED,
		domain.StatusPickedUp:  orderv1.OrderStatus_ORDER_STATUS_PICKED_UP,
		domain.StatusInTransit: orderv1.OrderStatus_ORDER_STATUS_IN_TRANSIT,
		domain.StatusDelivered: orderv1.OrderStatus_ORDER_STATUS_DELIVERED,
		domain.StatusCancelled: orderv1.OrderStatus_ORDER_STATUS_CANCELLED,
		domain.StatusFailed:    orderv1.OrderStatus_ORDER_STATUS_FAILED,
		domain.StatusReturned:  orderv1.OrderStatus_ORDER_STATUS_RETURNED,
	}
	if v, ok := m[s]; ok {
		return v
	}
	return orderv1.OrderStatus_ORDER_STATUS_UNSPECIFIED
}

// protoStatusToDomain maps a proto OrderStatus enum to domain.OrderStatus string.
func protoStatusToDomain(s orderv1.OrderStatus) domain.OrderStatus {
	m := map[orderv1.OrderStatus]domain.OrderStatus{
		orderv1.OrderStatus_ORDER_STATUS_CREATED:   domain.StatusCreated,
		orderv1.OrderStatus_ORDER_STATUS_CONFIRMED:  domain.StatusConfirmed,
		orderv1.OrderStatus_ORDER_STATUS_ASSIGNED:   domain.StatusAssigned,
		orderv1.OrderStatus_ORDER_STATUS_PICKED_UP:  domain.StatusPickedUp,
		orderv1.OrderStatus_ORDER_STATUS_IN_TRANSIT: domain.StatusInTransit,
		orderv1.OrderStatus_ORDER_STATUS_DELIVERED:  domain.StatusDelivered,
		orderv1.OrderStatus_ORDER_STATUS_CANCELLED:  domain.StatusCancelled,
		orderv1.OrderStatus_ORDER_STATUS_FAILED:     domain.StatusFailed,
		orderv1.OrderStatus_ORDER_STATUS_RETURNED:   domain.StatusReturned,
	}
	if v, ok := m[s]; ok {
		return v
	}
	return domain.StatusCreated
}

// protoItemsToJSON converts a slice of proto OrderItems to JSON.
func protoItemsToJSON(items []*orderv1.OrderItem) (json.RawMessage, error) {
	type itemJSON struct {
		Name     string  `json:"name"`
		Quantity int32   `json:"quantity"`
		Price    float64 `json:"price"`
	}
	raw := make([]itemJSON, 0, len(items))
	for _, item := range items {
		raw = append(raw, itemJSON{
			Name:     item.GetName(),
			Quantity: item.GetQuantity(),
			Price:    item.GetPrice(),
		})
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal items: %w", err)
	}
	return b, nil
}

