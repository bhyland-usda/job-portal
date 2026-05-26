CREATE TABLE IF NOT EXISTS posting_attachments (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    posting_id    UUID        NOT NULL REFERENCES postings(id) ON DELETE CASCADE,
    file_data     BYTEA       NOT NULL,
    original_name VARCHAR(255) NOT NULL,
    content_type  VARCHAR(100) NOT NULL,
    file_size     INTEGER     NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_posting_attachments_posting
    ON posting_attachments(posting_id);
