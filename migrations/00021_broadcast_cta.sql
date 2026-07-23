-- +goose Up
-- +goose StatementBegin

ALTER TABLE broadcasts
    ADD COLUMN IF NOT EXISTS cta_enabled BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE broadcasts
    DROP COLUMN IF EXISTS cta_enabled;

-- +goose StatementEnd
