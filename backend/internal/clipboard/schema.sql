CREATE TABLE IF NOT EXISTS clipboard_items (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT,
    content TEXT NOT NULL,
    source VARCHAR(32) NOT NULL DEFAULT 'web',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE clipboard_items ADD COLUMN IF NOT EXISTS user_id BIGINT;
ALTER TABLE clipboard_items ADD COLUMN IF NOT EXISTS kind VARCHAR(16) NOT NULL DEFAULT 'text';
ALTER TABLE clipboard_items ADD COLUMN IF NOT EXISTS file_name TEXT NOT NULL DEFAULT '';
ALTER TABLE clipboard_items ADD COLUMN IF NOT EXISTS media_type VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE clipboard_items ADD COLUMN IF NOT EXISTS size_bytes BIGINT NOT NULL DEFAULT 0;
ALTER TABLE clipboard_items ADD COLUMN IF NOT EXISTS storage_key TEXT NOT NULL DEFAULT '';
ALTER TABLE clipboard_items ADD COLUMN IF NOT EXISTS tags TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE clipboard_items ADD COLUMN IF NOT EXISTS favorite BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_clipboard_items_created_at
    ON clipboard_items (created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_clipboard_items_user_created
    ON clipboard_items (user_id, created_at DESC, id DESC);

CREATE UNIQUE INDEX IF NOT EXISTS idx_clipboard_items_storage_key
    ON clipboard_items (storage_key)
    WHERE storage_key <> '';

CREATE INDEX IF NOT EXISTS idx_clipboard_items_user_favorite
    ON clipboard_items (user_id, created_at DESC, id DESC)
    WHERE favorite = TRUE;

CREATE TABLE IF NOT EXISTS clipboard_shares (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    item_id BIGINT NOT NULL REFERENCES clipboard_items(id) ON DELETE CASCADE,
    token VARCHAR(64) NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (expires_at > created_at)
);

CREATE INDEX IF NOT EXISTS idx_clipboard_shares_user_created
    ON clipboard_shares (user_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_clipboard_shares_active_token
    ON clipboard_shares (token, expires_at)
    WHERE revoked_at IS NULL;
