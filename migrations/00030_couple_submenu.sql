-- +goose Up
-- +goose StatementBegin

-- За кнопкой «Парное/семейное фото» появляется промежуточное меню из трёх пунктов.
-- Все существующие парные категории уезжают под «Парное фото», два новых узла
-- остаются пустыми — заказчик наполняет их промтами из админки без релиза.
--
-- Три узла заводятся корнями раздела couple, а не детьми одного технического
-- корня: навигатор этапа 5 показывает корни раздела сразу после загрузки фото,
-- поэтому лишний уровень означал бы лишний тап.
--
-- prompt_gender = 'couple' наследуется вниз по дереву, поэтому промты под
-- «Парным фото» отбираются так же, как до миграции.

-- Перенос делается ДО вставки новых узлов: так под родителя не попадут
-- сами новые узлы, и условие остаётся простым.
CREATE TEMP TABLE couple_roots_before ON COMMIT DROP AS
SELECT id FROM categories WHERE section = 'couple' AND parent_id IS NULL;

INSERT INTO categories (name, gender, sort_order, is_active, section, prompt_gender, screen_key)
VALUES
    -- screen_key: внутри «Парного фото» показывается прежний экран выбора
    -- категории, чтобы текст «Теперь выбери категорию» остался на своём уровне.
    ('Парное фото',    'couple', 1, true, 'couple', 'couple', 'couple_categories'),
    ('Семейное фото',  'couple', 2, true, 'couple', 'couple', NULL),
    ('Фото поколений', 'couple', 3, true, 'couple', 'couple', NULL);

UPDATE categories
SET parent_id = (
    SELECT id FROM categories
    WHERE section = 'couple' AND parent_id IS NULL AND name = 'Парное фото'
    ORDER BY id DESC LIMIT 1
)
WHERE id IN (SELECT id FROM couple_roots_before);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Дети возвращаются в корень раздела, три новых узла удаляются.
-- ON DELETE CASCADE у parent_id снёс бы вместе с ними все парные категории,
-- поэтому порядок здесь важен.
UPDATE categories AS c
SET parent_id = NULL
FROM categories AS p
WHERE c.parent_id = p.id
  AND p.section = 'couple'
  AND p.parent_id IS NULL
  AND p.name IN ('Парное фото', 'Семейное фото', 'Фото поколений');

DELETE FROM categories
WHERE section = 'couple'
  AND parent_id IS NULL
  AND name IN ('Парное фото', 'Семейное фото', 'Фото поколений');

-- +goose StatementEnd
