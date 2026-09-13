-- +goose Up
-- +goose StatementBegin

CREATE TABLE notifications.deliveries (
    event_key TEXT NOT NULL,
    channel TEXT NOT NULL CHECK (channel IN ('email', 'telegram')),
    status TEXT NOT NULL CHECK (status IN ('processing', 'sent', 'failed')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    delivered_at TIMESTAMPTZ,
    PRIMARY KEY (event_key, channel),
    CHECK (length(event_key) BETWEEN 1 AND 200),
    CHECK (length(last_error) <= 500)
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DO $$
BEGIN
    RAISE EXCEPTION 'notification deliveries migration is forward-only';
END
$$;

-- +goose StatementEnd
