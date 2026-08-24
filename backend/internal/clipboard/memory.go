package clipboard

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

type MemoryRepository struct {
	mu          sync.RWMutex
	nextID      int64
	nextShareID int64
	items       map[int64]Item
	shares      map[int64]Share
	now         func() time.Time
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		nextID: 1, nextShareID: 1, items: make(map[int64]Item), shares: make(map[int64]Share), now: time.Now,
	}
}

func (r *MemoryRepository) CreateText(_ context.Context, userID int64, content, source string) (Item, error) {
	if err := validateContent(content); err != nil {
		return Item{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	item := Item{
		ID:        r.nextID,
		UserID:    userID,
		Kind:      KindText,
		Content:   content,
		Source:    normalizeSource(source),
		CreatedAt: r.now().UTC(),
	}
	r.nextID++
	r.items[item.ID] = item
	return item, nil
}

func (r *MemoryRepository) CreateAttachment(_ context.Context, userID int64, attachment Attachment) (Item, error) {
	if err := validateAttachment(attachment); err != nil {
		return Item{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	item := Item{
		ID:         r.nextID,
		UserID:     userID,
		Kind:       attachment.Kind,
		FileName:   normalizeFileName(attachment.FileName),
		MediaType:  attachment.MediaType,
		SizeBytes:  attachment.SizeBytes,
		StorageKey: attachment.StorageKey,
		Source:     normalizeSource(attachment.Source),
		CreatedAt:  r.now().UTC(),
	}
	r.nextID++
	r.items[item.ID] = item
	return item, nil
}

func (r *MemoryRepository) Get(_ context.Context, userID, id int64) (Item, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.items[id]
	if !ok || item.UserID != userID {
		return Item{}, ErrNotFound
	}
	return item, nil
}

func (r *MemoryRepository) List(_ context.Context, userID int64, filter ListFilter) ([]Item, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query := strings.ToLower(strings.TrimSpace(filter.Query))
	items := make([]Item, 0, len(r.items))
	for _, item := range r.items {
		if item.UserID != userID {
			continue
		}
		if filter.Kind != "" && item.Kind != filter.Kind {
			continue
		}
		if filter.FavoriteOnly && !item.Favorite {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(item.Content), query) && !strings.Contains(strings.ToLower(item.FileName), query) && !strings.Contains(strings.ToLower(strings.Join(item.Tags, " ")), query) {
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
	if filter.Limit > 0 && len(items) > filter.Limit {
		items = items[:filter.Limit]
	}
	return items, nil
}

func (r *MemoryRepository) UpdateMetadata(_ context.Context, userID, id int64, metadata ItemMetadata) (Item, error) {
	normalized, err := normalizeMetadata(metadata)
	if err != nil {
		return Item{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.items[id]
	if !ok || item.UserID != userID {
		return Item{}, ErrNotFound
	}
	item.Tags = normalized.Tags
	item.Favorite = normalized.Favorite
	r.items[id] = item
	for shareID, share := range r.shares {
		if share.Item.ID == id {
			share.Item = item
			r.shares[shareID] = share
		}
	}
	return item, nil
}

func (r *MemoryRepository) Delete(_ context.Context, userID, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	item, ok := r.items[id]
	if !ok || item.UserID != userID {
		return ErrNotFound
	}
	delete(r.items, id)
	for shareID, share := range r.shares {
		if share.Item.ID == id {
			delete(r.shares, shareID)
		}
	}
	return nil
}

func (r *MemoryRepository) CreateShare(_ context.Context, userID int64, input CreateShareInput) (Share, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now().UTC()
	if err := validateShareInput(input, now); err != nil {
		return Share{}, err
	}
	item, ok := r.items[input.ItemID]
	if !ok || item.UserID != userID {
		return Share{}, ErrNotFound
	}
	for _, existing := range r.shares {
		if existing.Token == input.Token {
			return Share{}, ErrInvalidShare
		}
	}
	share := Share{
		ID: r.nextShareID, UserID: userID, Token: input.Token, Item: item,
		ExpiresAt: input.ExpiresAt.UTC(), CreatedAt: now,
	}
	r.nextShareID++
	r.shares[share.ID] = share
	return share, nil
}

func (r *MemoryRepository) ListShares(_ context.Context, userID int64, limit int) ([]Share, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	shares := make([]Share, 0)
	for _, share := range r.shares {
		if share.UserID == userID {
			shares = append(shares, share)
		}
	}
	sort.Slice(shares, func(i, j int) bool {
		if shares[i].CreatedAt.Equal(shares[j].CreatedAt) {
			return shares[i].ID > shares[j].ID
		}
		return shares[i].CreatedAt.After(shares[j].CreatedAt)
	})
	if limit > 0 && len(shares) > limit {
		shares = shares[:limit]
	}
	return shares, nil
}

func (r *MemoryRepository) GetPublicShare(_ context.Context, token string) (Share, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	now := r.now().UTC()
	for _, share := range r.shares {
		if share.Token != token || share.RevokedAt != nil || !share.ExpiresAt.After(now) {
			continue
		}
		if _, ok := r.items[share.Item.ID]; !ok {
			return Share{}, ErrNotFound
		}
		return share, nil
	}
	return Share{}, ErrNotFound
}

func (r *MemoryRepository) RevokeShare(_ context.Context, userID, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	share, ok := r.shares[id]
	if !ok || share.UserID != userID || share.RevokedAt != nil {
		return ErrNotFound
	}
	revokedAt := r.now().UTC()
	share.RevokedAt = &revokedAt
	r.shares[id] = share
	return nil
}
