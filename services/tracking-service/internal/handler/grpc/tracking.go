package grpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonv1 "github.com/mozgovojnikita/delivery-tracker/gen/common/v1"
	trackingv1 "github.com/mozgovojnikita/delivery-tracker/gen/tracking/v1"
	pkgerrors "github.com/mozgovojnikita/delivery-tracker/pkg/errors"
	"github.com/mozgovojnikita/delivery-tracker/services/tracking-service/internal/service"
)

// TrackingHandler implements trackingv1.TrackingServiceServer.
type TrackingHandler struct {
	trackingv1.UnimplementedTrackingServiceServer
	svc service.TrackingServiceIface
}

// NewTrackingHandler creates a new gRPC tracking handler.
func NewTrackingHandler(svc service.TrackingServiceIface) *TrackingHandler {
	return &TrackingHandler{svc: svc}
}

// UpdateLocation updates the current position of a courier.
func (h *TrackingHandler) UpdateLocation(ctx context.Context, req *trackingv1.UpdateLocationRequest) (*trackingv1.UpdateLocationResponse, error) {
	if req.GetCourierId() == "" {
		return nil, status.Error(codes.InvalidArgument, "courier_id is required")
	}
	if req.GetOrderId() == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}
	if req.GetCoordinates() == nil {
		return nil, status.Error(codes.InvalidArgument, "coordinates is required")
	}

	input := service.UpdateLocationInput{
		CourierID: req.GetCourierId(),
		OrderID:   req.GetOrderId(),
		Lat:       req.GetCoordinates().GetLatitude(),
		Lng:       req.GetCoordinates().GetLongitude(),
	}
	if err := h.svc.UpdateLocation(ctx, input); err != nil {
		return nil, toGRPCError(err)
	}
	return &trackingv1.UpdateLocationResponse{Success: true}, nil
}

// GetCourierLocation retrieves the last known location of a courier.
// Returns cache data if available, otherwise falls back to DB.
func (h *TrackingHandler) GetCourierLocation(ctx context.Context, req *trackingv1.GetCourierLocationRequest) (*trackingv1.GetCourierLocationResponse, error) {
	if req.GetCourierId() == "" {
		return nil, status.Error(codes.InvalidArgument, "courier_id is required")
	}

	cachedLoc, dbLoc, err := h.svc.GetCourierLocation(ctx, req.GetCourierId())
	if err != nil {
		return nil, toGRPCError(err)
	}

	if cachedLoc != nil {
		return &trackingv1.GetCourierLocationResponse{
			Location: &trackingv1.LocationUpdate{
				CourierId: cachedLoc.CourierID,
				OrderId:   cachedLoc.OrderID,
				Coordinates: &commonv1.Coordinates{
					Latitude:  cachedLoc.Lat,
					Longitude: cachedLoc.Lng,
				},
				Timestamp: timestamppb.Now(),
			},
		}, nil
	}

	if dbLoc != nil {
		return &trackingv1.GetCourierLocationResponse{
			Location: &trackingv1.LocationUpdate{
				CourierId: dbLoc.CourierID,
				OrderId:   dbLoc.OrderID,
				Coordinates: &commonv1.Coordinates{
					Latitude:  dbLoc.Lat,
					Longitude: dbLoc.Lng,
				},
				Timestamp: timestamppb.New(dbLoc.RecordedAt),
			},
		}, nil
	}

	return nil, status.Error(codes.NotFound, "courier location not found")
}

// GetLocationHistory retrieves the location history for a courier/order.
// IMPORTANT: Both courier_id and order_id must be provided. Service queries by order_id.
func (h *TrackingHandler) GetLocationHistory(ctx context.Context, req *trackingv1.GetLocationHistoryRequest) (*trackingv1.GetLocationHistoryResponse, error) {
	if req.GetCourierId() == "" {
		return nil, status.Error(codes.InvalidArgument, "courier_id is required")
	}
	if req.GetOrderId() == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}

	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 50
	}

	// Service layer queries PostGIS by order_id, so pass order_id (not courier_id).
	points, err := h.svc.GetLocationHistory(ctx, req.GetOrderId(), limit)
	if err != nil {
		return nil, toGRPCError(err)
	}

	protoLocations := make([]*trackingv1.LocationUpdate, 0, len(points))
	for _, pt := range points {
		protoLocations = append(protoLocations, &trackingv1.LocationUpdate{
			CourierId: pt.CourierID,
			OrderId:   pt.OrderID,
			Coordinates: &commonv1.Coordinates{
				Latitude:  pt.Lat,
				Longitude: pt.Lng,
			},
			Timestamp: timestamppb.New(pt.RecordedAt),
		})
	}
	return &trackingv1.GetLocationHistoryResponse{Locations: protoLocations}, nil
}

// GetHealth returns the health status of the tracking service.
func (h *TrackingHandler) GetHealth(ctx context.Context, req *trackingv1.HealthRequest) (*trackingv1.HealthResponse, error) {
	return &trackingv1.HealthResponse{Status: "ok"}, nil
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
