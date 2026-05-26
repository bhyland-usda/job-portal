ALTER TABLE users ADD COLUMN IF NOT EXISTS manager_id UUID REFERENCES users(id);

CREATE INDEX IF NOT EXISTS idx_users_manager ON users(manager_id);
