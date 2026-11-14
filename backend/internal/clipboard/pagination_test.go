package clipboard

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sy1063259659/suidu/backend/internal/filestore"
)

type clipboardPage struct {
	Items      []Item `json:"items"`
	NextCursor string `json:"nextCursor"`
	HasMore    bool   `json:"hasMore"`
}

func readClipboardPage(t *testing.T, router http.Handler, query url.Values) clipboardPage {
	t.Helper()
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/clipboard?"+query.Encode(), nil))
	if response.Code != http.StatusOK {
		t.Fatalf("page response = %d, %s", response.Code, response.Body.String())
	}
	var page clipboardPage
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.HasMore != (page.NextCursor != "") {
		t.Fatalf("inconsistent continuation metadata: %#v", page)
	}
	return page
}

func TestHandlerCursorPagesAllItemsWithTiedAndMicrosecondTimestamps(t *testing.T) {
	repo := NewMemoryRepository()
	base := time.Date(2026, 11, 14, 10, 30, 0, 123456000, time.UTC)
	for i := 0; i < 127; i++ {
		repo.now = func() time.Time { return base.Add(time.Duration(i%7) * time.Microsecond) }
		if _, err := repo.CreateText(t.Context(), 1, fmt.Sprintf("item %d", i), "web"); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.CreateText(t.Context(), 2, "other user", "web"); err != nil {
			t.Fatal(err)
		}
	}
	want, err := repo.List(t.Context(), 1, ListFilter{})
	if err != nil || len(want) != 127 {
		t.Fatalf("unlimited backup list = %d items, %v", len(want), err)
	}
	router := newTestRouter(repo)
	query := url.Values{}
	var got []Item
	for pageIndex, expectedSize := range []int{50, 50, 27} {
		page := readClipboardPage(t, router, query)
		if len(page.Items) != expectedSize || page.HasMore != (pageIndex < 2) {
			t.Fatalf("page %d = %d items, hasMore %v", pageIndex, len(page.Items), page.HasMore)
		}
		got = append(got, page.Items...)
		if page.HasMore {
			cursor, err := decodeListCursor(page.NextCursor)
			last := page.Items[len(page.Items)-1]
			if err != nil || cursor.ID != last.ID || !cursor.CreatedAt.Equal(last.CreatedAt) {
				t.Fatalf("continuation lost exact timestamp or ID: %#v, %v", cursor, err)
			}
			query.Set("cursor", page.NextCursor)
		}
	}
	for i, item := range got {
		if item.ID != want[i].ID || !item.CreatedAt.Equal(want[i].CreatedAt) {
			t.Fatalf("item %d = %d at %s, want %d at %s", i, item.ID, item.CreatedAt, want[i].ID, want[i].CreatedAt)
		}
	}
	// Even a cursor at the final row returns an empty, final page.
	query.Set("cursor", encodeListCursor(got[len(got)-1]))
	page := readClipboardPage(t, router, query)
	if len(page.Items) != 0 || page.HasMore {
		t.Fatalf("page beyond end = %#v", page)
	}
}

func TestHandlerCursorCombinesAllFiltersAcrossPages(t *testing.T) {
	repo := NewMemoryRepository()
	base := time.Date(2026, 11, 14, 10, 30, 0, 123456789, time.UTC)
	var want []int64
	for i := 0; i < 123; i++ {
		input := ImportItem{Kind: KindText, Content: "matching", CreatedAt: base, Metadata: ItemMetadata{Favorite: true, Tags: []string{"alpha"}}}
		item, err := repo.Import(t.Context(), 1, input)
		if err != nil {
			t.Fatal(err)
		}
		want = append([]int64{item.ID}, want...)
		for noise := 0; noise < 6; noise++ {
			input.Kind, input.CreatedAt = KindText, base
			input.Metadata = ItemMetadata{Favorite: true, Tags: []string{"alpha"}}
			userID := int64(1)
			switch noise {
			case 0:
				input.Metadata.Favorite = false
			case 1:
				input.Metadata.Tags = nil
			case 2:
				input.CreatedAt = base.Add(-time.Hour)
			case 3:
				input.CreatedAt = base.Add(time.Hour)
			case 4:
				userID = 2
			case 5:
				input.Kind = KindFile
				input.Attachment = Attachment{Kind: KindFile, FileName: "alpha.txt", StorageKey: "file"}
			}
			if _, err := repo.Import(t.Context(), userID, input); err != nil {
				t.Fatal(err)
			}
		}
	}
	router := newTestRouter(repo)
	query := url.Values{"q": {"alpha"}, "kind": {"text"}, "favorite": {"true"}, "limit": {"50"}, "from": {base.Format(time.RFC3339Nano)}, "to": {base.Add(time.Nanosecond).Format(time.RFC3339Nano)}}
	var got []int64
	for pageIndex := 0; pageIndex < 3; pageIndex++ {
		page := readClipboardPage(t, router, query)
		for _, item := range page.Items {
			got = append(got, item.ID)
		}
		if page.HasMore != (pageIndex < 2) {
			t.Fatalf("page %d hasMore = %v", pageIndex, page.HasMore)
		}
		query.Set("cursor", page.NextCursor)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filtered page IDs = %v, want %v", got, want)
	}
}

func TestHandlerCursorUsesLookaheadAndKeepsExistingLimits(t *testing.T) {
	for _, test := range []struct {
		limit string
		want  int
	}{{"", 50}, {"7", 7}, {"200", 100}} {
		t.Run("limit_"+test.limit, func(t *testing.T) {
			repo := &listCaptureRepo{items: []Item{}}
			query := url.Values{"limit": {test.limit}}
			page := readClipboardPage(t, newTestRouter(repo), query)
			if repo.filter.Limit != test.want+1 || page.HasMore {
				t.Fatalf("fetch limit = %d, want %d; page = %#v", repo.filter.Limit, test.want+1, page)
			}
		})
	}
}

func TestHandlerCursorLastFullPageAndMissingBoundary(t *testing.T) {
	repo := NewMemoryRepository()
	base := time.Date(2026, 11, 14, 10, 30, 0, 123456000, time.UTC)
	repo.now = func() time.Time { return base }
	for i := 0; i < 6; i++ {
		if _, err := repo.CreateText(t.Context(), 1, "item", "web"); err != nil {
			t.Fatal(err)
		}
	}
	router := newTestRouter(repo)
	query := url.Values{"limit": {"3"}}
	first := readClipboardPage(t, router, query)
	if !first.HasMore || len(first.Items) != 3 {
		t.Fatalf("first page = %#v", first)
	}
	// The cursor is an order bound, so trashing its source row does not break it.
	if err := repo.Trash(t.Context(), 1, first.Items[2].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateText(t.Context(), 1, "newer item", "web"); err != nil {
		t.Fatal(err)
	}
	query.Set("cursor", first.NextCursor)
	last := readClipboardPage(t, router, query)
	if len(last.Items) != 3 || last.HasMore || last.Items[0].ID != 3 || last.Items[2].ID != 1 {
		t.Fatalf("last full page = %#v", last)
	}
}

func TestHandlerCursorDoesNotGrantOtherUserAccessOrBypassAuth(t *testing.T) {
	repo := NewMemoryRepository()
	repo.now = func() time.Time { return time.Date(2026, 11, 14, 10, 30, 0, 123456000, time.UTC) }
	for i := 0; i < 4; i++ {
		for _, userID := range []int64{1, 2} {
			if _, err := repo.CreateText(t.Context(), userID, "private", "web"); err != nil {
				t.Fatal(err)
			}
		}
	}
	other := newTestRouterWithStore(repo, filestore.NewMemoryStore(), 2, MaxFileBytes)
	page := readClipboardPage(t, other, url.Values{"limit": {"2"}})
	query := url.Values{"cursor": {page.NextCursor}}
	own := readClipboardPage(t, newTestRouter(repo), query)
	if len(own.Items) != 3 || own.Items[0].ID != 5 || own.Items[1].ID != 3 || own.Items[2].ID != 1 {
		t.Fatalf("foreign cursor must remain scoped to authenticated user: %#v", own)
	}
	unauthenticated := gin.New()
	NewHandler(repo, filestore.NewMemoryStore(), MaxFileBytes).RegisterRoutes(unauthenticated.Group("/api"))
	response := httptest.NewRecorder()
	unauthenticated.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/clipboard?"+query.Encode(), nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("cursor bypassed authentication: %d", response.Code)
	}
}

func TestHandlerRejectsMalformedCursorsBeforeRepositoryCall(t *testing.T) {
	encode := func(payload string) string { return base64.RawURLEncoding.EncodeToString([]byte(payload)) }
	timestamp := "2026-11-14T10:30:00.123456789Z"
	valid := encode(timestamp + "\n1")
	for _, raw := range []string{
		"not-a-cursor", "+++", valid + "=", valid + "\n", strings.Repeat("A", maxListCursorBytes+1),
		encode("{}"), encode(timestamp), encode("bad-time\n1"), encode("2026-13-14T10:30:00Z\n1"),
		encode("2026-11-14T10:30:00.1234567891Z\n1"), encode("2026-11-14T10:30:00+24:00\n1"),
		encode(timestamp + "\n0"), encode(timestamp + "\n-1"), encode(timestamp + "\n+1"), encode(timestamp + "\n01"),
		encode(timestamp + "\n9223372036854775808"), encode(timestamp + "\n1.0"), encode(timestamp + "\n1\n2"), encode(timestamp + "\n"),
	} {
		t.Run(raw, func(t *testing.T) {
			repo := &listCaptureRepo{}
			response := httptest.NewRecorder()
			newTestRouter(repo).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/clipboard?"+url.Values{"cursor": {raw}}.Encode(), nil))
			if response.Code != http.StatusBadRequest || repo.userID != 0 {
				t.Fatalf("cursor response = %d, repository user = %d", response.Code, repo.userID)
			}
		})
	}
}

func TestListCursorRoundTripsExactNanosecondsAndMaxID(t *testing.T) {
	item := Item{ID: 9223372036854775807, CreatedAt: time.Date(2026, 11, 14, 10, 30, 0, 123456789, time.FixedZone("offset", 8*60*60))}
	cursor, err := decodeListCursor(encodeListCursor(item))
	if err != nil || cursor.ID != item.ID || !cursor.CreatedAt.Equal(item.CreatedAt) || cursor.CreatedAt.Nanosecond() != 123456789 {
		t.Fatalf("exact cursor roundtrip = %#v, %v", cursor, err)
	}
}
