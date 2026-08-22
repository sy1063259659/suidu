package clipboard

import (
	"context"
	"errors"
	"testing"
)

func TestMemoryRepositoryLifecycle(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()

	first, err := repo.Create(ctx, "first", "web")
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	second, err := repo.Create(ctx, "second", "web")
	if err != nil {
		t.Fatalf("create second: %v", err)
	}

	items, err := repo.List(ctx, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 2 || items[0].ID != second.ID || items[1].ID != first.ID {
		t.Fatalf("unexpected list order: %#v", items)
	}

	if err := repo.Delete(ctx, first.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := repo.Delete(ctx, first.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestMemoryRepositoryRejectsEmptyContent(t *testing.T) {
	repo := NewMemoryRepository()
	if _, err := repo.Create(context.Background(), "  \n", "web"); !errors.Is(err, ErrEmptyContent) {
		t.Fatalf("expected empty content error, got %v", err)
	}
}
