package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const (
	ctxUserID   contextKey = "user_id"
	ctxUserRole contextKey = "user_role"
)

// Claims mirrors the JWT claims structure used by order-service auth.
// Gateway does NOT import order-service — it defines its own identical struct.
// Purpose is set to "ws" for short-lived tickets issued by POST /v1/ws-ticket;
// regular access tokens leave it empty.
type Claims struct {
	UserID  string `json:"user_id"`
	Role    string `json:"role"`
	Purpose string `json:"purpose,omitempty"`
	jwt.RegisteredClaims
}

// AuthMiddleware validates JWT from Authorization: Bearer header.
// On success, injects user_id and role into context and sets X-User-Id / X-User-Role headers
// for grpc-gateway to forward as gRPC metadata.
// Paths matching any skipPaths prefix bypass auth.
// WebSocket upgrade requests are authenticated via ?token= query parameter (D-03, D-12).
func AuthMiddleware(jwtSecret string, skipPaths ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, prefix := range skipPaths {
				if strings.HasPrefix(r.URL.Path, prefix) {
					next.ServeHTTP(w, r)
					return
				}
			}

			// WebSocket connections pass credentials via the URL — browsers cannot set
			// Authorization headers on Upgrade requests. Preferred: short-lived ticket
			// (?ticket=, 60s TTL, purpose=ws claim) issued by POST /v1/ws-ticket.
			// Legacy: access token (?token=) — still accepted so older clients/tooling
			// (courier-sim, integration tests) keep working.
			if r.Header.Get("Upgrade") == "websocket" {
				tokenString := r.URL.Query().Get("ticket")
				requirePurposeWS := tokenString != ""
				if tokenString == "" {
					tokenString = r.URL.Query().Get("token")
				}
				if tokenString == "" {
					http.Error(w, `{"error":"missing ticket query parameter for websocket"}`, http.StatusUnauthorized)
					return
				}
				claims := &Claims{}
				token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
					if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
						return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
					}
					return []byte(jwtSecret), nil
				})
				if err != nil || !token.Valid {
					http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
					return
				}
				if requirePurposeWS && claims.Purpose != "ws" {
					// A token presented via ?ticket= must be a ws-ticket. This blocks
					// long-lived access tokens from being smuggled through the preferred path.
					http.Error(w, `{"error":"ticket has wrong purpose"}`, http.StatusUnauthorized)
					return
				}
				ctx := context.WithValue(r.Context(), ctxUserID, claims.UserID)
				ctx = context.WithValue(ctx, ctxUserRole, claims.Role)
				r.Header.Set("X-User-Id", claims.UserID)
				r.Header.Set("X-User-Role", claims.Role)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				http.Error(w, `{"error":"missing or invalid authorization header"}`, http.StatusUnauthorized)
				return
			}

			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			claims := &Claims{}
			token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
				}
				return []byte(jwtSecret), nil
			})
			if err != nil || !token.Valid {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), ctxUserID, claims.UserID)
			ctx = context.WithValue(ctx, ctxUserRole, claims.Role)

			r.Header.Set("X-User-Id", claims.UserID)
			r.Header.Set("X-User-Role", claims.Role)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromContext extracts user_id set by AuthMiddleware.
func UserIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ctxUserID).(string)
	return v, ok
}

// UserRoleFromContext extracts user_role set by AuthMiddleware.
func UserRoleFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ctxUserRole).(string)
	return v, ok
}
