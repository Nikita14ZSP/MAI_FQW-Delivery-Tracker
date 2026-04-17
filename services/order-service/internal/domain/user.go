package domain

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Role defines the user role in the system.
type Role string

const (
	RoleUser    Role = "user"
	RoleCourier Role = "courier"
	RoleAdmin   Role = "admin"
)

// User represents an authenticated user of the delivery system.
type User struct {
	ID           string
	Email        string
	PasswordHash string
	FirstName    string
	LastName     string
	Phone        string
	Role         Role
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// HashPassword hashes a plaintext password using bcrypt with cost=12.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword compares a bcrypt hash against a plaintext password.
// Returns nil if they match, error otherwise.
func CheckPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
