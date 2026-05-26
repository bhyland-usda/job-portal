CREATE TABLE employee_spotlights (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    week_of DATE NOT NULL,
    reason TEXT,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (week_of)
);

CREATE INDEX idx_employee_spotlights_week ON employee_spotlights(week_of DESC);
