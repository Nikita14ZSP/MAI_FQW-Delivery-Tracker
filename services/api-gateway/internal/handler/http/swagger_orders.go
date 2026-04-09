package http

// --- Swagger model definitions for Order Service ---

// CoordinatesModel represents geographic coordinates.
type CoordinatesModel struct {
	Latitude  float64 `json:"latitude" example:"55.7558"`
	Longitude float64 `json:"longitude" example:"37.6173"`
}

// OrderItemModel represents an item in an order.
type OrderItemModel struct {
	Name     string  `json:"name" example:"Pizza Margherita"`
	Quantity int32   `json:"quantity" example:"2"`
	Price    float64 `json:"price" example:"500"`
}

// OrderModel represents a delivery order.
type OrderModel struct {
	ID                  string           `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	UserID              string           `json:"userId" example:"660e8400-e29b-41d4-a716-446655440000"`
	Status              string           `json:"status" example:"ORDER_STATUS_CREATED"`
	DeliveryAddress     string           `json:"deliveryAddress" example:"ул. Пушкина 10, Москва"`
	DeliveryCoordinates CoordinatesModel `json:"deliveryCoordinates"`
	Items               []OrderItemModel `json:"items"`
	ContactPhone        string           `json:"contactPhone" example:"+79001234567"`
	PaymentMethod       string           `json:"paymentMethod" example:"card"`
	CreatedAt           string           `json:"createdAt" example:"2026-03-29T10:00:00Z"`
	UpdatedAt           string           `json:"updatedAt" example:"2026-03-29T10:00:00Z"`
}

// CreateOrderSwaggerRequest is the request to create an order.
type CreateOrderSwaggerRequest struct {
	DeliveryAddress     string           `json:"delivery_address" example:"ул. Пушкина 10, Москва"`
	DeliveryCoordinates CoordinatesModel `json:"delivery_coordinates"`
	Items               []OrderItemModel `json:"items"`
	ContactPhone        string           `json:"contact_phone" example:"+79001234567"`
	PaymentMethod       string           `json:"payment_method" example:"card"`
}

// CreateOrderSwaggerResponse wraps the created order.
type CreateOrderSwaggerResponse struct {
	Order OrderModel `json:"order"`
}

// GetOrderSwaggerResponse wraps a single order.
type GetOrderSwaggerResponse struct {
	Order OrderModel `json:"order"`
}

// ListOrdersSwaggerResponse wraps a list of orders.
type ListOrdersSwaggerResponse struct {
	Orders []OrderModel `json:"orders"`
}

// UpdateOrderStatusSwaggerRequest is the request to update order status.
type UpdateOrderStatusSwaggerRequest struct {
	Status string `json:"status" example:"ORDER_STATUS_CONFIRMED"`
}

// UpdateOrderStatusSwaggerResponse wraps the updated order.
type UpdateOrderStatusSwaggerResponse struct {
	Order OrderModel `json:"order"`
}

// CancelOrderSwaggerRequest is the request to cancel an order.
type CancelOrderSwaggerRequest struct {
	Reason string `json:"reason" example:"Передумал"`
}

// CancelOrderSwaggerResponse wraps the cancelled order.
type CancelOrderSwaggerResponse struct {
	Order OrderModel `json:"order"`
}

// --- Swagger-only route documentation (grpc-gateway handles actual routing) ---

// CreateOrder godoc
// @Summary Create a new order
// @Description Create a delivery order with address, coordinates, items and payment method
// @Tags orders
// @Accept json
// @Produce json
// @Param body body CreateOrderSwaggerRequest true "Order data"
// @Success 200 {object} CreateOrderSwaggerResponse
// @Failure 400 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/orders [post]
func _swaggerCreateOrder() {}

// GetOrder godoc
// @Summary Get order by ID
// @Description Retrieve order details by ID
// @Tags orders
// @Produce json
// @Param id path string true "Order ID"
// @Success 200 {object} GetOrderSwaggerResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/orders/{id} [get]
func _swaggerGetOrder() {}

// ListOrders godoc
// @Summary List orders
// @Description Retrieve a list of orders for the authenticated user
// @Tags orders
// @Produce json
// @Param user_id query string false "Filter by user ID"
// @Param status query string false "Filter by status (e.g. ORDER_STATUS_CREATED)"
// @Success 200 {object} ListOrdersSwaggerResponse
// @Security BearerAuth
// @Router /v1/orders [get]
func _swaggerListOrders() {}

// UpdateOrderStatus godoc
// @Summary Update order status
// @Description Transition order to a new status following the state machine rules
// @Tags orders
// @Accept json
// @Produce json
// @Param id path string true "Order ID"
// @Param body body UpdateOrderStatusSwaggerRequest true "New status"
// @Success 200 {object} UpdateOrderStatusSwaggerResponse
// @Failure 400 {object} ErrorResponse "Invalid status transition"
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/orders/{id}/status [patch]
func _swaggerUpdateOrderStatus() {}

// CancelOrder godoc
// @Summary Cancel an order
// @Description Cancel an eligible order with an optional reason
// @Tags orders
// @Accept json
// @Produce json
// @Param id path string true "Order ID"
// @Param body body CancelOrderSwaggerRequest true "Cancel reason"
// @Success 200 {object} CancelOrderSwaggerResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/orders/{id}/cancel [post]
func _swaggerCancelOrder() {}
