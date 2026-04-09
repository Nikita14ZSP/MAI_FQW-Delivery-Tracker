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

// rbacRule maps a method+path prefix to the allowed role.
type rbacRule struct {
	method string
	prefix string
	role   string
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
					if role != rule.role {
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
		{method: "GET", prefix: "/v1/tracking/couriers/", role: "courier"},
		// User-only
		{method: "POST", prefix: "/v1/orders", role: "user"},
		{method: "POST", prefix: "/v1/deliveries/", role: "user"},
	}
}
