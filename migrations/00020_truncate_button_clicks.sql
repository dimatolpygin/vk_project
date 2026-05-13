-- +goose Up
TRUNCATE button_clicks;

-- +goose Down
SELECT 1; -- данные не восстанавливаются
