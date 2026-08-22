package clipboard

import (
	"context"
	"errors"
	"time"
)

var (
	ErrEmptyContent    = errors.New("clipboard content cannot be empty")
	ErrContentTooLarge = errors.New("clipboard content is too large")
	ErrNotFound        = errors.New("clipboard item not found")
)

const MaxContentBytes = 1 << 20

type Item struct {
	ID        int64     `json:"id"`
	Content   string    `json:"content"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"createdAt"`
}

type Repository interface {
	Create(ctx context.Context, content, source string) (Item, error)
	List(ctx context.Context, limit int) ([]Item, error)
	Delete(ctx context.Context, id int64) error
}
