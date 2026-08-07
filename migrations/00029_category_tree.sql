-- +goose Up
-- +goose StatementBegin

-- Категории превращаются из плоского списка в дерево. Раньше «раздел» подделывался
-- колонкой gender ('couple' против 'any'/'male'/'female'), из-за чего в 00006
-- пришлось ослаблять CHECK. Теперь раздел — отдельная колонка, а вложенность —
-- parent_id.

ALTER TABLE categories ADD COLUMN IF NOT EXISTS parent_id INT REFERENCES categories(id) ON DELETE CASCADE;
ALTER TABLE categories ADD COLUMN IF NOT EXISTS section TEXT;
ALTER TABLE categories ADD COLUMN IF NOT EXISTS screen_key TEXT;
ALTER TABLE categories ADD COLUMN IF NOT EXISTS media_kind TEXT NOT NULL DEFAULT 'photo';
ALTER TABLE categories ADD COLUMN IF NOT EXISTS prompt_gender TEXT;

ALTER TABLE categories DROP CONSTRAINT IF EXISTS categories_media_kind_check;
ALTER TABLE categories ADD CONSTRAINT categories_media_kind_check
    CHECK (media_kind IN ('photo', 'video'));

-- prompt_gender — «пол задаётся кнопкой подменю», а не полом пользователя.
-- Сейчас это парные категории, дальше — адресат поздравления (мужчине/женщине).
-- NULL означает «берём пол пользователя», значение наследуется вниз по дереву.
ALTER TABLE categories DROP CONSTRAINT IF EXISTS categories_prompt_gender_check;
ALTER TABLE categories ADD CONSTRAINT categories_prompt_gender_check
    CHECK (prompt_gender IS NULL OR prompt_gender IN ('male', 'female', 'any', 'couple'));

-- Существующие категории становятся корнями своих разделов: parent_id остаётся NULL,
-- раздел выводится из текущего gender. Так выборки после миграции дают ровно те же
-- наборы, что и до неё.
UPDATE categories
SET section = CASE WHEN gender = 'couple' THEN 'couple' ELSE 'self' END
WHERE section IS NULL;

UPDATE categories SET prompt_gender = 'couple'
WHERE gender = 'couple' AND prompt_gender IS NULL;

ALTER TABLE categories ALTER COLUMN section SET DEFAULT 'self';
ALTER TABLE categories ALTER COLUMN section SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_categories_parent ON categories(parent_id, sort_order);
CREATE INDEX IF NOT EXISTS idx_categories_section ON categories(section, sort_order);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_categories_section;
DROP INDEX IF EXISTS idx_categories_parent;

ALTER TABLE categories DROP CONSTRAINT IF EXISTS categories_prompt_gender_check;
ALTER TABLE categories DROP CONSTRAINT IF EXISTS categories_media_kind_check;

ALTER TABLE categories DROP COLUMN IF EXISTS prompt_gender;
ALTER TABLE categories DROP COLUMN IF EXISTS media_kind;
ALTER TABLE categories DROP COLUMN IF EXISTS screen_key;
ALTER TABLE categories DROP COLUMN IF EXISTS section;
ALTER TABLE categories DROP COLUMN IF EXISTS parent_id;

-- +goose StatementEnd
