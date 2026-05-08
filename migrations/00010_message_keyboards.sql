-- +goose Up
ALTER TABLE messages ADD COLUMN IF NOT EXISTS keyboard JSONB;

-- +goose Down
ALTER TABLE messages DROP COLUMN IF EXISTS keyboard;
