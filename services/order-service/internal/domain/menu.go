package domain

// MenuCategory represents a category of menu items (e.g., Закуски, Горячее).
type MenuCategory struct {
	ID        string
	Name      string
	SortOrder int32
}

// MenuItem represents a single dish in the menu catalog.
type MenuItem struct {
	ID         string
	CategoryID string
	Name       string
	Price      float64
	ImageURL   string
	Available  bool
}
