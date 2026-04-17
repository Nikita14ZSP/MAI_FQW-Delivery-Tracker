package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	menuv1 "github.com/mozgovojnikita/delivery-tracker/gen/menu/v1"
	"github.com/mozgovojnikita/delivery-tracker/services/order-service/internal/domain"
)

// MenuServiceIface is the contract MenuHandler depends on.
// Defined as an interface to keep the handler unit-testable with mocks.
type MenuServiceIface interface {
	ListCategories(ctx context.Context) ([]domain.MenuCategory, error)
	ListItems(ctx context.Context, categoryID string) ([]domain.MenuItem, error)
}

// MenuHandler implements menuv1.MenuServiceServer.
type MenuHandler struct {
	menuv1.UnimplementedMenuServiceServer
	svc MenuServiceIface
}

// NewMenuHandler creates a new gRPC menu handler.
func NewMenuHandler(svc MenuServiceIface) *MenuHandler {
	return &MenuHandler{svc: svc}
}

// ListMenuCategories returns the catalog categories sorted by sort_order ASC.
func (h *MenuHandler) ListMenuCategories(ctx context.Context, _ *menuv1.ListMenuCategoriesRequest) (*menuv1.ListMenuCategoriesResponse, error) {
	cats, err := h.svc.ListCategories(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list menu categories: %v", err)
	}
	out := make([]*menuv1.MenuCategory, 0, len(cats))
	for _, c := range cats {
		out = append(out, &menuv1.MenuCategory{
			Id:        c.ID,
			Name:      c.Name,
			SortOrder: c.SortOrder,
		})
	}
	return &menuv1.ListMenuCategoriesResponse{Categories: out}, nil
}

// ListMenuItems returns available menu items, optionally filtered by category_id.
func (h *MenuHandler) ListMenuItems(ctx context.Context, req *menuv1.ListMenuItemsRequest) (*menuv1.ListMenuItemsResponse, error) {
	items, err := h.svc.ListItems(ctx, req.GetCategoryId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list menu items: %v", err)
	}
	out := make([]*menuv1.MenuItem, 0, len(items))
	for _, it := range items {
		out = append(out, &menuv1.MenuItem{
			Id:         it.ID,
			CategoryId: it.CategoryID,
			Name:       it.Name,
			Price:      it.Price,
			ImageUrl:   it.ImageURL,
			Available:  it.Available,
		})
	}
	return &menuv1.ListMenuItemsResponse{Items: out}, nil
}
