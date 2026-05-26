CREATE TABLE IF NOT EXISTS accomplishments (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title        VARCHAR(255) NOT NULL,
    description  TEXT         NOT NULL,
    period_type  VARCHAR(20)  NOT NULL CHECK (period_type IN ('quarterly', 'yearly')),
    period_start DATE         NOT NULL,
    period_end   DATE         NOT NULL,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_accomplishments_user_period
    ON accomplishments(user_id, period_start);
