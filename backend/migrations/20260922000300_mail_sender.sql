-- +goose Up
-- +goose StatementBegin

-- Отправитель писем, заданный супер-админом в панели. Строки нет - письма уходят от EMAIL_FROM.
CREATE TABLE notifications.mail_sender (
    id BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    name TEXT NOT NULL CHECK (length(name) <= 100),
    address TEXT NOT NULL CHECK (length(address) BETWEEN 3 AND 254),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE notifications.mail_sender;

-- +goose StatementEnd
