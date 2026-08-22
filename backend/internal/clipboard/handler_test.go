package clipboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newTestRouter(repo Repository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewHandler(repo).RegisterRoutes(router.Group("/api"))
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
