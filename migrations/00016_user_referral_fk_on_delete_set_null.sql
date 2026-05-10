-- +goose Up
ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_referred_by_fkey;

ALTER TABLE users
    ADD CONSTRAINT users_referred_by_fkey
    FOREIGN KEY (referred_by) REFERENCES users(vk_id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_referred_by_fkey;

ALTER TABLE users
    ADD CONSTRAINT users_referred_by_fkey
    FOREIGN KEY (referred_by) REFERENCES users(vk_id);
