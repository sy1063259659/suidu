package clipboard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func readRevision(t *testing.T, repo Repository, userID int64) uint64 {
	t.Helper()
	version, err := repo.Revision(t.Context(), userID)
	if err != nil {
		t.Fatal(err)
	}
	value, err := strconv.ParseUint(version, 10, 64)
	if err != nil {
		t.Fatalf("revision %q: %v", version, err)
	}
	return value
}

func TestMemoryRevisionTracksMutationsAndIsolatesUsers(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := t.Context()
	if got := readRevision(t, repo, 1); got != 0 {
		t.Fatalf("initial revision = %d", got)
	}
	var item Item
	var share Share
	for _, step := range []struct {
		name string
		run  func() error
	}{
		{"create text", func() (err error) { item, err = repo.CreateText(ctx, 1, "original", "web"); return }},
		{"edit content", func() error {
			content := "edited"
			_, err := repo.UpdateMetadata(ctx, 1, item.ID, ItemMetadata{Content: &content})
			return err
		}},
		{"metadata and favorite", func() error {
			_, err := repo.UpdateMetadata(ctx, 1, item.ID, ItemMetadata{Note: "note", Tags: []string{"tag"}, Favorite: true})
			return err
		}},
		{"share", func() (err error) {
			share, err = repo.CreateShare(ctx, 1, CreateShareInput{ItemID: item.ID, Token: "share", ExpiresAt: time.Now().Add(time.Hour)})
			return
		}},
		{"revoke", func() error { return repo.RevokeShare(ctx, 1, share.ID) }},
		{"trash", func() error { return repo.Trash(ctx, 1, item.ID) }},
		{"restore", func() error { _, err := repo.Restore(ctx, 1, item.ID); return err }},
		{"hard delete", func() error { return repo.Delete(ctx, 1, item.ID) }},
		{"create attachment", func() (err error) {
			item, err = repo.CreateAttachment(ctx, 1, Attachment{Kind: KindFile, FileName: "file", StorageKey: "file"})
			return
		}},
		{"trash attachment", func() error { return repo.Trash(ctx, 1, item.ID) }},
		{"purge attachment", func() error {
			return repo.Purge(ctx, 1, item.ID, func(context.Context, string) error { return nil })
		}},
		{"import text", func() error {
			_, err := repo.Import(ctx, 1, ImportItem{Kind: KindText, Content: "imported", Metadata: ItemMetadata{Favorite: true}})
			return err
		}},
		{"import attachment", func() error {
			_, err := repo.Import(ctx, 1, ImportItem{Kind: KindImage, Attachment: Attachment{Kind: KindImage, FileName: "image", StorageKey: "image"}})
			return err
		}},
	} {
		t.Run(step.name, func(t *testing.T) {
			before := readRevision(t, repo, 1)
			if err := step.run(); err != nil {
				t.Fatal(err)
			}
			if got := readRevision(t, repo, 1); got <= before {
				t.Fatalf("revision did not advance: %d -> %d", before, got)
			}
			if got := readRevision(t, repo, 2); got != 0 {
				t.Fatalf("another user's revision advanced: %d", got)
			}
		})
	}
	before := readRevision(t, repo, 1)
	_, _ = repo.List(ctx, 1, ListFilter{})
	_, _ = repo.ListTrash(ctx, 1, 100)
	_, _ = repo.ListShares(ctx, 1, 100)
	if got := readRevision(t, repo, 1); got != before {
		t.Fatalf("reads advanced revision: %d -> %d", before, got)
	}
}

func TestMemoryRevisionRejectsInvalidAndForeignMutations(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := t.Context()
	item, _ := repo.CreateText(ctx, 1, "owned", "web")
	share, _ := repo.CreateShare(ctx, 1, CreateShareInput{ItemID: item.ID, Token: "owned", ExpiresAt: time.Now().Add(time.Hour)})
	before := readRevision(t, repo, 1)
	for _, step := range []struct {
		name string
		run  func() error
	}{
		{"invalid text", func() error { _, err := repo.CreateText(ctx, 1, " ", "web"); return err }},
		{"invalid file", func() error { _, err := repo.CreateAttachment(ctx, 1, Attachment{}); return err }},
		{"invalid metadata", func() error {
			_, err := repo.UpdateMetadata(ctx, 1, item.ID, ItemMetadata{Note: strings.Repeat("a", MaxNoteRunes+1)})
			return err
		}},
		{"invalid import", func() error { _, err := repo.Import(ctx, 1, ImportItem{Kind: KindText}); return err }},
		{"invalid share", func() error { _, err := repo.CreateShare(ctx, 1, CreateShareInput{}); return err }},
		{"foreign metadata", func() error { _, err := repo.UpdateMetadata(ctx, 2, item.ID, ItemMetadata{}); return err }},
		{"foreign delete", func() error { return repo.Delete(ctx, 2, item.ID) }},
		{"foreign trash", func() error { return repo.Trash(ctx, 2, item.ID) }},
		{"foreign restore", func() error { _, err := repo.Restore(ctx, 2, item.ID); return err }},
		{"foreign share", func() error {
			_, err := repo.CreateShare(ctx, 2, CreateShareInput{ItemID: item.ID, Token: "foreign", ExpiresAt: time.Now().Add(time.Hour)})
			return err
		}},
		{"foreign revoke", func() error { return repo.RevokeShare(ctx, 2, share.ID) }},
		{"foreign purge", func() error {
			return repo.Purge(ctx, 2, item.ID, func(context.Context, string) error { t.Fatal("foreign source deletion"); return nil })
		}},
	} {
		t.Run(step.name, func(t *testing.T) {
			if err := step.run(); err == nil {
				t.Fatal("invalid mutation succeeded")
			}
			if got := readRevision(t, repo, 1); got != before {
				t.Fatalf("failed mutation advanced owner revision: %d -> %d", before, got)
			}
			if got := readRevision(t, repo, 2); got != 0 {
				t.Fatalf("failed mutation advanced caller revision: %d", got)
			}
		})
	}
}

func TestMemoryRevisionReportsPartialPurgeAndRetry(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := t.Context()
	item, _ := repo.CreateAttachment(ctx, 1, Attachment{Kind: KindFile, FileName: "file", StorageKey: "file"})
	_ = repo.Trash(ctx, 1, item.ID)
	before := readRevision(t, repo, 1)
	storageErr := errors.New("storage unavailable")
	fail := func(context.Context, string) error { return storageErr }
	if err := repo.Purge(ctx, 1, item.ID, fail); !errors.Is(err, storageErr) {
		t.Fatal(err)
	}
	pending := readRevision(t, repo, 1)
	if pending <= before {
		t.Fatal("purge intent must notify clients even when source removal fails")
	}
	if err := repo.Purge(ctx, 1, item.ID, fail); !errors.Is(err, storageErr) {
		t.Fatal(err)
	}
	if got := readRevision(t, repo, 1); got != pending {
		t.Fatal("failed retry without state change advanced revision")
	}
	if err := repo.Purge(ctx, 1, item.ID, func(context.Context, string) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if got := readRevision(t, repo, 1); got <= pending {
		t.Fatal("successful purge retry did not advance revision")
	}
}

func TestMemoryRevisionConcurrentMutations(t *testing.T) {
	repo := NewMemoryRepository()
	var group sync.WaitGroup
	for range 100 {
		group.Add(1)
		go func() {
			defer group.Done()
			if _, err := repo.CreateText(t.Context(), 1, "content", "web"); err != nil {
				t.Error(err)
			}
		}()
	}
	group.Wait()
	if got := readRevision(t, repo, 1); got != 100 {
		t.Fatalf("concurrent mutations lost revision increments: %d", got)
	}
}

type revisionErrorRepo struct{ Repository }

func (revisionErrorRepo) Revision(context.Context, int64) (string, error) {
	return "", errors.New("private database error")
}

func TestHandlerChangesAuthenticationOwnershipAndErrors(t *testing.T) {
	repo := NewMemoryRepository()
	_, _ = repo.CreateText(t.Context(), 2, "private foreign content", "web")
	for _, test := range []struct {
		name   string
		router *gin.Engine
		status int
		body   string
	}{
		{"initial ignores query owner", newTestRouter(repo), http.StatusOK, `{"version":"0"}`},
		{"foreign owner sees own version", newTestRouterWithStore(repo, nil, 2, MaxFileBytes), http.StatusOK, `{"version":"1"}`},
		{"repository failure", newTestRouter(revisionErrorRepo{}), http.StatusInternalServerError, `{"error":"failed to get changes version"}`},
		{"unauthenticated", func() *gin.Engine {
			router := gin.New()
			NewHandler(repo, nil, MaxFileBytes).RegisterRoutes(router.Group("/api"))
			return router
		}(), http.StatusUnauthorized, `{"error":"authentication required"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			test.router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/changes?userId=2", nil))
			if response.Code != test.status || response.Body.String() != test.body {
				t.Fatalf("response = %d %s", response.Code, response.Body.String())
			}
			if got := response.Header().Get("Cache-Control"); got != "no-store" {
				t.Fatalf("cache control = %q", got)
			}
		})
	}
	_, _ = repo.CreateText(t.Context(), 1, "new content", "web")
	response := httptest.NewRecorder()
	newTestRouter(repo).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/changes", nil))
	var body map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || len(body) != 1 || body["version"] != "1" {
		t.Fatalf("updated changes response = %s, %v", response.Body.String(), err)
	}
}

func TestRevisionSchemaIsAdditiveAndCoversBothTables(t *testing.T) {
	for _, clause := range []string{
		"CREATE TABLE IF NOT EXISTS clipboard_user_revisions",
		"CREATE OR REPLACE FUNCTION clipboard_bump_user_revision()",
		"AFTER INSERT OR UPDATE OR DELETE ON clipboard_items",
		"AFTER INSERT OR UPDATE OR DELETE ON clipboard_shares",
		"FOR EACH ROW EXECUTE FUNCTION clipboard_bump_user_revision()",
		"ON CONFLICT (user_id) DO UPDATE",
		"SET revision = clipboard_user_revisions.revision + 1",
		"previous_user_id := OLD.user_id",
		"current_user_id := NEW.user_id",
		"WHERE owner_id IS NOT NULL",
		"SELECT DISTINCT owner_id",
		"IF NOT EXISTS (SELECT 1 FROM pg_trigger",
	} {
		if !strings.Contains(schema, clause) {
			t.Fatalf("revision schema missing %q", clause)
		}
	}
	revisionTable := strings.Split(strings.Split(schema, "CREATE TABLE IF NOT EXISTS clipboard_user_revisions (")[1], ");")[0]
	if strings.Contains(revisionTable, "REFERENCES") {
		t.Fatal("revision table FK can break cascaded user deletion")
	}
	if strings.Contains(schema, "UPDATE clipboard_items SET") || strings.Contains(schema, "nextval(") {
		t.Fatal("revision migration must not rewrite existing items or use sequence-based versions")
	}
}
