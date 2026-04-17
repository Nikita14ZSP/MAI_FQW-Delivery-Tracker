package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	pkgerrors "github.com/mozgovojnikita/delivery-tracker/pkg/errors"
	"github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/domain"
	httphandler "github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/handler/http"
	"github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/service"
)

// mockAuthService is a manual mock for the auth service.
type mockAuthService struct {
	registerFn     func(ctx context.Context, input service.RegisterInput) (*service.TokenPair, *domain.User, error)
	loginFn        func(ctx context.Context, input service.LoginInput) (*service.TokenPair, *domain.User, error)
	refreshTokenFn func(ctx context.Context, refreshToken string) (*service.TokenPair, error)
}

func (m *mockAuthService) Register(ctx context.Context, input service.RegisterInput) (*service.TokenPair, *domain.User, error) {
	if m.registerFn != nil {
		return m.registerFn(ctx, input)
	}
	return nil, nil, nil
}

func (m *mockAuthService) Login(ctx context.Context, input service.LoginInput) (*service.TokenPair, *domain.User, error) {
	if m.loginFn != nil {
		return m.loginFn(ctx, input)
	}
	return nil, nil, nil
}

func (m *mockAuthService) RefreshToken(ctx context.Context, refreshToken string) (*service.TokenPair, error) {
	if m.refreshTokenFn != nil {
		return m.refreshTokenFn(ctx, refreshToken)
	}
	return nil, nil
}

func postJSON(t *testing.T, handler http.Handler, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func TestAuthHandler_Register_Success(t *testing.T) {
	mock := &mockAuthService{
		registerFn: func(ctx context.Context, input service.RegisterInput) (*service.TokenPair, *domain.User, error) {
			return &service.TokenPair{
				AccessToken:  "access-token-value",
				RefreshToken: "refresh-token-value",
			}, &domain.User{
				ID:    "user-1",
				Email: input.Email,
				Role:  domain.RoleUser,
			}, nil
		},
	}
	handler := httphandler.NewAuthHandler(mock).Routes()
	w := postJSON(t, handler, "/register", map[string]string{
		"email":      "test@example.com",
		"password":   "securepassword123",
		"first_name": "Иван",
		"last_name":  "Иванов",
	})

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["access_token"] == "" || resp["access_token"] == nil {
		t.Error("expected access_token in response")
	}
	if resp["refresh_token"] == "" || resp["refresh_token"] == nil {
		t.Error("expected refresh_token in response")
	}
}

func TestAuthHandler_Register_WithValidPhone(t *testing.T) {
	var capturedPhone string
	mock := &mockAuthService{
		registerFn: func(ctx context.Context, input service.RegisterInput) (*service.TokenPair, *domain.User, error) {
			capturedPhone = input.Phone
			return &service.TokenPair{
					AccessToken:  "access-token-value",
					RefreshToken: "refresh-token-value",
				}, &domain.User{
					ID:    "user-phone-1",
					Email: input.Email,
					Phone: input.Phone,
					Role:  domain.RoleUser,
				}, nil
		},
	}
	handler := httphandler.NewAuthHandler(mock).Routes()
	w := postJSON(t, handler, "/register", map[string]string{
		"email":      "phone@example.com",
		"password":   "securepassword123",
		"first_name": "Иван",
		"last_name":  "Иванов",
		"phone":      "+79991234567",
	})

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}
	if capturedPhone != "+79991234567" {
		t.Errorf("expected service to receive phone=+79991234567, got %q", capturedPhone)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	user, ok := resp["user"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected user object in response, got: %v", resp["user"])
	}
	if user["phone"] != "+79991234567" {
		t.Errorf("expected user.phone=+79991234567 in response, got %v", user["phone"])
	}
}

func TestAuthHandler_Register_WithoutPhoneSucceeds(t *testing.T) {
	var capturedPhone string
	mock := &mockAuthService{
		registerFn: func(ctx context.Context, input service.RegisterInput) (*service.TokenPair, *domain.User, error) {
			capturedPhone = input.Phone
			return &service.TokenPair{
					AccessToken:  "access-token-value",
					RefreshToken: "refresh-token-value",
				}, &domain.User{
					ID:    "user-no-phone",
					Email: input.Email,
					Role:  domain.RoleUser,
				}, nil
		},
	}
	handler := httphandler.NewAuthHandler(mock).Routes()
	w := postJSON(t, handler, "/register", map[string]string{
		"email":      "nophone@example.com",
		"password":   "securepassword123",
		"first_name": "Иван",
		"last_name":  "Иванов",
	})

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}
	if capturedPhone != "" {
		t.Errorf("expected service to receive empty phone, got %q", capturedPhone)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	user, ok := resp["user"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected user object in response, got: %v", resp["user"])
	}
	// Phone should be either absent (omitempty) or empty string
	if phone, exists := user["phone"]; exists && phone != "" {
		t.Errorf("expected user.phone to be empty/omitted, got %v", phone)
	}
}

func TestAuthHandler_Register_WithInvalidPhone(t *testing.T) {
	mock := &mockAuthService{
		registerFn: func(ctx context.Context, input service.RegisterInput) (*service.TokenPair, *domain.User, error) {
			// Service-layer validation should reject invalid phone
			return nil, nil, pkgerrors.ErrInvalidInput
		},
	}
	handler := httphandler.NewAuthHandler(mock).Routes()
	w := postJSON(t, handler, "/register", map[string]string{
		"email":      "bad@example.com",
		"password":   "securepassword123",
		"first_name": "Иван",
		"last_name":  "Иванов",
		"phone":      "12345",
	})

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 (invalid input), got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuthHandler_Register_WithCourierRole(t *testing.T) {
	mock := &mockAuthService{
		registerFn: func(ctx context.Context, input service.RegisterInput) (*service.TokenPair, *domain.User, error) {
			return &service.TokenPair{
					AccessToken:  "access-token-value",
					RefreshToken: "refresh-token-value",
				}, &domain.User{
					ID:    "courier-1",
					Email: input.Email,
					Role:  input.Role,
				}, nil
		},
	}
	handler := httphandler.NewAuthHandler(mock).Routes()
	w := postJSON(t, handler, "/register", map[string]string{
		"email":      "c@x.io",
		"password":   "password123",
		"first_name": "C",
		"last_name":  "X",
		"role":       "courier",
	})

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	user, ok := resp["user"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected user object in response, got: %v", resp["user"])
	}
	if user["role"] != "courier" {
		t.Errorf("expected role=courier in response, got %v", user["role"])
	}
}

func TestAuthHandler_Register_InvalidInput(t *testing.T) {
	mock := &mockAuthService{
		registerFn: func(ctx context.Context, input service.RegisterInput) (*service.TokenPair, *domain.User, error) {
			return nil, nil, pkgerrors.ErrInvalidInput
		},
	}
	handler := httphandler.NewAuthHandler(mock).Routes()
	w := postJSON(t, handler, "/register", map[string]string{
		"email":      "",
		"password":   "password123",
		"first_name": "Test",
		"last_name":  "User",
	})

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestAuthHandler_Register_DuplicateEmail(t *testing.T) {
	mock := &mockAuthService{
		registerFn: func(ctx context.Context, input service.RegisterInput) (*service.TokenPair, *domain.User, error) {
			return nil, nil, pkgerrors.ErrAlreadyExists
		},
	}
	handler := httphandler.NewAuthHandler(mock).Routes()
	w := postJSON(t, handler, "/register", map[string]string{
		"email":      "existing@example.com",
		"password":   "password123",
		"first_name": "Test",
		"last_name":  "User",
	})

	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d", w.Code)
	}
}

func TestAuthHandler_Login_Success(t *testing.T) {
	mock := &mockAuthService{
		loginFn: func(ctx context.Context, input service.LoginInput) (*service.TokenPair, *domain.User, error) {
			return &service.TokenPair{
				AccessToken:  "access-token",
				RefreshToken: "refresh-token",
			}, &domain.User{
				ID:    "user-2",
				Email: input.Email,
				Role:  domain.RoleUser,
			}, nil
		},
	}
	handler := httphandler.NewAuthHandler(mock).Routes()
	w := postJSON(t, handler, "/login", map[string]string{
		"email":    "test@example.com",
		"password": "password123",
	})

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["access_token"] == nil {
		t.Error("expected access_token in response")
	}
}

func TestAuthHandler_Login_WrongPassword(t *testing.T) {
	mock := &mockAuthService{
		loginFn: func(ctx context.Context, input service.LoginInput) (*service.TokenPair, *domain.User, error) {
			return nil, nil, pkgerrors.ErrUnauthorized
		},
	}
	handler := httphandler.NewAuthHandler(mock).Routes()
	w := postJSON(t, handler, "/login", map[string]string{
		"email":    "test@example.com",
		"password": "wrongpassword",
	})

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestAuthHandler_RefreshToken_Success(t *testing.T) {
	mock := &mockAuthService{
		refreshTokenFn: func(ctx context.Context, refreshToken string) (*service.TokenPair, error) {
			return &service.TokenPair{
				AccessToken:  "new-access-token",
				RefreshToken: "new-refresh-token",
			}, nil
		},
	}
	handler := httphandler.NewAuthHandler(mock).Routes()
	w := postJSON(t, handler, "/refresh", map[string]string{
		"refresh_token": "old-refresh-token",
	})

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["access_token"] == nil {
		t.Error("expected access_token in response")
	}
	if resp["refresh_token"] == nil {
		t.Error("expected refresh_token in response")
	}
}

func TestAuthHandler_RefreshToken_Invalid(t *testing.T) {
	mock := &mockAuthService{
		refreshTokenFn: func(ctx context.Context, refreshToken string) (*service.TokenPair, error) {
			return nil, pkgerrors.ErrUnauthorized
		},
	}
	handler := httphandler.NewAuthHandler(mock).Routes()
	w := postJSON(t, handler, "/refresh", map[string]string{
		"refresh_token": "invalid-token",
	})

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}
