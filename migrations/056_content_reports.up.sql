CREATE TABLE IF NOT EXISTS content_reports (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    reporter_id  UUID        REFERENCES users(id),
    content_type TEXT        NOT NULL,
    content_id   UUID        NOT NULL,
    reason       TEXT,
    status       TEXT        NOT NULL DEFAULT 'pending',
    reviewed_by  UUID        REFERENCES users(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_content_reports_status_created
    ON content_reports(status, created_at DESC);
