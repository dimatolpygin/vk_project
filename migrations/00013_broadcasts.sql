-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS broadcasts (
    id                BIGSERIAL PRIMARY KEY,
    audience_filter   TEXT NOT NULL CHECK (audience_filter IN ('all', 'free', 'paid')),
    text              TEXT NOT NULL,
    image_url         TEXT,
    vk_attachment     TEXT,
    status            TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'processing', 'completed', 'completed_with_errors')),
    total_recipients  INT NOT NULL DEFAULT 0,
    sent_count        INT NOT NULL DEFAULT 0,
    failed_count      INT NOT NULL DEFAULT 0,
    last_error        TEXT,
    started_at        TIMESTAMPTZ,
    completed_at      TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_broadcasts_status_created_at
    ON broadcasts(status, created_at DESC);

CREATE TABLE IF NOT EXISTS broadcast_deliveries (
    id                    BIGSERIAL PRIMARY KEY,
    broadcast_id          BIGINT NOT NULL REFERENCES broadcasts(id) ON DELETE CASCADE,
    user_vk_id            BIGINT NOT NULL,
    status                TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'sent', 'failed')),
    attempts              INT NOT NULL DEFAULT 0,
    error_text            TEXT,
    processing_started_at TIMESTAMPTZ,
    sent_at               TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_broadcast_deliveries_broadcast_status
    ON broadcast_deliveries(broadcast_id, status, id);

CREATE INDEX IF NOT EXISTS idx_broadcast_deliveries_processing_started
    ON broadcast_deliveries(status, processing_started_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS broadcast_deliveries;
DROP TABLE IF EXISTS broadcasts;
-- +goose StatementEnd
