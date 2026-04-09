package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequireRole_AllowedRole(t *testing.T) {
	handler := RequireRole("admin")(newTestHandler())

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/zones", nil)
	ctx := context.WithValue(req.Context(), ctxUserRole, "admin")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestRequireRole_MultipleRoles(t *testing.T) {
	handler := RequireRole("admin", "courier")(newTestHandler())

	req := httptest.NewRequest(http.MethodPost, "/v1/tracking/location", nil)
	ctx := context.WithValue(req.Context(), ctxUserRole, "courier")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for courier in allowed list, got %d", rr.Code)
	}
}

func TestRequireRole_ForbiddenRole(t *testing.T) {
	handler := RequireRole("admin")(newTestHandler())

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/zones", nil)
	ctx := context.WithValue(req.Context(), ctxUserRole, "user")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "forbidden") {
		t.Errorf("expected body to contain 'forbidden', got: %s", rr.Body.String())
	}
}

func TestRequireRole_MissingRole(t *testing.T) {
	handler := RequireRole("admin")(newTestHandler())

	// No role in context
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/zones", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "unauthorized") {
		t.Errorf("expected body to contain 'unauthorized', got: %s", rr.Body.String())
	}
}

func TestRequireRole_EmptyStringRole(t *testing.T) {
	handler := RequireRole("admin")(newTestHandler())

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/zones", nil)
	ctx := context.WithValue(req.Context(), ctxUserRole, "")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Empty string role should be treated as missing — 401
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for empty string role, got %d", rr.Code)
	}
}
