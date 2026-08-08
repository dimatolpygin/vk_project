-- +goose Up
-- +goose StatementBegin

-- Раздел «Детские фото»: два корня, «Мальчик» и «Девочка». Пол здесь задаётся
-- кнопкой подменю, а не полом пользователя — ровно тот случай, ради которого
-- в 00029 заводилась колонка prompt_gender.
--
-- Узлы включаются пустыми: без промтов они показывают экран section_soon
-- («Скоро») из этапа 6, поэтому раздел не ждёт текстов от заказчика.

ALTER TABLE generations DROP CONSTRAINT IF EXISTS generations_type_check;
ALTER TABLE generations ADD CONSTRAINT generations_type_check
    CHECK (type IN ('free', 'ready_prompt', 'custom', 'edit', 'couple', 'family', 'kids'));

INSERT INTO categories (name, gender, sort_order, is_active, section, prompt_gender)
VALUES
    -- gender остаётся 'any': раздел и пол промтов теперь живут в своих колонках,
    -- а gender нужен только старым выборкам по разделу «Фото для себя».
    ('Мальчик', 'any', 1, true, 'kids', 'male'),
    ('Девочка', 'any', 2, true, 'kids', 'female');

-- Кнопка «Детские фото» приезжает из определения экрана, но сохранённая в БД
-- раскладка перебивает код (см. этап 4), поэтому сбрасываем её — EnsureDefaults
-- соберёт клавиатуру заново уже с новой кнопкой.
UPDATE messages SET keyboard = NULL WHERE key = 'main_menu';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DELETE FROM categories WHERE section = 'kids';

UPDATE messages SET keyboard = NULL WHERE key = 'main_menu';

ALTER TABLE generations DROP CONSTRAINT IF EXISTS generations_type_check;
ALTER TABLE generations ADD CONSTRAINT generations_type_check
    CHECK (type IN ('free', 'ready_prompt', 'custom', 'edit', 'couple', 'family'));

-- +goose StatementEnd
