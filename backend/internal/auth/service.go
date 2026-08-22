package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type SessionStore interface {
	Create(ctx context.Context, token string, userID int64, ttl time.Duration) error
	Get(ctx context.Context, token string) (int64, error)
	Delete(ctx context.Context, token string) error
	DeleteUser(ctx context.Context, userID int64) error
	AllowLogin(ctx context.Context, key string) (bool, error)
	RegisterLoginFailure(ctx context.Context, key string) error
	ClearLoginFailures(ctx context.Context, key string) error
	Close() error
}

type Service struct {
	users        UserStore
	sessions     SessionStore
	sessionTTL   time.Duration
	cookieName   string
	cookieSecure bool
}

func NewService(users UserStore, sessions SessionStore, sessionTTL time.Duration, cookieSecure bool) *Service {
	if sessionTTL <= 0 {
		sessionTTL = 7 * 24 * time.Hour
	}
	return &Service{users: users, sessions: sessions, sessionTTL: sessionTTL, cookieName: "suidu_session", cookieSecure: cookieSecure}
}

func (s *Service) Login(ctx context.Context, username, password, attemptKey string) (User, string, error) {
	normalized, err := normalizeUsername(username)
	if err != nil {
		return User{}, "", ErrInvalidCredentials
	}
	allowed, err := s.sessions.AllowLogin(ctx, attemptKey)
	if err != nil {
		return User{}, "", fmt.Errorf("check login rate limit: %w", err)
	}
	if !allowed {
		return User{}, "", ErrInvalidCredentials
	}
	record, err := s.users.FindByUsername(ctx, normalized)
	if err != nil || record.Disabled || !verifyPassword(record.PasswordHash, password) {
		_ = s.sessions.RegisterLoginFailure(ctx, attemptKey)
		return User{}, "", ErrInvalidCredentials
	}
	if err := s.sessions.ClearLoginFailures(ctx, attemptKey); err != nil {
		return User{}, "", fmt.Errorf("clear login rate limit: %w", err)
	}
	token, err := newSessionToken()
	if err != nil {
		return User{}, "", fmt.Errorf("create session token: %w", err)
	}
	if err := s.sessions.Create(ctx, token, record.ID, s.sessionTTL); err != nil {
		return User{}, "", fmt.Errorf("create session: %w", err)
	}
	return record.User, token, nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (User, error) {
	if token == "" {
		return User{}, ErrSessionNotFound
	}
	userID, err := s.sessions.Get(ctx, token)
	if err != nil {
		return User{}, ErrSessionNotFound
	}
	record, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return User{}, ErrSessionNotFound
	}
	if record.Disabled {
		return User{}, ErrUserDisabled
	}
	return record.User, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.sessions.Delete(ctx, token)
}

func (s *Service) CreateUser(ctx context.Context, username, password string, role Role) (User, error) {
	normalized, err := normalizeUsername(username)
	if err != nil {
		return User{}, err
	}
	if err := validateRole(role); err != nil {
		return User{}, err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return User{}, err
	}
	return s.users.CreateUser(ctx, normalized, hash, role)
}

func (s *Service) ChangePassword(ctx context.Context, userID int64, currentPassword, newPassword string) error {
	record, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	if !verifyPassword(record.PasswordHash, currentPassword) {
		return ErrInvalidCredentials
	}
	hash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	if err := s.users.UpdatePassword(ctx, userID, hash); err != nil {
		return err
	}
	return s.sessions.DeleteUser(ctx, userID)
}

func (s *Service) ResetPassword(ctx context.Context, userID int64, newPassword string) error {
	hash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	if err := s.users.UpdatePassword(ctx, userID, hash); err != nil {
		return err
	}
	return s.sessions.DeleteUser(ctx, userID)
}

func (s *Service) ListUsers(ctx context.Context) ([]User, error) {
	return s.users.ListUsers(ctx)
}

func (s *Service) SetDisabled(ctx context.Context, userID int64, disabled bool) error {
	if err := s.users.SetDisabled(ctx, userID, disabled); err != nil {
		return err
	}
	if disabled {
		return s.sessions.DeleteUser(ctx, userID)
	}
	return nil
}

func (s *Service) CookieName() string { return s.cookieName }

func (s *Service) SetSessionCookie(c *gin.Context, token string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     s.cookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(s.sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Service) ClearSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     s.cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Service) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Request.Cookie(s.cookieName)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		user, err := s.Authenticate(c.Request.Context(), cookie.Value)
		if err != nil {
			s.ClearSessionCookie(c)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		c.Set(ContextUserKey, user)
		c.Next()
	}
}

func RequireRole(role Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := CurrentUser(c)
		if !ok || user.Role != role {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "administrator access required"})
			return
		}
		c.Next()
	}
}

func CurrentUser(c *gin.Context) (User, bool) {
	value, exists := c.Get(ContextUserKey)
	if !exists {
		return User{}, false
	}
	user, ok := value.(User)
	return user, ok
}

func loginAttemptKey(c *gin.Context, username string) string {
	remote := c.ClientIP()
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(username)) + "\x00" + remote))
	return hex.EncodeToString(sum[:])
}

func isExpectedAuthError(err error) bool {
	return errors.Is(err, ErrInvalidCredentials) || errors.Is(err, ErrUserNotFound) || errors.Is(err, ErrUserDisabled)
}
