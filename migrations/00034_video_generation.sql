-- +goose Up
-- +goose StatementBegin

-- Видео-генерация: шаблон хранит два промта сразу.
--
-- Схема, выбранная заказчиком: пользователь выбирает шаблон ровно как в фото-разделах,
-- присылает свои фото, фото-модель собирает по `prompt` сцену, готовая сцена уходит
-- в Seedance с `video_prompt` — пользователь получает видео. Поэтому `prompt`
-- остаётся промтом для фото и у видео-шаблонов тоже: это не два разных промта на
-- выбор, а два звена одной цепочки.
--
-- Параметры видео зафиксированы в коде (9:16, 720p, 10 секунд) и пользователю
-- не предлагаются, поэтому колонок под них здесь нет.

ALTER TABLE prompts ADD COLUMN IF NOT EXISTS media_kind TEXT NOT NULL DEFAULT 'photo';
ALTER TABLE prompts DROP CONSTRAINT IF EXISTS prompts_media_kind_check;
ALTER TABLE prompts ADD CONSTRAINT prompts_media_kind_check
    CHECK (media_kind IN ('photo', 'video'));

ALTER TABLE prompts ADD COLUMN IF NOT EXISTS video_prompt TEXT;

-- Цена промта в генерациях (п. 8.3 ТЗ). Фото стоит одну, видео — десятки,
-- и цена может отличаться от тренда к тренду, поэтому она на промте, а не в коде.
ALTER TABLE prompts ADD COLUMN IF NOT EXISTS price_gens INT NOT NULL DEFAULT 1;
ALTER TABLE prompts DROP CONSTRAINT IF EXISTS prompts_price_gens_check;
ALTER TABLE prompts ADD CONSTRAINT prompts_price_gens_check CHECK (price_gens >= 1);

ALTER TABLE generations ADD COLUMN IF NOT EXISTS output_video_url TEXT;

-- Сколько генераций стоила эта задача и с каких балансов они сняты. Одной
-- колонки charged_balance_kind перестало хватать: списание в 40 генераций может
-- не поместиться целиком ни в платный баланс, ни в бесплатный, и тогда оно
-- разъезжается по обоим. Возврат должен вернуть ровно то, что снял.
ALTER TABLE generations ADD COLUMN IF NOT EXISTS cost_gens INT NOT NULL DEFAULT 1;
ALTER TABLE generations ADD COLUMN IF NOT EXISTS charged_paid_gens INT NOT NULL DEFAULT 0;
ALTER TABLE generations ADD COLUMN IF NOT EXISTS charged_free_gens INT NOT NULL DEFAULT 0;

ALTER TABLE generations DROP CONSTRAINT IF EXISTS generations_type_check;
ALTER TABLE generations ADD CONSTRAINT generations_type_check
    CHECK (type IN ('free', 'ready_prompt', 'custom', 'edit', 'couple', 'family', 'kids', 'greetings', 'trends', 'video'));

-- Три видео-узла «Трендов» лежали выключенными с этапа 9 — движок появился,
-- включаем. Промтов в них пока нет, поэтому до наполнения из админки они
-- показывают экран «Скоро».
UPDATE categories SET is_active = true WHERE section = 'trends' AND media_kind = 'video';

-- Видеотариф (п. 8.2 ТЗ). Считано из себестоимости: Seedance 10 с 720p — $2.40,
-- сцена на nano banana в 1k — $0.06, итого $2.46 ≈ 197₽ по курсу 80₽/$.
-- Одна фото-генерация стоит нам те же $0.06 ≈ 5₽ и продаётся по 16.6–27.8₽,
-- то есть наценка в сетке 3.5–5.6×. Видео при 40 генерациях и цене 690₽ даёт
-- 17.25₽ за генерацию — ровно уровень пакета «30 фотографий», то есть купить
-- видеотариф ради фото невыгодно, а наценка выходит 3.5×.
-- Цифры — расчётный дефолт, заказчик правит их из админки без релиза.
INSERT INTO tariffs (name, description, price, gens_count, is_active, sort_order)
VALUES ('1 видео', 'Хватает на одно видео или на 40 фотографий', 690.00, 40, true, 4);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DELETE FROM tariffs WHERE name = '1 видео' AND gens_count = 40;

UPDATE categories SET is_active = false WHERE section = 'trends' AND media_kind = 'video';

ALTER TABLE generations DROP CONSTRAINT IF EXISTS generations_type_check;
ALTER TABLE generations ADD CONSTRAINT generations_type_check
    CHECK (type IN ('free', 'ready_prompt', 'custom', 'edit', 'couple', 'family', 'kids', 'greetings', 'trends'));

ALTER TABLE generations DROP COLUMN IF EXISTS charged_free_gens;
ALTER TABLE generations DROP COLUMN IF EXISTS charged_paid_gens;
ALTER TABLE generations DROP COLUMN IF EXISTS cost_gens;
ALTER TABLE generations DROP COLUMN IF EXISTS output_video_url;

ALTER TABLE prompts DROP CONSTRAINT IF EXISTS prompts_price_gens_check;
ALTER TABLE prompts DROP COLUMN IF EXISTS price_gens;
ALTER TABLE prompts DROP COLUMN IF EXISTS video_prompt;
ALTER TABLE prompts DROP CONSTRAINT IF EXISTS prompts_media_kind_check;
ALTER TABLE prompts DROP COLUMN IF EXISTS media_kind;

-- +goose StatementEnd
