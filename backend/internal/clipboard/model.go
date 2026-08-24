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
	ErrNotFound        = errors.New("clipboard item not found")
)

const MaxContentBytes = 1 << 20

type Kind string

const (
	KindText  Kind = "text"
	KindImage Kind = "image"
	KindFile  Kind = "file"
)

type Item struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"-"`
	Kind       Kind      `json:"kind"`
	Content    string    `json:"content,omitempty"`
	FileName   string    `json:"fileName,omitempty"`
	MediaType  string    `json:"mediaType,omitempty"`
	SizeBytes  int64     `json:"sizeBytes,omitempty"`
	StorageKey string    `json:"-"`
	Source     string    `json:"source"`
	CreatedAt  time.Time `json:"createdAt"`
}

type Attachment struct {
	Kind       Kind
	FileName   string
	MediaType  string
	SizeBytes  int64
	StorageKey string
	Source     string
}

type Repository interface {
	CreateText(ctx context.Context, userID int64, content, source string) (Item, error)
	CreateAttachment(ctx context.Context, userID int64, attachment Attachment) (Item, error)
	Get(ctx context.Context, userID, id int64) (Item, error)
	List(ctx context.Context, userID int64, limit int) ([]Item, error)
	Delete(ctx context.Context, userID, id int64) error
}
