package clipboard

import (
	"context"
	_ "embed"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schema string

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(ctx context.Context, databaseURL string) (*PostgresRepository, error) {
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
	return &PostgresRepository{pool: pool}, nil
}

func (r *PostgresRepository) Close() {
	r.pool.Close()
}

func (r *PostgresRepository) Create(ctx context.Context, userID int64, content, source string) (Item, error) {
	if err := validateContent(content); err != nil {
		return Item{}, err
	}

	var item Item
	err := r.pool.QueryRow(ctx, `
		INSERT INTO clipboard_items (user_id, content, source)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, content, source, created_at
	`, userID, content, normalizeSource(source)).Scan(&item.ID, &item.UserID, &item.Content, &item.Source, &item.CreatedAt)
	return item, err
}

func (r *PostgresRepository) List(ctx context.Context, userID int64, limit int) ([]Item, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, content, source, created_at
		FROM clipboard_items
		WHERE user_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Item, 0)
	for rows.Next() {
		var item Item
		if err := rows.Scan(&item.ID, &item.UserID, &item.Content, &item.Source, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *PostgresRepository) Delete(ctx context.Context, userID, id int64) error {
	result, err := r.pool.Exec(ctx, `DELETE FROM clipboard_items WHERE user_id = $1 AND id = $2`, userID, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) AssignOrphanedItems(ctx context.Context, userID int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE clipboard_items SET user_id = $1 WHERE user_id IS NULL`, userID)
	return err
}
