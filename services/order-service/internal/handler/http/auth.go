package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	pkgerrors "github.com/mozgovojnikita/delivery-tracker/pkg/errors"
	"github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/domain"
	"github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/service"
)

// registerRequest is the request body for registration.
type registerRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone,omitempty"`
	Role      string `json:"role"`
}

// loginRequest is the request body for login.
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// refreshRequest is the request body for token refresh.
type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// userResponse is the user data included in auth responses.
type userResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone,omitempty"`
	Role      string `json:"role"`
}

// authResponse is the response for register and login.
type authResponse struct {
	User         userResponse `json:"user"`
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
}

// tokenResponse is the response for token refresh.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// errorResponse is the generic error response.
type errorResponse struct {
	Error string `json:"error"`
}

// AuthServiceIface defines the methods required by the AuthHandler.
type AuthServiceIface interface {
	Register(ctx context.Context, input service.RegisterInput) (*service.TokenPair, *domain.User, error)
	Login(ctx context.Context, input service.LoginInput) (*service.TokenPair, *domain.User, error)
	RefreshToken(ctx context.Context, refreshToken string) (*service.TokenPair, error)
}

// AuthHandler serves HTTP auth endpoints.
type AuthHandler struct {
	svc AuthServiceIface
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(svc AuthServiceIface) *AuthHandler {
	return &AuthHandler{svc: svc}
}

// Routes returns a chi.Router with auth endpoints mounted.
func (h *AuthHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/register", h.Register)
	r.Post("/login", h.Login)
	r.Post("/refresh", h.RefreshToken)
	return r
}

// Register handles POST /v1/auth/register.
// Returns 201 on success, 400 on invalid input, 409 on duplicate email.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, pkgerrors.ErrInvalidInput)
		return
	}

	tokens, user, err := h.svc.Register(r.Context(), service.RegisterInput{
		Email:     req.Email,
		Password:  req.Password,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Phone:     req.Phone,
		Role:      domain.Role(req.Role),
	})
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, authResponse{
		User:         toUserResponse(user),
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
	})
}

// Login handles POST /v1/auth/login.
// Returns 200 on success, 400 on invalid input, 401 on wrong credentials.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, pkgerrors.ErrInvalidInput)
		return
	}

	tokens, user, err := h.svc.Login(r.Context(), service.LoginInput{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, authResponse{
		User:         toUserResponse(user),
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
	})
}

// RefreshToken handles POST /v1/auth/refresh.
// Returns 200 on success, 401 on invalid or expired refresh token.
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, pkgerrors.ErrInvalidInput)
		return
	}

	tokens, err := h.svc.RefreshToken(r.Context(), req.RefreshToken)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, tokenResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
	})
}

// writeJSON sets Content-Type to application/json, writes the status code, and encodes v.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Best-effort: headers already sent
		return
	}
}

// writeError maps sentinel errors to HTTP status codes and writes a JSON error response.
func writeError(w http.ResponseWriter, err error) {
	code := httpStatusFromError(err)
	writeJSON(w, code, errorResponse{Error: err.Error()})
}

// httpStatusFromError maps pkg/errors sentinels to HTTP status codes.
func httpStatusFromError(err error) int {
	switch {
	case isErr(err, pkgerrors.ErrNotFound):
		return http.StatusNotFound
	case isErr(err, pkgerrors.ErrAlreadyExists):
		return http.StatusConflict
	case isErr(err, pkgerrors.ErrInvalidInput):
		return http.StatusBadRequest
	case isErr(err, pkgerrors.ErrUnauthorized):
		return http.StatusUnauthorized
	case isErr(err, pkgerrors.ErrForbidden):
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}

// isErr checks if err wraps target.
func isErr(err, target error) bool {
	unwrapped := err
	for unwrapped != nil {
		if unwrapped == target {
			return true
		}
		type unwrapper interface{ Unwrap() error }
		if u, ok := unwrapped.(unwrapper); ok {
			unwrapped = u.Unwrap()
		} else {
			break
		}
	}
	return false
}

// toUserResponse converts a domain.User to a userResponse.
func toUserResponse(u *domain.User) userResponse {
	if u == nil {
		return userResponse{}
	}
	return userResponse{
		ID:        u.ID,
		Email:     u.Email,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Phone:     u.Phone,
		Role:      string(u.Role),
	}
}
