package middleware

import (
	"net/http"
	"strings"
)

// RequireRole returns a chi-compatible middleware that checks the user role
// from context (set by AuthMiddleware via ctxUserRole key).
// Returns 403 Forbidden if role is not in the allowed list.
// Returns 401 Unauthorized if role is missing from context.
// MUST be placed AFTER AuthMiddleware in the middleware chain (per D-01).
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, ok := UserRoleFromContext(r.Context())
			if !ok || role == "" {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			if _, permitted := allowed[role]; !permitted {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// rbacRule maps a method+path prefix to the allowed role(s).
// Most rules use the single-role `role` field. When a route must permit more
// than one role (e.g. an order-owner AND a courier may read a courier's last
// location — quick-260516-e30), set `roles` instead; if `roles` is non-empty
// it takes precedence over `role`.
type rbacRule struct {
	method string
	prefix string
	role   string
	roles  []string
}

// allows reports whether the given role satisfies this rule. When the rule
// declares a `roles` set, membership in that set grants access; otherwise the
// single `role` must match exactly (preserves prior behavior for all other rules).
func (rule rbacRule) allows(role string) bool {
	if len(rule.roles) > 0 {
		for _, allowed := range rule.roles {
			if role == allowed {
				return true
			}
		}
		return false
	}
	return role == rule.role
}

// PathRBAC returns a middleware that enforces role-based access based on
// request method and path prefix. Admin role bypasses all checks.
// This avoids registering per-path chi routes that conflict with grpc-gateway.
func PathRBAC(rules []rbacRule) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, _ := UserRoleFromContext(r.Context())

			// Admin bypasses all RBAC checks
			if role == "admin" {
				next.ServeHTTP(w, r)
				return
			}

			for _, rule := range rules {
				if r.Method == rule.method && strings.HasPrefix(r.URL.Path, rule.prefix) {
					if role == "" {
						http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
						return
					}
					if !rule.allows(role) {
						http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
						return
					}
					break
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RBACRules returns the RBAC rules for the API Gateway.
func RBACRules() []rbacRule {
	return []rbacRule{
		// Courier-only
		{method: "POST", prefix: "/v1/tracking/location", role: "courier"},
		// GET /v1/tracking/couriers/{courier_id}/location is the ONLY GET route
		// under this prefix (proto/tracking/v1/tracking.proto). An order owner
		// (role "user") must read their tracked courier's last position; courier
		// keeps access. POST GPS above stays courier-only. (quick-260516-e30)
		{method: "GET", prefix: "/v1/tracking/couriers/", roles: []string{"user", "courier"}},
		{method: "PATCH", prefix: "/v1/couriers/me/status", role: "courier"},
		{method: "GET", prefix: "/v1/couriers/me/available-orders", role: "courier"},
		{method: "POST", prefix: "/v1/couriers/me/deliveries/", role: "courier"},
		// User-only
		{method: "POST", prefix: "/v1/orders", role: "user"},
		{method: "POST", prefix: "/v1/deliveries/", role: "user"},
	}
}
