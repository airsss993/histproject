-- +goose Up
-- +goose StatementBegin

-- Публичный код вместо числового id: по нему не видно, сколько записей в базе, и не перебрать соседей.
-- 6 символов Crockford base32 (без I, L, O, U), байты из gen_random_uuid(): 256 делится на 32, перекоса нет.
-- Префикс (RH- у заявок, RHU- у пользователей) не хранится, его добавляет SELECT.
CREATE FUNCTION public.rh_code() RETURNS TEXT
LANGUAGE sql VOLATILE AS $$
    SELECT string_agg(substr('0123456789ABCDEFGHJKMNPQRSTVWXYZ', get_byte(bytes, position) % 32 + 1, 1), '' ORDER BY position)
    FROM (SELECT uuid_send(gen_random_uuid()) AS bytes) AS random, generate_series(0, 5) AS position
$$;

-- Volatile default считается на каждую строку, так что существующие записи получат разные коды.
ALTER TABLE users.users
    ADD COLUMN public_id TEXT NOT NULL DEFAULT public.rh_code(),
    ADD CONSTRAINT users_public_id_key UNIQUE (public_id);

ALTER TABLE submissions.submissions
    ADD COLUMN public_id TEXT NOT NULL DEFAULT public.rh_code(),
    ADD CONSTRAINT submissions_public_id_key UNIQUE (public_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE submissions.submissions DROP COLUMN public_id;
ALTER TABLE users.users DROP COLUMN public_id;
DROP FUNCTION public.rh_code();

-- +goose StatementEnd
