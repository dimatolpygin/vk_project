-- +goose Up
-- +goose StatementBegin

-- Раздел «Тренды»: четыре узла корнями раздела.
--
-- Работает пока только «Фото тренды». Три видео-узла заводятся сразу, но с
-- is_active = false и media_kind = 'video': в боте их не видно, в админке они
-- есть, и заказчик может наполнять их промтами заранее. Здесь именно скрытие,
-- а не заглушка «Скоро» из этапа 6: за видео-кнопками нет не контента, а самого
-- движка генерации — обещать «скоро» до этапа 10 нечестно.
--
-- Промтов у «Фото трендов» пока нет, поэтому узел покажет экран «Скоро» —
-- раздел включается пустым и наполняется из админки без релиза.

ALTER TABLE generations DROP CONSTRAINT IF EXISTS generations_type_check;
ALTER TABLE generations ADD CONSTRAINT generations_type_check
    CHECK (type IN ('free', 'ready_prompt', 'custom', 'edit', 'couple', 'family', 'kids', 'greetings', 'trends'));

INSERT INTO categories (name, gender, sort_order, is_active, section, media_kind)
VALUES
    ('📸 Фото тренды',  'any', 1, true,  'trends', 'photo'),
    ('🎬 Видео тренды', 'any', 2, false, 'trends', 'video'),
    ('✨ Оживить фото', 'any', 3, false, 'trends', 'video'),
    ('💃 Танцы по фото','any', 4, false, 'trends', 'video');

-- Кнопка раздела приезжает из определения экрана, а сохранённая в БД раскладка
-- перебивает код (см. этап 4), поэтому сбрасываем её под пересборку.
UPDATE messages SET keyboard = NULL WHERE key = 'main_menu';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Сначала дети, потом корни: ON DELETE CASCADE тут сработал бы и сам,
-- но порядок делает намерение явным.
DELETE FROM categories WHERE section = 'trends' AND parent_id IS NOT NULL;
DELETE FROM categories WHERE section = 'trends';

UPDATE messages SET keyboard = NULL WHERE key = 'main_menu';

ALTER TABLE generations DROP CONSTRAINT IF EXISTS generations_type_check;
ALTER TABLE generations ADD CONSTRAINT generations_type_check
    CHECK (type IN ('free', 'ready_prompt', 'custom', 'edit', 'couple', 'family', 'kids', 'greetings'));

-- +goose StatementEnd
