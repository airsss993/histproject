-- +goose Up
-- +goose StatementBegin

-- Имя модератора для приветствия в панели. У заведённых раньше оно пустое, панель показывает логин.
ALTER TABLE auth.admins
    ADD COLUMN name VARCHAR(100) NOT NULL DEFAULT '';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE auth.admins
    DROP COLUMN name;

-- +goose StatementEnd
