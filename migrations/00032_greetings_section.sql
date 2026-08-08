-- +goose Up
-- +goose StatementBegin

-- Раздел «Поздравления», три уровня: адресат → праздник → промты.
--
-- Адресат задаёт пол промтов через prompt_gender, и он наследуется вниз по дереву,
-- поэтому у праздников пол не проставляется: «День рождения» под «Мужчине»
-- отдаст мужские промты, тот же праздник под «Женщине» — женские.
-- Пол пользователя здесь не участвует вовсе.
--
-- Праздники заводятся стартовым набором, чтобы структура была видна и её было
-- с чего править: заказчик переименовывает, удаляет и добавляет их из админки.
-- Пока в празднике нет промтов, он показывает экран «Скоро» из этапа 6.

ALTER TABLE generations DROP CONSTRAINT IF EXISTS generations_type_check;
ALTER TABLE generations ADD CONSTRAINT generations_type_check
    CHECK (type IN ('free', 'ready_prompt', 'custom', 'edit', 'couple', 'family', 'kids', 'greetings'));

-- screen_key у адресатов: их открытие показывает список праздников,
-- у него свой экран, отдельный от входа в раздел.
INSERT INTO categories (name, gender, sort_order, is_active, section, prompt_gender, screen_key)
VALUES
    ('Мужчине', 'any', 1, true, 'greetings', 'male',   'greetings_holidays'),
    ('Женщине', 'any', 2, true, 'greetings', 'female', 'greetings_holidays');

INSERT INTO categories (name, gender, sort_order, is_active, section, parent_id)
SELECT holiday.name, 'any', holiday.sort_order, true, 'greetings', addressee.id
FROM (
    SELECT id, name FROM categories
    WHERE section = 'greetings' AND parent_id IS NULL AND name IN ('Мужчине', 'Женщине')
) AS addressee
CROSS JOIN LATERAL (
    VALUES
        ('День рождения', 1),
        ('Новый год', 2),
        ('23 февраля', 3),
        ('8 марта', 4)
) AS holiday(name, sort_order)
-- 23 февраля показываем только мужчинам, 8 марта — только женщинам.
WHERE NOT (addressee.name = 'Женщине' AND holiday.name = '23 февраля')
  AND NOT (addressee.name = 'Мужчине' AND holiday.name = '8 марта');

-- Кнопка раздела приезжает из определения экрана, а сохранённая в БД раскладка
-- перебивает код (см. этап 4), поэтому сбрасываем её под пересборку.
UPDATE messages SET keyboard = NULL WHERE key = 'main_menu';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Сначала дети, потом корни: ON DELETE CASCADE тут сработал бы и сам,
-- но порядок делает намерение явным.
DELETE FROM categories WHERE section = 'greetings' AND parent_id IS NOT NULL;
DELETE FROM categories WHERE section = 'greetings';

UPDATE messages SET keyboard = NULL WHERE key = 'main_menu';

ALTER TABLE generations DROP CONSTRAINT IF EXISTS generations_type_check;
ALTER TABLE generations ADD CONSTRAINT generations_type_check
    CHECK (type IN ('free', 'ready_prompt', 'custom', 'edit', 'couple', 'family', 'kids'));

-- +goose StatementEnd
