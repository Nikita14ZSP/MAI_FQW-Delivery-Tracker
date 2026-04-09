package http

// --- Swagger model definitions for Notification Service ---

// NotificationModel represents a notification.
type NotificationModel struct {
	ID           string            `json:"id" example:"990e8400-e29b-41d4-a716-446655440000"`
	UserID       string            `json:"userId" example:"550e8400-e29b-41d4-a716-446655440000"`
	Channel      string            `json:"channel" example:"email"`
	TemplateName string            `json:"templateName" example:"order_confirmed"`
	TemplateVars map[string]string `json:"templateVars" example:"order_id:123,status:confirmed"`
	Status       string            `json:"status" example:"NOTIFICATION_STATUS_SENT"`
	CreatedAt    string            `json:"createdAt" example:"2026-03-29T10:00:00Z"`
}

// SendNotificationSwaggerRequest is the request to send a notification.
type SendNotificationSwaggerRequest struct {
	UserID       string            `json:"userId" example:"550e8400-e29b-41d4-a716-446655440000"`
	Channel      string            `json:"channel" example:"email"`
	TemplateName string            `json:"templateName" example:"order_confirmed"`
	TemplateVars map[string]string `json:"templateVars"`
}

// SendNotificationSwaggerResponse wraps the notification result.
type SendNotificationSwaggerResponse struct {
	Notification NotificationModel `json:"notification"`
}

// GetNotificationSwaggerResponse wraps a single notification.
type GetNotificationSwaggerResponse struct {
	Notification NotificationModel `json:"notification"`
}

// --- Swagger-only route documentation (grpc-gateway handles actual routing) ---

// SendNotification godoc
// @Summary Send a notification
// @Description Send a notification to a user using a template. Normally triggered by Kafka events, but can be called directly.
// @Tags notifications
// @Accept json
// @Produce json
// @Param body body SendNotificationSwaggerRequest true "Notification request"
// @Success 200 {object} SendNotificationSwaggerResponse
// @Failure 400 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/notifications/send [post]
func _swaggerSendNotification() {}

// GetNotification godoc
// @Summary Get notification by ID
// @Description Retrieve notification details by ID
// @Tags notifications
// @Produce json
// @Param id path string true "Notification ID"
// @Success 200 {object} GetNotificationSwaggerResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/notifications/{id} [get]
func _swaggerGetNotification() {}
