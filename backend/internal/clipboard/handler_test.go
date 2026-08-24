package clipboard

import (
	"bytes"
	"encoding/json"
	"errors"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sy1063259659/suidu/backend/internal/auth"
	"github.com/sy1063259659/suidu/backend/internal/filestore"
)

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
