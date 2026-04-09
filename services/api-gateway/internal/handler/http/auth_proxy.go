package http

import (
	"net/http"
	"net/http/httputil"
)

// --- Swagger model definitions ---

// RegisterRequest is the registration payload.
type RegisterRequest struct {
	Email     string `json:"email" example:"user@example.com"`
	Password  string `json:"password" example:"mypassword123"`
	FirstName string `json:"first_name" example:"Никита"`
	LastName  string `json:"last_name" example:"Мозговой"`
}

// LoginRequest is the login payload.
type LoginRequest struct {
	Email    string `json:"email" example:"user@example.com"`
	Password string `json:"password" example:"mypassword123"`
}

// RefreshRequest is the token refresh payload.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" example:"550e8400-e29b-41d4-a716-446655440000"`
}

// UserInfo is the user object in auth responses.
type UserInfo struct {
	ID        string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Email     string `json:"email" example:"user@example.com"`
	FirstName string `json:"first_name" example:"Никита"`
	LastName  string `json:"last_name" example:"Мозговой"`
	Role      string `json:"role" example:"user"`
}

// AuthResponse is the response for register and login.
type AuthResponse struct {
	User         UserInfo `json:"user"`
	AccessToken  string   `json:"access_token" example:"eyJhbGciOiJIUzI1NiIs..."`
	RefreshToken string   `json:"refresh_token" example:"550e8400-e29b-41d4-a716-446655440000"`
}

// TokenResponse is the response for token refresh.
type TokenResponse struct {
	AccessToken  string `json:"access_token" example:"eyJhbGciOiJIUzI1NiIs..."`
	RefreshToken string `json:"refresh_token" example:"550e8400-e29b-41d4-a716-446655440000"`
}

// ErrorResponse is the generic error response.
type ErrorResponse struct {
	Error string `json:"error" example:"invalid input"`
}

// --- Proxy handler ---

// AuthProxy wraps a reverse proxy to order-service HTTP with Swagger-annotated handler methods.
type AuthProxy struct {
	proxy *httputil.ReverseProxy
}

// NewAuthProxy creates an auth proxy targeting the order-service HTTP endpoint.
func NewAuthProxy(proxy *httputil.ReverseProxy) *AuthProxy {
	return &AuthProxy{proxy: proxy}
}

// Register godoc
// @Summary Register a new user
// @Description Create a new user account with email, password, first name and last name
// @Tags auth
// @Accept json
// @Produce json
// @Param body body RegisterRequest true "Registration payload"
// @Success 201 {object} AuthResponse
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /v1/auth/register [post]
func (p *AuthProxy) Register(w http.ResponseWriter, r *http.Request) {
	r.URL.Path = "/v1/auth/register"
	p.proxy.ServeHTTP(w, r)
}

// Login godoc
// @Summary Log in with credentials
// @Description Authenticate with email and password, receive JWT tokens
// @Tags auth
// @Accept json
// @Produce json
// @Param body body LoginRequest true "Login payload"
// @Success 200 {object} AuthResponse
// @Failure 401 {object} ErrorResponse
// @Router /v1/auth/login [post]
func (p *AuthProxy) Login(w http.ResponseWriter, r *http.Request) {
	r.URL.Path = "/v1/auth/login"
	p.proxy.ServeHTTP(w, r)
}

// RefreshToken godoc
// @Summary Refresh access token
// @Description Exchange a valid refresh token for a new token pair
// @Tags auth
// @Accept json
// @Produce json
// @Param body body RefreshRequest true "Refresh payload"
// @Success 200 {object} TokenResponse
// @Failure 401 {object} ErrorResponse
// @Router /v1/auth/refresh [post]
func (p *AuthProxy) RefreshToken(w http.ResponseWriter, r *http.Request) {
	r.URL.Path = "/v1/auth/refresh"
	p.proxy.ServeHTTP(w, r)
}

