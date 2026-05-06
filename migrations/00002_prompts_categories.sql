-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS categories (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL,
    group_id    INT,
    preview_url TEXT,
    gender      TEXT DEFAULT 'any' CHECK (gender IN ('male','female','any')),
    sort_order  INT DEFAULT 0,
    is_active   BOOLEAN DEFAULT TRUE
);

CREATE TABLE IF NOT EXISTS prompts (
    id          SERIAL PRIMARY KEY,
    category_id INT REFERENCES categories(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    prompt      TEXT NOT NULL,
    preview_url TEXT,
    gender      TEXT DEFAULT 'any' CHECK (gender IN ('male','female','any')),
    sort_order  INT DEFAULT 0,
    is_active   BOOLEAN DEFAULT TRUE
);

CREATE INDEX IF NOT EXISTS idx_prompts_category ON prompts(category_id);

-- Seed demo categories
INSERT INTO categories (name, gender, sort_order) VALUES
    ('Деловой стиль', 'any', 1),
    ('Природа и пейзажи', 'any', 2),
    ('Романтика', 'female', 3),
    ('Спорт и активность', 'any', 4)
ON CONFLICT DO NOTHING;

-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS prompts;
DROP TABLE IF EXISTS categories;
