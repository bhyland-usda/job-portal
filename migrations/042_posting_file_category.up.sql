-- Add category column to existing posting_attachments for rendering logic
ALTER TABLE posting_attachments ADD COLUMN IF NOT EXISTS file_category VARCHAR(20) NOT NULL DEFAULT 'document';
