package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func newTestService() *Service {
	return NewService(NewMemoryStore(), NewMemorySessionStore(), time.Hour, false)
}

func TestPasswordHashAndVerification(t *testing.T) {
	hash, err := hashPassword("correct horse battery")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if !verifyPassword(hash, "correct horse battery") {
		t.Fatal("expected password to verify")
	}
	if verifyPassword(hash, "wrong password") {
		t.Fatal("expected wrong password to fail")
	}
	if err := validatePassword("short"); err != ErrWeakPassword {
		t.Fatalf("expected weak password error, got %v", err)
	}
}

func TestServiceLoginAndPasswordChange(t *testing.T) {
	service := newTestService()
	ctx := context.Background()
	admin, err := service.CreateUser(ctx, "Admin", "administrator password", RoleAdmin)
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	if admin.Role != RoleAdmin || admin.Username != "admin" {
		t.Fatalf("unexpected admin: %#v", admin)
	}
	if _, err := service.CreateUser(ctx, "admin", "another password", RoleUser); err != ErrUsernameTaken {
		t.Fatalf("expected duplicate username error, got %v", err)
	}

	user, token, err := service.Login(ctx, "admin", "administrator password", "test-attempt")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if user.ID != admin.ID || token == "" {
		t.Fatalf("unexpected login result: %#v, token=%q", user, token)
	}
	if _, err := service.Authenticate(ctx, token); err != nil {
		t.Fatalf("authenticate session: %v", err)
	}
	if err := service.ChangePassword(ctx, admin.ID, "administrator password", "new administrator password"); err != nil {
		t.Fatalf("change password: %v", err)
	}
	if _, err := service.Authenticate(ctx, token); err != ErrSessionNotFound {
		t.Fatalf("expected old session to be revoked, got %v", err)
	}
	if _, _, err := service.Login(ctx, "admin", "administrator password", "test-attempt"); err != ErrInvalidCredentials {
		t.Fatalf("expected old password to fail, got %v", err)
	}
}

func TestHandlerLoginAndProtectedMe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := newTestService()
	if _, err := service.CreateUser(context.Background(), "admin", "administrator password", RoleAdmin); err != nil {
		t.Fatalf("create user: %v", err)
	}
	handler := NewHandler(service)
	router := gin.New()
	api := router.Group("/api")
	handler.RegisterPublicRoutes(api)
	protected := api.Group("")
	protected.Use(service.RequireAuth())
	handler.RegisterProtectedRoutes(protected)

	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"username":"admin","password":"administrator password"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || len(response.Result().Cookies()) != 1 {
		t.Fatalf("login response = %d, cookies = %#v", response.Code, response.Result().Cookies())
	}

	meRequest := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meRequest.AddCookie(response.Result().Cookies()[0])
	meResponse := httptest.NewRecorder()
	router.ServeHTTP(meResponse, meRequest)
	if meResponse.Code != http.StatusOK || !strings.Contains(meResponse.Body.String(), `"username":"admin"`) {
		t.Fatalf("me response = %d, body = %s", meResponse.Code, meResponse.Body.String())
	}
}
