-- Hardening: prevent duplicate 1:1 (direct) conversations between the same pair.
-- A direct conversation has exactly two participants. We store a canonical,
-- order-independent key (the two user ids sorted ascending, joined by '-') and
-- enforce uniqueness on it via a partial index. Group conversations leave
-- direct_key NULL and remain unconstrained.

ALTER TABLE conversations ADD COLUMN IF NOT EXISTS direct_key TEXT;

-- Backfill existing direct conversations (exactly two participants). Ordering by
-- the text form matches the application's directKey() string comparison.
UPDATE conversations c
SET direct_key = sub.pair
FROM (
    SELECT conversation_id,
           string_agg(user_id::text, '-' ORDER BY user_id::text) AS pair
    FROM conversation_participants
    GROUP BY conversation_id
    HAVING COUNT(*) = 2
) sub
WHERE c.id = sub.conversation_id
  AND c.direct_key IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS ux_conversations_direct_key
    ON conversations (direct_key)
    WHERE direct_key IS NOT NULL;
