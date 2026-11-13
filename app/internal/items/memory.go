package items

import (
	"context"
	"fmt"
	"sync"
)

// MemoryRepository is an in-process Repository. It keeps insertion order so
// pagination is stable. It is intentionally simple: a durable store (DynamoDB
// or RDS) would implement the same interface without touching the HTTP layer.
type MemoryRepository struct {
	mu    sync.RWMutex
	byID  map[string]Item
	order []string
}

// NewMemoryRepository returns an empty repository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{byID: make(map[string]Item)}
}

// List returns items[offset:offset+limit] in insertion order.
func (r *MemoryRepository) List(_ context.Context, offset, limit int) (Page, error) {
	if offset < 0 || limit <= 0 {
		return Page{}, fmt.Errorf("invalid page window offset=%d limit=%d", offset, limit)
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	total := len(r.order)
	page := Page{Total: total, Limit: limit, Offset: offset, Items: []Item{}}
	if offset >= total {
		return page, nil
	}
	end := min(offset+limit, total)
	page.Items = make([]Item, 0, end-offset)
	for _, id := range r.order[offset:end] {
		page.Items = append(page.Items, r.byID[id])
	}
	return page, nil
}

// Get returns the item with the given ID or ErrNotFound.
func (r *MemoryRepository) Get(_ context.Context, id string) (Item, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.byID[id]
	if !ok {
		return Item{}, ErrNotFound
	}
	return item, nil
}

// Insert stores a new item. Duplicate IDs are rejected.
func (r *MemoryRepository) Insert(_ context.Context, item Item) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byID[item.ID]; exists {
		return fmt.Errorf("item %s already exists", item.ID)
	}
	r.byID[item.ID] = item
	r.order = append(r.order, item.ID)
	return nil
}
