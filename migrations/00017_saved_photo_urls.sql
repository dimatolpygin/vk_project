-- +goose Up
ALTER TABLE users ADD COLUMN IF NOT EXISTS saved_photo_urls TEXT[] NOT NULL DEFAULT '{}';
UPDATE users
   SET saved_photo_urls = ARRAY[saved_photo_url]
 WHERE saved_photo_url IS NOT NULL AND saved_photo_url != '';

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS saved_photo_urls;
