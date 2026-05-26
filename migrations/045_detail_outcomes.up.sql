ALTER TABLE postings
    ADD COLUMN outcome TEXT,
    ADD COLUMN outcome_status VARCHAR(20),
    ADD COLUMN completed_at TIMESTAMPTZ;
