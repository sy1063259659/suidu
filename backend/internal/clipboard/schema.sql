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
ALTER TABLE clipboard_items ADD COLUMN IF NOT EXISTS content_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE clipboard_items ADD COLUMN IF NOT EXISTS note TEXT NOT NULL DEFAULT '';
ALTER TABLE clipboard_items ADD COLUMN IF NOT EXISTS tags TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE clipboard_items ADD COLUMN IF NOT EXISTS favorite BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE clipboard_items ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE clipboard_items ADD COLUMN IF NOT EXISTS purge_pending BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_clipboard_items_user_trash
    ON clipboard_items (user_id, deleted_at DESC, id DESC)
    WHERE deleted_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_clipboard_items_created_at
    ON clipboard_items (created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_clipboard_items_user_created
    ON clipboard_items (user_id, created_at DESC, id DESC);

CREATE UNIQUE INDEX IF NOT EXISTS idx_clipboard_items_storage_key
    ON clipboard_items (storage_key)
    WHERE storage_key <> '';

CREATE INDEX IF NOT EXISTS idx_clipboard_items_user_kind_hash
    ON clipboard_items (user_id, kind, content_hash)
    WHERE content_hash <> '';

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

-- A per-user row serializes mutations until commit, so concurrently committed
-- changes cannot be hidden by an earlier transaction's larger sequence value.
-- No user FK: cascaded user deletion must still be able to run these triggers.
CREATE TABLE IF NOT EXISTS clipboard_user_revisions (
    user_id BIGINT PRIMARY KEY,
    revision BIGINT NOT NULL DEFAULT 0
);

CREATE OR REPLACE FUNCTION clipboard_bump_user_revision() RETURNS TRIGGER AS $$
DECLARE
    previous_user_id BIGINT;
    current_user_id BIGINT;
    changed_user_id BIGINT;
BEGIN
    IF TG_OP <> 'INSERT' THEN
        previous_user_id := OLD.user_id;
    END IF;
    IF TG_OP <> 'DELETE' THEN
        current_user_id := NEW.user_id;
    END IF;
    -- Updating ownership invalidates both owners, once each; legacy unowned
    -- rows have no user to notify. Lock multiple owner rows in a stable order.
    FOR changed_user_id IN
        SELECT DISTINCT owner_id
        FROM (VALUES (previous_user_id), (current_user_id)) AS owners(owner_id)
        WHERE owner_id IS NOT NULL
        ORDER BY owner_id
    LOOP
        INSERT INTO clipboard_user_revisions (user_id, revision)
        VALUES (changed_user_id, 1)
        ON CONFLICT (user_id) DO UPDATE
        SET revision = clipboard_user_revisions.revision + 1;
    END LOOP;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'clipboard_items_revision'
                   AND tgrelid = 'clipboard_items'::regclass) THEN
        CREATE TRIGGER clipboard_items_revision
            AFTER INSERT OR UPDATE OR DELETE ON clipboard_items
            FOR EACH ROW EXECUTE FUNCTION clipboard_bump_user_revision();
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'clipboard_shares_revision'
                   AND tgrelid = 'clipboard_shares'::regclass) THEN
        CREATE TRIGGER clipboard_shares_revision
            AFTER INSERT OR UPDATE OR DELETE ON clipboard_shares
            FOR EACH ROW EXECUTE FUNCTION clipboard_bump_user_revision();
    END IF;
END;
$$;
