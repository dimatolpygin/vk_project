-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS button_clicks (
    id         BIGSERIAL PRIMARY KEY,
    user_vk_id BIGINT,
    payload    TEXT,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_button_clicks_payload ON button_clicks(payload);
CREATE INDEX IF NOT EXISTS idx_button_clicks_created ON button_clicks(created_at);

-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS button_clicks;
