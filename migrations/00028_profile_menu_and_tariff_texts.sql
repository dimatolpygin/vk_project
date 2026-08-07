-- +goose Up
-- +goose StatementBegin

-- Этап 4. Три несвязанные правки, которые обязаны приехать одной пачкой,
-- потому что все три меняют то, что пользователь видит на экране.

-- 1. Раскладки меню. Сохранённая в БД клавиатура перебивает каноническую
-- (label из БД выигрывает у label из кода), поэтому переименования и переезды
-- кнопок не доедут сами. Сбрасываем раскладку — EnsureDefaults на старте бота
-- поднимет её из определений экранов:
--   main_menu         — убраны «Запомнить фото» и «Настройки»
--   bottom_menu       — «Настройки» → «Мой профиль»
--   settings_overview — добавлено «Запомнить фото»
--   *_list / *_intro  — «Назад» → «В меню» (внизу теперь стоит «Назад» листалки)
UPDATE messages
SET keyboard   = NULL,
    buttons    = NULL,
    updated_at = now()
WHERE key IN (
    'main_menu',
    'bottom_menu',
    'settings_overview',
    'prompts_list',
    'ready_prompts_intro',
    'couple_categories'
);

-- 2. Пустой текст нижнего меню. ВК отбивает messages.send без текста и без
-- вложения ошибкой 100 — за пять дней 521 отказ, это единственный пустой экран
-- из семидесяти. После переезда профиля вниз нижнее меню стало единственным
-- входом в него, поэтому текст обязателен.
UPDATE messages
SET text       = 'Быстрое меню',
    updated_at = now()
WHERE key = 'bottom_menu'
  AND (text IS NULL OR btrim(text) = '');

-- 3. Хвосты «(5 штук)» в тарифах. Количество генераций уже стоит в названии,
-- в скобках оно дублировалось. Чистим и название, и описание.
UPDATE tariffs
SET name = btrim(regexp_replace(name, '\s*\([^)]*шт[^)]*\)', '', 'g'))
WHERE name ~ '\([^)]*шт[^)]*\)';

UPDATE tariffs
SET description = btrim(regexp_replace(description, '\s*\([^)]*шт[^)]*\)', '', 'g'))
WHERE description ~ '\([^)]*шт[^)]*\)';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Раскладки восстанавливаются из кода предыдущей версии тем же сбросом.
UPDATE messages
SET keyboard   = NULL,
    buttons    = NULL,
    updated_at = now()
WHERE key IN (
    'main_menu',
    'bottom_menu',
    'settings_overview',
    'prompts_list',
    'ready_prompts_intro',
    'couple_categories'
);

-- +goose StatementEnd
