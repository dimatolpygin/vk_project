-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS users (
    id             BIGSERIAL PRIMARY KEY,
    vk_id          BIGINT UNIQUE NOT NULL,
    username       TEXT,
    first_name     TEXT,
    gender         TEXT CHECK (gender IN ('male','female','unknown')) DEFAULT 'unknown',
    referral_code  TEXT UNIQUE NOT NULL,
    referred_by    BIGINT REFERENCES users(vk_id),
    free_gens      INT NOT NULL DEFAULT 1,
    paid_gens      INT NOT NULL DEFAULT 0,
    subscribed     BOOLEAN NOT NULL DEFAULT FALSE,
    saved_photo_url TEXT,
    status         TEXT DEFAULT 'free' CHECK (status IN ('free','paid')),
    created_at     TIMESTAMPTZ DEFAULT now(),
    updated_at     TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_users_vk_id ON users(vk_id);
CREATE INDEX IF NOT EXISTS idx_users_referral_code ON users(referral_code);

CREATE TABLE IF NOT EXISTS tariffs (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT,
    price       NUMERIC(10,2) NOT NULL,
    gens_count  INT NOT NULL,
    is_active   BOOLEAN DEFAULT TRUE,
    sort_order  INT DEFAULT 0,
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS orders (
    id                  BIGSERIAL PRIMARY KEY,
    user_vk_id          BIGINT REFERENCES users(vk_id),
    tariff_id           INT REFERENCES tariffs(id),
    yukassa_payment_id  TEXT UNIQUE,
    amount              NUMERIC(10,2),
    status              TEXT DEFAULT 'pending' CHECK (status IN ('pending','succeeded','failed')),
    created_at          TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_orders_user_vk_id ON orders(user_vk_id);
CREATE INDEX IF NOT EXISTS idx_orders_yukassa_payment_id ON orders(yukassa_payment_id);

CREATE TABLE IF NOT EXISTS generations (
    id                BIGSERIAL PRIMARY KEY,
    user_vk_id        BIGINT REFERENCES users(vk_id),
    type              TEXT CHECK (type IN ('free','ready_prompt','custom','edit','couple','family')),
    prompt            TEXT,
    input_photo_url   TEXT,
    output_photo_url  TEXT,
    wavespeed_task_id TEXT,
    model             TEXT,
    status            TEXT DEFAULT 'pending' CHECK (status IN ('pending','processing','completed','failed')),
    error_text        TEXT,
    created_at        TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_generations_user_vk_id ON generations(user_vk_id);
CREATE INDEX IF NOT EXISTS idx_generations_wavespeed_task_id ON generations(wavespeed_task_id);
CREATE INDEX IF NOT EXISTS idx_generations_status ON generations(status);

CREATE TABLE IF NOT EXISTS referrals (
    id              BIGSERIAL PRIMARY KEY,
    referrer_vk_id  BIGINT REFERENCES users(vk_id),
    referred_vk_id  BIGINT REFERENCES users(vk_id) UNIQUE,
    bonus_given     BOOLEAN DEFAULT FALSE,
    created_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_referrals_referrer ON referrals(referrer_vk_id);

-- Default tariffs seed
INSERT INTO tariffs (name, description, price, gens_count, sort_order) VALUES
    ('Стартовый', '10 генераций для знакомства', 199.00, 10, 1),
    ('Базовый',   '30 генераций — хватит на месяц', 499.00, 30, 2),
    ('Профи',     '100 генераций по лучшей цене', 1299.00, 100, 3)
ON CONFLICT DO NOTHING;

-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS referrals;
DROP TABLE IF EXISTS generations;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS tariffs;
DROP TABLE IF EXISTS users;
