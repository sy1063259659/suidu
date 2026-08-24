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
	"time"

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

func TestHandlerCreatesListsResolvesAndRevokesShare(t *testing.T) {
	repo := NewMemoryRepository()
	item, err := repo.CreateText(t.Context(), 1, "share me", "web")
	if err != nil {
		t.Fatalf("create item: %v", err)
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
