package repository

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
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

	// UpdateCourierStatus performs a simple status update on a courier record (BKND-01).
	// Used for "available" transitions and system-managed transitions.
	// Returns domain.ErrNotFound if the courier does not exist.
	UpdateCourierStatus(ctx context.Context, courierID, newStatus string) (*domain.Courier, error)

	// UpdateCourierStatusToOfflineGuarded atomically transitions a courier to offline
	// only when no active deliveries exist (D-03 offline guard).
	// Returns (courier, true, nil) on success.
	// Returns (nil, false, nil) when an active delivery blocks the transition (caller maps to ErrActiveDeliveryExists).
	// Returns (nil, false, domain.ErrNotFound) when the courier does not exist.
	UpdateCourierStatusToOfflineGuarded(ctx context.Context, courierID string) (*domain.Courier, bool, error)

	// ListAvailableForCourier returns pending, unassigned deliveries in zones the courier serves.
	// Cursor-based pagination (D-05): cursor encodes created_at+id of last seen row.
	ListAvailableForCourier(ctx context.Context, courierID, cursor string, limit int32) (rows []domain.AvailableOrderRow, nextCursor string, err error)

	// AcceptByCourier atomically self-assigns a pending delivery to a courier (D-07).
	// Uses a conditional UPDATE..RETURNING with no explicit locks.
	// Returns (delivery, true, nil) on success.
	// Returns (nil, false, nil) when the delivery is already taken or not in pending state.
	AcceptByCourier(ctx context.Context, deliveryID, courierID string, eta time.Time) (*domain.Delivery, bool, error)

	// GetCourier retrieves a courier record by ID.
	// Returns domain.ErrNotFound if no courier exists with that ID.
	GetCourier(ctx context.Context, courierID string) (*domain.Courier, error)
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

// UpdateCourierStatus performs a simple status update on the courier row (BKND-01).
// Used for "available" transitions and system-managed "busy" transitions.
// Returns domain.ErrNotFound (wrapped) if no courier row matches.
func (r *pgxDeliveryRepository) UpdateCourierStatus(ctx context.Context, courierID, newStatus string) (*domain.Courier, error) {
	const q = `UPDATE couriers SET status = $2, updated_at = NOW() WHERE id = $1
	            RETURNING id, status, updated_at`

	c := &domain.Courier{}
	err := r.pool.QueryRow(ctx, q, courierID, newStatus).Scan(&c.ID, &c.Status, &c.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("courier %s: %w", courierID, pkgerrors.ErrNotFound)
		}
		return nil, fmt.Errorf("update courier status: %w", err)
	}
	return c, nil
}

// UpdateCourierStatusToOfflineGuarded atomically transitions a courier to offline
// only when no active deliveries exist (D-03 offline guard, RESEARCH §Pattern 2).
// Returns (courier, true, nil) on success.
// Returns (nil, false, nil) when an active delivery blocks the transition.
// Returns (nil, false, ErrNotFound) when the courier does not exist.
func (r *pgxDeliveryRepository) UpdateCourierStatusToOfflineGuarded(ctx context.Context, courierID string) (*domain.Courier, bool, error) {
	const offlineGuardQuery = `
		UPDATE couriers
		SET status = 'offline', updated_at = NOW()
		WHERE id = $1
		  AND NOT EXISTS (
		      SELECT 1 FROM deliveries d
		      WHERE d.courier_id = $1
		        AND d.status IN ('assigned','picked_up','in_transit')
		  )
		RETURNING id, status, updated_at`

	c := &domain.Courier{}
	err := r.pool.QueryRow(ctx, offlineGuardQuery, courierID).Scan(&c.ID, &c.Status, &c.UpdatedAt)
	if err != nil {
		if err != pgx.ErrNoRows {
			return nil, false, fmt.Errorf("offline guard update: %w", err)
		}
		// 0 rows: could be (a) active delivery exists, or (b) courier not found.
		// Disambiguate with a follow-up existence check.
		const existsQ = `SELECT 1 FROM couriers WHERE id = $1`
		var dummy int
		existsErr := r.pool.QueryRow(ctx, existsQ, courierID).Scan(&dummy)
		if existsErr == pgx.ErrNoRows {
			// Courier does not exist.
			return nil, false, fmt.Errorf("courier %s: %w", courierID, pkgerrors.ErrNotFound)
		}
		if existsErr != nil {
			return nil, false, fmt.Errorf("courier existence check: %w", existsErr)
		}
		// Courier exists but active delivery blocks the transition.
		return nil, false, nil
	}
	return c, true, nil
}

// AcceptByCourier atomically self-assigns a pending delivery to a courier (D-07).
// Uses a single conditional UPDATE..RETURNING — no SELECT FOR UPDATE, no explicit locks.
// Returns (delivery, true, nil) on success.
// Returns (nil, false, nil) when delivery is already taken or not in pending state (caller → 409).
func (r *pgxDeliveryRepository) AcceptByCourier(ctx context.Context, deliveryID, courierID string, eta time.Time) (*domain.Delivery, bool, error) {
	const query = `
		UPDATE deliveries
		SET courier_id = $2, status = 'assigned', estimated_delivery = $3, updated_at = NOW()
		WHERE id = $1 AND courier_id IS NULL AND status = 'pending'
		RETURNING id, order_id, courier_id, status, zone_id, estimated_delivery, created_at, updated_at`

	d := &domain.Delivery{}
	err := scanDelivery(r.pool.QueryRow(ctx, query, deliveryID, courierID, eta), d)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, false, nil // contention: someone else took it OR not pending
		}
		return nil, false, fmt.Errorf("accept by courier: %w", err)
	}
	return d, true, nil
}

// GetCourier retrieves a courier by ID.
// Returns pkgerrors.ErrNotFound (wrapped) if no courier exists with that ID.
func (r *pgxDeliveryRepository) GetCourier(ctx context.Context, courierID string) (*domain.Courier, error) {
	const q = `SELECT id, status, updated_at FROM couriers WHERE id = $1`

	c := &domain.Courier{}
	err := r.pool.QueryRow(ctx, q, courierID).Scan(&c.ID, &c.Status, &c.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("courier %s: %w", courierID, pkgerrors.ErrNotFound)
		}
		return nil, fmt.Errorf("get courier: %w", err)
	}
	return c, nil
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

// listAvailableQuery selects pending, unassigned deliveries in zones served by the given courier.
// Cursor encoding: base64(created_at_unix_nano + ":" + delivery_id).
// Empty cursor ($2) → first page; non-empty → decode to (created_at_ts, id) and apply keyset condition.
const listAvailableQuery = `
	SELECT d.id AS delivery_id, d.order_id, dz.name AS zone_name, d.created_at
	FROM deliveries d
	JOIN courier_zones cz ON cz.zone_id = d.zone_id
	JOIN delivery_zones dz ON dz.id = d.zone_id
	WHERE cz.courier_id = $1
	  AND d.status = 'pending'
	  AND d.courier_id IS NULL
	  AND (
	    $2::timestamptz IS NULL
	    OR (d.created_at, d.id) > ($2::timestamptz, $3::uuid)
	  )
	ORDER BY d.created_at ASC, d.id ASC
	LIMIT $4`

// ListAvailableForCourier returns pending, unassigned deliveries in zones the courier serves.
// Cursor pagination per D-05: cursor encodes (created_at_unix_nano + ":" + delivery_id).
func (r *pgxDeliveryRepository) ListAvailableForCourier(ctx context.Context, courierID, cursor string, limit int32) ([]domain.AvailableOrderRow, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	// Decode cursor → (created_at, id) or nil for first page.
	var cursorTS *time.Time
	var cursorID *string
	if cursor != "" {
		decoded, err := base64.StdEncoding.DecodeString(cursor)
		if err == nil {
			parts := strings.SplitN(string(decoded), ":", 2)
			if len(parts) == 2 {
				nanos, nerr := strconv.ParseInt(parts[0], 10, 64)
				if nerr == nil {
					ts := time.Unix(0, nanos).UTC()
					id := parts[1]
					cursorTS = &ts
					cursorID = &id
				}
			}
		}
	}

	// For nil cursor case we pass NULL — pgx handles *time.Time nil as SQL NULL.
	var cursorIDStr string
	if cursorID != nil {
		cursorIDStr = *cursorID
	} else {
		// placeholder uuid — ignored by the WHERE clause when cursorTS IS NULL
		cursorIDStr = "00000000-0000-0000-0000-000000000000"
	}

	rows, err := r.pool.Query(ctx, listAvailableQuery, courierID, cursorTS, cursorIDStr, limit)
	if err != nil {
		return nil, "", fmt.Errorf("list available for courier: %w", err)
	}
	defer rows.Close()

	var result []domain.AvailableOrderRow
	for rows.Next() {
		var row domain.AvailableOrderRow
		if err := rows.Scan(&row.DeliveryID, &row.OrderID, &row.ZoneName, &row.CreatedAt); err != nil {
			return nil, "", fmt.Errorf("scan available order row: %w", err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("iterate available orders: %w", err)
	}

	// Build next cursor from last row if we got a full page.
	var nextCursor string
	if int32(len(result)) == limit {
		last := result[len(result)-1]
		raw := strconv.FormatInt(last.CreatedAt.UnixNano(), 10) + ":" + last.DeliveryID
		nextCursor = base64.StdEncoding.EncodeToString([]byte(raw))
	}

	return result, nextCursor, nil
}
