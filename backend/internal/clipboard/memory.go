package clipboard

import (
	"context"
	"sort"
	"sync"
	"time"
)

type MemoryRepository struct {
	mu     sync.RWMutex
	nextID int64
	items  map[int64]Item
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{nextID: 1, items: make(map[int64]Item)}
}

func (r *MemoryRepository) Create(_ context.Context, userID int64, content, source string) (Item, error) {
	if err := validateContent(content); err != nil {
		return Item{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	item := Item{
		ID:        r.nextID,
		UserID:    userID,
		Content:   content,
		Source:    normalizeSource(source),
		CreatedAt: time.Now().UTC(),
	}
	r.nextID++
	r.items[item.ID] = item
	return item, nil
}

func (r *MemoryRepository) List(_ context.Context, userID int64, limit int) ([]Item, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]Item, 0, len(r.items))
	for _, item := range r.items {
		if item.UserID != userID {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (r *MemoryRepository) Delete(_ context.Context, userID, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	item, ok := r.items[id]
	if !ok || item.UserID != userID {
		return ErrNotFound
	}
	delete(r.items, id)
	return nil
}
