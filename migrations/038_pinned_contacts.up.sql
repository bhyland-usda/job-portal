CREATE TABLE IF NOT EXISTS pinned_contacts (
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    contact_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    pinned_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, contact_id),
    CHECK (user_id != contact_id)
);

CREATE INDEX IF NOT EXISTS idx_pinned_contacts_user ON pinned_contacts(user_id);
