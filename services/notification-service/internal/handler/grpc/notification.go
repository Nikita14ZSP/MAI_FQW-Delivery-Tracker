package grpc

import (
	"context"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	notificationv1 "github.com/mozgovojnikita/delivery-tracker/gen/notification/v1"
	pkgerrors "github.com/mozgovojnikita/delivery-tracker/pkg/errors"
	"github.com/mozgovojnikita/delivery-tracker/services/notification-service/internal/domain"
)

// NotificationServiceIface defines the service methods used by the gRPC handler.
// This interface enables test mocking without importing the full service package.
type NotificationServiceIface interface {
	SendNotification(ctx context.Context, userID, channel, templateName string, vars map[string]string) (*domain.Notification, error)
	GetNotification(ctx context.Context, id string) (*domain.Notification, error)
}

// NotificationHandler implements notificationv1.NotificationServiceServer.
type NotificationHandler struct {
	notificationv1.UnimplementedNotificationServiceServer
	svc NotificationServiceIface
}

// NewNotificationHandler creates a new gRPC notification handler.
func NewNotificationHandler(svc NotificationServiceIface) *NotificationHandler {
	return &NotificationHandler{svc: svc}
}

// SendNotification sends a notification to a user via the specified channel.
func (h *NotificationHandler) SendNotification(ctx context.Context, req *notificationv1.SendNotificationRequest) (*notificationv1.SendNotificationResponse, error) {
	vars := req.GetTemplateVars()
	if vars == nil {
		vars = make(map[string]string)
	}

	n, err := h.svc.SendNotification(ctx, req.GetUserId(), req.GetChannel(), req.GetTemplateName(), vars)
	if err != nil {
		slog.Error("SendNotification failed", "err", err)
		return nil, toGRPCError(err)
	}

	return &notificationv1.SendNotificationResponse{
		Notification: toProtoNotification(n),
	}, nil
}

// GetNotification retrieves a notification by ID.
func (h *NotificationHandler) GetNotification(ctx context.Context, req *notificationv1.GetNotificationRequest) (*notificationv1.GetNotificationResponse, error) {
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	n, err := h.svc.GetNotification(ctx, req.GetId())
	if err != nil {
		slog.Error("GetNotification failed", "err", err, "id", req.GetId())
		return nil, toGRPCError(err)
	}

	return &notificationv1.GetNotificationResponse{
		Notification: toProtoNotification(n),
	}, nil
}

// GetHealth returns a health status response.
func (h *NotificationHandler) GetHealth(ctx context.Context, req *notificationv1.HealthRequest) (*notificationv1.HealthResponse, error) {
	return &notificationv1.HealthResponse{Status: "ok"}, nil
}

// toProtoNotification converts a domain.Notification to a notificationv1.Notification proto message.
func toProtoNotification(n *domain.Notification) *notificationv1.Notification {
	if n == nil {
		return nil
	}
	return &notificationv1.Notification{
		Id:           n.ID,
		UserId:       n.UserID,
		Channel:      n.Channel,
		TemplateName: n.TemplateName,
		TemplateVars: n.TemplateVars,
		Status:       domainStatusToProto(n.Status),
		CreatedAt:    timestamppb.New(n.CreatedAt),
	}
}

// domainStatusToProto maps a domain.NotificationStatus to its proto enum value.
func domainStatusToProto(s domain.NotificationStatus) notificationv1.NotificationStatus {
	m := map[domain.NotificationStatus]notificationv1.NotificationStatus{
		domain.StatusPending: notificationv1.NotificationStatus_NOTIFICATION_STATUS_PENDING,
		domain.StatusSent:    notificationv1.NotificationStatus_NOTIFICATION_STATUS_SENT,
		domain.StatusFailed:  notificationv1.NotificationStatus_NOTIFICATION_STATUS_FAILED,
	}
	if v, ok := m[s]; ok {
		return v
	}
	return notificationv1.NotificationStatus_NOTIFICATION_STATUS_UNSPECIFIED
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
