package clipboard

import (
	"context"
	"errors"
	"testing"
)

func TestMemoryRepositoryLifecycle(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()

	first, err := repo.CreateText(ctx, 1, "first", "web")
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	second, err := repo.CreateText(ctx, 1, "second", "web")
	if err != nil {
		t.Fatalf("create second: %v", err)
	}

	items, err := repo.List(ctx, 1, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 2 || items[0].ID != second.ID || items[1].ID != first.ID {
		t.Fatalf("unexpected list order: %#v", items)
	}

	if err := repo.Delete(ctx, 1, first.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := repo.Delete(ctx, 1, first.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestMemoryRepositoryRejectsEmptyContent(t *testing.T) {
	repo := NewMemoryRepository()
	if _, err := repo.CreateText(context.Background(), 1, "  \n", "web"); !errors.Is(err, ErrEmptyContent) {
		t.Fatalf("expected empty content error, got %v", err)
	}
}

func TestMemoryRepositoryAttachmentIsolation(t *testing.T) {
	repo := NewMemoryRepository()
	created, err := repo.CreateAttachment(context.Background(), 2, Attachment{
		Kind: KindImage, FileName: "photo.png", MediaType: "image/png", SizeBytes: 12, StorageKey: "2/photo.png", Source: "web",
	})
	if err != nil {
		t.Fatalf("create attachment: %v", err)
	}
	if created.Kind != KindImage || created.FileName != "photo.png" || created.StorageKey == "" {
		t.Fatalf("unexpected attachment: %#v", created)
	}
	if _, err := repo.Get(context.Background(), 1, created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-user get should be hidden, got %v", err)
	}
}
