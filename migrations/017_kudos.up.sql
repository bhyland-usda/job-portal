CREATE TABLE IF NOT EXISTS kudos (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    sender_id   UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    receiver_id UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    message     TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT no_self_kudos CHECK (sender_id <> receiver_id)
);

CREATE INDEX IF NOT EXISTS idx_kudos_receiver_id ON kudos(receiver_id);
CREATE INDEX IF NOT EXISTS idx_kudos_sender_id   ON kudos(sender_id);
