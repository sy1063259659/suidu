package clipboard

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sy1063259659/suidu/backend/internal/filestore"
)

func TestMemoryTrashHidesItemAndRestorePreservesMetadata(t *testing.T) {
	repo := NewMemoryRepository()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	repo.now = func() time.Time { return now }
	ctx := t.Context()
	item, err := repo.CreateText(ctx, 1, "keep this", "desktop")
	if err != nil {
		t.Fatal(err)
	}
	item, err = repo.UpdateMetadata(ctx, 1, item.ID, ItemMetadata{Note: "note", Tags: []string{"tag"}, Favorite: true})
	if err != nil {
		t.Fatal(err)
	}
	share, err := repo.CreateShare(ctx, 1, CreateShareInput{ItemID: item.ID, Token: "original", ExpiresAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Trash(ctx, 2, item.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other user trash: %v", err)
	}
	now = now.Add(time.Minute)
	if err := repo.Trash(ctx, 1, item.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.Trash(ctx, 1, item.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("repeat trash: %v", err)
	}
	if _, err := repo.Get(ctx, 1, item.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get trashed: %v", err)
	}
	if _, err := repo.UpdateMetadata(ctx, 1, item.ID, ItemMetadata{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("update trashed: %v", err)
	}
	if _, err := repo.FindDuplicate(ctx, 1, item.Kind, item.ContentHash); !errors.Is(err, ErrNotFound) {
		t.Fatalf("find duplicate trashed: %v", err)
	}
	if _, err := repo.CreateShare(ctx, 1, CreateShareInput{ItemID: item.ID, Token: "new", ExpiresAt: now.Add(time.Hour)}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("share trashed: %v", err)
	}
	if _, err := repo.GetPublicShare(ctx, share.Token); !errors.Is(err, ErrNotFound) {
		t.Fatalf("public trashed share: %v", err)
	}
	if items, err := repo.List(ctx, 1, ListFilter{}); err != nil || len(items) != 0 {
		t.Fatalf("list: %#v, %v", items, err)
	}
	if shares, err := repo.ListShares(ctx, 1, 100); err != nil || len(shares) != 0 {
		t.Fatalf("shares list: %#v, %v", shares, err)
	}
	if items, err := repo.ListTrash(ctx, 2, 100); err != nil || len(items) != 0 {
		t.Fatalf("other user trash list: %#v, %v", items, err)
	}
	if _, err := repo.Restore(ctx, 2, item.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other user restore: %v", err)
	}
	// No time-based cleanup, including after a year.
	now = now.Add(366 * 24 * time.Hour)
	items, err := repo.ListTrash(ctx, 1, 100)
	if err != nil || len(items) != 1 || items[0].DeletedAt == nil {
		t.Fatalf("trash retained: %#v, %v", items, err)
	}
	restored, err := repo.Restore(ctx, 1, item.ID)
	if err != nil || !reflect.DeepEqual(restored, item) {
		t.Fatalf("restore = %#v, %v; want %#v", restored, err, item)
	}
	// Move the clock back within the share's TTL to prove permanent revocation.
	now = item.CreatedAt.Add(2 * time.Minute)
	if _, err := repo.GetPublicShare(ctx, share.Token); !errors.Is(err, ErrNotFound) {
		t.Fatalf("restore must not reactivate public share: %v", err)
	}
	if _, err := repo.Restore(ctx, 1, item.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("restore active item: %v", err)
	}
	if items, err := repo.List(ctx, 1, ListFilter{}); err != nil || len(items) != 1 {
		t.Fatalf("restoration must not create extra records: %#v, %v", items, err)
	}
}

func TestMemoryTrashOrderingAndPurgeFailure(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := t.Context()
	first, _ := repo.CreateAttachment(ctx, 1, Attachment{Kind: KindFile, FileName: "file", StorageKey: "1/source"})
	second, _ := repo.CreateText(ctx, 1, "second", "web")
	callback := func(context.Context, string) error { t.Fatal("source deletion for active or foreign item"); return nil }
	if err := repo.Purge(ctx, 1, first.ID, callback); !errors.Is(err, ErrNotFound) {
		t.Fatalf("purge active: %v", err)
	}
	_ = repo.Trash(ctx, 1, first.ID)
	_ = repo.Trash(ctx, 1, second.ID)
	items, err := repo.ListTrash(ctx, 1, 1)
	if err != nil || len(items) != 1 || items[0].ID != second.ID {
		t.Fatalf("trash order and limit: %#v, %v", items, err)
	}
	if err := repo.Purge(ctx, 2, first.ID, callback); !errors.Is(err, ErrNotFound) {
		t.Fatalf("purge foreign: %v", err)
	}
	storageErr := errors.New("storage unavailable")
	if err := repo.Purge(ctx, 1, first.ID, func(_ context.Context, key string) error {
		if key != first.StorageKey {
			t.Fatalf("source key = %q", key)
		}
		return storageErr
	}); !errors.Is(err, storageErr) {
		t.Fatalf("purge source failure: %v", err)
	}
	if items, _ := repo.ListTrash(ctx, 1, 0); len(items) != 2 {
		t.Fatalf("source failure lost record: %#v", items)
	}
	items, _ = repo.ListTrash(ctx, 1, 0)
	if !items[1].PurgePending {
		t.Fatalf("failed source removal must retain permanent deletion intent: %#v", items)
	}
	if _, err := repo.Restore(ctx, 1, first.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("pending purge must block restoration: %v", err)
	}
	if err := repo.Purge(ctx, 1, first.ID, func(context.Context, string) error { return nil }); err != nil {
		t.Fatalf("retry purge: %v", err)
	}
	if _, err := repo.Restore(ctx, 1, first.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("purged item restored: %v", err)
	}
}

func TestMemoryPurgeSerializesRestore(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := t.Context()
	item, _ := repo.CreateAttachment(ctx, 1, Attachment{Kind: KindFile, FileName: "file", StorageKey: "1/source"})
	_ = repo.Trash(ctx, 1, item.ID)
	deleting := make(chan struct{})
	release := make(chan struct{})
	purged := make(chan error, 1)
	go func() {
		purged <- repo.Purge(ctx, 1, item.ID, func(context.Context, string) error {
			close(deleting)
			<-release
			return nil
		})
	}()
	<-deleting
	restored := make(chan error, 1)
	go func() { _, err := repo.Restore(ctx, 1, item.ID); restored <- err }()
	select {
	case err := <-restored:
		close(release)
		t.Fatalf("restore returned during source deletion: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	if err := <-purged; err != nil {
		t.Fatalf("purge: %v", err)
	}
	if err := <-restored; !errors.Is(err, ErrNotFound) {
		t.Fatalf("restore after purge: %v", err)
	}
}

func TestHandlerTrashRestoreAndPermanentDelete(t *testing.T) {
	repo := NewMemoryRepository()
	files := filestore.NewMemoryStore()
	ctx := t.Context()
	object, _ := files.Put(ctx, 1, ".txt", strings.NewReader("source"), 100)
	item, _ := repo.CreateAttachment(ctx, 1, Attachment{Kind: KindFile, FileName: "saved.txt", StorageKey: object.Key, SizeBytes: object.Size})
	token := strings.Repeat("a", 43)
	_, err := repo.CreateShare(ctx, 1, CreateShareInput{ItemID: item.ID, Token: token, ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	router := newShareTestRouter(repo, files, 1)
	id := strconv.FormatInt(item.ID, 10)
	request := func(method, path string, want int) *httptest.ResponseRecorder {
		t.Helper()
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(method, path, nil))
		if response.Code != want {
			t.Fatalf("%s %s = %d, body %s; want %d", method, path, response.Code, response.Body.String(), want)
		}
		return response
	}
	request(http.MethodDelete, "/api/trash/"+id, http.StatusNotFound)
	request(http.MethodDelete, "/api/clipboard/"+id, http.StatusNoContent)
	request(http.MethodGet, "/api/clipboard/"+id, http.StatusNotFound)
	request(http.MethodGet, "/api/clipboard/"+id+"/content", http.StatusNotFound)
	request(http.MethodGet, "/api/public/shares/"+token, http.StatusNotFound)
	response := request(http.MethodGet, "/api/trash", http.StatusOK)
	var trash struct {
		Items []Item `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &trash); err != nil || len(trash.Items) != 1 || trash.Items[0].DeletedAt == nil {
		t.Fatalf("trash response: %s, %v", response.Body.String(), err)
	}
	backup := request(http.MethodGet, "/api/backup/export", http.StatusOK)
	archive, err := zip.NewReader(bytes.NewReader(backup.Body.Bytes()), int64(backup.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	manifest, _, err := readBackupArchive(archive)
	if err != nil || len(manifest.Items) != 0 {
		t.Fatalf("backup includes trash: %#v, %v", manifest.Items, err)
	}
	response = request(http.MethodPost, "/api/clipboard/"+id+"/restore", http.StatusOK)
	var restored Item
	if err := json.Unmarshal(response.Body.Bytes(), &restored); err != nil || restored.ID != item.ID || restored.DeletedAt != nil {
		t.Fatalf("restore response: %s, %v", response.Body.String(), err)
	}
	request(http.MethodGet, "/api/clipboard/"+id+"/content", http.StatusOK)
	request(http.MethodGet, "/api/public/shares/"+token, http.StatusNotFound)
	request(http.MethodDelete, "/api/clipboard/"+id, http.StatusNoContent)
	request(http.MethodDelete, "/api/trash/"+id, http.StatusNoContent)
	request(http.MethodPost, "/api/clipboard/"+id+"/restore", http.StatusNotFound)
	if _, err := files.Open(ctx, object.Key); !errors.Is(err, filestore.ErrNotFound) {
		t.Fatalf("purge source: %v", err)
	}
}

func TestHandlerTrashOwnershipAndInvalidIDs(t *testing.T) {
	repo := NewMemoryRepository()
	item, _ := repo.CreateText(t.Context(), 2, "private", "web")
	_ = repo.Trash(t.Context(), 2, item.ID)
	router := newTestRouter(repo)
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{http.MethodGet, "/api/trash?limit=no", http.StatusBadRequest},
		{http.MethodGet, "/api/trash?limit=0", http.StatusBadRequest},
		{http.MethodPost, "/api/clipboard/no/restore", http.StatusBadRequest},
		{http.MethodDelete, "/api/trash/-1", http.StatusBadRequest},
		{http.MethodPost, "/api/clipboard/1/restore", http.StatusNotFound},
		{http.MethodDelete, "/api/trash/1", http.StatusNotFound},
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(tc.method, tc.path, nil))
		if response.Code != tc.status {
			t.Errorf("%s %s = %d; want %d", tc.method, tc.path, response.Code, tc.status)
		}
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/trash", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"items":[]`) {
		t.Fatalf("foreign trash leaked: %s", response.Body.String())
	}
}

type trashCaptureRepo struct {
	Repository
	limit  int
	userID int64
}

func (r *trashCaptureRepo) ListTrash(_ context.Context, userID int64, limit int) ([]Item, error) {
	r.limit, r.userID = limit, userID
	return []Item{}, nil
}

func TestHandlerTrashLimit(t *testing.T) {
	for _, tc := range []struct {
		query string
		want  int
	}{{"", 100}, {"?limit=1", 1}, {"?limit=200", 200}, {"?limit=99999", 200}} {
		repo := &trashCaptureRepo{}
		router := newTestRouterWithStore(repo, filestore.NewMemoryStore(), 42, MaxFileBytes)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/trash"+tc.query, nil))
		if response.Code != http.StatusOK || repo.limit != tc.want || repo.userID != 42 {
			t.Errorf("query %q: status %d, limit %d, user %d", tc.query, response.Code, repo.limit, repo.userID)
		}
	}
}

func TestHandlerTrashRequiresAuthentication(t *testing.T) {
	router := gin.New()
	NewHandler(NewMemoryRepository(), filestore.NewMemoryStore(), MaxFileBytes).RegisterRoutes(router.Group("/api"))
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/trash"},
		{http.MethodPost, "/api/clipboard/1/restore"},
		{http.MethodDelete, "/api/trash/1"},
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(tc.method, tc.path, nil))
		if response.Code != http.StatusUnauthorized {
			t.Errorf("%s %s status = %d", tc.method, tc.path, response.Code)
		}
	}
}

func TestHandlerFailedPurgeRemainsPendingAndRetryable(t *testing.T) {
	repo := NewMemoryRepository()
	files := filestore.NewMemoryStore()
	ctx := t.Context()
	object, _ := files.Put(ctx, 1, ".txt", strings.NewReader("source"), 100)
	item, _ := repo.CreateAttachment(ctx, 1, Attachment{Kind: KindFile, FileName: "saved.txt", StorageKey: object.Key, SizeBytes: object.Size})
	_ = repo.Trash(ctx, 1, item.ID)
	path := "/api/trash/" + strconv.FormatInt(item.ID, 10)
	failingRouter := newTestRouterWithStore(repo, deleteFailStore{Store: files, err: errors.New("storage unavailable")}, 1, MaxFileBytes)
	response := httptest.NewRecorder()
	failingRouter.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, path, nil))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("failed purge status: %d", response.Code)
	}
	response = httptest.NewRecorder()
	failingRouter.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/trash", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"purgePending":true`) {
		t.Fatalf("pending purge must be visible to UI: %s", response.Body.String())
	}
	response = httptest.NewRecorder()
	failingRouter.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/clipboard/"+strconv.FormatInt(item.ID, 10)+"/restore", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("restore pending purge: %d", response.Code)
	}
	// Simulate source deletion succeeding before a database failure. The pending
	// item must still reject restoration, and retry tolerates the missing source.
	if err := files.Delete(ctx, object.Key); err != nil {
		t.Fatal(err)
	}
	router := newTestRouterWithStore(repo, files, 1, MaxFileBytes)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, path, nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("retry after source disappeared: %d, %s", response.Code, response.Body.String())
	}
}
