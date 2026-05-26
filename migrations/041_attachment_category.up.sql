-- Add category column to existing post_attachments for rendering logic
ALTER TABLE post_attachments ADD COLUMN IF NOT EXISTS file_category VARCHAR(20) NOT NULL DEFAULT 'image';
