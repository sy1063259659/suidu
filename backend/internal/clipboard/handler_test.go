package clipboard

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sy1063259659/suidu/backend/internal/auth"
	"github.com/sy1063259659/suidu/backend/internal/filestore"
)

type listCaptureRepo struct {
	items   []Item
	userID  int64
	filter  ListFilter
	listErr error
}

func (r *listCaptureRepo) CreateText(context.Context, int64, string, string) (Item, error) {
	panic("unexpected CreateText call")
}

func (r *listCaptureRepo) CreateAttachment(context.Context, int64, Attachment) (Item, error) {
	panic("unexpected CreateAttachment call")
}

func (r *listCaptureRepo) Get(context.Context, int64, int64) (Item, error) {
	panic("unexpected Get call")
}

func (r *listCaptureRepo) List(_ context.Context, userID int64, filter ListFilter) ([]Item, error) {
	r.userID = userID
	r.filter = filter
	return r.items, r.listErr
}

func (r *listCaptureRepo) UpdateMetadata(context.Context, int64, int64, ItemMetadata) (Item, error) {
	panic("unexpected UpdateMetadata call")
}

func (r *listCaptureRepo) Delete(context.Context, int64, int64) error {
	panic("unexpected Delete call")
}

func (r *listCaptureRepo) CreateShare(context.Context, int64, CreateShareInput) (Share, error) {
	panic("unexpected CreateShare call")
}

func (r *listCaptureRepo) ListShares(context.Context, int64, int) ([]Share, error) {
	panic("unexpected ListShares call")
}

func (r *listCaptureRepo) GetPublicShare(context.Context, string) (Share, error) {
	panic("unexpected GetPublicShare call")
}

func (r *listCaptureRepo) RevokeShare(context.Context, int64, int64) error {
	panic("unexpected RevokeShare call")
}

func (r *listCaptureRepo) FindDuplicate(context.Context, int64, Kind, string) (Item, error) {
	panic("unexpected FindDuplicate call")
}

func (r *listCaptureRepo) Import(context.Context, int64, ImportItem) (Item, error) {
	panic("unexpected Import call")
}

func newTestRouter(repo Repository) *gin.Engine {
	return newTestRouterWithStore(repo, filestore.NewMemoryStore(), 1, MaxFileBytes)
}

func newTestRouterWithStore(repo Repository, files filestore.Store, userID, maxFileBytes int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(auth.ContextUserKey, auth.User{ID: userID, Username: "tester", Role: auth.RoleUser})
		c.Next()
	})
	NewHandler(repo, files, maxFileBytes).RegisterRoutes(router.Group("/api"))
	return router
}

func TestHandlerCreateListDelete(t *testing.T) {
	router := newTestRouter(NewMemoryRepository())

	createRequest := httptest.NewRequest(http.MethodPost, "/api/clipboard", strings.NewReader(`{"content":"hello","source":"web"}`))
	createRequest.Header.Set("Content-Type", "application/json")
	createResponse := httptest.NewRecorder()
	router.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createResponse.Code, createResponse.Body.String())
	}

	var created Item
	if err := json.Unmarshal(createResponse.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	listResponse := httptest.NewRecorder()
	router.ServeHTTP(listResponse, httptest.NewRequest(http.MethodGet, "/api/clipboard", nil))
	if listResponse.Code != http.StatusOK || !strings.Contains(listResponse.Body.String(), "hello") {
		t.Fatalf("list response = %d, body = %s", listResponse.Code, listResponse.Body.String())
	}

	deleteResponse := httptest.NewRecorder()
	router.ServeHTTP(deleteResponse, httptest.NewRequest(http.MethodDelete, "/api/clipboard/"+strconv.FormatInt(created.ID, 10), nil))
	if deleteResponse.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, body = %s", deleteResponse.Code, deleteResponse.Body.String())
	}
}

func TestHandlerExportAndImportBackup(t *testing.T) {
	repo := NewMemoryRepository()
	item, err := repo.CreateText(t.Context(), 1, "backup me", "web")
	if err != nil {
		t.Fatalf("create item: %v", err)
	}
	if _, err := repo.UpdateMetadata(t.Context(), 1, item.ID, ItemMetadata{Note: "迁移测试", Tags: []string{"test"}, Favorite: true}); err != nil {
		t.Fatalf("update metadata: %v", err)
	}
	router := newTestRouter(repo)
	exportResponse := httptest.NewRecorder()
	router.ServeHTTP(exportResponse, httptest.NewRequest(http.MethodGet, "/api/backup/export", nil))
	if exportResponse.Code != http.StatusOK || exportResponse.Header().Get("Content-Type") != "application/zip" {
		t.Fatalf("export response = %d, content type = %q", exportResponse.Code, exportResponse.Header().Get("Content-Type"))
	}
	archive, err := zip.NewReader(bytes.NewReader(exportResponse.Body.Bytes()), int64(exportResponse.Body.Len()))
	if err != nil {
		t.Fatalf("open export: %v", err)
	}
	manifestEntry := archive.File[0]
	if manifestEntry.Name != "manifest.json" && len(archive.File) > 1 {
		manifestEntry = archive.File[len(archive.File)-1]
	}
	manifestReader, err := manifestEntry.Open()
	if err != nil {
		t.Fatalf("open manifest: %v", err)
	}
	var manifest backupManifest
	if err := json.NewDecoder(manifestReader).Decode(&manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	_ = manifestReader.Close()
	if manifest.Version != backupVersion || len(manifest.Items) != 1 || !manifest.Items[0].Favorite || manifest.Items[0].Note != "迁移测试" {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}

	importRepo := NewMemoryRepository()
	importRouter := newTestRouter(importRepo)
	importResponse := httptest.NewRecorder()
	importRouter.ServeHTTP(importResponse, multipartBackupRequest(t, exportResponse.Body.Bytes()))
	if importResponse.Code != http.StatusOK || !strings.Contains(importResponse.Body.String(), `"imported":1`) {
		t.Fatalf("import response = %d, body = %s", importResponse.Code, importResponse.Body.String())
	}
	items, err := importRepo.List(t.Context(), 1, ListFilter{})
	if err != nil || len(items) != 1 || items[0].Note != "迁移测试" || !items[0].Favorite {
		t.Fatalf("imported items = %#v, err = %v", items, err)
	}
}

func TestHandlerRejectsDuplicateTextUnlessExplicitlyAllowed(t *testing.T) {
	router := newTestRouter(NewMemoryRepository())
	create := func(body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "/api/clipboard", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}
	if response := create(`{"content":"  hello\r\nworld  "}`); response.Code != http.StatusCreated {
		t.Fatalf("first create status = %d, body = %s", response.Code, response.Body.String())
	}
	duplicate := create(`{"content":"hello\nworld"}`)
	if duplicate.Code != http.StatusConflict || !strings.Contains(duplicate.Body.String(), `"duplicate"`) {
		t.Fatalf("duplicate status = %d, body = %s", duplicate.Code, duplicate.Body.String())
	}
	allowed := create(`{"content":"hello\nworld","allowDuplicate":true}`)
	if allowed.Code != http.StatusCreated {
		t.Fatalf("allowed duplicate status = %d, body = %s", allowed.Code, allowed.Body.String())
	}
}

func TestHandlerGetsOwnedItem(t *testing.T) {
	repo := NewMemoryRepository()
	owned, err := repo.CreateText(t.Context(), 1, "owned detail", "web")
	if err != nil {
		t.Fatalf("create owned item: %v", err)
	}
	foreign, err := repo.CreateText(t.Context(), 2, "foreign detail", "web")
	if err != nil {
		t.Fatalf("create foreign item: %v", err)
	}
	router := newTestRouter(repo)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/clipboard/"+strconv.FormatInt(owned.ID, 10), nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "owned detail") {
		t.Fatalf("owned item response = %d, %s", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/clipboard/"+strconv.FormatInt(foreign.ID, 10), nil))
	if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), "foreign detail") {
		t.Fatalf("foreign item response = %d, %s", response.Code, response.Body.String())
	}
}

func TestHandlerListSearchAndKindFilter(t *testing.T) {
	repo := NewMemoryRepository()
	if _, err := repo.CreateText(t.Context(), 1, "Alpha release note", "web"); err != nil {
		t.Fatalf("create matching text: %v", err)
	}
	if _, err := repo.CreateText(t.Context(), 1, "unrelated", "web"); err != nil {
		t.Fatalf("create unrelated text: %v", err)
	}
	if _, err := repo.CreateAttachment(t.Context(), 1, Attachment{
		Kind: KindImage, FileName: "alpha.png", MediaType: "image/png", SizeBytes: 12, StorageKey: "1/alpha.png", Source: "web",
	}); err != nil {
		t.Fatalf("create matching image: %v", err)
	}

	response := httptest.NewRecorder()
	newTestRouter(repo).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/clipboard?q=alpha&kind=text", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("search status = %d, body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "Alpha release note") || strings.Contains(response.Body.String(), "alpha.png") || strings.Contains(response.Body.String(), "unrelated") {
		t.Fatalf("unexpected search response: %s", response.Body.String())
	}
}

func TestHandlerListRejectsInvalidFilters(t *testing.T) {
	router := newTestRouter(NewMemoryRepository())
	tests := []struct {
		name string
		url  string
	}{
		{name: "kind", url: "/api/clipboard?kind=video"},
		{name: "query", url: "/api/clipboard?q=" + strings.Repeat("a", 201)},
		{name: "favorite", url: "/api/clipboard?favorite=yes"},
		{name: "invalid from", url: "/api/clipboard?from=not-a-date"},
		{name: "invalid to", url: "/api/clipboard?to=not-a-date"},
		{name: "from after to", url: "/api/clipboard?from=2026-08-25T10:00:00Z&to=2026-08-25T09:00:00Z"},
		{name: "from equal to", url: "/api/clipboard?from=2026-08-25T10:00:00Z&to=2026-08-25T10:00:00Z"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.url, nil))
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestHandlerListParsesTimeRangeFilters(t *testing.T) {
	repo := &listCaptureRepo{
		items: []Item{{ID: 9, UserID: 1, Kind: KindText, Content: "within range", CreatedAt: time.Date(2026, 8, 25, 10, 30, 0, 0, time.UTC)}},
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/clipboard?from=2026-08-25T10:00:00%2B08:00&to=2026-08-25T11:00:00%2B08:00", nil)

	newTestRouter(repo).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	wantFrom := time.Date(2026, 8, 25, 2, 0, 0, 0, time.UTC)
	wantTo := time.Date(2026, 8, 25, 3, 0, 0, 0, time.UTC)
	if repo.userID != 1 {
		t.Fatalf("list userID = %d", repo.userID)
	}
	if repo.filter.CreatedFrom == nil || !repo.filter.CreatedFrom.Equal(wantFrom) {
		t.Fatalf("CreatedFrom = %#v, want %s", repo.filter.CreatedFrom, wantFrom)
	}
	if repo.filter.CreatedBefore == nil || !repo.filter.CreatedBefore.Equal(wantTo) {
		t.Fatalf("CreatedBefore = %#v, want %s", repo.filter.CreatedBefore, wantTo)
	}
	if !strings.Contains(response.Body.String(), "within range") {
		t.Fatalf("response body = %s", response.Body.String())
	}
}

func TestHandlerUploadDownloadAndDeleteImage(t *testing.T) {
	repo := NewMemoryRepository()
	files := filestore.NewMemoryStore()
	router := newTestRouterWithStore(repo, files, 1, 1024)
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 24)...)

	upload := multipartRequest(t, "/api/clipboard/files", "photo.png", "image/png", png)
	uploadResponse := httptest.NewRecorder()
	router.ServeHTTP(uploadResponse, upload)
	if uploadResponse.Code != http.StatusCreated {
		t.Fatalf("upload status = %d, body = %s", uploadResponse.Code, uploadResponse.Body.String())
	}
	var created Item
	if err := json.Unmarshal(uploadResponse.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode upload: %v", err)
	}
	if created.Kind != KindImage || created.FileName != "photo.png" || created.MediaType != "image/png" || created.SizeBytes != int64(len(png)) {
		t.Fatalf("unexpected uploaded item: %#v", created)
	}

	downloadResponse := httptest.NewRecorder()
	router.ServeHTTP(downloadResponse, httptest.NewRequest(http.MethodGet, "/api/clipboard/"+strconv.FormatInt(created.ID, 10)+"/content", nil))
	if downloadResponse.Code != http.StatusOK || !bytes.Equal(downloadResponse.Body.Bytes(), png) {
		t.Fatalf("download response = %d, body size = %d", downloadResponse.Code, downloadResponse.Body.Len())
	}
	if !strings.HasPrefix(downloadResponse.Header().Get("Content-Disposition"), "inline") {
		t.Fatalf("content disposition = %q", downloadResponse.Header().Get("Content-Disposition"))
	}

	stored, err := repo.Get(t.Context(), 1, created.ID)
	if err != nil {
		t.Fatalf("get stored item: %v", err)
	}
	deleteResponse := httptest.NewRecorder()
	router.ServeHTTP(deleteResponse, httptest.NewRequest(http.MethodDelete, "/api/clipboard/"+strconv.FormatInt(created.ID, 10), nil))
	if deleteResponse.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, body = %s", deleteResponse.Code, deleteResponse.Body.String())
	}
	if _, err := files.Open(t.Context(), stored.StorageKey); !errors.Is(err, filestore.ErrNotFound) {
		t.Fatalf("expected stored content deletion, got %v", err)
	}
}

func TestHandlerRejectsDuplicateAttachmentAndCleansTemporaryObject(t *testing.T) {
	repo := NewMemoryRepository()
	files := filestore.NewMemoryStore()
	router := newTestRouterWithStore(repo, files, 1, 1024)
	content := []byte("same file content")
	first := httptest.NewRecorder()
	router.ServeHTTP(first, multipartRequest(t, "/api/clipboard/files", "one.txt", "text/plain", content))
	if first.Code != http.StatusCreated {
		t.Fatalf("first upload status = %d, body = %s", first.Code, first.Body.String())
	}
	second := httptest.NewRecorder()
	router.ServeHTTP(second, multipartRequest(t, "/api/clipboard/files", "two.txt", "text/plain", content))
	if second.Code != http.StatusConflict || !strings.Contains(second.Body.String(), `"duplicate"`) {
		t.Fatalf("duplicate upload status = %d, body = %s", second.Code, second.Body.String())
	}
	items, err := repo.List(t.Context(), 1, ListFilter{})
	if err != nil || len(items) != 1 {
		t.Fatalf("items after duplicate upload = %#v, err = %v", items, err)
	}
}

func TestHandlerTreatsSVGAsDownload(t *testing.T) {
	router := newTestRouter(NewMemoryRepository())
	upload := multipartRequest(t, "/api/clipboard/files", "drawing.svg", "image/svg+xml", []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, upload)
	if response.Code != http.StatusCreated || !strings.Contains(response.Body.String(), `"kind":"file"`) {
		t.Fatalf("response = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestHandlerRejectsOversizedUpload(t *testing.T) {
	router := newTestRouterWithStore(NewMemoryRepository(), filestore.NewMemoryStore(), 1, 3)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, multipartRequest(t, "/api/clipboard/files", "large.bin", "application/octet-stream", []byte("large")))
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestHandlerHidesOtherUsersAttachments(t *testing.T) {
	repo := NewMemoryRepository()
	files := filestore.NewMemoryStore()
	object, err := files.Put(t.Context(), 2, ".txt", strings.NewReader("secret"), 100)
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	item, err := repo.CreateAttachment(t.Context(), 2, Attachment{Kind: KindFile, FileName: "secret.txt", MediaType: "text/plain", SizeBytes: object.Size, StorageKey: object.Key})
	if err != nil {
		t.Fatalf("create attachment: %v", err)
	}
	router := newTestRouterWithStore(repo, files, 1, 100)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/clipboard/"+strconv.FormatInt(item.ID, 10)+"/content", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func multipartRequest(t *testing.T, target, fileName, mediaType string, content []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": "file", "filename": fileName}))
	header.Set("Content-Type", mediaType)
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatalf("create multipart part: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write multipart content: %v", err)
	}
	_ = writer.WriteField("source", "web")
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, target, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func multipartBackupRequest(t *testing.T, content []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": "backup", "filename": "backup.zip"}))
	header.Set("Content-Type", "application/zip")
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatalf("create backup part: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write backup content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close backup writer: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/backup/import", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func TestHandlerRejectsInvalidContent(t *testing.T) {
	router := newTestRouter(NewMemoryRepository())
	request := httptest.NewRequest(http.MethodPost, "/api/clipboard", strings.NewReader(`{"content":"   "}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestHandlerUpdatesMetadataAndFiltersFavorites(t *testing.T) {
	repo := NewMemoryRepository()
	item, err := repo.CreateText(t.Context(), 1, "release checklist", "web")
	if err != nil {
		t.Fatalf("create item: %v", err)
	}
	if _, err := repo.CreateText(t.Context(), 1, "ordinary note", "web"); err != nil {
		t.Fatalf("create ordinary item: %v", err)
	}
	router := newTestRouter(repo)

	request := httptest.NewRequest(http.MethodPatch, "/api/clipboard/"+strconv.FormatInt(item.ID, 10), strings.NewReader(`{"note":"上线前逐项确认","tags":["work","发布"],"favorite":true}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"favorite":true`) || !strings.Contains(response.Body.String(), "发布") || !strings.Contains(response.Body.String(), "上线前逐项确认") {
		t.Fatalf("update response = %d, %s", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/clipboard?favorite=true&q=发布", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "release checklist") || strings.Contains(response.Body.String(), "ordinary note") {
		t.Fatalf("favorite list = %d, %s", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/clipboard?q=逐项确认", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "release checklist") {
		t.Fatalf("note search = %d, %s", response.Code, response.Body.String())
	}
}

func TestHandlerRejectsInvalidOrUnauthorizedMetadata(t *testing.T) {
	repo := NewMemoryRepository()
	item, err := repo.CreateText(t.Context(), 1, "private", "web")
	if err != nil {
		t.Fatalf("create item: %v", err)
	}
	tooMany := make([]string, MaxTags+1)
	for index := range tooMany {
		tooMany[index] = fmt.Sprintf("tag-%d", index)
	}
	body, err := json.Marshal(map[string]any{"tags": tooMany, "favorite": false})
	if err != nil {
		t.Fatalf("encode metadata: %v", err)
	}

	request := httptest.NewRequest(http.MethodPatch, "/api/clipboard/"+strconv.FormatInt(item.ID, 10), bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	newTestRouter(repo).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid metadata status = %d, body = %s", response.Code, response.Body.String())
	}

	body, err = json.Marshal(map[string]any{"note": strings.Repeat("说", MaxNoteRunes+1), "tags": []string{}, "favorite": false})
	if err != nil {
		t.Fatalf("encode long note: %v", err)
	}
	request = httptest.NewRequest(http.MethodPatch, "/api/clipboard/"+strconv.FormatInt(item.ID, 10), bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	newTestRouter(repo).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("long note status = %d, body = %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPatch, "/api/clipboard/"+strconv.FormatInt(item.ID, 10), strings.NewReader(`{"tags":[],"favorite":true}`))
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	newTestRouterWithStore(repo, filestore.NewMemoryStore(), 2, MaxFileBytes).ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("unauthorized metadata status = %d, body = %s", response.Code, response.Body.String())
	}
}

type deleteFailStore struct {
	filestore.Store
	err error
}

func (s deleteFailStore) Delete(context.Context, string) error {
	return s.err
}

func TestHandlerKeepsAttachmentWhenSourceDeleteFails(t *testing.T) {
	repo := NewMemoryRepository()
	baseStore := filestore.NewMemoryStore()
	object, err := baseStore.Put(t.Context(), 1, ".png", bytes.NewReader([]byte("image")), 100)
	if err != nil {
		t.Fatalf("store source: %v", err)
	}
	item, err := repo.CreateAttachment(t.Context(), 1, Attachment{
		Kind: KindImage, FileName: "screenshot.png", MediaType: "image/png", SizeBytes: object.Size, StorageKey: object.Key,
	})
	if err != nil {
		t.Fatalf("create attachment: %v", err)
	}
	router := newTestRouterWithStore(repo, deleteFailStore{Store: baseStore, err: errors.New("storage unavailable")}, 1, 100)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/api/clipboard/"+strconv.FormatInt(item.ID, 10), nil))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if _, err := repo.Get(t.Context(), 1, item.ID); err != nil {
		t.Fatalf("metadata should remain retryable, got %v", err)
	}
}

func TestHandlerCreatesListsResolvesAndRevokesShare(t *testing.T) {
	repo := NewMemoryRepository()
	item, err := repo.CreateText(t.Context(), 1, "share me", "web")
	if err != nil {
		t.Fatalf("create item: %v", err)
	}
	item, err = repo.UpdateMetadata(t.Context(), 1, item.ID, ItemMetadata{Note: "private-note", Tags: []string{"private-tag"}, Favorite: true})
	if err != nil {
		t.Fatalf("organize item: %v", err)
	}
	router := newShareTestRouter(repo, filestore.NewMemoryStore(), 1)

	create := httptest.NewRequest(http.MethodPost, "/api/clipboard/"+strconv.FormatInt(item.ID, 10)+"/shares", strings.NewReader(`{"expiresInSeconds":3600}`))
	create.Header.Set("Content-Type", "application/json")
	createResponse := httptest.NewRecorder()
	router.ServeHTTP(createResponse, create)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create share = %d, %s", createResponse.Code, createResponse.Body.String())
	}
	var share Share
	if err := json.Unmarshal(createResponse.Body.Bytes(), &share); err != nil {
		t.Fatalf("decode share: %v", err)
	}
	if len(share.Token) != 43 || share.Item.ID != item.ID {
		t.Fatalf("unexpected share: %#v", share)
	}

	listResponse := httptest.NewRecorder()
	router.ServeHTTP(listResponse, httptest.NewRequest(http.MethodGet, "/api/shares", nil))
	if listResponse.Code != http.StatusOK || !strings.Contains(listResponse.Body.String(), share.Token) {
		t.Fatalf("list shares = %d, %s", listResponse.Code, listResponse.Body.String())
	}

	publicResponse := httptest.NewRecorder()
	router.ServeHTTP(publicResponse, httptest.NewRequest(http.MethodGet, "/api/public/shares/"+share.Token, nil))
	if publicResponse.Code != http.StatusOK || !strings.Contains(publicResponse.Body.String(), "share me") {
		t.Fatalf("public share = %d, %s", publicResponse.Code, publicResponse.Body.String())
	}
	if strings.Contains(publicResponse.Body.String(), "private-note") || strings.Contains(publicResponse.Body.String(), "private-tag") || strings.Contains(publicResponse.Body.String(), `"note"`) || strings.Contains(publicResponse.Body.String(), `"favorite"`) || strings.Contains(publicResponse.Body.String(), `"tags"`) {
		t.Fatalf("public share leaked private organization metadata: %s", publicResponse.Body.String())
	}
	if publicResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("public cache control = %q", publicResponse.Header().Get("Cache-Control"))
	}

	revokeResponse := httptest.NewRecorder()
	router.ServeHTTP(revokeResponse, httptest.NewRequest(http.MethodPost, "/api/shares/"+strconv.FormatInt(share.ID, 10)+"/revoke", nil))
	if revokeResponse.Code != http.StatusNoContent {
		t.Fatalf("revoke share = %d, %s", revokeResponse.Code, revokeResponse.Body.String())
	}
	publicResponse = httptest.NewRecorder()
	router.ServeHTTP(publicResponse, httptest.NewRequest(http.MethodGet, "/api/public/shares/"+share.Token, nil))
	if publicResponse.Code != http.StatusNotFound {
		t.Fatalf("revoked public share = %d, %s", publicResponse.Code, publicResponse.Body.String())
	}
}

func TestHandlerStreamsPublicSharedAttachmentWithoutAuthentication(t *testing.T) {
	repo := NewMemoryRepository()
	files := filestore.NewMemoryStore()
	content := []byte("shared file")
	object, err := files.Put(t.Context(), 2, ".txt", bytes.NewReader(content), 100)
	if err != nil {
		t.Fatalf("store file: %v", err)
	}
	item, err := repo.CreateAttachment(t.Context(), 2, Attachment{
		Kind: KindFile, FileName: "shared.txt", MediaType: "text/plain; charset=utf-8", SizeBytes: object.Size, StorageKey: object.Key,
	})
	if err != nil {
		t.Fatalf("create attachment: %v", err)
	}
	share, err := repo.CreateShare(t.Context(), 2, CreateShareInput{ItemID: item.ID, Token: strings.Repeat("a", 43), ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatalf("create share: %v", err)
	}
	router := gin.New()
	NewHandler(repo, files, 100).RegisterPublicRoutes(router.Group("/api"))

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/public/shares/"+share.Token+"/content", nil))
	if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), content) {
		t.Fatalf("public content = %d, %q", response.Code, response.Body.Bytes())
	}
	if !strings.HasPrefix(response.Header().Get("Content-Disposition"), "attachment") || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("public content headers = %#v", response.Header())
	}
}

func TestHandlerRejectsInvalidShareRequests(t *testing.T) {
	repo := NewMemoryRepository()
	item, err := repo.CreateText(t.Context(), 2, "private", "web")
	if err != nil {
		t.Fatalf("create item: %v", err)
	}
	router := newShareTestRouter(repo, filestore.NewMemoryStore(), 1)

	for name, body := range map[string]string{
		"too short": `{"expiresInSeconds":60}`,
		"too long":  `{"expiresInSeconds":31536001}`,
		"overflow":  `{"expiresInSeconds":9223372036854775807}`,
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/clipboard/1/shares", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
		})
	}

	request := httptest.NewRequest(http.MethodPost, "/api/clipboard/"+strconv.FormatInt(item.ID, 10)+"/shares", strings.NewReader(`{"expiresInSeconds":3600}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("other-user item status = %d, body = %s", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/public/shares/not-a-token", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("malformed token status = %d", response.Code)
	}
}

func newShareTestRouter(repo Repository, files filestore.Store, userID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewHandler(repo, files, MaxFileBytes)
	handler.RegisterPublicRoutes(router.Group("/api"))
	protected := router.Group("/api")
	protected.Use(func(c *gin.Context) {
		c.Set(auth.ContextUserKey, auth.User{ID: userID, Username: "tester", Role: auth.RoleUser})
		c.Next()
	})
	handler.RegisterRoutes(protected)
	return router
}
