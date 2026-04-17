package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	pkgerrors "github.com/mozgovojnikita/delivery-tracker/pkg/errors"
	"github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/domain"
	"github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/service"
)

// mockUserRepository is a manual mock implementing repository.UserRepository.
type mockUserRepository struct {
	createFn     func(ctx context.Context, email, passwordHash, firstName, lastName, phone string, role domain.Role) (*domain.User, error)
	getByEmailFn func(ctx context.Context, email string) (*domain.User, error)
	getByIDFn    func(ctx context.Context, id string) (*domain.User, error)
}

func (m *mockUserRepository) Create(ctx context.Context, email, passwordHash, firstName, lastName, phone string, role domain.Role) (*domain.User, error) {
	if m.createFn != nil {
		return m.createFn(ctx, email, passwordHash, firstName, lastName, phone, role)
	}
	return nil, nil
}

func (m *mockUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	if m.getByEmailFn != nil {
		return m.getByEmailFn(ctx, email)
	}
	return nil, pkgerrors.ErrNotFound
}

func (m *mockUserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, pkgerrors.ErrNotFound
}

// newTestRedis creates a miniredis-backed *redis.Client for testing.
func newTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	t.Cleanup(func() {
		client.Close()
		mr.Close()
	})
	return client, mr
}

func newTestAuthService(t *testing.T, repo *mockUserRepository) (*service.AuthService, *redis.Client, *miniredis.Miniredis) {
	t.Helper()
	rc, mr := newTestRedis(t)
	svc := service.NewAuthService(repo, rc, "test-jwt-secret-at-least-32-chars!!")
	return svc, rc, mr
}

func TestAuthService_Register_Success(t *testing.T) {
	repo := &mockUserRepository{
		createFn: func(ctx context.Context, email, passwordHash, firstName, lastName, phone string, role domain.Role) (*domain.User, error) {
			return &domain.User{
				ID:           "user-1",
				Email:        email,
				PasswordHash: passwordHash,
				FirstName:    firstName,
				LastName:     lastName,
				Phone:        phone,
				Role:         role,
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			}, nil
		},
	}
	svc, _, _ := newTestAuthService(t, repo)

	tokens, user, err := svc.Register(context.Background(), service.RegisterInput{
		Email:     "test@example.com",
		Password:  "securepassword123",
		FirstName: "Иван",
		LastName:  "Иванов",
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if tokens == nil {
		t.Fatal("expected token pair, got nil")
	}
	if tokens.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
	if tokens.RefreshToken == "" {
		t.Error("expected non-empty refresh token")
	}
	if user == nil {
		t.Fatal("expected user, got nil")
	}
	if user.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", user.Email)
	}
}

func TestAuthService_Register_DuplicateEmail(t *testing.T) {
	repo := &mockUserRepository{
		createFn: func(ctx context.Context, email, passwordHash, firstName, lastName, phone string, role domain.Role) (*domain.User, error) {
			return nil, pkgerrors.ErrAlreadyExists
		},
	}
	svc, _, _ := newTestAuthService(t, repo)

	_, _, err := svc.Register(context.Background(), service.RegisterInput{
		Email:     "existing@example.com",
		Password:  "securepassword123",
		FirstName: "Test",
		LastName:  "User",
	})

	if !errors.Is(err, pkgerrors.ErrAlreadyExists) {
		t.Errorf("expected ErrAlreadyExists, got: %v", err)
	}
}

func TestAuthService_Register_Courier(t *testing.T) {
	repo := &mockUserRepository{
		createFn: func(ctx context.Context, email, passwordHash, firstName, lastName, phone string, role domain.Role) (*domain.User, error) {
			return &domain.User{
				ID:           "user-courier-1",
				Email:        email,
				PasswordHash: passwordHash,
				FirstName:    firstName,
				LastName:     lastName,
				Phone:        phone,
				Role:         role,
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			}, nil
		},
	}
	svc, _, _ := newTestAuthService(t, repo)

	tokens, user, err := svc.Register(context.Background(), service.RegisterInput{
		Email:     "courier@example.com",
		Password:  "securepassword123",
		FirstName: "Курьер",
		LastName:  "Один",
		Role:      domain.RoleCourier,
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if tokens == nil || tokens.AccessToken == "" {
		t.Error("expected token pair")
	}
	if user == nil {
		t.Fatal("expected user, got nil")
	}
	if user.Role != domain.RoleCourier {
		t.Errorf("expected role courier, got %s", user.Role)
	}
}

func TestAuthService_Register_InvalidRole(t *testing.T) {
	repo := &mockUserRepository{}
	svc, _, _ := newTestAuthService(t, repo)

	_, _, err := svc.Register(context.Background(), service.RegisterInput{
		Email:     "admin@example.com",
		Password:  "securepassword123",
		FirstName: "Admin",
		LastName:  "User",
		Role:      domain.Role("admin"),
	})

	if !errors.Is(err, pkgerrors.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for role=admin, got: %v", err)
	}
}

func TestAuthService_Register_InvalidEmail(t *testing.T) {
	repo := &mockUserRepository{}
	svc, _, _ := newTestAuthService(t, repo)

	_, _, err := svc.Register(context.Background(), service.RegisterInput{
		Email:     "not-an-email",
		Password:  "securepassword123",
		FirstName: "Test",
		LastName:  "User",
	})

	if !errors.Is(err, pkgerrors.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got: %v", err)
	}
}

func TestAuthService_Login_Success(t *testing.T) {
	password := "securepassword123"
	hash, err := domain.HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	repo := &mockUserRepository{
		getByEmailFn: func(ctx context.Context, email string) (*domain.User, error) {
			return &domain.User{
				ID:           "user-2",
				Email:        email,
				PasswordHash: hash,
				Role:         domain.RoleUser,
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			}, nil
		},
	}
	svc, _, _ := newTestAuthService(t, repo)

	tokens, user, err := svc.Login(context.Background(), service.LoginInput{
		Email:    "login@example.com",
		Password: password,
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if tokens == nil || tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Error("expected valid token pair")
	}
	if user == nil {
		t.Fatal("expected user, got nil")
	}
}

func TestAuthService_Login_WrongPassword(t *testing.T) {
	hash, _ := domain.HashPassword("correctpassword")
	repo := &mockUserRepository{
		getByEmailFn: func(ctx context.Context, email string) (*domain.User, error) {
			return &domain.User{
				ID:           "user-3",
				Email:        email,
				PasswordHash: hash,
				Role:         domain.RoleUser,
			}, nil
		},
	}
	svc, _, _ := newTestAuthService(t, repo)

	_, _, err := svc.Login(context.Background(), service.LoginInput{
		Email:    "login@example.com",
		Password: "wrongpassword",
	})

	if !errors.Is(err, pkgerrors.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestAuthService_Login_UserNotFound(t *testing.T) {
	repo := &mockUserRepository{
		getByEmailFn: func(ctx context.Context, email string) (*domain.User, error) {
			return nil, pkgerrors.ErrNotFound
		},
	}
	svc, _, _ := newTestAuthService(t, repo)

	_, _, err := svc.Login(context.Background(), service.LoginInput{
		Email:    "notfound@example.com",
		Password: "somepassword",
	})

	// Login should return ErrUnauthorized even when user not found (don't leak info)
	if !errors.Is(err, pkgerrors.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestAuthService_RefreshToken_Success(t *testing.T) {
	password := "securepassword123"
	hash, _ := domain.HashPassword(password)

	repo := &mockUserRepository{
		createFn: func(ctx context.Context, email, passwordHash, firstName, lastName, phone string, role domain.Role) (*domain.User, error) {
			return &domain.User{
				ID:           "user-4",
				Email:        email,
				PasswordHash: passwordHash,
				FirstName:    firstName,
				LastName:     lastName,
				Phone:        phone,
				Role:         role,
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			}, nil
		},
		getByIDFn: func(ctx context.Context, id string) (*domain.User, error) {
			return &domain.User{
				ID:           id,
				Email:        "refresh@example.com",
				PasswordHash: hash,
				Role:         domain.RoleUser,
			}, nil
		},
	}
	svc, _, _ := newTestAuthService(t, repo)

	// First register to get a refresh token
	tokens, _, err := svc.Register(context.Background(), service.RegisterInput{
		Email:     "refresh@example.com",
		Password:  password,
		FirstName: "Test",
		LastName:  "User",
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	// Use refresh token to get new tokens
	newTokens, err := svc.RefreshToken(context.Background(), tokens.RefreshToken)
	if err != nil {
		t.Fatalf("expected no error on refresh, got: %v", err)
	}
	if newTokens == nil || newTokens.AccessToken == "" || newTokens.RefreshToken == "" {
		t.Error("expected valid new token pair")
	}
	// Old refresh token should be invalidated; trying again should fail
	_, err = svc.RefreshToken(context.Background(), tokens.RefreshToken)
	if !errors.Is(err, pkgerrors.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized on reuse of old token, got: %v", err)
	}
}

func TestAuthService_RefreshToken_Invalid(t *testing.T) {
	repo := &mockUserRepository{}
	svc, _, _ := newTestAuthService(t, repo)

	_, err := svc.RefreshToken(context.Background(), "invalid-refresh-token")
	if !errors.Is(err, pkgerrors.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
}
