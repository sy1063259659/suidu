package auth

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisSessionStore struct {
	client *redis.Client
	prefix string
}

func NewRedisSessionStore(ctx context.Context, addr, password string, database int, prefix string) (*RedisSessionStore, error) {
	client := redis.NewClient(&redis.Options{Addr: addr, Password: password, DB: database})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return &RedisSessionStore{client: client, prefix: prefix}, nil
}

func (s *RedisSessionStore) sessionKey(token string) string {
	return s.prefix + "auth:session:" + token
}

func (s *RedisSessionStore) userSessionsKey(userID int64) string {
	return s.prefix + "auth:user-sessions:" + strconv.FormatInt(userID, 10)
}

func (s *RedisSessionStore) loginFailureKey(key string) string {
	return s.prefix + "auth:login-failure:" + key
}

func (s *RedisSessionStore) Create(ctx context.Context, token string, userID int64, ttl time.Duration) error {
	key := s.sessionKey(token)
	if err := s.client.Set(ctx, key, userID, ttl).Err(); err != nil {
		return err
	}
	return s.client.SAdd(ctx, s.userSessionsKey(userID), token).Err()
}

func (s *RedisSessionStore) Get(ctx context.Context, token string) (int64, error) {
	value, err := s.client.Get(ctx, s.sessionKey(token)).Int64()
	if errors.Is(err, redis.Nil) {
		return 0, ErrSessionNotFound
	}
	return value, err
}

func (s *RedisSessionStore) Delete(ctx context.Context, token string) error {
	value, err := s.client.Get(ctx, s.sessionKey(token)).Int64()
	if err != nil && !errors.Is(err, redis.Nil) {
		return err
	}
	if err := s.client.Del(ctx, s.sessionKey(token)).Err(); err != nil {
		return err
	}
	if err == nil {
		return s.client.SRem(ctx, s.userSessionsKey(value), token).Err()
	}
	return nil
}

func (s *RedisSessionStore) DeleteUser(ctx context.Context, userID int64) error {
	setKey := s.userSessionsKey(userID)
	tokens, err := s.client.SMembers(ctx, setKey).Result()
	if err != nil {
		return err
	}
	keys := make([]string, 0, len(tokens)+1)
	for _, token := range tokens {
		keys = append(keys, s.sessionKey(token))
	}
	keys = append(keys, setKey)
	return s.client.Del(ctx, keys...).Err()
}

func (s *RedisSessionStore) AllowLogin(ctx context.Context, key string) (bool, error) {
	count, err := s.client.Get(ctx, s.loginFailureKey(key)).Int()
	if errors.Is(err, redis.Nil) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return count < 10, nil
}

func (s *RedisSessionStore) RegisterLoginFailure(ctx context.Context, key string) error {
	failureKey := s.loginFailureKey(key)
	count, err := s.client.Incr(ctx, failureKey).Result()
	if err != nil {
		return err
	}
	if count == 1 {
		return s.client.Expire(ctx, failureKey, 15*time.Minute).Err()
	}
	return nil
}

func (s *RedisSessionStore) ClearLoginFailures(ctx context.Context, key string) error {
	return s.client.Del(ctx, s.loginFailureKey(key)).Err()
}

func (s *RedisSessionStore) Close() error { return s.client.Close() }
