-- +goose Up
-- +goose StatementBegin

-- Узел дерева получает собственный экран на каждом своём шаге, а не на одном.
--
-- До этой миграции у узла был ровно один screen_key — экран входа, и работал он
-- только когда у узла есть активные дети (см. openCategoryNode). Отсюда два бага
-- с прода:
--   * тренды: подраздел показывает промты сразу, детей у него нет, поэтому своя
--     картинка и описание не показывались никогда — уходил общий prompts_list;
--   * детский режим: после выбора промта бот просит фото общим текстом
--     «Теперь загрузите своё фото», и персонализировать его под «Мальчика»
--     и «Девочку» было нечем.
--
-- Шага у узла три — подменю, список промтов, запрос фото, — значит и ключей
-- экрана три. NULL в любом означает прежнее поведение: экран раздела.

ALTER TABLE categories ADD COLUMN IF NOT EXISTS prompts_screen_key TEXT;
ALTER TABLE categories ADD COLUMN IF NOT EXISTS photo_screen_key TEXT;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Экраны, созданные под эти ключи, остаются в messages: удалять чужой контент
-- на откате нельзя, а лишняя запись экрана никому не мешает.
ALTER TABLE categories DROP COLUMN IF EXISTS photo_screen_key;
ALTER TABLE categories DROP COLUMN IF EXISTS prompts_screen_key;

-- +goose StatementEnd
