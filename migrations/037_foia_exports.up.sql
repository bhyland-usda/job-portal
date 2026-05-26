CREATE TABLE foia_exports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    requested_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    query_params JSONB NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    file_data BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    CHECK (status IN ('pending', 'processing', 'completed'))
);

CREATE INDEX idx_foia_exports_requested_by ON foia_exports(requested_by);
