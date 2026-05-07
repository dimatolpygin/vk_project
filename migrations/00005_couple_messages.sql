-- +goose Up
-- +goose StatementBegin

INSERT INTO messages (key, text) VALUES
    ('couple_pair_styles',
     '👫 Пара — выбери стиль фотосессии:'),
    ('couple_family_styles',
     '👨‍👩‍👧 Семейное фото — выбери стиль фотосессии:')
ON CONFLICT (key) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
DELETE FROM messages WHERE key IN ('couple_pair_styles', 'couple_family_styles');
