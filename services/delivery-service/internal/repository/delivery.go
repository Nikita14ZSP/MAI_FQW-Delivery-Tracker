package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	pkgerrors "github.com/mozgovojnikita/delivery-tracker/pkg/errors"
	"github.com/mozgovojnikita/delivery-tracker/services/delivery-service/internal/domain"
)

// DeliveryRepository defines the data access interface for deliveries.
type DeliveryRepository interface {
	CreateDelivery(ctx context.Context, orderID, zoneID string, lat, lng float64) (*domain.Delivery, error)
	GetDeliveryByID(ctx context.Context, id string) (*domain.Delivery, error)
	GetDeliveryByOrderID(ctx context.Context, orderID string) (*domain.Delivery, error)
	UpdateStatus(ctx context.Context, id string, status domain.DeliveryStatus) error
	AssignCourier(ctx context.Context, deliveryID, courierID string, eta time.Time) error
	ListPendingDeliveries(ctx context.Context) ([]*domain.Delivery, error)
	ListByCourierID(ctx context.Context, courierID string) ([]*domain.Delivery, error)

	FindNearestCourier(ctx context.Context, zoneID string, lat, lng float64) (*domain.CourierCandidate, error)
	CountActiveByCourier(ctx context.Context, courierID string) (int, error)

	CreateZone(ctx context.Context, name, polygonGeoJSON string) (*domain.DeliveryZone, error)
	ListZones(ctx context.Context, limit, offset int) ([]*domain.DeliveryZone, int, error)
	FindZoneByPoint(ctx context.Context, lat, lng float64) (*domain.DeliveryZone, error)

	UpsertCourier(ctx context.Context, courierID string) error
	AssignCourierToZone(ctx context.Context, courierID, zoneID string) error

	CalculateETA(ctx context.Context, zoneID string, lat, lng float64) (time.Time, error)
}

type pgxDeliveryRepository struct {
	pool *pgxpool.Pool
}

// NewDeliveryRepository creates a new DeliveryRepository backed by a pgx connection pool.
func NewDeliveryRepository(pool *pgxpool.Pool) DeliveryRepository {
	return &pgxDeliveryRepository{pool: pool}
}

// scanDelivery scans a row into a Delivery struct, handling nullable fields.
func scanDelivery(row pgx.Row, d *domain.Delivery) error {
	var courierID *string
	var zoneID *string
	var estimatedDelivery *time.Time
	err := row.Scan(
		&d.ID, &d.OrderID, &courierID, &d.Status, &zoneID,
		&estimatedDelivery, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return err
	}
	if courierID != nil {
		d.CourierID = *courierID
	}
	if zoneID != nil {
		d.ZoneID = *zoneID
	}
	if estimatedDelivery != nil {
		d.EstimatedDelivery = *estimatedDelivery
	}
	return nil
}

// CreateDelivery inserts a new delivery with status 'pending' and returns the created record.
func (r *pgxDeliveryRepository) CreateDelivery(ctx context.Context, orderID, zoneID string, lat, lng float64) (*domain.Delivery, error) {
	var query string
	var args []interface{}

	if zoneID == "" {
		query = `
			INSERT INTO deliveries (order_id, status)
			VALUES ($1, 'pending')
			RETURNING id, order_id, courier_id, status, zone_id, estimated_delivery, created_at, updated_at`
		args = []interface{}{orderID}
	} else {
		query = `
			INSERT INTO deliveries (order_id, zone_id, status)
			VALUES ($1, $2, 'pending')
			RETURNING id, order_id, courier_id, status, zone_id, estimated_delivery, created_at, updated_at`
		args = []interface{}{orderID, zoneID}
	}

	d := &domain.Delivery{}
	if err := scanDelivery(r.pool.QueryRow(ctx, query, args...), d); err != nil {
		return nil, fmt.Errorf("create delivery: %w", err)
	}
	return d, nil
}

// GetDeliveryByID retrieves a delivery by its UUID.
// Returns ErrNotFound if no delivery exists with that ID.
func (r *pgxDeliveryRepository) GetDeliveryByID(ctx context.Context, id string) (*domain.Delivery, error) {
	const query = `
		SELECT id, order_id, courier_id, status, zone_id, estimated_delivery, created_at, updated_at
		FROM deliveries
		WHERE id = $1`

	d := &domain.Delivery{}
	if err := scanDelivery(r.pool.QueryRow(ctx, query, id), d); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("delivery %s: %w", id, pkgerrors.ErrNotFound)
		}
		return nil, fmt.Errorf("get delivery by id: %w", err)
	}
	return d, nil
}

// GetDeliveryByOrderID retrieves a delivery by the associated order UUID.
// Returns ErrNotFound if no delivery exists for that order.
func (r *pgxDeliveryRepository) GetDeliveryByOrderID(ctx context.Context, orderID string) (*domain.Delivery, error) {
	const query = `
		SELECT id, order_id, courier_id, status, zone_id, estimated_delivery, created_at, updated_at
		FROM deliveries
		WHERE order_id = $1`

	d := &domain.Delivery{}
	if err := scanDelivery(r.pool.QueryRow(ctx, query, orderID), d); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("delivery for order %s: %w", orderID, pkgerrors.ErrNotFound)
		}
		return nil, fmt.Errorf("get delivery by order id: %w", err)
	}
	return d, nil
}

// UpdateStatus updates the delivery's status.
func (r *pgxDeliveryRepository) UpdateStatus(ctx context.Context, id string, status domain.DeliveryStatus) error {
	const query = `UPDATE deliveries SET status = $1, updated_at = NOW() WHERE id = $2`
	tag, err := r.pool.Exec(ctx, query, string(status), id)
	if err != nil {
		return fmt.Errorf("update delivery status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("delivery %s: %w", id, pkgerrors.ErrNotFound)
	}
	return nil
}

// AssignCourier assigns a courier to a delivery and sets the estimated delivery time.
func (r *pgxDeliveryRepository) AssignCourier(ctx context.Context, deliveryID, courierID string, eta time.Time) error {
	const query = `
		UPDATE deliveries
		SET courier_id = $2, status = 'assigned', estimated_delivery = $3, updated_at = NOW()
		WHERE id = $1`
	tag, err := r.pool.Exec(ctx, query, deliveryID, courierID, eta)
	if err != nil {
		return fmt.Errorf("assign courier: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("delivery %s: %w", deliveryID, pkgerrors.ErrNotFound)
	}
	return nil
}

// ListPendingDeliveries returns all deliveries with 'pending' status.
func (r *pgxDeliveryRepository) ListPendingDeliveries(ctx context.Context) ([]*domain.Delivery, error) {
	const query = `
		SELECT id, order_id, courier_id, status, zone_id, estimated_delivery, created_at, updated_at
		FROM deliveries
		WHERE status = 'pending'`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list pending deliveries: %w", err)
	}
	defer rows.Close()

	var deliveries []*domain.Delivery
	for rows.Next() {
		d := &domain.Delivery{}
		var courierID *string
		var zoneID *string
		var estimatedDelivery *time.Time
		if err := rows.Scan(
			&d.ID, &d.OrderID, &courierID, &d.Status, &zoneID,
			&estimatedDelivery, &d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan delivery: %w", err)
		}
		if courierID != nil {
			d.CourierID = *courierID
		}
		if zoneID != nil {
			d.ZoneID = *zoneID
		}
		if estimatedDelivery != nil {
			d.EstimatedDelivery = *estimatedDelivery
		}
		deliveries = append(deliveries, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate deliveries: %w", err)
	}
	return deliveries, nil
}

// ListByCourierID returns all active deliveries for a courier (status not in 'delivered' or 'failed').
func (r *pgxDeliveryRepository) ListByCourierID(ctx context.Context, courierID string) ([]*domain.Delivery, error) {
	const query = `
		SELECT id, order_id, courier_id, status, zone_id, estimated_delivery, created_at, updated_at
		FROM deliveries
		WHERE courier_id = $1 AND status NOT IN ('delivered', 'failed')
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query, courierID)
	if err != nil {
		return nil, fmt.Errorf("list deliveries by courier: %w", err)
	}
	defer rows.Close()

	var deliveries []*domain.Delivery
	for rows.Next() {
		d := &domain.Delivery{}
		var cID *string
		var zoneID *string
		var estimatedDelivery *time.Time
		if err := rows.Scan(
			&d.ID, &d.OrderID, &cID, &d.Status, &zoneID,
			&estimatedDelivery, &d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan delivery: %w", err)
		}
		if cID != nil {
			d.CourierID = *cID
		}
		if zoneID != nil {
			d.ZoneID = *zoneID
		}
		if estimatedDelivery != nil {
			d.EstimatedDelivery = *estimatedDelivery
		}
		deliveries = append(deliveries, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate deliveries by courier: %w", err)
	}
	// Return empty slice (not nil) when no rows found.
	if deliveries == nil {
		deliveries = []*domain.Delivery{}
	}
	return deliveries, nil
}

// FindNearestCourier finds the nearest available courier in the given zone using PostGIS.
// Enforces max 5 active deliveries per courier (D-05).
// CRITICAL: ST_MakePoint takes (longitude, latitude) - lng=$3, lat=$2.
func (r *pgxDeliveryRepository) FindNearestCourier(ctx context.Context, zoneID string, lat, lng float64) (*domain.CourierCandidate, error) {
	const query = `
		SELECT c.id AS courier_id,
		    ST_Distance(
		        ST_Centroid(dz.boundary)::geography,
		        ST_SetSRID(ST_MakePoint($3, $2), 4326)::geography
		    ) AS distance_meters
		FROM couriers c
		JOIN courier_zones cz ON cz.courier_id = c.id
		JOIN delivery_zones dz ON dz.id = cz.zone_id
		WHERE dz.id = $1
		    AND c.status = 'available'
		    AND ST_Contains(dz.boundary, ST_SetSRID(ST_MakePoint($3, $2), 4326))
		    AND (SELECT COUNT(*) FROM deliveries d WHERE d.courier_id = c.id AND d.status IN ('assigned', 'picked_up', 'in_transit')) < 5
		ORDER BY distance_meters ASC
		LIMIT 1`

	cc := &domain.CourierCandidate{}
	err := r.pool.QueryRow(ctx, query, zoneID, lat, lng).Scan(&cc.CourierID, &cc.DistanceMeters)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // no available courier - not an error
		}
		return nil, fmt.Errorf("find nearest courier: %w", err)
	}
	return cc, nil
}

// CountActiveByCourier counts the number of active deliveries for a courier.
func (r *pgxDeliveryRepository) CountActiveByCourier(ctx context.Context, courierID string) (int, error) {
	const query = `
		SELECT COUNT(*) FROM deliveries
		WHERE courier_id = $1 AND status IN ('assigned', 'picked_up', 'in_transit')`

	var count int
	if err := r.pool.QueryRow(ctx, query, courierID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count active deliveries: %w", err)
	}
	return count, nil
}

// CreateZone creates a new delivery zone from a GeoJSON polygon.
func (r *pgxDeliveryRepository) CreateZone(ctx context.Context, name, polygonGeoJSON string) (*domain.DeliveryZone, error) {
	const query = `
		INSERT INTO delivery_zones (name, boundary)
		VALUES ($1, ST_GeomFromGeoJSON($2))
		RETURNING id, name, ST_AsGeoJSON(boundary), created_at`

	z := &domain.DeliveryZone{}
	err := r.pool.QueryRow(ctx, query, name, polygonGeoJSON).Scan(
		&z.ID, &z.Name, &z.PolygonGeoJSON, &z.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create zone: %w", err)
	}
	return z, nil
}

// ListZones returns a paginated list of delivery zones with total count.
func (r *pgxDeliveryRepository) ListZones(ctx context.Context, limit, offset int) ([]*domain.DeliveryZone, int, error) {
	const countQuery = `SELECT COUNT(*) FROM delivery_zones`
	var total int
	if err := r.pool.QueryRow(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count zones: %w", err)
	}

	const query = `
		SELECT id, name, ST_AsGeoJSON(boundary), created_at
		FROM delivery_zones
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`

	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list zones: %w", err)
	}
	defer rows.Close()

	var zones []*domain.DeliveryZone
	for rows.Next() {
		z := &domain.DeliveryZone{}
		if err := rows.Scan(&z.ID, &z.Name, &z.PolygonGeoJSON, &z.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan zone: %w", err)
		}
		zones = append(zones, z)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate zones: %w", err)
	}
	return zones, total, nil
}

// FindZoneByPoint finds the delivery zone that contains the given GPS coordinate.
// Returns nil (no error) if no zone contains the point.
// CRITICAL: ST_MakePoint takes (longitude, latitude) - $2=lng, $1=lat.
func (r *pgxDeliveryRepository) FindZoneByPoint(ctx context.Context, lat, lng float64) (*domain.DeliveryZone, error) {
	const query = `
		SELECT id, name, ST_AsGeoJSON(boundary) AS polygon_geojson, created_at
		FROM delivery_zones
		WHERE ST_Contains(boundary, ST_SetSRID(ST_MakePoint($2, $1), 4326))
		LIMIT 1`

	z := &domain.DeliveryZone{}
	err := r.pool.QueryRow(ctx, query, lat, lng).Scan(
		&z.ID, &z.Name, &z.PolygonGeoJSON, &z.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // point not in any zone - not an error
		}
		return nil, fmt.Errorf("find zone by point: %w", err)
	}
	return z, nil
}

// UpsertCourier inserts a courier if not already present (idempotent).
func (r *pgxDeliveryRepository) UpsertCourier(ctx context.Context, courierID string) error {
	const query = `INSERT INTO couriers (id) VALUES ($1) ON CONFLICT (id) DO NOTHING`
	if _, err := r.pool.Exec(ctx, query, courierID); err != nil {
		return fmt.Errorf("upsert courier: %w", err)
	}
	return nil
}

// AssignCourierToZone assigns a courier to a delivery zone (idempotent).
func (r *pgxDeliveryRepository) AssignCourierToZone(ctx context.Context, courierID, zoneID string) error {
	const query = `INSERT INTO courier_zones (courier_id, zone_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	if _, err := r.pool.Exec(ctx, query, courierID, zoneID); err != nil {
		return fmt.Errorf("assign courier to zone: %w", err)
	}
	return nil
}

// CalculateETA calculates the estimated delivery time using PostGIS ST_Distance from zone centroid.
// Formula: distance / 500 m/min (D-07).
// CRITICAL: ST_MakePoint takes (longitude, latitude) - $3=lng, $2=lat.
func (r *pgxDeliveryRepository) CalculateETA(ctx context.Context, zoneID string, lat, lng float64) (time.Time, error) {
	const query = `
		SELECT NOW() + INTERVAL '1 minute' * (
		    ST_Distance(
		        ST_Centroid(dz.boundary)::geography,
		        ST_SetSRID(ST_MakePoint($3, $2), 4326)::geography
		    ) / 500.0
		) AS estimated_delivery
		FROM delivery_zones dz
		WHERE dz.id = $1`

	var eta time.Time
	err := r.pool.QueryRow(ctx, query, zoneID, lat, lng).Scan(&eta)
	if err != nil {
		if err == pgx.ErrNoRows {
			return time.Time{}, fmt.Errorf("zone %s: %w", zoneID, pkgerrors.ErrNotFound)
		}
		return time.Time{}, fmt.Errorf("calculate eta: %w", err)
	}
	return eta, nil
}
