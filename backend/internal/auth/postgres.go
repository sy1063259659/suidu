package auth

import (
	"context"
	_ "embed"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schema string

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(ctx context.Context, databaseURL string) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if _, err := pool.Exec(ctx, schema); err != nil {
		pool.Close()
		return nil, err
	}
	return &PostgresStore{pool: pool}, nil
}

func (s *PostgresStore) CreateUser(ctx context.Context, username, passwordHash string, role Role) (User, error) {
	var user User
	err := s.pool.QueryRow(ctx, `
		INSERT INTO users (username, password_hash, role)
		VALUES ($1, $2, $3)
		RETURNING id, username, role, disabled, created_at
	`, username, passwordHash, role).Scan(&user.ID, &user.Username, &user.Role, &user.Disabled, &user.CreatedAt)
	var constraintErr *pgconn.PgError
	if errors.As(err, &constraintErr) && constraintErr.Code == "23505" {
		return User{}, ErrUsernameTaken
	}
	return user, err
}

func (s *PostgresStore) FindByUsername(ctx context.Context, username string) (userRecord, error) {
	return s.find(ctx, `WHERE username = $1`, username)
}

func (s *PostgresStore) FindByID(ctx context.Context, id int64) (userRecord, error) {
	return s.find(ctx, `WHERE id = $1`, id)
}

func (s *PostgresStore) find(ctx context.Context, clause string, arg any) (userRecord, error) {
	var record userRecord
	err := s.pool.QueryRow(ctx, `
		SELECT id, username, password_hash, role, disabled, created_at
		FROM users `+clause, arg).Scan(&record.ID, &record.Username, &record.PasswordHash, &record.Role, &record.Disabled, &record.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return userRecord{}, ErrUserNotFound
	}
	return record, err
}

func (s *PostgresStore) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, username, role, disabled, created_at FROM users ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := make([]User, 0)
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Username, &user.Role, &user.Disabled, &user.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (s *PostgresStore) UpdatePassword(ctx context.Context, id int64, passwordHash string) error {
	result, err := s.pool.Exec(ctx, `UPDATE users SET password_hash = $1 WHERE id = $2`, passwordHash, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (s *PostgresStore) SetDisabled(ctx context.Context, id int64, disabled bool) error {
	result, err := s.pool.Exec(ctx, `UPDATE users SET disabled = $1 WHERE id = $2`, disabled, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

// AssignOrphanedClipboardItems preserves records created before authentication
// by assigning them to the first administrator. The migration is allowed to
// run before the clipboard table/column exists; the API startup will retry it.
func (s *PostgresStore) AssignOrphanedClipboardItems(ctx context.Context, userID int64) error {
	_, err := s.pool.Exec(ctx, `UPDATE clipboard_items SET user_id = $1 WHERE user_id IS NULL`, userID)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == "42P01" || pgErr.Code == "42703") {
		return nil
	}
	return err
}

func (s *PostgresStore) Close() { s.pool.Close() }
