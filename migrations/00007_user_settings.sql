-- +goose Up
-- +goose StatementBegin
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS pref_model        TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS pref_resolution   TEXT NOT NULL DEFAULT '1k',
    ADD COLUMN IF NOT EXISTS pref_aspect_ratio TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users
    DROP COLUMN IF EXISTS pref_model,
    DROP COLUMN IF EXISTS pref_resolution,
    DROP COLUMN IF EXISTS pref_aspect_ratio;
-- +goose StatementEnd

