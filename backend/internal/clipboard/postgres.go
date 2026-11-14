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

func (r *PostgresRepository) Revision(ctx context.Context, userID int64) (string, error) {
	var revision string
	err := r.pool.QueryRow(ctx, `SELECT COALESCE(
		(SELECT revision::text FROM clipboard_user_revisions WHERE user_id = $1), '0')`, userID).Scan(&revision)
	return revision, err
}

// Lock before touching any item or share row: AFTER revision triggers alone
// would allow item -> revision -> share and share -> revision lock cycles.
// The lock is transaction-scoped and shared by all mutation entrypoints.
func beginUserMutation(ctx context.Context, begin func(context.Context) (pgx.Tx, error), userID int64) (pgx.Tx, error) {
	tx, err := begin(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, userID); err != nil {
		_ = tx.Rollback(context.Background())
		return nil, err
	}
	return tx, nil
}

func (r *PostgresRepository) mutate(ctx context.Context, userID int64, change func(pgx.Tx) error) error {
	tx, err := beginUserMutation(ctx, r.pool.Begin, userID)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if err := change(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

const itemColumns = `id, user_id, kind, content, file_name, media_type, size_bytes, storage_key, content_hash, note, source, created_at, tags, favorite, deleted_at, purge_pending`

func (r *PostgresRepository) CreateText(ctx context.Context, userID int64, content, source string) (Item, error) {
	if err := validateContent(content); err != nil {
		return Item{}, err
	}

	var item Item
	err := r.mutate(ctx, userID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
		INSERT INTO clipboard_items (user_id, kind, content, content_hash, source)
		VALUES ($1, 'text', $2, $3, $4)
		RETURNING `+itemColumns+`
	`, userID, content, textContentHash(content), normalizeSource(source)).Scan(itemDestinations(&item)...)
	})
	return item, err
}

func (r *PostgresRepository) CreateAttachment(ctx context.Context, userID int64, attachment Attachment) (Item, error) {
	if err := validateAttachment(attachment); err != nil {
		return Item{}, err
	}
	var item Item
	err := r.mutate(ctx, userID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
		INSERT INTO clipboard_items (user_id, kind, content, file_name, media_type, size_bytes, storage_key, content_hash, source)
		VALUES ($1, $2, '', $3, $4, $5, $6, $7, $8)
		RETURNING `+itemColumns+`
	`, userID, attachment.Kind, normalizeFileName(attachment.FileName), attachment.MediaType, attachment.SizeBytes, attachment.StorageKey, attachment.ContentHash, normalizeSource(attachment.Source)).Scan(itemDestinations(&item)...)
	})
	return item, err
}

func (r *PostgresRepository) FindDuplicate(ctx context.Context, userID int64, kind Kind, contentHash string) (Item, error) {
	var item Item
	err := r.pool.QueryRow(ctx, `
		SELECT `+itemColumns+`
		FROM clipboard_items
		WHERE user_id = $1 AND kind = $2 AND content_hash = $3 AND deleted_at IS NULL
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`, userID, kind, contentHash).Scan(itemDestinations(&item)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return Item{}, ErrNotFound
	}
	return item, err
}

func (r *PostgresRepository) Import(ctx context.Context, userID int64, input ImportItem) (Item, error) {
	metadata, err := normalizeMetadata(input.Metadata)
	if err != nil {
		return Item{}, err
	}
	createdAt := input.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	var item Item
	if input.Kind == KindText {
		if err := validateContent(input.Content); err != nil {
			return Item{}, err
		}
		err = r.mutate(ctx, userID, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx, `
			INSERT INTO clipboard_items (user_id, kind, content, content_hash, note, source, created_at, tags, favorite)
			VALUES ($1, 'text', $2, $3, $4, $5, $6, $7, $8)
			RETURNING `+itemColumns+`
		`, userID, input.Content, textContentHash(input.Content), metadata.Note, normalizeSource(input.Source), createdAt, metadata.Tags, metadata.Favorite).Scan(itemDestinations(&item)...)
		})
		return item, err
	}
	if err := validateAttachment(input.Attachment); err != nil || input.Attachment.Kind != input.Kind {
		return Item{}, ErrInvalidFile
	}
	err = r.mutate(ctx, userID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
		INSERT INTO clipboard_items (user_id, kind, content, file_name, media_type, size_bytes, storage_key, content_hash, note, source, created_at, tags, favorite)
		VALUES ($1, $2, '', $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING `+itemColumns+`
	`, userID, input.Kind, normalizeFileName(input.Attachment.FileName), input.Attachment.MediaType, input.Attachment.SizeBytes, input.Attachment.StorageKey, input.Attachment.ContentHash, metadata.Note, normalizeSource(input.Attachment.Source), createdAt, metadata.Tags, metadata.Favorite).Scan(itemDestinations(&item)...)
	})
	return item, err
}

func (r *PostgresRepository) Get(ctx context.Context, userID, id int64) (Item, error) {
	var item Item
	err := r.pool.QueryRow(ctx, `SELECT `+itemColumns+` FROM clipboard_items WHERE user_id = $1 AND id = $2 AND deleted_at IS NULL`, userID, id).Scan(itemDestinations(&item)...)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Item{}, ErrNotFound
		}
		return Item{}, err
	}
	return item, nil
}

func (r *PostgresRepository) List(ctx context.Context, userID int64, filter ListFilter) ([]Item, error) {
	query, args := buildListItemsQuery(userID, filter)
	rows, err := r.pool.Query(ctx, query, args...)
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

func buildListItemsQuery(userID int64, filter ListFilter) (string, []any) {
	return `
		SELECT ` + itemColumns + `
		FROM clipboard_items
		WHERE user_id = $1 AND deleted_at IS NULL
		  AND ($2 = '' OR kind = $2)
		  AND (NOT $3 OR favorite = TRUE)
		  AND (
			$4 = ''
			OR strpos(lower(COALESCE(content, '')), lower($4)) > 0
			OR strpos(lower(COALESCE(file_name, '')), lower($4)) > 0
			OR strpos(lower(COALESCE(note, '')), lower($4)) > 0
			OR strpos(lower(array_to_string(tags, ' ')), lower($4)) > 0
		  )
		  AND ($5::timestamptz IS NULL OR created_at >= $5)
		  AND ($6::timestamptz IS NULL OR created_at < $6)
		ORDER BY created_at DESC, id DESC
		LIMIT NULLIF($7, 0)
	`, []any{
			userID,
			filter.Kind,
			filter.FavoriteOnly,
			strings.TrimSpace(filter.Query),
			timeValueOrNil(filter.CreatedFrom),
			timeValueOrNil(filter.CreatedBefore),
			filter.Limit,
		}
}

func timeValueOrNil(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}

func itemDestinations(item *Item) []any {
	return []any{&item.ID, &item.UserID, &item.Kind, &item.Content, &item.FileName, &item.MediaType, &item.SizeBytes, &item.StorageKey, &item.ContentHash, &item.Note, &item.Source, &item.CreatedAt, &item.Tags, &item.Favorite, &item.DeletedAt, &item.PurgePending}
}

func (r *PostgresRepository) UpdateMetadata(ctx context.Context, userID, id int64, metadata ItemMetadata) (Item, error) {
	normalized, err := normalizeMetadata(metadata)
	if err != nil {
		return Item{}, err
	}
	var item Item
	err = r.mutate(ctx, userID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
		UPDATE clipboard_items
		SET note = $3, tags = $4, favorite = $5,
		    content = COALESCE($6, content), content_hash = CASE WHEN $6::text IS NULL THEN content_hash ELSE $7 END
		WHERE user_id = $1 AND id = $2 AND deleted_at IS NULL AND ($6::text IS NULL OR kind = 'text')
		RETURNING `+itemColumns+`
	`, userID, id, normalized.Note, normalized.Tags, normalized.Favorite, normalized.Content, metadataContentHash(normalized)).Scan(itemDestinations(&item)...)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Item{}, ErrNotFound
	}
	return item, err
}

func metadataContentHash(metadata ItemMetadata) string {
	if metadata.Content == nil {
		return ""
	}
	return textContentHash(*metadata.Content)
}

func (r *PostgresRepository) Delete(ctx context.Context, userID, id int64) error {
	return r.mutate(ctx, userID, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `DELETE FROM clipboard_items WHERE user_id = $1 AND id = $2`, userID, id)
		if err != nil {
			return err
		}
		if result.RowsAffected() == 0 {
			return ErrNotFound
		}
		return nil
	})
}

func (r *PostgresRepository) Trash(ctx context.Context, userID, id int64) error {
	tx, err := beginUserMutation(ctx, r.pool.Begin, userID)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	result, err := tx.Exec(ctx, `UPDATE clipboard_items SET deleted_at = NOW() WHERE user_id = $1 AND id = $2 AND deleted_at IS NULL`, userID, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	// CreateShare locks the same item, so newly created shares cannot escape revocation.
	if _, err := tx.Exec(ctx, `UPDATE clipboard_shares SET revoked_at = NOW() WHERE user_id = $1 AND item_id = $2 AND revoked_at IS NULL`, userID, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func buildListTrashQuery(userID int64, limit int) (string, []any) {
	return `SELECT ` + itemColumns + ` FROM clipboard_items
		WHERE user_id = $1 AND deleted_at IS NOT NULL
		ORDER BY deleted_at DESC, id DESC LIMIT NULLIF($2, 0)`, []any{userID, limit}
}

func (r *PostgresRepository) ListTrash(ctx context.Context, userID int64, limit int) ([]Item, error) {
	query, args := buildListTrashQuery(userID, limit)
	rows, err := r.pool.Query(ctx, query, args...)
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
	return items, rows.Err()
}

func (r *PostgresRepository) Restore(ctx context.Context, userID, id int64) (Item, error) {
	var item Item
	err := r.mutate(ctx, userID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `UPDATE clipboard_items SET deleted_at = NULL
		WHERE user_id = $1 AND id = $2 AND deleted_at IS NOT NULL AND purge_pending = FALSE
		RETURNING `+itemColumns, userID, id).Scan(itemDestinations(&item)...)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Item{}, ErrNotFound
	}
	return item, err
}

func (r *PostgresRepository) Purge(ctx context.Context, userID, id int64, deleteSource func(context.Context, string) error) error {
	// Commit intent before touching the external file store. Even if a later
	// database operation fails, restore cannot revive an item whose source is gone.
	err := r.mutate(ctx, userID, func(tx pgx.Tx) error {
		intent, err := tx.Exec(ctx, `UPDATE clipboard_items SET purge_pending = TRUE
			WHERE user_id = $1 AND id = $2 AND deleted_at IS NOT NULL`, userID, id)
		if err != nil {
			return err
		}
		if intent.RowsAffected() == 0 {
			return ErrNotFound
		}
		return nil
	})
	if err != nil {
		return err
	}
	tx, err := beginUserMutation(ctx, r.pool.Begin, userID)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	var storageKey string
	err = tx.QueryRow(ctx, `SELECT storage_key FROM clipboard_items
		WHERE user_id = $1 AND id = $2 AND deleted_at IS NOT NULL AND purge_pending = TRUE FOR UPDATE`, userID, id).Scan(&storageKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	// Keep the row locked until its source is removed; restore must wait for this transaction.
	if storageKey != "" {
		if err := deleteSource(ctx, storageKey); err != nil {
			return err
		}
	}
	// Once the source is gone, finish the database deletion even if the client disconnects.
	// Pending intent already blocks restoration if this final transaction fails.
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if _, err := tx.Exec(finishCtx, `DELETE FROM clipboard_items WHERE user_id = $1 AND id = $2 AND deleted_at IS NOT NULL`, userID, id); err != nil {
		return err
	}
	return tx.Commit(finishCtx)
}

func (r *PostgresRepository) AssignOrphanedItems(ctx context.Context, userID int64) error {
	return r.mutate(ctx, userID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE clipboard_items SET user_id = $1 WHERE user_id IS NULL`, userID)
		return err
	})
}

const shareItemColumns = `
	s.id, s.user_id, s.token, s.expires_at, s.revoked_at, s.created_at,
	i.id, i.user_id, i.kind, i.content, i.file_name, i.media_type, i.size_bytes, i.storage_key, i.content_hash, i.note, i.source, i.created_at, i.tags, i.favorite, i.deleted_at, i.purge_pending`

func (r *PostgresRepository) CreateShare(ctx context.Context, userID int64, input CreateShareInput) (Share, error) {
	now := time.Now().UTC()
	if err := validateShareInput(input, now); err != nil {
		return Share{}, err
	}
	var share Share
	err := r.mutate(ctx, userID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
		WITH inserted AS (
			INSERT INTO clipboard_shares (user_id, item_id, token, expires_at)
			SELECT $1, i.id, $3, $4
			FROM clipboard_items i
			WHERE i.user_id = $1 AND i.id = $2 AND i.deleted_at IS NULL
			FOR UPDATE OF i
			RETURNING *
		)
		SELECT `+shareItemColumns+`
		FROM inserted s
		JOIN clipboard_items i ON i.id = s.item_id
	`, userID, input.ItemID, input.Token, input.ExpiresAt.UTC()).Scan(shareDestinations(&share)...)
	})
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
		WHERE s.user_id = $1 AND i.deleted_at IS NULL
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
		WHERE s.token = $1 AND s.revoked_at IS NULL AND s.expires_at > NOW() AND i.deleted_at IS NULL
	`, token).Scan(shareDestinations(&share)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return Share{}, ErrNotFound
	}
	return share, err
}

func (r *PostgresRepository) RevokeShare(ctx context.Context, userID, id int64) error {
	return r.mutate(ctx, userID, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
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
	})
}

func shareDestinations(share *Share) []any {
	destinations := []any{
		&share.ID, &share.UserID, &share.Token, &share.ExpiresAt, &share.RevokedAt, &share.CreatedAt,
	}
	return append(destinations, itemDestinations(&share.Item)...)
}
