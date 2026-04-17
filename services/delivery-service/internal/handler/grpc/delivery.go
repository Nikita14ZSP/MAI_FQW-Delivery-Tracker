package grpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonv1 "github.com/mozgovojnikita/delivery-tracker/gen/common/v1"
	deliveryv1 "github.com/mozgovojnikita/delivery-tracker/gen/delivery/v1"
	pkgerrors "github.com/mozgovojnikita/delivery-tracker/pkg/errors"
	"github.com/mozgovojnikita/delivery-tracker/services/delivery-service/internal/domain"
	"github.com/mozgovojnikita/delivery-tracker/services/delivery-service/internal/service"
)

// DeliveryHandler implements deliveryv1.DeliveryServiceServer.
type DeliveryHandler struct {
	deliveryv1.UnimplementedDeliveryServiceServer
	svc *service.DeliveryService
}

// NewDeliveryHandler creates a new gRPC delivery handler.
func NewDeliveryHandler(svc *service.DeliveryService) *DeliveryHandler {
	return &DeliveryHandler{svc: svc}
}

// AssignCourier assigns a courier to an order.
// If courier_id is set, performs manual assignment (DLVR-01).
// If courier_id is empty, returns InvalidArgument — auto-assignment happens via Kafka orders.created events (DLVR-02).
func (h *DeliveryHandler) AssignCourier(ctx context.Context, req *deliveryv1.AssignCourierRequest) (*deliveryv1.AssignCourierResponse, error) {
	if req.GetOrderId() == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}

	if req.GetCourierId() == "" {
		// Auto-assignment happens via Kafka orders.created events, not via direct RPC.
		return nil, status.Error(codes.InvalidArgument, "courier_id required for manual assignment; auto-assignment happens via Kafka orders.created events")
	}

	delivery, err := h.svc.ManualAssignCourier(ctx, req.GetOrderId(), req.GetCourierId())
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &deliveryv1.AssignCourierResponse{
		Delivery: toProtoDelivery(delivery),
	}, nil
}

// GetDelivery retrieves a delivery by ID.
func (h *DeliveryHandler) GetDelivery(ctx context.Context, req *deliveryv1.GetDeliveryRequest) (*deliveryv1.GetDeliveryResponse, error) {
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	delivery, err := h.svc.GetDelivery(ctx, req.GetId())
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &deliveryv1.GetDeliveryResponse{
		Delivery: toProtoDelivery(delivery),
	}, nil
}

// CreateZone creates a new delivery zone (DLVR-03).
func (h *DeliveryHandler) CreateZone(ctx context.Context, req *deliveryv1.CreateZoneRequest) (*deliveryv1.CreateZoneResponse, error) {
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.GetPolygonGeojson() == "" {
		return nil, status.Error(codes.InvalidArgument, "polygon_geojson is required")
	}

	zone, err := h.svc.CreateZone(ctx, req.GetName(), req.GetPolygonGeojson())
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &deliveryv1.CreateZoneResponse{
		Zone: toProtoZone(zone),
	}, nil
}

// ListZones retrieves a paginated list of delivery zones (DLVR-03).
func (h *DeliveryHandler) ListZones(ctx context.Context, req *deliveryv1.ListZonesRequest) (*deliveryv1.ListZonesResponse, error) {
	page := int(1)
	pageSize := int(20)

	if p := req.GetPagination(); p != nil {
		if p.GetPage() > 0 {
			page = int(p.GetPage())
		}
		if p.GetPageSize() > 0 {
			pageSize = int(p.GetPageSize())
		}
	}

	zones, total, err := h.svc.ListZones(ctx, page, pageSize)
	if err != nil {
		return nil, toGRPCError(err)
	}

	protoZones := make([]*deliveryv1.DeliveryZone, 0, len(zones))
	for _, z := range zones {
		protoZones = append(protoZones, toProtoZone(z))
	}

	return &deliveryv1.ListZonesResponse{
		Zones: protoZones,
		Pagination: &commonv1.PaginationResponse{
			Total:    int32(total),
			Page:     int32(page),
			PageSize: int32(pageSize),
		},
	}, nil
}

// AssignCourierToZone assigns a courier to a delivery zone (DLVR-04).
func (h *DeliveryHandler) AssignCourierToZone(ctx context.Context, req *deliveryv1.AssignCourierToZoneRequest) (*deliveryv1.AssignCourierToZoneResponse, error) {
	if req.GetCourierId() == "" {
		return nil, status.Error(codes.InvalidArgument, "courier_id is required")
	}
	if req.GetZoneId() == "" {
		return nil, status.Error(codes.InvalidArgument, "zone_id is required")
	}

	if err := h.svc.AssignCourierToZone(ctx, req.GetCourierId(), req.GetZoneId()); err != nil {
		return nil, toGRPCError(err)
	}

	return &deliveryv1.AssignCourierToZoneResponse{Success: true}, nil
}

// GetDeliveriesByCourier retrieves all active deliveries for a courier (TRAK-05).
func (h *DeliveryHandler) GetDeliveriesByCourier(ctx context.Context, req *deliveryv1.GetDeliveriesByCourierRequest) (*deliveryv1.GetDeliveriesByCourierResponse, error) {
	if req.GetCourierId() == "" {
		return nil, status.Error(codes.InvalidArgument, "courier_id is required")
	}
	deliveries, err := h.svc.ListDeliveriesByCourier(ctx, req.GetCourierId())
	if err != nil {
		return nil, toGRPCError(err)
	}
	protoDeliveries := make([]*deliveryv1.Delivery, 0, len(deliveries))
	for _, d := range deliveries {
		protoDeliveries = append(protoDeliveries, toProtoDelivery(d))
	}
	return &deliveryv1.GetDeliveriesByCourierResponse{Deliveries: protoDeliveries}, nil
}

// userIDFromMD extracts the x-user-id value from gRPC incoming metadata.
// Returns "" if metadata is absent or the key is not set.
// api-gateway forwards the JWT sub claim as the x-user-id header (D-01).
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

// AcceptOrder handles BKND-03: courier self-assigns a pending delivery (D-07..D-09).
// POST /v1/couriers/me/deliveries/{delivery_id}/accept — courier_id from x-user-id metadata.
// Error mapping (all → HTTP 409 via codes.Aborted per grpc-gateway runtime/errors.go):
//   - ErrAlreadyTaken  → Aborted → 409
//   - ErrMaxActiveReached → Aborted → 409
//   - ErrCourierOffline   → Aborted → 409
func (h *DeliveryHandler) AcceptOrder(ctx context.Context, req *deliveryv1.AcceptOrderRequest) (*deliveryv1.AcceptOrderResponse, error) {
	courierID := userIDFromMD(ctx)
	if courierID == "" {
		return nil, status.Error(codes.Unauthenticated, "missing x-user-id metadata")
	}
	if req.GetDeliveryId() == "" {
		return nil, status.Error(codes.InvalidArgument, "delivery_id is required")
	}
	delivery, err := h.svc.AcceptOrder(ctx, courierID, req.GetDeliveryId())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &deliveryv1.AcceptOrderResponse{Delivery: toProtoDelivery(delivery)}, nil
}

// UpdateCourierStatus handles BKND-01: courier updates own status (D-01, D-02, D-03).
// PATCH /v1/couriers/me/status — courier_id is taken from x-user-id metadata.
func (h *DeliveryHandler) UpdateCourierStatus(ctx context.Context, req *deliveryv1.UpdateCourierStatusRequest) (*deliveryv1.UpdateCourierStatusResponse, error) {
	courierID := userIDFromMD(ctx)
	if courierID == "" {
		return nil, status.Error(codes.Unauthenticated, "missing x-user-id metadata")
	}

	newStatus := req.GetStatus()
	if newStatus != "available" && newStatus != "offline" {
		return nil, status.Errorf(codes.InvalidArgument, "status must be 'available' or 'offline', got %q", newStatus)
	}

	courier, err := h.svc.UpdateCourierStatus(ctx, courierID, newStatus)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &deliveryv1.UpdateCourierStatusResponse{Courier: toProtoCourier(courier)}, nil
}

// toProtoCourier converts a domain.Courier to its proto representation.
func toProtoCourier(c *domain.Courier) *deliveryv1.Courier {
	if c == nil {
		return nil
	}
	return &deliveryv1.Courier{
		Id:        c.ID,
		Status:    c.Status,
		UpdatedAt: timestamppb.New(c.UpdatedAt),
	}
}

// RateCourier submits a rating for a courier after delivery completion (RATE-01).
// Reads x-user-id from gRPC incoming metadata for the rater's identity.
func (h *DeliveryHandler) RateCourier(ctx context.Context, req *deliveryv1.RateCourierRequest) (*deliveryv1.RateCourierResponse, error) {
	if req.GetDeliveryId() == "" {
		return nil, status.Error(codes.InvalidArgument, "delivery_id is required")
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}
	vals := md.Get("x-user-id")
	if len(vals) == 0 || vals[0] == "" {
		return nil, status.Error(codes.Unauthenticated, "x-user-id required")
	}
	userID := vals[0]

	ratingID, err := h.svc.RateCourier(ctx, req.GetDeliveryId(), userID, int(req.GetStars()), req.GetComment())
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &deliveryv1.RateCourierResponse{RatingId: ratingID}, nil
}

// GetDeliveryByOrderID handles BKND-04: cross-service lookup by order ID (D-10).
// Returns the delivery associated with the given order; codes.NotFound when none exists.
// Called by order-service to populate Order.delivery_id in GetOrder responses.
func (h *DeliveryHandler) GetDeliveryByOrderID(ctx context.Context, req *deliveryv1.GetDeliveryByOrderIDRequest) (*deliveryv1.GetDeliveryByOrderIDResponse, error) {
	if req.GetOrderId() == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}
	delivery, err := h.svc.GetDeliveryByOrderID(ctx, req.GetOrderId())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &deliveryv1.GetDeliveryByOrderIDResponse{Delivery: toProtoDelivery(delivery)}, nil
}

// GetCourierRating retrieves a courier's average rating and individual reviews (RATE-02, RATE-03).
// Accessible via REST GET /v1/couriers/{courier_id}/rating through grpc-gateway.
func (h *DeliveryHandler) GetCourierRating(ctx context.Context, req *deliveryv1.GetCourierRatingRequest) (*deliveryv1.GetCourierRatingResponse, error) {
	if req.GetCourierId() == "" {
		return nil, status.Error(codes.InvalidArgument, "courier_id is required")
	}

	result, err := h.svc.GetCourierRating(ctx, req.GetCourierId())
	if err != nil {
		return nil, toGRPCError(err)
	}

	ratings := make([]*deliveryv1.CourierRatingEntry, 0, len(result.Ratings))
	for _, r := range result.Ratings {
		ratings = append(ratings, &deliveryv1.CourierRatingEntry{
			DeliveryId: r.DeliveryID,
			Stars:      int32(r.Stars),
			Comment:    r.Comment,
			CreatedAt:  timestamppb.New(r.CreatedAt),
		})
	}

	return &deliveryv1.GetCourierRatingResponse{
		AverageStars: result.AverageStars,
		TotalRatings: int32(result.TotalRatings),
		Ratings:      ratings,
	}, nil
}

// ListAvailableOrders returns FIFO-ordered, zone-scoped pending deliveries for the courier (BKND-02).
// courier_id is taken from x-user-id gRPC metadata — api-gateway forwards the JWT sub claim (D-01).
func (h *DeliveryHandler) ListAvailableOrders(ctx context.Context, req *deliveryv1.ListAvailableOrdersRequest) (*deliveryv1.ListAvailableOrdersResponse, error) {
	courierID := userIDFromMD(ctx)
	if courierID == "" {
		return nil, status.Error(codes.Unauthenticated, "missing x-user-id metadata")
	}

	rows, nextCursor, err := h.svc.ListAvailableOrders(ctx, courierID, req.GetCursor(), req.GetLimit())
	if err != nil {
		return nil, toGRPCError(err)
	}

	previews := make([]*deliveryv1.AvailableOrderPreview, 0, len(rows))
	for _, r := range rows {
		items := make([]*deliveryv1.OrderItemPreview, 0, len(r.Items))
		for _, it := range r.Items {
			items = append(items, &deliveryv1.OrderItemPreview{
				Name:     it.Name,
				Quantity: it.Quantity,
				Price:    it.Price,
			})
		}
		previews = append(previews, &deliveryv1.AvailableOrderPreview{
			OrderId:         r.OrderID,
			DeliveryId:      r.DeliveryID,
			DeliveryAddress: r.DeliveryAddress,
			TotalPrice:      r.TotalPrice,
			ItemsCount:      r.ItemsCount,
			ZoneName:        r.ZoneName,
			CreatedAt:       timestamppb.New(r.CreatedAt),
			Items:           items,
		})
	}
	return &deliveryv1.ListAvailableOrdersResponse{
		Orders:     previews,
		NextCursor: nextCursor,
	}, nil
}

// GetCourierProfile returns the public profile (name + rating) for a courier (RATE-05).
// Accessible via REST GET /v1/couriers/{courier_id}/profile — public, no auth required.
func (h *DeliveryHandler) GetCourierProfile(ctx context.Context, req *deliveryv1.GetCourierProfileRequest) (*deliveryv1.GetCourierProfileResponse, error) {
	if req.GetCourierId() == "" {
		return nil, status.Error(codes.InvalidArgument, "courier_id is required")
	}
	result, err := h.svc.GetCourierProfile(ctx, req.GetCourierId())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &deliveryv1.GetCourierProfileResponse{
		FirstName:    result.FirstName,
		LastName:     result.LastName,
		AverageStars: result.AverageStars,
		TotalRatings: int32(result.TotalRatings),
	}, nil
}

// GetHealth returns the health status of the delivery service.
func (h *DeliveryHandler) GetHealth(ctx context.Context, req *deliveryv1.HealthRequest) (*deliveryv1.HealthResponse, error) {
	return &deliveryv1.HealthResponse{Status: "ok"}, nil
}

// toProtoDelivery converts a domain.Delivery to a deliveryv1.Delivery proto message.
func toProtoDelivery(d *domain.Delivery) *deliveryv1.Delivery {
	if d == nil {
		return nil
	}

	var estimatedDelivery *timestamppb.Timestamp
	if !d.EstimatedDelivery.IsZero() {
		estimatedDelivery = timestamppb.New(d.EstimatedDelivery)
	}

	return &deliveryv1.Delivery{
		Id:                d.ID,
		OrderId:           d.OrderID,
		CourierId:         d.CourierID,
		Status:            domainStatusToProto(d.Status),
		ZoneId:            d.ZoneID,
		EstimatedDelivery: estimatedDelivery,
		CreatedAt:         timestamppb.New(d.CreatedAt),
	}
}

// toProtoZone converts a domain.DeliveryZone to a deliveryv1.DeliveryZone proto message.
func toProtoZone(z *domain.DeliveryZone) *deliveryv1.DeliveryZone {
	if z == nil {
		return nil
	}
	return &deliveryv1.DeliveryZone{
		Id:             z.ID,
		Name:           z.Name,
		PolygonGeojson: z.PolygonGeoJSON,
	}
}

// domainStatusToProto maps a domain.DeliveryStatus to its proto enum value.
func domainStatusToProto(s domain.DeliveryStatus) deliveryv1.DeliveryStatus {
	m := map[domain.DeliveryStatus]deliveryv1.DeliveryStatus{
		domain.StatusPending:   deliveryv1.DeliveryStatus_DELIVERY_STATUS_PENDING,
		domain.StatusAssigned:  deliveryv1.DeliveryStatus_DELIVERY_STATUS_ASSIGNED,
		domain.StatusPickedUp:  deliveryv1.DeliveryStatus_DELIVERY_STATUS_PICKED_UP,
		domain.StatusInTransit: deliveryv1.DeliveryStatus_DELIVERY_STATUS_IN_TRANSIT,
		domain.StatusDelivered: deliveryv1.DeliveryStatus_DELIVERY_STATUS_DELIVERED,
		domain.StatusFailed:    deliveryv1.DeliveryStatus_DELIVERY_STATUS_FAILED,
	}
	if v, ok := m[s]; ok {
		return v
	}
	return deliveryv1.DeliveryStatus_DELIVERY_STATUS_UNSPECIFIED
}

// toGRPCError maps domain sentinel errors to gRPC status codes.
// codes.Aborted maps to HTTP 409 in grpc-gateway (per runtime/errors.go HTTPStatusFromCode).
func toGRPCError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, pkgerrors.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, pkgerrors.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, pkgerrors.ErrForbidden):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrInvalidStatusTransition):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrActiveDeliveryExists),
		errors.Is(err, domain.ErrAlreadyTaken),
		errors.Is(err, domain.ErrMaxActiveReached),
		errors.Is(err, domain.ErrCourierOffline):
		return status.Error(codes.Aborted, err.Error()) // grpc-gateway → HTTP 409
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
