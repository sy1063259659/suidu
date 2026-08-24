package clipboard

import (
	"context"
	_ "embed"
	"errors"
	"strings"
	"time"

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

const itemColumns = `id, user_id, kind, content, file_name, media_type, size_bytes, storage_key, source, created_at, tags, favorite`

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

func (r *PostgresRepository) List(ctx context.Context, userID int64, filter ListFilter) ([]Item, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+itemColumns+`
		FROM clipboard_items
		WHERE user_id = $1
		  AND ($2 = '' OR kind = $2)
		  AND (NOT $3 OR favorite = TRUE)
		  AND (
			$4 = ''
			OR strpos(lower(COALESCE(content, '')), lower($4)) > 0
			OR strpos(lower(COALESCE(file_name, '')), lower($4)) > 0
			OR strpos(lower(array_to_string(tags, ' ')), lower($4)) > 0
		  )
		ORDER BY created_at DESC, id DESC
		LIMIT NULLIF($5, 0)
	`, userID, filter.Kind, filter.FavoriteOnly, strings.TrimSpace(filter.Query), filter.Limit)
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
	return []any{&item.ID, &item.UserID, &item.Kind, &item.Content, &item.FileName, &item.MediaType, &item.SizeBytes, &item.StorageKey, &item.Source, &item.CreatedAt, &item.Tags, &item.Favorite}
}

func (r *PostgresRepository) UpdateMetadata(ctx context.Context, userID, id int64, metadata ItemMetadata) (Item, error) {
	normalized, err := normalizeMetadata(metadata)
	if err != nil {
		return Item{}, err
	}
	var item Item
	err = r.pool.QueryRow(ctx, `
		UPDATE clipboard_items
		SET tags = $3, favorite = $4
		WHERE user_id = $1 AND id = $2
		RETURNING `+itemColumns+`
	`, userID, id, normalized.Tags, normalized.Favorite).Scan(itemDestinations(&item)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return Item{}, ErrNotFound
	}
	return item, err
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

const shareItemColumns = `
	s.id, s.user_id, s.token, s.expires_at, s.revoked_at, s.created_at,
	i.id, i.user_id, i.kind, i.content, i.file_name, i.media_type, i.size_bytes, i.storage_key, i.source, i.created_at, i.tags, i.favorite`

func (r *PostgresRepository) CreateShare(ctx context.Context, userID int64, input CreateShareInput) (Share, error) {
	now := time.Now().UTC()
	if err := validateShareInput(input, now); err != nil {
		return Share{}, err
	}
	var share Share
	err := r.pool.QueryRow(ctx, `
		WITH inserted AS (
			INSERT INTO clipboard_shares (user_id, item_id, token, expires_at)
			SELECT $1, i.id, $3, $4
			FROM clipboard_items i
			WHERE i.user_id = $1 AND i.id = $2
			RETURNING *
		)
		SELECT `+shareItemColumns+`
		FROM inserted s
		JOIN clipboard_items i ON i.id = s.item_id
	`, userID, input.ItemID, input.Token, input.ExpiresAt.UTC()).Scan(shareDestinations(&share)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return Share{}, ErrNotFound
	}
	return share, err
}

func (r *PostgresRepository) ListShares(ctx context.Context, userID int64, limit int) ([]Share, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+shareItemColumns+`
		FROM clipboard_shares s
		JOIN clipboard_items i ON i.id = s.item_id
		WHERE s.user_id = $1
		ORDER BY s.created_at DESC, s.id DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	shares := make([]Share, 0)
	for rows.Next() {
		var share Share
		if err := rows.Scan(shareDestinations(&share)...); err != nil {
			return nil, err
		}
		shares = append(shares, share)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return shares, nil
}

func (r *PostgresRepository) GetPublicShare(ctx context.Context, token string) (Share, error) {
	var share Share
	err := r.pool.QueryRow(ctx, `
		SELECT `+shareItemColumns+`
		FROM clipboard_shares s
		JOIN clipboard_items i ON i.id = s.item_id
		WHERE s.token = $1 AND s.revoked_at IS NULL AND s.expires_at > NOW()
	`, token).Scan(shareDestinations(&share)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return Share{}, ErrNotFound
	}
	return share, err
}

func (r *PostgresRepository) RevokeShare(ctx context.Context, userID, id int64) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE clipboard_shares
		SET revoked_at = NOW()
		WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL
	`, id, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func shareDestinations(share *Share) []any {
	destinations := []any{
		&share.ID, &share.UserID, &share.Token, &share.ExpiresAt, &share.RevokedAt, &share.CreatedAt,
	}
	return append(destinations, itemDestinations(&share.Item)...)
}
