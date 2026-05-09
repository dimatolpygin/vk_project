-- +goose Up
-- +goose StatementBegin

ALTER TABLE generations
    ADD COLUMN IF NOT EXISTS charged_balance_kind TEXT CHECK (charged_balance_kind IN ('free', 'paid'));

ALTER TABLE generations
    ADD COLUMN IF NOT EXISTS balance_refunded BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE messages
SET text = '💳 Баланс

🎯 Доступно генераций: {{.TotalGens}}

Пополнить баланс:',
    updated_at = now()
WHERE key = 'settings_balance'
  AND text LIKE '%Бесплатных%'
  AND text LIKE '%Платных%';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE generations DROP COLUMN IF EXISTS balance_refunded;
ALTER TABLE generations DROP COLUMN IF EXISTS charged_balance_kind;

UPDATE messages
SET text = '💳 Баланс

🔹 Бесплатных: {{.FreeGens}}
🔸 Платных: {{.PaidGens}}

Пополнить баланс:',
    updated_at = now()
WHERE key = 'settings_balance'
  AND text LIKE '%Доступно генераций%';

-- +goose StatementEnd
