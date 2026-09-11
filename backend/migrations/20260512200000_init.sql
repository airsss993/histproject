-- Полная схема БД с поддержкой slug-ссылок на сайты заявок.

-- +goose Up
-- +goose StatementBegin

CREATE TYPE request_status AS ENUM ('В обработке', 'Новая', 'На проверке', 'Отклонена', 'Опубликована');
CREATE TYPE admin_role AS ENUM ('super_admin', 'admin');

CREATE TABLE admins
(
    id            SERIAL PRIMARY KEY,
    login         VARCHAR(100) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    role          admin_role   NOT NULL DEFAULT 'admin',
    email         VARCHAR(255) NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE admin_sessions
(
    id            SERIAL PRIMARY KEY,
    admin_id      INT         NOT NULL REFERENCES admins (id) ON DELETE CASCADE,
    refresh_token TEXT        NOT NULL UNIQUE,
    expires_at    TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE requests
(
    id                SERIAL PRIMARY KEY,
    title             VARCHAR(200)   NOT NULL,
    description       TEXT           NOT NULL,
    event_date        DATE           NOT NULL,
    event_type_id     INTEGER        NOT NULL,
    email             VARCHAR(70)    NOT NULL,
    telegram_username VARCHAR(100)   NOT NULL,
    archive_id        VARCHAR(200)   NOT NULL,
    site_url          VARCHAR(500),
    screenshot_url    VARCHAR(500),
    slug              VARCHAR(255)   UNIQUE,
    status            request_status NOT NULL DEFAULT 'В обработке',
    admin_comment     TEXT,
    created_at        TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

CREATE INDEX ind_requests_email ON requests (email);
CREATE INDEX ind_requests_status ON requests (status);

CREATE TABLE objects
(
    id                SERIAL PRIMARY KEY,
    request_id        INTEGER          NOT NULL REFERENCES requests (id),
    title             VARCHAR(200)     NOT NULL,
    description       TEXT             NOT NULL,
    latitude          DOUBLE PRECISION NOT NULL CHECK (latitude >= -90 AND latitude <= 90),
    longitude         DOUBLE PRECISION NOT NULL CHECK (longitude >= -180 AND longitude <= 180),
    event_date        DATE             NOT NULL,
    event_type_id     INTEGER          NOT NULL,
    site_url          VARCHAR(500)     NOT NULL,
    preview_image_url VARCHAR(500),
    created_at        TIMESTAMPTZ      NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ      NOT NULL DEFAULT NOW()
);

CREATE INDEX ind_objects_event_type ON objects (event_type_id);
CREATE INDEX ind_objects_request_id ON objects (request_id);

CREATE TABLE request_audit_log
(
    id          SERIAL PRIMARY KEY,
    request_id  INT         NOT NULL REFERENCES requests (id) ON DELETE CASCADE,
    admin_login VARCHAR(100) NOT NULL,
    admin_role  VARCHAR(50)  NOT NULL,
    action      VARCHAR(50)  NOT NULL,
    comment     TEXT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX ind_request_audit_log_request_id ON request_audit_log (request_id);

CREATE TABLE bot_subscriptions
(
    id         SERIAL PRIMARY KEY,
    username   VARCHAR(100) NOT NULL UNIQUE,
    chat_id    BIGINT       NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS request_audit_log;
DROP TABLE IF EXISTS bot_subscriptions;
DROP TABLE IF EXISTS objects;
DROP TABLE IF EXISTS requests;
DROP TABLE IF EXISTS admin_sessions;
DROP TABLE IF EXISTS admins;
DROP TYPE IF EXISTS request_status;
DROP TYPE IF EXISTS admin_role;

-- +goose StatementEnd
