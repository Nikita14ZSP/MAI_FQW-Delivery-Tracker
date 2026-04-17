package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newPathRBACHandler wraps newTestHandler with PathRBAC(RBACRules()) middleware.
func newPathRBACHandler() http.Handler {
	return PathRBAC(RBACRules())(newTestHandler())
}

// TestRBAC_PatchCourierMeStatus_AllowedForCourier verifies that a courier role can PATCH the status endpoint.
func TestRBAC_PatchCourierMeStatus_AllowedForCourier(t *testing.T) {
	handler := newPathRBACHandler()

	req := httptest.NewRequest(http.MethodPatch, "/v1/couriers/me/status", nil)
	ctx := context.WithValue(req.Context(), ctxUserRole, "courier")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for courier role, got %d", rr.Code)
	}
}

// TestRBAC_PatchCourierMeStatus_DeniedForUser verifies that a user role is forbidden from PATCH status endpoint.
func TestRBAC_PatchCourierMeStatus_DeniedForUser(t *testing.T) {
	handler := newPathRBACHandler()

	req := httptest.NewRequest(http.MethodPatch, "/v1/couriers/me/status", nil)
	ctx := context.WithValue(req.Context(), ctxUserRole, "user")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for user role, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "forbidden") {
		t.Errorf("expected body to contain 'forbidden', got: %s", rr.Body.String())
	}
}

// TestRBAC_PatchCourierMeStatus_DeniedForAnon verifies that an unauthenticated request gets 401.
func TestRBAC_PatchCourierMeStatus_DeniedForAnon(t *testing.T) {
	handler := newPathRBACHandler()

	// No role in context — anonymous caller.
	req := httptest.NewRequest(http.MethodPatch, "/v1/couriers/me/status", nil)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for anonymous, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "unauthorized") {
		t.Errorf("expected body to contain 'unauthorized', got: %s", rr.Body.String())
	}
}

// TestRBAC_GetAvailableOrders_AllowedForCourier verifies courier role can GET available-orders.
func TestRBAC_GetAvailableOrders_AllowedForCourier(t *testing.T) {
	handler := newPathRBACHandler()

	req := httptest.NewRequest(http.MethodGet, "/v1/couriers/me/available-orders", nil)
	ctx := context.WithValue(req.Context(), ctxUserRole, "courier")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for courier role on available-orders, got %d", rr.Code)
	}
}

// TestRBAC_GetAvailableOrders_DeniedForUser verifies user role is forbidden from available-orders.
func TestRBAC_GetAvailableOrders_DeniedForUser(t *testing.T) {
	handler := newPathRBACHandler()

	req := httptest.NewRequest(http.MethodGet, "/v1/couriers/me/available-orders", nil)
	ctx := context.WithValue(req.Context(), ctxUserRole, "user")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for user role on available-orders, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "forbidden") {
		t.Errorf("expected body to contain 'forbidden', got: %s", rr.Body.String())
	}
}

// TestRBAC_GetAvailableOrders_DeniedForAnon verifies anonymous request gets 401 on available-orders.
func TestRBAC_GetAvailableOrders_DeniedForAnon(t *testing.T) {
	handler := newPathRBACHandler()

	req := httptest.NewRequest(http.MethodGet, "/v1/couriers/me/available-orders", nil)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for anonymous on available-orders, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "unauthorized") {
		t.Errorf("expected body to contain 'unauthorized', got: %s", rr.Body.String())
	}
}

// --- quick-260516-e30: order-owner (user) may GET courier last-location ---
// The ONLY GET route under prefix /v1/tracking/couriers/ is
// /v1/tracking/couriers/{courier_id}/location (per proto/tracking/v1/tracking.proto).
// An order owner (role "user") must be able to read their tracked courier's
// last position; POST /v1/tracking/location (a different prefix) stays
// courier-only.

// TestRBAC_GetCourierLocation_AllowedForUser: order-owner GET → 200 (was 403).
func TestRBAC_GetCourierLocation_AllowedForUser(t *testing.T) {
	handler := newPathRBACHandler()

	req := httptest.NewRequest(http.MethodGet, "/v1/tracking/couriers/crr-1/location", nil)
	ctx := context.WithValue(req.Context(), ctxUserRole, "user")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for user GET courier last-location, got %d", rr.Code)
	}
}

// TestRBAC_PostTrackingLocation_DeniedForUser: user POST GPS → still 403.
func TestRBAC_PostTrackingLocation_DeniedForUser(t *testing.T) {
	handler := newPathRBACHandler()

	req := httptest.NewRequest(http.MethodPost, "/v1/tracking/location", nil)
	ctx := context.WithValue(req.Context(), ctxUserRole, "user")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for user POST /v1/tracking/location, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "forbidden") {
		t.Errorf("expected body to contain 'forbidden', got: %s", rr.Body.String())
	}
}

// TestRBAC_GetCourierLocation_AllowedForCourier: courier GET → 200 (unchanged).
func TestRBAC_GetCourierLocation_AllowedForCourier(t *testing.T) {
	handler := newPathRBACHandler()

	req := httptest.NewRequest(http.MethodGet, "/v1/tracking/couriers/crr-1/location", nil)
	ctx := context.WithValue(req.Context(), ctxUserRole, "courier")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for courier GET courier last-location, got %d", rr.Code)
	}
}

// TestRBAC_PostTrackingLocation_AllowedForCourier: courier POST GPS → 200 (unchanged).
func TestRBAC_PostTrackingLocation_AllowedForCourier(t *testing.T) {
	handler := newPathRBACHandler()

	req := httptest.NewRequest(http.MethodPost, "/v1/tracking/location", nil)
	ctx := context.WithValue(req.Context(), ctxUserRole, "courier")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for courier POST /v1/tracking/location, got %d", rr.Code)
	}
}

// TestRBAC_GetCourierLocation_DeniedForAnon: anonymous GET → 401.
func TestRBAC_GetCourierLocation_DeniedForAnon(t *testing.T) {
	handler := newPathRBACHandler()

	req := httptest.NewRequest(http.MethodGet, "/v1/tracking/couriers/crr-1/location", nil)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for anonymous GET courier last-location, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "unauthorized") {
		t.Errorf("expected body to contain 'unauthorized', got: %s", rr.Body.String())
	}
}

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

// TestRBAC_GetCourierProfile_PublicAnon verifies that GET /v1/couriers/{id}/profile is public:
// no RBAC rule is registered for this path so anonymous requests pass through (RATE-05).
func TestRBAC_GetCourierProfile_PublicAnon(t *testing.T) {
	handler := newPathRBACHandler()
	req := httptest.NewRequest(http.MethodGet, "/v1/couriers/crr-1/profile", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for anonymous GET /profile, got %d", rr.Code)
	}
}
