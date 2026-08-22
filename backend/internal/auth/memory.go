package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sort"
	"sync"
	"time"
)

type MemoryStore struct {
	mu     sync.RWMutex
	nextID int64
	users  map[int64]userRecord
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{nextID: 1, users: make(map[int64]userRecord)}
}

func (s *MemoryStore) CreateUser(_ context.Context, username, passwordHash string, role Role) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.users {
		if existing.Username == username {
			return User{}, ErrUsernameTaken
		}
	}
	user := User{ID: s.nextID, Username: username, Role: role, CreatedAt: time.Now().UTC()}
	s.nextID++
	s.users[user.ID] = userRecord{User: user, PasswordHash: passwordHash}
	return user, nil
}

func (s *MemoryStore) FindByUsername(_ context.Context, username string) (userRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, user := range s.users {
		if user.Username == username {
			return user, nil
		}
	}
	return userRecord{}, ErrUserNotFound
}

func (s *MemoryStore) FindByID(_ context.Context, id int64) (userRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.users[id]
	if !ok {
		return userRecord{}, ErrUserNotFound
	}
	return user, nil
}

func (s *MemoryStore) ListUsers(_ context.Context) ([]User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	users := make([]User, 0, len(s.users))
	for _, record := range s.users {
		users = append(users, record.User)
	}
	sort.Slice(users, func(i, j int) bool { return users[i].ID < users[j].ID })
	return users, nil
}

func (s *MemoryStore) UpdatePassword(_ context.Context, id int64, passwordHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.users[id]
	if !ok {
		return ErrUserNotFound
	}
	user.PasswordHash = passwordHash
	s.users[id] = user
	return nil
}

func (s *MemoryStore) SetDisabled(_ context.Context, id int64, disabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.users[id]
	if !ok {
		return ErrUserNotFound
	}
	user.Disabled = disabled
	s.users[id] = user
	return nil
}

func (s *MemoryStore) Close() {}

type memorySession struct {
	userID    int64
	expiresAt time.Time
}

type MemorySessionStore struct {
	mu       sync.Mutex
	sessions map[string]memorySession
	failures map[string]struct {
		count     int
		expiresAt time.Time
	}
}

func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{sessions: make(map[string]memorySession), failures: make(map[string]struct {
		count     int
		expiresAt time.Time
	})}
}

func (s *MemorySessionStore) Create(_ context.Context, token string, userID int64, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[token] = memorySession{userID: userID, expiresAt: time.Now().Add(ttl)}
	return nil
}

func (s *MemorySessionStore) Get(_ context.Context, token string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[token]
	if !ok || time.Now().After(session.expiresAt) {
		delete(s.sessions, token)
		return 0, ErrSessionNotFound
	}
	return session.userID, nil
}

func (s *MemorySessionStore) Delete(_ context.Context, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, token)
	return nil
}

func (s *MemorySessionStore) DeleteUser(_ context.Context, userID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for token, session := range s.sessions {
		if session.userID == userID {
			delete(s.sessions, token)
		}
	}
	return nil
}

func (s *MemorySessionStore) AllowLogin(_ context.Context, key string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.failures[key]
	if !ok || time.Now().After(entry.expiresAt) {
		delete(s.failures, key)
		return true, nil
	}
	return entry.count < 10, nil
}

func (s *MemorySessionStore) RegisterLoginFailure(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.failures[key]
	if time.Now().After(entry.expiresAt) {
		entry.count = 0
	}
	entry.count++
	entry.expiresAt = time.Now().Add(15 * time.Minute)
	s.failures[key] = entry
	return nil
}

func (s *MemorySessionStore) ClearLoginFailures(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.failures, key)
	return nil
}

func (s *MemorySessionStore) Close() error { return nil }

func newSessionToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}
