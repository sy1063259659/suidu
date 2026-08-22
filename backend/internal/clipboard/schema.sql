CREATE TABLE IF NOT EXISTS clipboard_items (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT,
    content TEXT NOT NULL,
    source VARCHAR(32) NOT NULL DEFAULT 'web',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE clipboard_items ADD COLUMN IF NOT EXISTS user_id BIGINT;

CREATE INDEX IF NOT EXISTS idx_clipboard_items_created_at
    ON clipboard_items (created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_clipboard_items_user_created
    ON clipboard_items (user_id, created_at DESC, id DESC);
