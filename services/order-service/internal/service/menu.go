package service

import (
	"context"

	"github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/domain"
	"github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/repository"
)

// MenuService exposes menu catalog read operations.
type MenuService struct {
	repo repository.MenuRepository
}

// NewMenuService creates a new MenuService.
func NewMenuService(repo repository.MenuRepository) *MenuService {
	return &MenuService{repo: repo}
}

// ListCategories returns menu categories ordered by sort_order ASC.
func (s *MenuService) ListCategories(ctx context.Context) ([]domain.MenuCategory, error) {
	return s.repo.ListCategories(ctx)
}

// ListItems returns available menu items, optionally filtered by category.
func (s *MenuService) ListItems(ctx context.Context, categoryID string) ([]domain.MenuItem, error) {
	return s.repo.ListItems(ctx, categoryID)
}
