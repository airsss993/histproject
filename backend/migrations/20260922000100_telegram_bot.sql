-- +goose Up
-- +goose StatementBegin

-- Токен бота задаёт супер-админ в панели. Строка одна: id всегда TRUE.
CREATE TABLE notifications.telegram_bot (
    id BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    token TEXT NOT NULL CHECK (length(token) BETWEEN 1 AND 200),
    username TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Чаты, которым бот шлёт уведомления о заявках. Нет в списке - нет доступа.
CREATE TABLE notifications.telegram_recipients (
    chat_id BIGINT PRIMARY KEY,
    title TEXT NOT NULL,
    username TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE notifications.telegram_recipients;
DROP TABLE notifications.telegram_bot;

-- +goose StatementEnd
