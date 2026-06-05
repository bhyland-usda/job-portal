CREATE TABLE IF NOT EXISTS skill_aliases (
    alias_skill TEXT PRIMARY KEY,
    canonical_skill TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (alias_skill = lower(alias_skill)),
    CHECK (canonical_skill = lower(canonical_skill))
);

CREATE TABLE IF NOT EXISTS skill_adjacency (
    from_skill TEXT NOT NULL,
    to_skill TEXT NOT NULL,
    weight DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (from_skill, to_skill),
    CHECK (from_skill = lower(from_skill)),
    CHECK (to_skill = lower(to_skill)),
    CHECK (from_skill <> to_skill),
    CHECK (weight > 0 AND weight <= 1)
);

CREATE INDEX IF NOT EXISTS idx_skill_aliases_canonical
    ON skill_aliases(canonical_skill);

CREATE INDEX IF NOT EXISTS idx_skill_adjacency_to
    ON skill_adjacency(to_skill);

CREATE INDEX IF NOT EXISTS idx_skill_adjacency_from_weight
    ON skill_adjacency(from_skill, weight DESC);
