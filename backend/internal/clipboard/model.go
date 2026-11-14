package clipboard

import (
	"context"
	"errors"
	"time"
)

var (
	ErrEmptyContent    = errors.New("clipboard content cannot be empty")
	ErrContentTooLarge = errors.New("clipboard content is too large")
	ErrInvalidFile     = errors.New("clipboard file metadata is invalid")
	ErrInvalidShare    = errors.New("clipboard share is invalid")
	ErrInvalidMetadata = errors.New("clipboard metadata is invalid")
	ErrNotFound        = errors.New("clipboard item not found")
)

const (
	MaxContentBytes = 1 << 20
	MaxNoteRunes    = 500
	MaxTags         = 10
	MaxTagRunes     = 24
)

type Kind string

const (
	KindText  Kind = "text"
	KindImage Kind = "image"
	KindFile  Kind = "file"
)

type Item struct {
	ID           int64      `json:"id"`
	UserID       int64      `json:"-"`
	Kind         Kind       `json:"kind"`
	Content      string     `json:"content,omitempty"`
	FileName     string     `json:"fileName,omitempty"`
	MediaType    string     `json:"mediaType,omitempty"`
	SizeBytes    int64      `json:"sizeBytes,omitempty"`
	StorageKey   string     `json:"-"`
	ContentHash  string     `json:"-"`
	Note         string     `json:"note,omitempty"`
	Source       string     `json:"source"`
	CreatedAt    time.Time  `json:"createdAt"`
	Tags         []string   `json:"tags,omitempty"`
	Favorite     bool       `json:"favorite,omitempty"`
	DeletedAt    *time.Time `json:"deletedAt,omitempty"`
	PurgePending bool       `json:"purgePending,omitempty"`
}

type Attachment struct {
	Kind        Kind
	FileName    string
	MediaType   string
	SizeBytes   int64
	StorageKey  string
	ContentHash string
	Source      string
}

type ListFilter struct {
	Limit         int
	Query         string
	Kind          Kind
	FavoriteOnly  bool
	CreatedFrom   *time.Time
	CreatedBefore *time.Time
	Cursor        *ListCursor
}

// ListCursor is an exclusive bound in the descending (created_at, id) order.
type ListCursor struct {
	CreatedAt time.Time
	ID        int64
}

type ItemMetadata struct {
	Content  *string
	Note     string
	Tags     []string
	Favorite bool
}

type ImportItem struct {
	Kind       Kind
	Content    string
	Source     string
	Attachment Attachment
	Metadata   ItemMetadata
	CreatedAt  time.Time
}

type Repository interface {
	// Revision is an opaque, per-user change token; absent users start at "0".
	Revision(ctx context.Context, userID int64) (string, error)
	CreateText(ctx context.Context, userID int64, content, source string) (Item, error)
	CreateAttachment(ctx context.Context, userID int64, attachment Attachment) (Item, error)
	Get(ctx context.Context, userID, id int64) (Item, error)
	List(ctx context.Context, userID int64, filter ListFilter) ([]Item, error)
	UpdateMetadata(ctx context.Context, userID, id int64, metadata ItemMetadata) (Item, error)
	Delete(ctx context.Context, userID, id int64) error
	Trash(ctx context.Context, userID, id int64) error
	ListTrash(ctx context.Context, userID int64, limit int) ([]Item, error)
	Restore(ctx context.Context, userID, id int64) (Item, error)
	// Purge records permanent deletion intent before source removal. A failed purge
	// remains pending and retryable, and cannot be restored with a missing source.
	Purge(ctx context.Context, userID, id int64, deleteSource func(context.Context, string) error) error
	CreateShare(ctx context.Context, userID int64, input CreateShareInput) (Share, error)
	ListShares(ctx context.Context, userID int64, limit int) ([]Share, error)
	GetPublicShare(ctx context.Context, token string) (Share, error)
	RevokeShare(ctx context.Context, userID, id int64) error
	FindDuplicate(ctx context.Context, userID int64, kind Kind, contentHash string) (Item, error)
	Import(ctx context.Context, userID int64, input ImportItem) (Item, error)
}
