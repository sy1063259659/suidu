package filestore

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type LocalStore struct {
	root string
}

func NewLocalStore(root string) (*LocalStore, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(absoluteRoot, 0o750); err != nil {
		return nil, err
	}
	return &LocalStore{root: absoluteRoot}, nil
}

func (s *LocalStore) Put(ctx context.Context, userID int64, extension string, source io.Reader, maxBytes int64) (Object, error) {
	key, err := newKey(userID, extension)
	if err != nil {
		return Object{}, err
	}
	destination, err := s.path(key)
	if err != nil {
		return Object{}, err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o750); err != nil {
		return Object{}, err
	}
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return Object{}, err
	}
	limited := &io.LimitedReader{R: source, N: maxBytes + 1}
	written, copyErr := copyWithContext(ctx, file, limited)
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil || written > maxBytes {
		_ = os.Remove(destination)
		if written > maxBytes {
			return Object{}, ErrTooLarge
		}
		if copyErr != nil {
			return Object{}, copyErr
		}
		return Object{}, closeErr
	}
	return Object{Key: key, Size: written}, nil
}

func (s *LocalStore) Open(_ context.Context, key string) (ReadSeekCloser, error) {
	path, err := s.path(key)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	return file, err
}

func (s *LocalStore) Delete(_ context.Context, key string) error {
	path, err := s.path(key)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return ErrNotFound
	}
	return err
}

func (s *LocalStore) path(key string) (string, error) {
	if key == "" || filepath.IsAbs(key) || strings.ContainsRune(key, '\x00') {
		return "", ErrInvalidKey
	}
	cleanKey := filepath.Clean(filepath.FromSlash(key))
	if cleanKey == "." || cleanKey == ".." || strings.HasPrefix(cleanKey, ".."+string(filepath.Separator)) {
		return "", ErrInvalidKey
	}
	path := filepath.Join(s.root, cleanKey)
	if !strings.HasPrefix(path, s.root+string(filepath.Separator)) {
		return "", ErrInvalidKey
	}
	return path, nil
}

func copyWithContext(ctx context.Context, destination io.Writer, source io.Reader) (int64, error) {
	buffer := make([]byte, 32*1024)
	var written int64
	for {
		if err := ctx.Err(); err != nil {
			return written, err
		}
		read, readErr := source.Read(buffer)
		if read > 0 {
			count, writeErr := destination.Write(buffer[:read])
			written += int64(count)
			if writeErr != nil {
				return written, writeErr
			}
			if count != read {
				return written, io.ErrShortWrite
			}
		}
		if errors.Is(readErr, io.EOF) {
			return written, nil
		}
		if readErr != nil {
			return written, readErr
		}
	}
}
