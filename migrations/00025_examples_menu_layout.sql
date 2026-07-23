-- +goose Up
-- +goose StatementBegin

-- Раскладка меню примеров снова изменилась: длинные названия по одной кнопке
-- в строке, короткие «Разное» и «Назад» — общим последним рядом (лимит ВК —
-- 6 строк у inline-клавиатуры). Сохранённая в БД раскладка перебивает
-- каноническую, поэтому сбрасываем её.
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
