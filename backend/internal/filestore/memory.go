package filestore

import (
	"bytes"
	"context"
	"io"
	"sync"
)

type MemoryStore struct {
	mu      sync.RWMutex
	objects map[string][]byte
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{objects: make(map[string][]byte)}
}

func (s *MemoryStore) Put(_ context.Context, userID int64, extension string, source io.Reader, maxBytes int64) (Object, error) {
	key, err := newKey(userID, extension)
	if err != nil {
		return Object{}, err
	}
	content, err := readBounded(source, maxBytes)
	if err != nil {
		return Object{}, err
	}
	s.mu.Lock()
	s.objects[key] = content
	s.mu.Unlock()
	return Object{Key: key, Size: int64(len(content))}, nil
}

func (s *MemoryStore) Open(_ context.Context, key string) (ReadSeekCloser, error) {
	s.mu.RLock()
	content, ok := s.objects[key]
	s.mu.RUnlock()
	if !ok {
		return nil, ErrNotFound
	}
	copyOfContent := append([]byte(nil), content...)
	return &memoryReader{Reader: bytes.NewReader(copyOfContent)}, nil
}

func (s *MemoryStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.objects[key]; !ok {
		return ErrNotFound
	}
	delete(s.objects, key)
	return nil
}

type memoryReader struct {
	*bytes.Reader
}

func (r *memoryReader) Close() error { return nil }

func readBounded(source io.Reader, maxBytes int64) ([]byte, error) {
	if maxBytes < 1 {
		return nil, ErrTooLarge
	}
	content, err := io.ReadAll(io.LimitReader(source, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > maxBytes {
		return nil, ErrTooLarge
	}
	return content, nil
}
