package clipboard

import (
	"context"
	_ "embed"
	"errors"

	"github.com/jackc/pgx/v5"
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

const itemColumns = `id, user_id, kind, content, file_name, media_type, size_bytes, storage_key, source, created_at`

func (r *PostgresRepository) CreateText(ctx context.Context, userID int64, content, source string) (Item, error) {
	if err := validateContent(content); err != nil {
		return Item{}, err
	}

	var item Item
	err := r.pool.QueryRow(ctx, `
		INSERT INTO clipboard_items (user_id, kind, content, source)
		VALUES ($1, 'text', $2, $3)
		RETURNING `+itemColumns+`
	`, userID, content, normalizeSource(source)).Scan(itemDestinations(&item)...)
	return item, err
}

func (r *PostgresRepository) CreateAttachment(ctx context.Context, userID int64, attachment Attachment) (Item, error) {
	if err := validateAttachment(attachment); err != nil {
		return Item{}, err
	}
	var item Item
	err := r.pool.QueryRow(ctx, `
		INSERT INTO clipboard_items (user_id, kind, content, file_name, media_type, size_bytes, storage_key, source)
		VALUES ($1, $2, '', $3, $4, $5, $6, $7)
		RETURNING `+itemColumns+`
	`, userID, attachment.Kind, normalizeFileName(attachment.FileName), attachment.MediaType, attachment.SizeBytes, attachment.StorageKey, normalizeSource(attachment.Source)).Scan(itemDestinations(&item)...)
	return item, err
}

func (r *PostgresRepository) Get(ctx context.Context, userID, id int64) (Item, error) {
	var item Item
	err := r.pool.QueryRow(ctx, `SELECT `+itemColumns+` FROM clipboard_items WHERE user_id = $1 AND id = $2`, userID, id).Scan(itemDestinations(&item)...)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Item{}, ErrNotFound
		}
		return Item{}, err
	}
	return item, nil
}

func (r *PostgresRepository) List(ctx context.Context, userID int64, limit int) ([]Item, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+itemColumns+`
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
		if err := rows.Scan(itemDestinations(&item)...); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func itemDestinations(item *Item) []any {
	return []any{&item.ID, &item.UserID, &item.Kind, &item.Content, &item.FileName, &item.MediaType, &item.SizeBytes, &item.StorageKey, &item.Source, &item.CreatedAt}
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
