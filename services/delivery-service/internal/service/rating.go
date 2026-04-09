package service

import (
	"context"
	"fmt"

	pkgerrors "github.com/mozgovojnikita/delivery-tracker/pkg/errors"
	"github.com/mozgovojnikita/delivery-tracker/services/delivery-service/internal/domain"
)

// RatingRepository defines the data access interface for courier ratings.
type RatingRepository interface {
	CreateRating(ctx context.Context, deliveryID, courierID, userID string, stars int, comment string) (*domain.CourierRating, error)
	GetCourierRating(ctx context.Context, courierID string) (*domain.CourierRatingResult, error)
}

// RateCourier submits a rating for a courier after delivery completion (RATE-01).
// Validates stars range (1-5), verifies delivery is in delivered status, then persists the rating.
func (s *DeliveryService) RateCourier(ctx context.Context, deliveryID, userID string, stars int, comment string) (string, error) {
	// 1. Validate stars range before any DB calls.
	if stars < 1 || stars > 5 {
		return "", fmt.Errorf("stars must be 1-5: %w", pkgerrors.ErrInvalidInput)
	}

	// 2. Get delivery to verify it exists and check status.
	delivery, err := s.repo.GetDeliveryByID(ctx, deliveryID)
	if err != nil {
		return "", err
	}

	// 3. Ensure delivery is completed before allowing rating.
	if delivery.Status != domain.StatusDelivered {
		return "", fmt.Errorf("delivery not completed: %w", pkgerrors.ErrInvalidInput)
	}

	// 4. Persist the rating.
	rating, err := s.ratingRepo.CreateRating(ctx, deliveryID, delivery.CourierID, userID, stars, comment)
	if err != nil {
		return "", err
	}

	return rating.ID, nil
}

// GetCourierRating retrieves a courier's average rating and individual reviews (RATE-02).
func (s *DeliveryService) GetCourierRating(ctx context.Context, courierID string) (*domain.CourierRatingResult, error) {
	return s.ratingRepo.GetCourierRating(ctx, courierID)
}
