-- Scheduled posts: a future scheduled_at hides the post from the social feed
-- until that time. NULL means the post is published immediately.
ALTER TABLE posts ADD COLUMN IF NOT EXISTS scheduled_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_posts_scheduled_at ON posts(scheduled_at);
