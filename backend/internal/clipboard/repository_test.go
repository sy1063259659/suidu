package clipboard

import (
	"context"
	"errors"
	"testing"
	"time"
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

func TestNormalizeFileNameRemovesPathsAndControls(t *testing.T) {
	if got := normalizeFileName("../folder\\unsafe\r\nname.txt"); got != "unsafename.txt" {
		t.Fatalf("normalized file name = %q", got)
	}
}

func TestMemoryRepositoryShareLifecycle(t *testing.T) {
	now := time.Date(2026, 8, 24, 8, 0, 0, 0, time.UTC)
	repo := NewMemoryRepository()
	repo.now = func() time.Time { return now }
	item, err := repo.CreateText(t.Context(), 1, "public text", "web")
	if err != nil {
		t.Fatalf("create text: %v", err)
	}

	share, err := repo.CreateShare(t.Context(), 1, CreateShareInput{
		ItemID: item.ID, Token: "token", ExpiresAt: now.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("create share: %v", err)
	}
	if share.Item.ID != item.ID || share.Token != "token" || share.RevokedAt != nil {
		t.Fatalf("unexpected share: %#v", share)
	}

	shares, err := repo.ListShares(t.Context(), 1, 20)
	if err != nil || len(shares) != 1 || shares[0].ID != share.ID {
		t.Fatalf("list shares = %#v, %v", shares, err)
	}
	resolved, err := repo.GetPublicShare(t.Context(), "token")
	if err != nil || resolved.ID != share.ID {
		t.Fatalf("resolve share = %#v, %v", resolved, err)
	}

	if err := repo.RevokeShare(t.Context(), 1, share.ID); err != nil {
		t.Fatalf("revoke share: %v", err)
	}
	if _, err := repo.GetPublicShare(t.Context(), "token"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("revoked share should be hidden, got %v", err)
	}
	shares, err = repo.ListShares(t.Context(), 1, 20)
	if err != nil || shares[0].RevokedAt == nil {
		t.Fatalf("revoked share should remain manageable: %#v, %v", shares, err)
	}
}

func TestMemoryRepositoryShareOwnershipAndExpiry(t *testing.T) {
	now := time.Date(2026, 8, 24, 8, 0, 0, 0, time.UTC)
	repo := NewMemoryRepository()
	repo.now = func() time.Time { return now }
	item, err := repo.CreateText(t.Context(), 2, "private", "web")
	if err != nil {
		t.Fatalf("create text: %v", err)
	}
	if _, err := repo.CreateShare(t.Context(), 1, CreateShareInput{ItemID: item.ID, Token: "other", ExpiresAt: now.Add(time.Hour)}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other user's item should be hidden, got %v", err)
	}
	if _, err := repo.CreateShare(t.Context(), 2, CreateShareInput{ItemID: item.ID, Token: "short", ExpiresAt: now.Add(time.Minute)}); !errors.Is(err, ErrInvalidShare) {
		t.Fatalf("short expiry should be rejected, got %v", err)
	}
	if _, err := repo.CreateShare(t.Context(), 2, CreateShareInput{ItemID: item.ID, Token: "long", ExpiresAt: now.Add(366 * 24 * time.Hour)}); !errors.Is(err, ErrInvalidShare) {
		t.Fatalf("long expiry should be rejected, got %v", err)
	}

	share, err := repo.CreateShare(t.Context(), 2, CreateShareInput{ItemID: item.ID, Token: "expires", ExpiresAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatalf("create share: %v", err)
	}
	if shares, err := repo.ListShares(t.Context(), 1, 20); err != nil || len(shares) != 0 {
		t.Fatalf("other user's shares should be hidden: %#v, %v", shares, err)
	}
	if err := repo.RevokeShare(t.Context(), 1, share.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other user's revoke should be hidden, got %v", err)
	}
	now = now.Add(2 * time.Hour)
	if _, err := repo.GetPublicShare(t.Context(), share.Token); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired share should be hidden, got %v", err)
	}
	if shares, err := repo.ListShares(t.Context(), 2, 20); err != nil || len(shares) != 1 {
		t.Fatalf("expired share should remain manageable: %#v, %v", shares, err)
	}
}

func TestMemoryRepositoryDeletingItemInvalidatesShare(t *testing.T) {
	repo := NewMemoryRepository()
	item, err := repo.CreateText(t.Context(), 1, "temporary", "web")
	if err != nil {
		t.Fatalf("create text: %v", err)
	}
	share, err := repo.CreateShare(t.Context(), 1, CreateShareInput{
		ItemID: item.ID, Token: "deleted", ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("create share: %v", err)
	}
	if err := repo.Delete(t.Context(), 1, item.ID); err != nil {
		t.Fatalf("delete item: %v", err)
	}
	if _, err := repo.GetPublicShare(t.Context(), share.Token); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted item share should be hidden, got %v", err)
	}
}
