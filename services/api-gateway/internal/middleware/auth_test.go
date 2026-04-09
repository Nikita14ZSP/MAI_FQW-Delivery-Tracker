package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret"

func makeToken(t *testing.T, userID, role string, expiry time.Time) string {
	t.Helper()
	claims := &Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiry),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return signed
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	tokenStr := makeToken(t, "user-123", "user", time.Now().Add(time.Hour))

	h := AuthMiddleware(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify context values
		userID, ok := UserIDFromContext(r.Context())
		if !ok || userID != "user-123" {
			t.Errorf("expected user_id='user-123', got '%s' (ok=%v)", userID, ok)
		}
		role, ok := UserRoleFromContext(r.Context())
		if !ok || role != "user" {
			t.Errorf("expected role='user', got '%s' (ok=%v)", role, ok)
		}
		// Verify headers set for gRPC metadata forwarding
		if r.Header.Get("X-User-Id") != "user-123" {
			t.Errorf("expected X-User-Id='user-123', got '%s'", r.Header.Get("X-User-Id"))
		}
		if r.Header.Get("X-User-Role") != "user" {
			t.Errorf("expected X-User-Role='user', got '%s'", r.Header.Get("X-User-Role"))
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/orders", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	h := AuthMiddleware(testSecret)(newTestHandler())

	req := httptest.NewRequest(http.MethodGet, "/v1/orders", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
	body := rr.Body.String()
	if body == "" {
		t.Error("expected non-empty error body")
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	h := AuthMiddleware(testSecret)(newTestHandler())

	req := httptest.NewRequest(http.MethodGet, "/v1/orders", nil)
	req.Header.Set("Authorization", "Bearer this.is.garbage")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestAuthMiddleware_ExpiredToken(t *testing.T) {
	tokenStr := makeToken(t, "user-123", "user", time.Now().Add(-time.Hour))

	h := AuthMiddleware(testSecret)(newTestHandler())

	req := httptest.NewRequest(http.MethodGet, "/v1/orders", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for expired token, got %d", rr.Code)
	}
}

func TestAuthMiddleware_SkipPaths(t *testing.T) {
	h := AuthMiddleware(testSecret, "/v1/auth/", "/health", "/swagger/")(newTestHandler())

	// No Authorization header but path is in skip list — should pass through
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for skipped path, got %d", rr.Code)
	}
}
