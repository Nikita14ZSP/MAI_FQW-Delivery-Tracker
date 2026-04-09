package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	pkgerrors "github.com/mozgovojnikita/delivery-tracker/pkg/errors"
	"github.com/mozgovojnikita/delivery-tracker/services/tracking-service/internal/domain"
)

const (
	redisCourierLocationKey = "courier:location:"
	redisCacheTTL           = 5 * time.Minute
)

// LocationRepository defines the data access interface for courier location data.
// It abstracts PostGIS persistence and Redis caching.
type LocationRepository interface {
	// InsertLocation persists a GPS point to the PostGIS location_history table.
	InsertLocation(ctx context.Context, courierID, orderID string, lat, lng float64, recordedAt time.Time) error
	// GetLocationHistory returns the last `limit` location points for an order.
	GetLocationHistory(ctx context.Context, orderID string, limit int) ([]*domain.LocationPoint, error)
	// GetLastLocation returns the most recent location for a courier from the DB.
	GetLastLocation(ctx context.Context, courierID string) (*domain.LocationPoint, error)
	// CacheLastLocation stores the courier's current location in Redis with a 5-minute TTL.
	CacheLastLocation(ctx context.Context, loc domain.CachedLocation) error
	// GetCachedLocation retrieves the courier's cached location from Redis.
	// Returns (nil, nil) on cache miss.
	GetCachedLocation(ctx context.Context, courierID string) (*domain.CachedLocation, error)
}

type pgxLocationRepository struct {
	pool *pgxpool.Pool
	rdb  *redis.Client
}

// NewLocationRepository creates a new LocationRepository backed by pgx pool and Redis client.
func NewLocationRepository(pool *pgxpool.Pool, rdb *redis.Client) LocationRepository {
	return &pgxLocationRepository{pool: pool, rdb: rdb}
}

// InsertLocation inserts a GPS point into location_history using PostGIS ST_MakePoint.
// CRITICAL: ST_MakePoint takes (longitude, latitude) — $3=lng, $4=lat.
func (r *pgxLocationRepository) InsertLocation(ctx context.Context, courierID, orderID string, lat, lng float64, recordedAt time.Time) error {
	const query = `
		INSERT INTO location_history (courier_id, order_id, coordinates, recorded_at)
		VALUES ($1, $2, ST_SetSRID(ST_MakePoint($3, $4), 4326), $5)`
	// $3=lng, $4=lat — PostGIS convention: longitude first, latitude second.
	if _, err := r.pool.Exec(ctx, query, courierID, orderID, lng, lat, recordedAt); err != nil {
		return fmt.Errorf("insert location: %w", err)
	}
	return nil
}

// GetLocationHistory returns the last `limit` location points for a given order_id.
func (r *pgxLocationRepository) GetLocationHistory(ctx context.Context, orderID string, limit int) ([]*domain.LocationPoint, error) {
	const query = `
		SELECT id, courier_id, order_id, ST_Y(coordinates) AS lat, ST_X(coordinates) AS lng, recorded_at
		FROM location_history
		WHERE order_id = $1
		ORDER BY recorded_at DESC
		LIMIT $2`

	rows, err := r.pool.Query(ctx, query, orderID, limit)
	if err != nil {
		return nil, fmt.Errorf("query location history: %w", err)
	}
	defer rows.Close()

	var points []*domain.LocationPoint
	for rows.Next() {
		p := &domain.LocationPoint{}
		if err := rows.Scan(&p.ID, &p.CourierID, &p.OrderID, &p.Lat, &p.Lng, &p.RecordedAt); err != nil {
			return nil, fmt.Errorf("scan location point: %w", err)
		}
		points = append(points, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate location history: %w", err)
	}
	return points, nil
}

// GetLastLocation retrieves the most recent location point for a courier from the DB.
// Returns ErrNotFound if no location exists for the courier.
func (r *pgxLocationRepository) GetLastLocation(ctx context.Context, courierID string) (*domain.LocationPoint, error) {
	const query = `
		SELECT id, courier_id, order_id, ST_Y(coordinates) AS lat, ST_X(coordinates) AS lng, recorded_at
		FROM location_history
		WHERE courier_id = $1
		ORDER BY recorded_at DESC
		LIMIT 1`

	p := &domain.LocationPoint{}
	err := r.pool.QueryRow(ctx, query, courierID).Scan(
		&p.ID, &p.CourierID, &p.OrderID, &p.Lat, &p.Lng, &p.RecordedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("courier %s: %w", courierID, pkgerrors.ErrNotFound)
		}
		return nil, fmt.Errorf("get last location: %w", err)
	}
	return p, nil
}

// CacheLastLocation stores a CachedLocation in Redis under key "courier:location:{courierID}".
// TTL is 5 minutes per D-05.
func (r *pgxLocationRepository) CacheLastLocation(ctx context.Context, loc domain.CachedLocation) error {
	data, err := json.Marshal(loc)
	if err != nil {
		return fmt.Errorf("marshal cached location: %w", err)
	}
	key := redisCourierLocationKey + loc.CourierID
	if err := r.rdb.Set(ctx, key, data, redisCacheTTL).Err(); err != nil {
		return fmt.Errorf("cache last location: %w", err)
	}
	return nil
}

// GetCachedLocation retrieves a courier's cached location from Redis.
// Returns (nil, nil) on cache miss (redis.Nil error).
func (r *pgxLocationRepository) GetCachedLocation(ctx context.Context, courierID string) (*domain.CachedLocation, error) {
	key := redisCourierLocationKey + courierID
	data, err := r.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // cache miss — not an error
		}
		return nil, fmt.Errorf("get cached location: %w", err)
	}

	var loc domain.CachedLocation
	if err := json.Unmarshal(data, &loc); err != nil {
		return nil, fmt.Errorf("unmarshal cached location: %w", err)
	}
	return &loc, nil
}
