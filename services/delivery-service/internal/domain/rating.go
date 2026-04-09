package domain

import "time"

// CourierRating represents a user's rating of a courier for a specific delivery.
type CourierRating struct {
	ID         string
	DeliveryID string
	CourierID  string
	UserID     string
	Stars      int
	Comment    string
	CreatedAt  time.Time
}

// CourierRatingResult holds aggregated rating data for a courier.
type CourierRatingResult struct {
	AverageStars float64
	TotalRatings int
	Ratings      []CourierRating
}
