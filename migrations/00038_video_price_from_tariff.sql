-- +goose Up
-- +goose StatementBegin

-- Цена видео в генерациях переезжает из карточки промта в тариф.
--
-- До этой миграции число жило в двух местах: prompts.price_gens решал, сколько
-- списать за видео, а тариф «1 видео» — сколько генераций продать за деньги.
-- Числа связаны по смыслу (пакет должен покрывать ровно одно видео), но не были
-- связаны технически: правку приходилось повторять дважды, и рассинхрон никак
-- себя не проявлял, кроме как выручкой.
--
-- Теперь источник истины один — тариф с галочкой is_video_pack. Сколько
-- генераций в этом пакете, столько и списывается за одно видео. Пакет
-- по определению равен одному видео, поэтому второе число не нужно.

ALTER TABLE tariffs ADD COLUMN IF NOT EXISTS is_video_pack BOOLEAN NOT NULL DEFAULT false;

-- Помечаем существующий видеопакет, заведённый миграцией 00034.
UPDATE tariffs SET is_video_pack = true WHERE name = '1 видео' AND gens_count = 40;

-- Пакет может быть только один: иначе «взять цену из тарифов» снова становится
-- вопросом «из какого именно».
CREATE UNIQUE INDEX IF NOT EXISTS tariffs_one_video_pack ON tariffs (is_video_pack) WHERE is_video_pack;

-- price_gens больше не читается кодом. Оставлять живую колонку, которая ни на
-- что не влияет, — это тот же рассинхрон, только тихий.
ALTER TABLE prompts DROP CONSTRAINT IF EXISTS prompts_price_gens_check;
ALTER TABLE prompts DROP COLUMN IF EXISTS price_gens;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE prompts ADD COLUMN IF NOT EXISTS price_gens INT NOT NULL DEFAULT 1;
ALTER TABLE prompts DROP CONSTRAINT IF EXISTS prompts_price_gens_check;
ALTER TABLE prompts ADD CONSTRAINT prompts_price_gens_check CHECK (price_gens >= 1);

-- Возвращаем цену видео-промтам из пакета, чтобы откат не обнулил списание.
UPDATE prompts SET price_gens = COALESCE(
    (SELECT gens_count FROM tariffs WHERE is_video_pack LIMIT 1), 40)
WHERE media_kind = 'video';

DROP INDEX IF EXISTS tariffs_one_video_pack;
ALTER TABLE tariffs DROP COLUMN IF EXISTS is_video_pack;

-- +goose StatementEnd
