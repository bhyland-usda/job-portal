-- Link previews: cached Open Graph metadata for the first http(s) URL in a post.
-- Keyed by URL so the same link shared by multiple users is fetched once.
CREATE TABLE IF NOT EXISTS link_previews (
    url         TEXT        PRIMARY KEY,
    title       TEXT,
    description TEXT,
    image_url   TEXT,
    fetched_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
