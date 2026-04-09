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
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
