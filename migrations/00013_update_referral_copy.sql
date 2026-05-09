-- +goose Up
-- +goose StatementBegin

UPDATE messages
SET text = '🎁 Реферальная программа

Ты пригласил уже {{.Count}} человек.

За каждого пользователя, который оплатит любое количество генераций, ты получишь 2 бесплатные генерации!

Твоя ссылка:
{{.RefLink}}',
    updated_at = now()
WHERE key = 'referral_status'
  AND text LIKE '%оплатит тариф%';

UPDATE messages
SET text = '🎁 Твой реферал оплатил генерации! +2 бесплатные генерации.',
    updated_at = now()
WHERE key = 'referral_bonus_awarded'
  AND text LIKE '%оплатил тариф%';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

UPDATE messages
SET text = '🎁 Реферальная программа

Ты пригласил уже {{.Count}} человек.

За каждого друга, который оплатит тариф, ты получишь 2 бесплатные генерации!

Твоя ссылка:
{{.RefLink}}',
    updated_at = now()
WHERE key = 'referral_status'
  AND text LIKE '%оплатит любое количество генераций%';

UPDATE messages
SET text = '🎁 Твой реферал оплатил тариф! +2 бесплатные генерации.',
    updated_at = now()
WHERE key = 'referral_bonus_awarded'
  AND text LIKE '%оплатил генерации%';

-- +goose StatementEnd
