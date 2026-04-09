package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	pkgerrors "github.com/mozgovojnikita/delivery-tracker/pkg/errors"
	"github.com/mozgovojnikita/delivery-tracker/services/delivery-service/internal/domain"
)

// RatingRepository handles data access for courier ratings.
type RatingRepository struct {
	pool *pgxpool.Pool
}

// NewRatingRepository creates a new RatingRepository backed by a pgx connection pool.
func NewRatingRepository(pool *pgxpool.Pool) *RatingRepository {
	return &RatingRepository{pool: pool}
}

// CreateRating inserts a new courier rating and returns the created record.
// Returns ErrInvalidInput (wrapped) on duplicate delivery_id (unique constraint violation).
func (r *RatingRepository) CreateRating(ctx context.Context, deliveryID, courierID, userID string, stars int, comment string) (*domain.CourierRating, error) {
	const query = `
		INSERT INTO courier_ratings (delivery_id, courier_id, user_id, stars, comment)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`

	rating := &domain.CourierRating{
		DeliveryID: deliveryID,
		CourierID:  courierID,
		UserID:     userID,
		Stars:      stars,
		Comment:    comment,
	}

	err := r.pool.QueryRow(ctx, query, deliveryID, courierID, userID, stars, comment).
		Scan(&rating.ID, &rating.CreatedAt)
	if err != nil {
		// Check for unique constraint violation (delivery already rated).
		errStr := err.Error()
		if strings.Contains(errStr, "unique") || strings.Contains(errStr, "duplicate") || strings.Contains(errStr, "23505") {
			return nil, fmt.Errorf("already rated: %w", pkgerrors.ErrInvalidInput)
		}
		return nil, fmt.Errorf("create rating: %w", err)
	}
	return rating, nil
}

// GetCourierRating returns aggregated rating data for a courier.
// Returns a result with AverageStars=0 and empty Ratings if no ratings exist.
func (r *RatingRepository) GetCourierRating(ctx context.Context, courierID string) (*domain.CourierRatingResult, error) {
	// Query 1: get aggregate stats.
	const aggQuery = `SELECT COALESCE(AVG(stars)::float, 0), COUNT(*) FROM courier_ratings WHERE courier_id = $1`

	result := &domain.CourierRatingResult{}
	if err := r.pool.QueryRow(ctx, aggQuery, courierID).Scan(&result.AverageStars, &result.TotalRatings); err != nil {
		return nil, fmt.Errorf("get courier rating aggregate: %w", err)
	}

	// Query 2: get individual rating entries.
	const listQuery = `
		SELECT id, delivery_id, courier_id, user_id, stars, comment, created_at
		FROM courier_ratings
		WHERE courier_id = $1
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, listQuery, courierID)
	if err != nil {
		return nil, fmt.Errorf("list courier ratings: %w", err)
	}
	defer rows.Close()

	result.Ratings = []domain.CourierRating{}
	for rows.Next() {
		var cr domain.CourierRating
		if err := rows.Scan(&cr.ID, &cr.DeliveryID, &cr.CourierID, &cr.UserID, &cr.Stars, &cr.Comment, &cr.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan rating: %w", err)
		}
		result.Ratings = append(result.Ratings, cr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ratings: %w", err)
	}
	return result, nil
}

