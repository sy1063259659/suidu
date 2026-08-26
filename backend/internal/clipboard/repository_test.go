package clipboard

import (
	"context"
	"errors"
	"strings"
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

	items, err := repo.List(ctx, 1, ListFilter{Limit: 10})
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

func TestMemoryRepositoryListFilters(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := t.Context()

	first, err := repo.CreateText(ctx, 1, "Project Alpha token", "web")
	if err != nil {
		t.Fatalf("create first text: %v", err)
	}
	if _, err := repo.CreateText(ctx, 1, "unrelated note", "web"); err != nil {
		t.Fatalf("create unrelated text: %v", err)
	}
	image, err := repo.CreateAttachment(ctx, 1, Attachment{
		Kind: KindImage, FileName: "ALPHA-diagram.png", MediaType: "image/png", SizeBytes: 12, StorageKey: "1/alpha.png", Source: "web",
	})
	if err != nil {
		t.Fatalf("create image: %v", err)
	}
	if _, err := repo.CreateText(ctx, 2, "alpha from another user", "web"); err != nil {
		t.Fatalf("create other user's text: %v", err)
	}
	literal, err := repo.CreateText(ctx, 1, "100% ready", "web")
	if err != nil {
		t.Fatalf("create literal wildcard text: %v", err)
	}

	items, err := repo.List(ctx, 1, ListFilter{Limit: 10, Query: "  aLpHa  "})
	if err != nil {
		t.Fatalf("search list: %v", err)
	}
	if len(items) != 2 || items[0].ID != image.ID || items[1].ID != first.ID {
		t.Fatalf("search results = %#v", items)
	}

	items, err = repo.List(ctx, 1, ListFilter{Limit: 10, Query: "alpha", Kind: KindText})
	if err != nil {
		t.Fatalf("filtered list: %v", err)
	}
	if len(items) != 1 || items[0].ID != first.ID {
		t.Fatalf("text results = %#v", items)
	}

	items, err = repo.List(ctx, 1, ListFilter{Limit: 1, Query: "alpha"})
	if err != nil {
		t.Fatalf("limited list: %v", err)
	}
	if len(items) != 1 || items[0].ID != image.ID {
		t.Fatalf("limited results = %#v", items)
	}

	items, err = repo.List(ctx, 1, ListFilter{Limit: 10, Query: "%"})
	if err != nil {
		t.Fatalf("literal search list: %v", err)
	}
	if len(items) != 1 || items[0].ID != literal.ID {
		t.Fatalf("literal search results = %#v", items)
	}
}

func TestMemoryRepositoryListFiltersByCreatedAt(t *testing.T) {
	base := time.Date(2026, 8, 24, 23, 59, 0, 0, time.UTC)
	times := []time.Time{
		base,
		base.Add(time.Minute),
		base.Add(2 * time.Minute),
		base.Add(3 * time.Minute),
	}
	repo := NewMemoryRepository()
	next := 0
	repo.now = func() time.Time {
		current := times[next]
		next++
		return current
	}

	ctx := t.Context()
	previousDay, err := repo.CreateText(ctx, 1, "previous day", "web")
	if err != nil {
		t.Fatalf("create previous day item: %v", err)
	}
	fromBoundary, err := repo.CreateText(ctx, 1, "from boundary", "web")
	if err != nil {
		t.Fatalf("create from boundary item: %v", err)
	}
	toBoundary, err := repo.CreateText(ctx, 1, "to boundary", "web")
	if err != nil {
		t.Fatalf("create to boundary item: %v", err)
	}
	if _, err := repo.CreateText(ctx, 2, "other user boundary", "web"); err != nil {
		t.Fatalf("create other user item: %v", err)
	}

	items, err := repo.List(ctx, 1, ListFilter{
		Limit:         10,
		CreatedFrom:   timePointer(times[1]),
		CreatedBefore: timePointer(times[2]),
	})
	if err != nil {
		t.Fatalf("list by created range: %v", err)
	}
	if len(items) != 1 || items[0].ID != fromBoundary.ID {
		t.Fatalf("range results = %#v", items)
	}

	items, err = repo.List(ctx, 1, ListFilter{
		Limit:         10,
		CreatedFrom:   timePointer(times[0]),
		CreatedBefore: timePointer(times[2]),
	})
	if err != nil {
		t.Fatalf("list cross-day range: %v", err)
	}
	if len(items) != 2 || items[0].ID != fromBoundary.ID || items[1].ID != previousDay.ID {
		t.Fatalf("cross-day results = %#v", items)
	}
	if items[0].CreatedAt.Day() == items[1].CreatedAt.Day() {
		t.Fatalf("expected cross-day ordering, got %#v", items)
	}
	if items[0].ID == toBoundary.ID {
		t.Fatalf("to boundary item should be excluded: %#v", items)
	}
}

func TestMemoryRepositoryMetadata(t *testing.T) {
	repo := NewMemoryRepository()
	item, err := repo.CreateText(t.Context(), 1, "release checklist", "web")
	if err != nil {
		t.Fatalf("create item: %v", err)
	}

	updated, err := repo.UpdateMetadata(t.Context(), 1, item.ID, ItemMetadata{
		Note: "  上线前确认接口和数据库\r\n完成后通知团队。  ",
		Tags: []string{" work ", "Work", "发布", ""}, Favorite: true,
	})
	if err != nil {
		t.Fatalf("update metadata: %v", err)
	}
	if updated.Note != "上线前确认接口和数据库\n完成后通知团队。" || !updated.Favorite || len(updated.Tags) != 2 || updated.Tags[0] != "work" || updated.Tags[1] != "发布" {
		t.Fatalf("updated metadata = %#v", updated)
	}

	items, err := repo.List(t.Context(), 1, ListFilter{Limit: 10, Query: "发布", FavoriteOnly: true})
	if err != nil {
		t.Fatalf("search favorite tags: %v", err)
	}
	if len(items) != 1 || items[0].ID != item.ID {
		t.Fatalf("favorite tag results = %#v", items)
	}
	items, err = repo.List(t.Context(), 1, ListFilter{Limit: 10, Query: "通知团队"})
	if err != nil || len(items) != 1 || items[0].ID != item.ID {
		t.Fatalf("note search results = %#v, err = %v", items, err)
	}
	if _, err := repo.UpdateMetadata(t.Context(), 2, item.ID, ItemMetadata{Favorite: true}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other user's update should be hidden, got %v", err)
	}
	if _, err := repo.UpdateMetadata(t.Context(), 1, item.ID, ItemMetadata{Tags: []string{strings.Repeat("x", MaxTagRunes+1)}}); !errors.Is(err, ErrInvalidMetadata) {
		t.Fatalf("long tag should be rejected, got %v", err)
	}
	if _, err := repo.UpdateMetadata(t.Context(), 1, item.ID, ItemMetadata{Note: strings.Repeat("说", MaxNoteRunes+1)}); !errors.Is(err, ErrInvalidMetadata) {
		t.Fatalf("long note should be rejected, got %v", err)
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

func timePointer(value time.Time) *time.Time {
	return &value
}
