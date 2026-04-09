package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	pkgerrors "github.com/mozgovojnikita/delivery-tracker/pkg/errors"
	"github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/domain"
)

// UserRepository defines the data access interface for users.
type UserRepository interface {
	Create(ctx context.Context, email, passwordHash, firstName, lastName string, role domain.Role) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByID(ctx context.Context, id string) (*domain.User, error)
}

type userRepository struct {
	pool *pgxpool.Pool
}

// NewUserRepository creates a new UserRepository backed by a pgx connection pool.
func NewUserRepository(pool *pgxpool.Pool) UserRepository {
	return &userRepository{pool: pool}
}

// Create inserts a new user into the database and returns the created record.
// Returns ErrAlreadyExists if the email is already taken (unique constraint violation).
func (r *userRepository) Create(ctx context.Context, email, passwordHash, firstName, lastName string, role domain.Role) (*domain.User, error) {
	const query = `
		INSERT INTO users (email, password_hash, first_name, last_name, role)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, email, password_hash, first_name, last_name, role, created_at, updated_at`

	u := &domain.User{}
	err := r.pool.QueryRow(ctx, query, email, passwordHash, firstName, lastName, string(role)).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.FirstName, &u.LastName, &u.Role, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if isPgError(err, &pgErr) && pgErr.Code == "23505" {
			return nil, fmt.Errorf("email %s: %w", email, pkgerrors.ErrAlreadyExists)
		}
		return nil, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

// GetByEmail retrieves a user by email address.
// Returns ErrNotFound if no user exists with that email.
func (r *userRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	const query = `
		SELECT id, email, password_hash, first_name, last_name, role, created_at, updated_at
		FROM users
		WHERE email = $1`

	u := &domain.User{}
	err := r.pool.QueryRow(ctx, query, email).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.FirstName, &u.LastName, &u.Role, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("user with email %s: %w", email, pkgerrors.ErrNotFound)
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return u, nil
}

// GetByID retrieves a user by their UUID.
// Returns ErrNotFound if no user exists with that ID.
func (r *userRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	const query = `
		SELECT id, email, password_hash, first_name, last_name, role, created_at, updated_at
		FROM users
		WHERE id = $1`

	u := &domain.User{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.FirstName, &u.LastName, &u.Role, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("user %s: %w", id, pkgerrors.ErrNotFound)
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return u, nil
}

// isPgError checks if err is a *pgconn.PgError and assigns it to target.
func isPgError(err error, target **pgconn.PgError) bool {
	pgErr, ok := err.(*pgconn.PgError)
	if ok {
		*target = pgErr
	}
	return ok
}
