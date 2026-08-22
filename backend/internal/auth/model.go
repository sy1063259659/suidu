package auth

import (
	"context"
	"errors"
	"strings"
	"time"
)

type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound       = errors.New("user not found")
	ErrUsernameTaken      = errors.New("username already exists")
	ErrUserDisabled       = errors.New("user is disabled")
	ErrSessionNotFound    = errors.New("session not found")
	ErrInvalidUsername    = errors.New("invalid username")
	ErrWeakPassword       = errors.New("password does not meet requirements")
	ErrInvalidRole        = errors.New("invalid role")
)

const (
	MinPasswordLength = 12
	MaxPasswordLength = 128
	ContextUserKey    = "suidu.auth.user"
)

type User struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Role      Role      `json:"role"`
	Disabled  bool      `json:"disabled"`
	CreatedAt time.Time `json:"createdAt"`
}

type userRecord struct {
	User
	PasswordHash string
}

type UserStore interface {
	CreateUser(ctx context.Context, username, passwordHash string, role Role) (User, error)
	FindByUsername(ctx context.Context, username string) (userRecord, error)
	FindByID(ctx context.Context, id int64) (userRecord, error)
	ListUsers(ctx context.Context) ([]User, error)
	UpdatePassword(ctx context.Context, id int64, passwordHash string) error
	SetDisabled(ctx context.Context, id int64, disabled bool) error
	Close()
}

func normalizeUsername(username string) (string, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	if len(username) < 3 || len(username) > 64 {
		return "", ErrInvalidUsername
	}
	for _, char := range username {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '.' || char == '_' || char == '-' {
			continue
		}
		return "", ErrInvalidUsername
	}
	return username, nil
}

func validateRole(role Role) error {
	if role != RoleAdmin && role != RoleUser {
		return ErrInvalidRole
	}
	return nil
}
