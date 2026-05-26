CREATE TABLE departments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    parent_id UUID REFERENCES departments(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_departments_parent ON departments(parent_id);

ALTER TABLE users
    ADD COLUMN department_id UUID REFERENCES departments(id) ON DELETE SET NULL;

INSERT INTO departments (id, name) VALUES
    (gen_random_uuid(), 'NRCS'),
    (gen_random_uuid(), 'Forest Service'),
    (gen_random_uuid(), 'ARS'),
    (gen_random_uuid(), 'APHIS'),
    (gen_random_uuid(), 'FSA'),
    (gen_random_uuid(), 'Rural Development'),
    (gen_random_uuid(), 'FNS'),
    (gen_random_uuid(), 'FSIS');
