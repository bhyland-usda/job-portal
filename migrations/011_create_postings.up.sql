CREATE TABLE postings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    type VARCHAR(20) NOT NULL,
    location VARCHAR(255),
    department VARCHAR(255),
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE posting_skills (
    posting_id UUID NOT NULL REFERENCES postings(id) ON DELETE CASCADE,
    skill_name VARCHAR(100) NOT NULL,
    PRIMARY KEY (posting_id, skill_name)
);

CREATE INDEX idx_postings_author ON postings(author_id);
CREATE INDEX idx_postings_status ON postings(status);
CREATE INDEX idx_postings_created ON postings(created_at DESC);
CREATE INDEX idx_postings_skills_name ON posting_skills(skill_name);
