package grpc_test

import (
	"context"
	"errors"
	"testing"

	menuv1 "github.com/mozgovojnikita/delivery-tracker/gen/menu/v1"
	"github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/domain"
	grpchandler "github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/handler/grpc"
)

// mockMenuService is a manual mock implementing grpchandler.MenuServiceIface.
type mockMenuService struct {
	listCategoriesFn func(ctx context.Context) ([]domain.MenuCategory, error)
	listItemsFn      func(ctx context.Context, categoryID string) ([]domain.MenuItem, error)
	lastCategoryID   string
}

func (m *mockMenuService) ListCategories(ctx context.Context) ([]domain.MenuCategory, error) {
	if m.listCategoriesFn != nil {
		return m.listCategoriesFn(ctx)
	}
	return nil, nil
}

func (m *mockMenuService) ListItems(ctx context.Context, categoryID string) ([]domain.MenuItem, error) {
	m.lastCategoryID = categoryID
	if m.listItemsFn != nil {
		return m.listItemsFn(ctx, categoryID)
	}
	return nil, nil
}

func TestListMenuCategories_maps_domain_to_proto(t *testing.T) {
	mock := &mockMenuService{
		listCategoriesFn: func(ctx context.Context) ([]domain.MenuCategory, error) {
			return []domain.MenuCategory{
				{ID: "cat-1", Name: "Закуски", SortOrder: 1},
				{ID: "cat-2", Name: "Горячее", SortOrder: 2},
			}, nil
		},
	}
	h := grpchandler.NewMenuHandler(mock)

	resp, err := h.ListMenuCategories(context.Background(), &menuv1.ListMenuCategoriesRequest{})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got := len(resp.GetCategories()); got != 2 {
		t.Fatalf("expected 2 categories, got %d", got)
	}
	if resp.GetCategories()[0].GetId() != "cat-1" || resp.GetCategories()[0].GetName() != "Закуски" {
		t.Errorf("unexpected first category: %+v", resp.GetCategories()[0])
	}
	if resp.GetCategories()[1].GetSortOrder() != 2 {
		t.Errorf("expected sort_order=2, got %d", resp.GetCategories()[1].GetSortOrder())
	}
}

func TestListMenuCategories_propagates_error(t *testing.T) {
	mock := &mockMenuService{
		listCategoriesFn: func(ctx context.Context) ([]domain.MenuCategory, error) {
			return nil, errors.New("db down")
		},
	}
	h := grpchandler.NewMenuHandler(mock)

	if _, err := h.ListMenuCategories(context.Background(), &menuv1.ListMenuCategoriesRequest{}); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListMenuItems_passes_category_id(t *testing.T) {
	mock := &mockMenuService{
		listItemsFn: func(ctx context.Context, categoryID string) ([]domain.MenuItem, error) {
			return []domain.MenuItem{
				{ID: "it-1", CategoryID: categoryID, Name: "Стейк", Price: 890, ImageURL: "https://example/x.jpg", Available: true},
			}, nil
		},
	}
	h := grpchandler.NewMenuHandler(mock)

	resp, err := h.ListMenuItems(context.Background(), &menuv1.ListMenuItemsRequest{CategoryId: "cat-2"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if mock.lastCategoryID != "cat-2" {
		t.Errorf("expected service to receive 'cat-2', got %q", mock.lastCategoryID)
	}
	if got := len(resp.GetItems()); got != 1 {
		t.Fatalf("expected 1 item, got %d", got)
	}
	item := resp.GetItems()[0]
	if item.GetId() != "it-1" || item.GetCategoryId() != "cat-2" || item.GetName() != "Стейк" {
		t.Errorf("unexpected item mapping: %+v", item)
	}
	if item.GetPrice() != 890 || item.GetImageUrl() != "https://example/x.jpg" || !item.GetAvailable() {
		t.Errorf("unexpected scalar mapping: price=%v imageUrl=%q available=%v", item.GetPrice(), item.GetImageUrl(), item.GetAvailable())
	}
}

func TestListMenuItems_empty_category_returns_all(t *testing.T) {
	mock := &mockMenuService{
		listItemsFn: func(ctx context.Context, categoryID string) ([]domain.MenuItem, error) {
			return []domain.MenuItem{
				{ID: "a", Name: "A", Available: true},
				{ID: "b", Name: "B", Available: true},
			}, nil
		},
	}
	h := grpchandler.NewMenuHandler(mock)

	resp, err := h.ListMenuItems(context.Background(), &menuv1.ListMenuItemsRequest{})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if mock.lastCategoryID != "" {
		t.Errorf("expected empty category id, got %q", mock.lastCategoryID)
	}
	if len(resp.GetItems()) != 2 {
		t.Errorf("expected 2 items, got %d", len(resp.GetItems()))
	}
}
