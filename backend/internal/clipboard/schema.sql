CREATE TABLE IF NOT EXISTS clipboard_items (
    id BIGSERIAL PRIMARY KEY,
    content TEXT NOT NULL,
    source VARCHAR(32) NOT NULL DEFAULT 'web',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_clipboard_items_created_at
    ON clipboard_items (created_at DESC, id DESC);

