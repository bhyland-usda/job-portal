CREATE TABLE IF NOT EXISTS skill_endorsements (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    skill_id    UUID        NOT NULL REFERENCES skills(id) ON DELETE CASCADE,
    endorser_id UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (skill_id, endorser_id)
);

CREATE INDEX IF NOT EXISTS idx_skill_endorsements_skill_id
    ON skill_endorsements(skill_id);
