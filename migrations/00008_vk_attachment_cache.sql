-- +goose Up
ALTER TABLE messages ADD COLUMN IF NOT EXISTS vk_attachment TEXT;

-- +goose Down
ALTER TABLE messages DROP COLUMN IF EXISTS vk_attachment;
