package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/domain"
)

// MenuRepository defines the data access interface for the menu catalog.
type MenuRepository interface {
	ListCategories(ctx context.Context) ([]domain.MenuCategory, error)
	ListItems(ctx context.Context, categoryID string) ([]domain.MenuItem, error)
}

type menuRepository struct {
	pool *pgxpool.Pool
}

// NewMenuRepository creates a new MenuRepository backed by a pgx connection pool.
func NewMenuRepository(pool *pgxpool.Pool) MenuRepository {
	return &menuRepository{pool: pool}
}

// ListCategories returns all menu categories ordered by sort_order ASC.
func (r *menuRepository) ListCategories(ctx context.Context) ([]domain.MenuCategory, error) {
	const query = `SELECT id, name, sort_order FROM menu_categories ORDER BY sort_order ASC`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list menu categories: %w", err)
	}
	defer rows.Close()

	cats := make([]domain.MenuCategory, 0)
	for rows.Next() {
		var c domain.MenuCategory
		if err := rows.Scan(&c.ID, &c.Name, &c.SortOrder); err != nil {
			return nil, fmt.Errorf("scan menu category: %w", err)
		}
		cats = append(cats, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate menu categories: %w", err)
	}
	return cats, nil
}

// ListItems returns all available menu items, optionally filtered by category_id.
// An empty categoryID returns all available items across categories.
func (r *menuRepository) ListItems(ctx context.Context, categoryID string) ([]domain.MenuItem, error) {
	query := `SELECT id, category_id, name, price, COALESCE(image_url, ''), available
              FROM menu_items
              WHERE available = true`
	args := []interface{}{}
	if categoryID != "" {
		query += ` AND category_id = $1`
		args = append(args, categoryID)
	}
	query += ` ORDER BY name ASC`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list menu items: %w", err)
	}
	defer rows.Close()

	items := make([]domain.MenuItem, 0)
	for rows.Next() {
		var it domain.MenuItem
		if err := rows.Scan(&it.ID, &it.CategoryID, &it.Name, &it.Price, &it.ImageURL, &it.Available); err != nil {
			return nil, fmt.Errorf("scan menu item: %w", err)
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate menu items: %w", err)
	}
	return items, nil
}
