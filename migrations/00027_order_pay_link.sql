-- +goose Up
-- +goose StatementBegin

-- Ссылку на оплату больше не отдаём в кнопку ВК напрямую: ВК заворачивает
-- внешние ссылки через away.vk.com и открывает их во встроенном браузере, где
-- тяжёлая страница ЮKassa у части пользователей не прогружается — человек видит
-- пустой экран и уходит. Вместо этого кнопка ведёт на наш /pay/<token>, который
-- отдаёт лёгкую страницу с авто-редиректом и видимой запасной кнопкой.
--
-- payment_url  — confirmation_url от ЮKassa, чтобы редирект работал и при
--                повторном открытии ссылки;
-- pay_token    — случайный публичный идентификатор, чтобы не светить id заказа
--                и не давать перебирать чужие платежи;
-- pay_opened_* — телеметрия открытий: до этой правки между «кнопка отправлена»
--                и «платёж истёк через час» не было ни одного события, и понять,
--                доходит ли человек до страницы оплаты, было невозможно.
ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS payment_url     text,
    ADD COLUMN IF NOT EXISTS pay_token       text,
    ADD COLUMN IF NOT EXISTS pay_opened_at   timestamptz,
    ADD COLUMN IF NOT EXISTS pay_opened_count integer NOT NULL DEFAULT 0;

CREATE UNIQUE INDEX IF NOT EXISTS orders_pay_token_key ON orders (pay_token);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS orders_pay_token_key;

ALTER TABLE orders
    DROP COLUMN IF EXISTS pay_opened_count,
    DROP COLUMN IF EXISTS pay_opened_at,
    DROP COLUMN IF EXISTS pay_token,
    DROP COLUMN IF EXISTS payment_url;

-- +goose StatementEnd
