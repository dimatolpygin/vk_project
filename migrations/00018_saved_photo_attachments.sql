-- +goose Up
ALTER TABLE users ADD COLUMN IF NOT EXISTS saved_photo_attachments TEXT[] NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS saved_photo_attachments;
