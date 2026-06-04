ALTER TABLE postings
    ADD COLUMN IF NOT EXISTS start_date DATE,
    ADD COLUMN IF NOT EXISTS duration_days INTEGER,
    ADD COLUMN IF NOT EXISTS reporting_manager_name VARCHAR(255),
    ADD COLUMN IF NOT EXISTS reporting_manager_email VARCHAR(255),
    ADD COLUMN IF NOT EXISTS location_type VARCHAR(20),
    ADD COLUMN IF NOT EXISTS application_close_date DATE,
    ADD COLUMN IF NOT EXISTS number_of_people INTEGER,
    ADD COLUMN IF NOT EXISTS learning_outcomes TEXT;

ALTER TABLE posting_applications
    ADD COLUMN IF NOT EXISTS selection_why TEXT,
    ADD COLUMN IF NOT EXISTS project_tackle_approach TEXT;

-- Posting validation constraints
ALTER TABLE postings
    ADD CONSTRAINT postings_duration_days_positive
        CHECK (duration_days IS NULL OR duration_days > 0),
    ADD CONSTRAINT postings_number_of_people_positive
        CHECK (number_of_people IS NULL OR number_of_people > 0),
    ADD CONSTRAINT postings_location_type_valid
        CHECK (
            location_type IS NULL OR
            location_type IN ('remote', 'hybrid', 'onsite')
        ),
    ADD CONSTRAINT postings_reporting_manager_email_format
        CHECK (
            reporting_manager_email IS NULL OR
            reporting_manager_email ~* '^[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}$'
        ),
    ADD CONSTRAINT postings_application_close_not_before_start
        CHECK (
            start_date IS NULL OR
            application_close_date IS NULL OR
            application_close_date <= start_date
        );

-- Helpful indexes for common manager/search flows
CREATE INDEX IF NOT EXISTS idx_postings_application_close_date
    ON postings(application_close_date);

CREATE INDEX IF NOT EXISTS idx_postings_location_type_status
    ON postings(location_type, status);

CREATE INDEX IF NOT EXISTS idx_posting_applications_status_updated
    ON posting_applications(status, updated_at DESC);