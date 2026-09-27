-- +goose Up
-- +goose StatementBegin

-- Журнал модерации по-прежнему только дополняется, кроме удаления по сроку хранения:
-- его делает воркер в транзакции с SET LOCAL histproject.retention = 'on'.
CREATE OR REPLACE FUNCTION submissions.audit_log_append_only() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' AND current_setting('histproject.retention', true) = 'on' THEN
        RETURN OLD;
    END IF;
    RAISE EXCEPTION 'submissions.audit_log is append-only' USING ERRCODE = 'insufficient_privilege';
END;
$$;

CREATE INDEX views_started_at_idx ON events.views (started_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX events.views_started_at_idx;
CREATE OR REPLACE FUNCTION submissions.audit_log_append_only() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'submissions.audit_log is append-only' USING ERRCODE = 'insufficient_privilege';
END;
$$;

-- +goose StatementEnd
