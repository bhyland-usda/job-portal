CREATE TABLE IF NOT EXISTS message_attachments (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id      UUID        NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    conversation_id UUID        NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sender_id       UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    file_data       BYTEA       NOT NULL,
    original_name   VARCHAR(255) NOT NULL,
    content_type    VARCHAR(100) NOT NULL,
    file_category   VARCHAR(20) NOT NULL DEFAULT 'document',
    file_size       INTEGER     NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_message_attachments_message
    ON message_attachments(message_id);
CREATE INDEX IF NOT EXISTS idx_message_attachments_conversation
    ON message_attachments(conversation_id);
CREATE INDEX IF NOT EXISTS idx_message_attachments_sender
    ON message_attachments(sender_id);
