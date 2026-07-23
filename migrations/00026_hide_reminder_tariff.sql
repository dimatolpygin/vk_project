-- +goose Up
-- +goose StatementBegin

-- Тариф «3 генерации» нужен только в догоняющем сообщении для тех, кто ничего
-- не купил, и не должен висеть в общем списке тарифов. Выключаем его и явно
-- закрепляем за напоминанием: воркер берёт тариф из reminder_tariff_id
-- независимо от is_active, а оплата по нему создаётся штатным сценарием.
UPDATE tariffs
SET is_active = false
WHERE name = '3 генерации'
  AND gens_count = 3
  AND price = 90.00;

INSERT INTO admin_config (key, value)
SELECT 'reminder_tariff_id', id::text
FROM tariffs
WHERE name = '3 генерации'
  AND gens_count = 3
  AND price = 90.00
ORDER BY id
LIMIT 1
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

UPDATE tariffs
SET is_active = true
WHERE name = '3 генерации'
  AND gens_count = 3
  AND price = 90.00;

DELETE FROM admin_config WHERE key = 'reminder_tariff_id';

-- +goose StatementEnd
