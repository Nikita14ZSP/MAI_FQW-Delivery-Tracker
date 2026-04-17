package service

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	pkgerrors "github.com/mozgovojnikita/delivery-tracker/pkg/errors"
	"github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/domain"
	"github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/repository"
)

// RegisterInput holds the input for user registration.
type RegisterInput struct {
	Email     string      `validate:"required,email"`
	Password  string      `validate:"required,min=8"`
	FirstName string      `validate:"required,min=1,max=100"`
	LastName  string      `validate:"required,min=1,max=100"`
	Phone     string      `validate:"omitempty,e164"`
	Role      domain.Role `validate:"required,oneof=user courier"`
}

// LoginInput holds the input for user login.
type LoginInput struct {
	Email    string `validate:"required,email"`
	Password string `validate:"required"`
}

// TokenPair contains an access token and a refresh token.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

// Claims are the JWT claims used for access tokens.
type Claims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// AuthService handles authentication: registration, login, and token refresh.
type AuthService struct {
	userRepo    repository.UserRepository
	redisClient *redis.Client
	jwtSecret   string
	validate    *validator.Validate
}

// NewAuthService creates a new AuthService.
// Panics if jwtSecret is empty — fail fast at startup.
func NewAuthService(userRepo repository.UserRepository, redisClient *redis.Client, jwtSecret string) *AuthService {
	if jwtSecret == "" {
		panic("JWT_SECRET must be set")
	}
	return &AuthService{
		userRepo:    userRepo,
		redisClient: redisClient,
		jwtSecret:   jwtSecret,
		validate:    validator.New(),
	}
}

// Register creates a new user with a hashed password and returns a token pair.
func (s *AuthService) Register(ctx context.Context, input RegisterInput) (*TokenPair, *domain.User, error) {
	if input.Role == "" {
		input.Role = domain.RoleUser
	}
	if err := s.validate.Struct(input); err != nil {
		return nil, nil, fmt.Errorf("validate: %w", pkgerrors.ErrInvalidInput)
	}

	hash, err := domain.HashPassword(input.Password)
	if err != nil {
		return nil, nil, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.userRepo.Create(ctx, input.Email, hash, input.FirstName, input.LastName, input.Phone, input.Role)
	if err != nil {
		return nil, nil, err
	}

	tokens, err := s.generateTokenPair(ctx, user.ID, string(user.Role))
	if err != nil {
		return nil, nil, fmt.Errorf("generate tokens: %w", err)
	}

	return tokens, user, nil
}

// Login authenticates a user and returns a token pair.
// Returns ErrUnauthorized for both wrong password and non-existent user to avoid user enumeration.
func (s *AuthService) Login(ctx context.Context, input LoginInput) (*TokenPair, *domain.User, error) {
	if err := s.validate.Struct(input); err != nil {
		return nil, nil, fmt.Errorf("validate: %w", pkgerrors.ErrInvalidInput)
	}

	user, err := s.userRepo.GetByEmail(ctx, input.Email)
	if err != nil {
		// Don't leak whether the user exists
		return nil, nil, fmt.Errorf("login: %w", pkgerrors.ErrUnauthorized)
	}

	if err := domain.CheckPassword(user.PasswordHash, input.Password); err != nil {
		return nil, nil, fmt.Errorf("login: %w", pkgerrors.ErrUnauthorized)
	}

	tokens, err := s.generateTokenPair(ctx, user.ID, string(user.Role))
	if err != nil {
		return nil, nil, fmt.Errorf("generate tokens: %w", err)
	}

	return tokens, user, nil
}

// RefreshToken atomically rotates the refresh token.
// The old token is deleted from Redis (GetDel) and a new pair is issued.
// Returns ErrUnauthorized if the token is not found or invalid.
func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	key := "refresh:" + refreshToken

	// Atomic get-and-delete — prevents token reuse
	userID, err := s.redisClient.GetDel(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("refresh token: %w", pkgerrors.ErrUnauthorized)
	}
	if userID == "" {
		return nil, fmt.Errorf("refresh token not found: %w", pkgerrors.ErrUnauthorized)
	}

	// Get current role for the user
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user for refresh: %w", pkgerrors.ErrUnauthorized)
	}

	tokens, err := s.generateTokenPair(ctx, user.ID, string(user.Role))
	if err != nil {
		return nil, fmt.Errorf("generate tokens: %w", err)
	}

	return tokens, nil
}

// ValidateToken validates an access token string and returns the claims.
func (s *AuthService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.jwtSecret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", pkgerrors.ErrUnauthorized)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims: %w", pkgerrors.ErrUnauthorized)
	}

	return claims, nil
}

// generateTokenPair creates a new JWT access token and a Redis-backed refresh token.
// Access token expires in 15 minutes; refresh token expires in 7 days.
func (s *AuthService) generateTokenPair(ctx context.Context, userID, role string) (*TokenPair, error) {
	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.jwtSecret))
	if err != nil {
		return nil, fmt.Errorf("sign access token: %w", err)
	}

	refreshID := uuid.NewString()
	refreshKey := "refresh:" + refreshID
	if err := s.redisClient.Set(ctx, refreshKey, userID, 7*24*time.Hour).Err(); err != nil {
		return nil, fmt.Errorf("store refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshID,
	}, nil
}
