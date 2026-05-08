-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS activity_events (
    id         BIGSERIAL PRIMARY KEY,
    user_vk_id BIGINT NOT NULL,
    event_type TEXT NOT NULL,
    action_key TEXT,
    screen_key TEXT,
    meta       JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_activity_events_user_created
    ON activity_events(user_vk_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_activity_events_type_created
    ON activity_events(event_type, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_activity_events_action_created
    ON activity_events(action_key, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_activity_events_screen_created
    ON activity_events(screen_key, created_at DESC);

-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS activity_events;
