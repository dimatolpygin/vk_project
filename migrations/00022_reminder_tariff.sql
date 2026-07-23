-- +goose Up
-- +goose StatementBegin

-- Тариф для догоняющего сообщения после экрана тарифов: 3 генерации за 90 ₽.
-- Добавляем только если активного тарифа на 3 генерации ещё нет.
INSERT INTO tariffs (name, description, price, gens_count, sort_order)
SELECT '3 генерации', 'Попробовать: 3 генерации', 90.00, 3, 0
WHERE NOT EXISTS (
    SELECT 1 FROM tariffs WHERE gens_count = 3 AND is_active = true
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DELETE FROM tariffs
WHERE name = '3 генерации' AND gens_count = 3 AND price = 90.00;

-- +goose StatementEnd
