-- +goose Up
-- +goose StatementBegin

-- Почта связывает обычный профиль с ролью модератора. Одна почта - одна роль.
CREATE UNIQUE INDEX admins_email_ci_unique ON auth.admins (LOWER(email)) WHERE email <> '';

-- Существующим админам создаём обычный профиль, чтобы они сразу входили по коду из письма.
INSERT INTO users.users (email, first_name, last_name)
SELECT
    LOWER(BTRIM(email)),
    LEFT(COALESCE(NULLIF(SPLIT_PART(BTRIM(name), ' ', 1), ''), login), 50),
    CASE
        WHEN STRPOS(BTRIM(name), ' ') > 0 THEN LEFT(BTRIM(SUBSTR(BTRIM(name), STRPOS(BTRIM(name), ' ') + 1)), 50)
        ELSE ''
    END
FROM auth.admins
WHERE email <> ''
ON CONFLICT (email) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Профили не удаляем: после входа в них уже могут быть заявки.
DROP INDEX auth.admins_email_ci_unique;

-- +goose StatementEnd
