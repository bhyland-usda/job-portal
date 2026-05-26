CREATE TABLE IF NOT EXISTS news_articles (
    id         UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id  UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title      VARCHAR(255) NOT NULL,
    content    TEXT         NOT NULL,
    published  BOOLEAN      NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_news_articles_created_published
    ON news_articles(created_at DESC, published);
