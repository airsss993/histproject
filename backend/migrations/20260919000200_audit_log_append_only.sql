-- +goose Up
-- +goose StatementBegin

-- История модерации только дополняется. Удаление заявки с историей тоже упрётся сюда через ON DELETE CASCADE.
CREATE FUNCTION submissions.audit_log_append_only() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'submissions.audit_log is append-only' USING ERRCODE = 'insufficient_privilege';
END;
$$;

CREATE TRIGGER audit_log_no_change
    BEFORE UPDATE OR DELETE ON submissions.audit_log
    FOR EACH ROW EXECUTE FUNCTION submissions.audit_log_append_only();

CREATE TRIGGER audit_log_no_truncate
    BEFORE TRUNCATE ON submissions.audit_log
    FOR EACH STATEMENT EXECUTE FUNCTION submissions.audit_log_append_only();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TRIGGER audit_log_no_truncate ON submissions.audit_log;
DROP TRIGGER audit_log_no_change ON submissions.audit_log;
DROP FUNCTION submissions.audit_log_append_only();

-- +goose StatementEnd
