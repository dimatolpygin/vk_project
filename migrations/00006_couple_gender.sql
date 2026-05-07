-- +goose Up
-- +goose StatementBegin

ALTER TABLE categories DROP CONSTRAINT IF EXISTS categories_gender_check;
ALTER TABLE categories ADD CONSTRAINT categories_gender_check
    CHECK (gender IN ('male', 'female', 'any', 'couple'));

ALTER TABLE prompts DROP CONSTRAINT IF EXISTS prompts_gender_check;
ALTER TABLE prompts ADD CONSTRAINT prompts_gender_check
    CHECK (gender IN ('male', 'female', 'any', 'couple'));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE categories DROP CONSTRAINT IF EXISTS categories_gender_check;
ALTER TABLE categories ADD CONSTRAINT categories_gender_check
    CHECK (gender IN ('male', 'female', 'any'));

ALTER TABLE prompts DROP CONSTRAINT IF EXISTS prompts_gender_check;
ALTER TABLE prompts ADD CONSTRAINT prompts_gender_check
    CHECK (gender IN ('male', 'female', 'any'));

-- +goose StatementEnd
