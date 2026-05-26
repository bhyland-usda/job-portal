CREATE TABLE IF NOT EXISTS articles (
    id         UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id  UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title      VARCHAR(255) NOT NULL,
    content    TEXT         NOT NULL,
    published  BOOLEAN      NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_articles_author_created
    ON articles(author_id, created_at DESC);
