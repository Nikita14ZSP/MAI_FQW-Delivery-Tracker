package http

// --- Swagger model definitions for Delivery Service ---

// DeliveryModel represents a delivery assignment.
type DeliveryModel struct {
	ID                string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	OrderID           string `json:"orderId" example:"660e8400-e29b-41d4-a716-446655440000"`
	CourierID         string `json:"courierId" example:"770e8400-e29b-41d4-a716-446655440000"`
	Status            string `json:"status" example:"DELIVERY_STATUS_ASSIGNED"`
	ZoneID            string `json:"zoneId" example:"880e8400-e29b-41d4-a716-446655440000"`
	EstimatedDelivery string `json:"estimatedDelivery" example:"2026-03-29T12:00:00Z"`
	CreatedAt         string `json:"createdAt" example:"2026-03-29T10:00:00Z"`
}

// AssignCourierRequest is the request to assign a courier.
type AssignCourierSwaggerRequest struct {
	OrderID   string `json:"orderId" example:"660e8400-e29b-41d4-a716-446655440000"`
	ZoneID    string `json:"zoneId" example:"880e8400-e29b-41d4-a716-446655440000"`
	CourierID string `json:"courierId,omitempty" example:"770e8400-e29b-41d4-a716-446655440000"`
}

// AssignCourierResponse wraps the delivery result.
type AssignCourierSwaggerResponse struct {
	Delivery DeliveryModel `json:"delivery"`
}

// GetDeliveryResponse wraps a single delivery.
type GetDeliverySwaggerResponse struct {
	Delivery DeliveryModel `json:"delivery"`
}

// DeliveryZoneModel represents a delivery zone.
type DeliveryZoneModel struct {
	ID             string `json:"id" example:"880e8400-e29b-41d4-a716-446655440000"`
	Name           string `json:"name" example:"Центральная Москва"`
	PolygonGeojson string `json:"polygonGeojson" example:"{\"type\":\"Polygon\",\"coordinates\":[[[37.5,55.7],[37.7,55.7],[37.7,55.8],[37.5,55.8],[37.5,55.7]]]}"`
}

// CreateZoneSwaggerRequest is the request to create a zone.
type CreateZoneSwaggerRequest struct {
	Name           string `json:"name" example:"Центральная Москва"`
	PolygonGeojson string `json:"polygonGeojson" example:"{\"type\":\"Polygon\",\"coordinates\":[[[37.5,55.7],[37.7,55.7],[37.7,55.8],[37.5,55.8],[37.5,55.7]]]}"`
}

// CreateZoneSwaggerResponse wraps the created zone.
type CreateZoneSwaggerResponse struct {
	Zone DeliveryZoneModel `json:"zone"`
}

// ListZonesSwaggerResponse wraps a list of zones.
type ListZonesSwaggerResponse struct {
	Zones []DeliveryZoneModel `json:"zones"`
}

// AssignCourierToZoneSwaggerRequest assigns a courier to a zone.
type AssignCourierToZoneSwaggerRequest struct {
	CourierID string `json:"courierId" example:"770e8400-e29b-41d4-a716-446655440000"`
	ZoneID    string `json:"zoneId" example:"880e8400-e29b-41d4-a716-446655440000"`
}

// AssignCourierToZoneSwaggerResponse is the response.
type AssignCourierToZoneSwaggerResponse struct {
	Success bool `json:"success" example:"true"`
}

// --- Swagger-only route documentation (grpc-gateway handles actual routing) ---

// AssignCourier godoc
// @Summary Assign courier to delivery
// @Description Manual assignment (provide courier_id) or auto-assignment (leave courier_id empty, system finds nearest in zone via PostGIS)
// @Tags deliveries
// @Accept json
// @Produce json
// @Param body body AssignCourierSwaggerRequest true "Assignment request"
// @Success 200 {object} AssignCourierSwaggerResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/deliveries/assign [post]
func _swaggerAssignCourier() {}

// GetDelivery godoc
// @Summary Get delivery by ID
// @Description Retrieve delivery details including courier, status and ETA
// @Tags deliveries
// @Produce json
// @Param id path string true "Delivery ID"
// @Success 200 {object} GetDeliverySwaggerResponse
// @Failure 404 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/deliveries/{id} [get]
func _swaggerGetDelivery() {}

// CreateZone godoc
// @Summary Create delivery zone
// @Description Create a new delivery zone with a GeoJSON polygon boundary
// @Tags zones
// @Accept json
// @Produce json
// @Param body body CreateZoneSwaggerRequest true "Zone definition"
// @Success 200 {object} CreateZoneSwaggerResponse
// @Failure 400 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/zones [post]
func _swaggerCreateZone() {}

// ListZones godoc
// @Summary List delivery zones
// @Description Retrieve all delivery zones
// @Tags zones
// @Produce json
// @Success 200 {object} ListZonesSwaggerResponse
// @Security BearerAuth
// @Router /v1/zones [get]
func _swaggerListZones() {}

// AssignCourierToZone godoc
// @Summary Assign courier to zone
// @Description Assign a courier to operate in a specific delivery zone
// @Tags zones
// @Accept json
// @Produce json
// @Param body body AssignCourierToZoneSwaggerRequest true "Assignment"
// @Success 200 {object} AssignCourierToZoneSwaggerResponse
// @Failure 400 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/zones/assign-courier [post]
func _swaggerAssignCourierToZone() {}
