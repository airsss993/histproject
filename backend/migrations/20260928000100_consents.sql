-- +goose Up
-- +goose StatementBegin

-- Согласия посетителя: на обработку при регистрации и на распространение фамилии.
-- Отзыв не удаляет строку, а ставит revoked_at: так видно, когда и какой текст действовал.
CREATE TABLE users.consents (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users.users (id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('registration', 'distribution')),
    version TEXT NOT NULL,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX consents_active_unique ON users.consents (user_id, kind) WHERE revoked_at IS NULL;

-- Согласие автора заявки хранится на самой заявке: учётки у автора может не быть.
ALTER TABLE submissions.submissions
    ADD COLUMN consent_version TEXT,
    ADD COLUMN consented_at TIMESTAMPTZ;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE submissions.submissions DROP COLUMN consented_at, DROP COLUMN consent_version;
DROP TABLE users.consents;

-- +goose StatementEnd
