-- +goose Up
-- +goose StatementBegin

-- Экран examples_collage из одиночного коллажа превратился в меню категорий.
-- В БД осталась старая раскладка, где «Назад» стоял на нулевой строке — она
-- перебивала новую и выносила кнопку наверх, в один ряд с первой категорией.
-- Сбрасываем сохранённую клавиатуру: при следующем чтении подхватится
-- каноническая раскладка из кода (категории сверху, «Назад» — последней строкой).
UPDATE messages
SET keyboard   = NULL,
    buttons    = NULL,
    updated_at = now()
WHERE key = 'examples_collage';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

SELECT 1;

-- +goose StatementEnd
