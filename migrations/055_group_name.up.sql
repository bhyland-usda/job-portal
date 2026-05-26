-- Group messaging: optional display name for group conversations.
-- A group conversation (more than two participants) may carry a human-friendly
-- name; 1:1 (direct) conversations leave this NULL and fall back to the
-- participant-name display. Nullable so it does not affect existing rows.
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS name TEXT;
