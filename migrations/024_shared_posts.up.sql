CREATE TABLE IF NOT EXISTS shared_posts (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    original_post_id UUID        NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    commentary       TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_shared_posts_user_id          ON shared_posts(user_id);
CREATE INDEX IF NOT EXISTS idx_shared_posts_original_post_id ON shared_posts(original_post_id);
