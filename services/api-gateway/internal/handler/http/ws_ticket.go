package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// wsTicketTTL is intentionally short — a ticket lives just long enough to open
// a WebSocket connection. Even if the URL leaks (proxy access log, Referer, browser history),
// it expires in seconds, not the 15-minute window of a regular access token.
const wsTicketTTL = 60 * time.Second

// wsTicketClaims is the JWT body for a single-use WebSocket ticket.
// The `purpose: "ws"` claim is verified by the gateway WS auth path; regular access tokens
// without that claim are still accepted on the legacy ?token= parameter, but the frontend
// now uses ?ticket= so long-lived access tokens never appear in WS URLs.
type wsTicketClaims struct {
	UserID  string `json:"user_id"`
	Role    string `json:"role"`
	Purpose string `json:"purpose"`
	jwt.RegisteredClaims
}

// wsTicketResponse is the JSON body returned to the client.
type wsTicketResponse struct {
	Ticket    string `json:"ticket"`
	ExpiresIn int    `json:"expires_in"` // seconds
}

// NewWSTicketHandler returns a handler that issues a short-lived WebSocket ticket
// for the authenticated caller. The caller's identity is read from X-User-Id / X-User-Role
// headers, which AuthMiddleware sets after validating the Bearer access token.
//
// Why a separate ticket and not just the access token: browsers cannot set an Authorization
// header on a WebSocket upgrade, so the credential must travel in the URL — and URLs are leaky.
// A 60-second ticket is the standard mitigation (RFC-style "single-use ticket" pattern).
func NewWSTicketHandler(jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-Id")
		role := r.Header.Get("X-User-Role")
		if userID == "" {
			http.Error(w, `{"error":"missing user context"}`, http.StatusUnauthorized)
			return
		}

		now := time.Now()
		claims := wsTicketClaims{
			UserID:  userID,
			Role:    role,
			Purpose: "ws",
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(now.Add(wsTicketTTL)),
				IssuedAt:  jwt.NewNumericDate(now),
			},
		}
		signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(jwtSecret))
		if err != nil {
			http.Error(w, `{"error":"failed to sign ticket"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(wsTicketResponse{
			Ticket:    signed,
			ExpiresIn: int(wsTicketTTL / time.Second),
		})
	}
}
