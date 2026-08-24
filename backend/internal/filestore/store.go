package filestore

import (
	"context"
	"errors"
	"io"
)

var (
	ErrInvalidKey = errors.New("invalid storage key")
	ErrNotFound   = errors.New("stored file not found")
	ErrTooLarge   = errors.New("stored file is too large")
)

type Object struct {
	Key  string
	Size int64
}

type ReadSeekCloser interface {
	io.Reader
	io.Seeker
	io.Closer
}

type Store interface {
	Put(ctx context.Context, userID int64, extension string, source io.Reader, maxBytes int64) (Object, error)
	Open(ctx context.Context, key string) (ReadSeekCloser, error)
	Delete(ctx context.Context, key string) error
}
