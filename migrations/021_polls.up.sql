CREATE TABLE IF NOT EXISTS polls (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id  UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    question   TEXT        NOT NULL,
    closes_at  TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS poll_options (
    id         UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    poll_id    UUID         NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
    label      VARCHAR(255) NOT NULL,
    sort_order INTEGER      NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_poll_options_poll_id
    ON poll_options(poll_id);

CREATE TABLE IF NOT EXISTS poll_votes (
    poll_id    UUID        NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    option_id  UUID        NOT NULL REFERENCES poll_options(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (poll_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_poll_votes_poll_id
    ON poll_votes(poll_id);
