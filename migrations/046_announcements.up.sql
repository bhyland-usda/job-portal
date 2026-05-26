CREATE TABLE IF NOT EXISTS announcements (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    title      TEXT        NOT NULL,
    body       TEXT        NOT NULL,
    author_id  UUID        REFERENCES users(id),
    is_pinned  BOOLEAN     NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_announcements_pinned_created
    ON announcements(is_pinned DESC, created_at DESC);
