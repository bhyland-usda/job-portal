CREATE TABLE posting_applications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    posting_id UUID NOT NULL REFERENCES postings(id) ON DELETE CASCADE,
    applicant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    cover_letter TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (posting_id, applicant_id),
    CHECK (status IN ('pending', 'shortlisted', 'accepted', 'rejected'))
);

CREATE INDEX idx_posting_applications_posting ON posting_applications(posting_id);
CREATE INDEX idx_posting_applications_applicant ON posting_applications(applicant_id);
