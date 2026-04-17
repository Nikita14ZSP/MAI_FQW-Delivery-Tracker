package repository_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/repository"
)

// Integration tests for MenuRepository require a live Postgres with migrations applied.
// They run only when TEST_DATABASE_URL is set; otherwise skipped (CI/local default).

func newMenuTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping menu repository integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestMenuRepository_ListCategories_returns_seeded_categories(t *testing.T) {
	pool := newMenuTestPool(t)
	repo := repository.NewMenuRepository(pool)

	cats, err := repo.ListCategories(context.Background())
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	if len(cats) < 3 {
		t.Fatalf("expected >=3 seeded categories, got %d", len(cats))
	}
	// Verify ASC sort_order ordering.
	for i := 1; i < len(cats); i++ {
		if cats[i-1].SortOrder > cats[i].SortOrder {
			t.Errorf("categories not sorted by sort_order: %v then %v", cats[i-1], cats[i])
		}
	}
}

func TestMenuRepository_ListItems_no_filter_returns_available(t *testing.T) {
	pool := newMenuTestPool(t)
	repo := repository.NewMenuRepository(pool)

	items, err := repo.ListItems(context.Background(), "")
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if len(items) < 12 {
		t.Fatalf("expected >=12 items, got %d", len(items))
	}
	for _, it := range items {
		if !it.Available {
			t.Errorf("expected available=true, got false for %s", it.ID)
		}
	}
}

func TestMenuRepository_ListItems_with_categoryID_filters(t *testing.T) {
	pool := newMenuTestPool(t)
	repo := repository.NewMenuRepository(pool)

	cats, err := repo.ListCategories(context.Background())
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	if len(cats) == 0 {
		t.Skip("no seeded categories")
	}
	// Pick the "Горячее" category if available, else first.
	var targetID string
	for _, c := range cats {
		if c.Name == "Горячее" {
			targetID = c.ID
			break
		}
	}
	if targetID == "" {
		targetID = cats[0].ID
	}

	items, err := repo.ListItems(context.Background(), targetID)
	if err != nil {
		t.Fatalf("ListItems(%s): %v", targetID, err)
	}
	if len(items) == 0 {
		t.Fatalf("expected at least 1 item for category %s", targetID)
	}
	for _, it := range items {
		if it.CategoryID != targetID {
			t.Errorf("item %s has CategoryID=%s, want %s", it.ID, it.CategoryID, targetID)
		}
	}
}
