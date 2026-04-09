package service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	pkgerrors "github.com/mozgovojnikita/delivery-tracker/pkg/errors"
	"github.com/mozgovojnikita/delivery-tracker/services/delivery-service/internal/domain"
	"github.com/mozgovojnikita/delivery-tracker/services/delivery-service/internal/service"
)

// mockRatingRepo is a manual mock for the RatingRepository interface.
type mockRatingRepo struct {
	createRatingFn      func(ctx context.Context, deliveryID, courierID, userID string, stars int, comment string) (*domain.CourierRating, error)
	getCourierRatingFn  func(ctx context.Context, courierID string) (*domain.CourierRatingResult, error)
}

func (m *mockRatingRepo) CreateRating(ctx context.Context, deliveryID, courierID, userID string, stars int, comment string) (*domain.CourierRating, error) {
	if m.createRatingFn != nil {
		return m.createRatingFn(ctx, deliveryID, courierID, userID, stars, comment)
	}
	return &domain.CourierRating{ID: "rating-1", DeliveryID: deliveryID, CourierID: courierID, UserID: userID, Stars: stars, Comment: comment}, nil
}

func (m *mockRatingRepo) GetCourierRating(ctx context.Context, courierID string) (*domain.CourierRatingResult, error) {
	if m.getCourierRatingFn != nil {
		return m.getCourierRatingFn(ctx, courierID)
	}
	return &domain.CourierRatingResult{}, nil
}

// TestRateCourier_Success: delivery exists, status=delivered -> returns rating ID, no error.
func TestRateCourier_Success(t *testing.T) {
	repo := &mockDeliveryRepo{
		getDeliveryByIDFn: func(ctx context.Context, id string) (*domain.Delivery, error) {
			return &domain.Delivery{
				ID:        id,
				CourierID: "courier-1",
				Status:    domain.StatusDelivered,
			}, nil
		},
	}
	ratingRepo := &mockRatingRepo{
		createRatingFn: func(ctx context.Context, deliveryID, courierID, userID string, stars int, comment string) (*domain.CourierRating, error) {
			return &domain.CourierRating{ID: "rating-1"}, nil
		},
	}
	pub := &mockPublisher{}

	svc := service.NewDeliveryService(repo, pub, ratingRepo)
	ratingID, err := svc.RateCourier(context.Background(), "delivery-1", "user-1", 5, "great service")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if ratingID == "" {
		t.Error("expected rating ID, got empty string")
	}
}

// TestRateCourier_NotDelivered: delivery exists, status=assigned -> returns ErrInvalidInput.
func TestRateCourier_NotDelivered(t *testing.T) {
	repo := &mockDeliveryRepo{
		getDeliveryByIDFn: func(ctx context.Context, id string) (*domain.Delivery, error) {
			return &domain.Delivery{
				ID:        id,
				CourierID: "courier-1",
				Status:    domain.StatusAssigned,
			}, nil
		},
	}
	ratingRepo := &mockRatingRepo{}
	pub := &mockPublisher{}

	svc := service.NewDeliveryService(repo, pub, ratingRepo)
	_, err := svc.RateCourier(context.Background(), "delivery-1", "user-1", 4, "")

	if err == nil {
		t.Fatal("expected error when delivery not delivered")
	}
	if !errors.Is(err, pkgerrors.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got: %v", err)
	}
}

// TestRateCourier_DeliveryNotFound: repo returns ErrNotFound -> returns ErrNotFound.
func TestRateCourier_DeliveryNotFound(t *testing.T) {
	repo := &mockDeliveryRepo{
		getDeliveryByIDFn: func(ctx context.Context, id string) (*domain.Delivery, error) {
			return nil, fmt.Errorf("delivery %s: %w", id, pkgerrors.ErrNotFound)
		},
	}
	ratingRepo := &mockRatingRepo{}
	pub := &mockPublisher{}

	svc := service.NewDeliveryService(repo, pub, ratingRepo)
	_, err := svc.RateCourier(context.Background(), "nonexistent", "user-1", 3, "")

	if err == nil {
		t.Fatal("expected error when delivery not found")
	}
	if !errors.Is(err, pkgerrors.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

// TestRateCourier_DuplicateRating: ratingRepo returns unique constraint error -> returns ErrAlreadyExists or ErrInvalidInput.
func TestRateCourier_DuplicateRating(t *testing.T) {
	repo := &mockDeliveryRepo{
		getDeliveryByIDFn: func(ctx context.Context, id string) (*domain.Delivery, error) {
			return &domain.Delivery{
				ID:        id,
				CourierID: "courier-1",
				Status:    domain.StatusDelivered,
			}, nil
		},
	}
	ratingRepo := &mockRatingRepo{
		createRatingFn: func(ctx context.Context, deliveryID, courierID, userID string, stars int, comment string) (*domain.CourierRating, error) {
			return nil, fmt.Errorf("already rated: %w", pkgerrors.ErrInvalidInput)
		},
	}
	pub := &mockPublisher{}

	svc := service.NewDeliveryService(repo, pub, ratingRepo)
	_, err := svc.RateCourier(context.Background(), "delivery-1", "user-1", 5, "")

	if err == nil {
		t.Fatal("expected error on duplicate rating")
	}
	isExpected := errors.Is(err, pkgerrors.ErrAlreadyExists) || errors.Is(err, pkgerrors.ErrInvalidInput)
	if !isExpected {
		t.Errorf("expected ErrAlreadyExists or ErrInvalidInput, got: %v", err)
	}
}

// TestRateCourier_InvalidStars: stars=0 or stars=6 -> returns ErrInvalidInput (service-level validation before DB).
func TestRateCourier_InvalidStars(t *testing.T) {
	repo := &mockDeliveryRepo{}
	ratingRepo := &mockRatingRepo{}
	pub := &mockPublisher{}

	tests := []struct {
		name  string
		stars int
	}{
		{"stars=0", 0},
		{"stars=6", 6},
		{"stars=-1", -1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := service.NewDeliveryService(repo, pub, ratingRepo)
			_, err := svc.RateCourier(context.Background(), "delivery-1", "user-1", tc.stars, "")

			if err == nil {
				t.Fatalf("expected error for stars=%d", tc.stars)
			}
			if !errors.Is(err, pkgerrors.ErrInvalidInput) {
				t.Errorf("expected ErrInvalidInput, got: %v", err)
			}
		})
	}
}

// TestGetCourierRating_Success: ratingRepo returns 3 ratings -> returns correct average, count=3, all ratings.
func TestGetCourierRating_Success(t *testing.T) {
	now := time.Now()
	ratings := []domain.CourierRating{
		{ID: "r-1", DeliveryID: "d-1", CourierID: "courier-1", Stars: 5, CreatedAt: now},
		{ID: "r-2", DeliveryID: "d-2", CourierID: "courier-1", Stars: 4, CreatedAt: now},
		{ID: "r-3", DeliveryID: "d-3", CourierID: "courier-1", Stars: 3, CreatedAt: now},
	}
	repo := &mockDeliveryRepo{}
	ratingRepo := &mockRatingRepo{
		getCourierRatingFn: func(ctx context.Context, courierID string) (*domain.CourierRatingResult, error) {
			return &domain.CourierRatingResult{
				AverageStars: 4.0,
				TotalRatings: 3,
				Ratings:      ratings,
			}, nil
		},
	}
	pub := &mockPublisher{}

	svc := service.NewDeliveryService(repo, pub, ratingRepo)
	result, err := svc.GetCourierRating(context.Background(), "courier-1")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.TotalRatings != 3 {
		t.Errorf("expected 3 ratings, got %d", result.TotalRatings)
	}
	if result.AverageStars != 4.0 {
		t.Errorf("expected average 4.0, got %f", result.AverageStars)
	}
	if len(result.Ratings) != 3 {
		t.Errorf("expected 3 rating entries, got %d", len(result.Ratings))
	}
}

// TestGetCourierRating_NoRatings: ratingRepo returns empty -> returns average=0, count=0, empty list.
func TestGetCourierRating_NoRatings(t *testing.T) {
	repo := &mockDeliveryRepo{}
	ratingRepo := &mockRatingRepo{
		getCourierRatingFn: func(ctx context.Context, courierID string) (*domain.CourierRatingResult, error) {
			return &domain.CourierRatingResult{
				AverageStars: 0,
				TotalRatings: 0,
				Ratings:      []domain.CourierRating{},
			}, nil
		},
	}
	pub := &mockPublisher{}

	svc := service.NewDeliveryService(repo, pub, ratingRepo)
	result, err := svc.GetCourierRating(context.Background(), "courier-new")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.TotalRatings != 0 {
		t.Errorf("expected 0 ratings, got %d", result.TotalRatings)
	}
	if result.AverageStars != 0 {
		t.Errorf("expected average 0, got %f", result.AverageStars)
	}
	if len(result.Ratings) != 0 {
		t.Errorf("expected empty ratings list, got %d", len(result.Ratings))
	}
}
