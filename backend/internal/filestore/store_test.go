package filestore

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestMemoryStoreRoundTripAndLimit(t *testing.T) {
	store := NewMemoryStore()
	object, err := store.Put(context.Background(), 7, "photo.PNG", strings.NewReader("image"), 5)
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	if !strings.HasPrefix(object.Key, "7/") || !strings.HasSuffix(object.Key, ".png") || object.Size != 5 {
		t.Fatalf("unexpected object: %#v", object)
	}
	reader, err := store.Open(context.Background(), object.Key)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	content, _ := io.ReadAll(reader)
	_ = reader.Close()
	if string(content) != "image" {
		t.Fatalf("content = %q", content)
	}
	if _, err := store.Put(context.Background(), 7, ".txt", strings.NewReader("too large"), 3); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("expected too large, got %v", err)
	}
}

func TestLocalStoreRejectsEscapingKeys(t *testing.T) {
	store, err := NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	if _, err := store.Open(context.Background(), "../secret"); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("expected invalid key, got %v", err)
	}
}

func TestLocalStoreRoundTrip(t *testing.T) {
	store, err := NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	object, err := store.Put(t.Context(), 9, ".txt", strings.NewReader("stored"), 10)
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	reader, err := store.Open(t.Context(), object.Key)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	content, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil || string(content) != "stored" {
		t.Fatalf("content = %q, err = %v", content, err)
	}
	if err := store.Delete(t.Context(), object.Key); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := store.Open(t.Context(), object.Key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}
