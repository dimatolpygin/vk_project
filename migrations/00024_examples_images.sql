-- +goose Up
-- +goose StatementBegin

-- Раскладка меню примеров поменялась на две колонки (лимит ВК — 6 строк
-- у inline-клавиатуры). Сбрасываем сохранённую раскладку, чтобы подхватилась
-- каноническая из кода.
UPDATE messages
SET keyboard   = NULL,
    buttons    = NULL,
    updated_at = now()
WHERE key = 'examples_collage';

-- Картинка-заглушка для меню и всех шести категорий примеров.
-- Ставим только там, где картинки ещё нет, чтобы не затирать загруженное в админке.
UPDATE messages
SET image_url     = 'https://s3.ru1.storage.beget.cloud/bbd5f068f995-project/admin_uploads/1778658113_photo_2026-05-13_16-37-11.jpg',
    vk_attachment = NULL,
    updated_at    = now()
WHERE key IN (
        'examples_collage',
        'examples_self',
        'examples_couple',
        'examples_kids',
        'examples_edit',
        'examples_greetings',
        'examples_misc'
    )
  AND (image_url IS NULL OR image_url = '');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

UPDATE messages
SET image_url     = NULL,
    vk_attachment = NULL,
    updated_at    = now()
WHERE key IN (
        'examples_collage',
        'examples_self',
        'examples_couple',
        'examples_kids',
        'examples_edit',
        'examples_greetings',
        'examples_misc'
    )
  AND image_url = 'https://s3.ru1.storage.beget.cloud/bbd5f068f995-project/admin_uploads/1778658113_photo_2026-05-13_16-37-11.jpg';

-- +goose StatementEnd
