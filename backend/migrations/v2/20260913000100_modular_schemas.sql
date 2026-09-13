-- +goose Up
-- +goose StatementBegin

DO $$
BEGIN
    IF to_regclass('public.admins') IS NULL
        OR to_regclass('public.admin_sessions') IS NULL
        OR to_regclass('public.requests') IS NULL
        OR to_regclass('public.objects') IS NULL
        OR to_regclass('public.request_audit_log') IS NULL
        OR to_regclass('public.bot_subscriptions') IS NULL THEN
        RAISE EXCEPTION 'legacy schema is incomplete';
    END IF;
END
$$;

CREATE SCHEMA auth;
CREATE SCHEMA submissions;
CREATE SCHEMA events;
CREATE SCHEMA notifications;

ALTER TYPE public.admin_role SET SCHEMA auth;
ALTER TYPE public.request_status SET SCHEMA submissions;
ALTER TYPE submissions.request_status RENAME TO submission_status;
ALTER TYPE submissions.submission_status RENAME VALUE 'В обработке' TO 'processing';
ALTER TYPE submissions.submission_status RENAME VALUE 'Новая' TO 'new';
ALTER TYPE submissions.submission_status RENAME VALUE 'На проверке' TO 'in_review';
ALTER TYPE submissions.submission_status RENAME VALUE 'Отклонена' TO 'rejected';
ALTER TYPE submissions.submission_status RENAME VALUE 'Опубликована' TO 'published';

ALTER TABLE public.objects DROP CONSTRAINT objects_request_id_fkey;

ALTER TABLE public.admins SET SCHEMA auth;
ALTER TABLE public.admin_sessions SET SCHEMA auth;
ALTER TABLE auth.admin_sessions RENAME TO legacy_sessions;
ALTER SEQUENCE auth.admin_sessions_id_seq RENAME TO legacy_sessions_id_seq;
ALTER TABLE auth.legacy_sessions RENAME CONSTRAINT admin_sessions_pkey TO legacy_sessions_pkey;
ALTER TABLE auth.legacy_sessions RENAME CONSTRAINT admin_sessions_refresh_token_key TO legacy_sessions_refresh_token_key;
ALTER TABLE auth.legacy_sessions RENAME CONSTRAINT admin_sessions_admin_id_fkey TO legacy_sessions_admin_id_fkey;

ALTER TABLE public.requests SET SCHEMA submissions;
ALTER TABLE submissions.requests RENAME TO submissions;
ALTER SEQUENCE submissions.requests_id_seq RENAME TO submissions_id_seq;
ALTER TABLE submissions.submissions RENAME CONSTRAINT requests_pkey TO submissions_pkey;
ALTER TABLE submissions.submissions RENAME CONSTRAINT requests_slug_key TO submissions_slug_key;
ALTER INDEX submissions.ind_requests_email RENAME TO submissions_email_idx;
ALTER INDEX submissions.ind_requests_status RENAME TO submissions_status_idx;

ALTER TABLE public.request_audit_log SET SCHEMA submissions;
ALTER TABLE submissions.request_audit_log RENAME TO audit_log;
ALTER TABLE submissions.audit_log RENAME COLUMN request_id TO submission_id;
ALTER SEQUENCE submissions.request_audit_log_id_seq RENAME TO audit_log_id_seq;
ALTER TABLE submissions.audit_log RENAME CONSTRAINT request_audit_log_pkey TO audit_log_pkey;
ALTER TABLE submissions.audit_log RENAME CONSTRAINT request_audit_log_request_id_fkey TO audit_log_submission_id_fkey;
ALTER INDEX submissions.ind_request_audit_log_request_id RENAME TO audit_log_submission_id_idx;

ALTER TABLE public.objects SET SCHEMA events;
ALTER TABLE events.objects RENAME TO events;
ALTER TABLE events.events RENAME COLUMN request_id TO submission_id;
ALTER SEQUENCE events.objects_id_seq RENAME TO events_id_seq;
ALTER TABLE events.events RENAME CONSTRAINT objects_pkey TO events_pkey;
ALTER INDEX events.ind_objects_event_type RENAME TO events_event_type_id_idx;
ALTER INDEX events.ind_objects_request_id RENAME TO events_submission_id_idx;

ALTER TABLE public.bot_subscriptions SET SCHEMA notifications;
ALTER TABLE notifications.bot_subscriptions RENAME TO telegram_subscriptions;
ALTER SEQUENCE notifications.bot_subscriptions_id_seq RENAME TO telegram_subscriptions_id_seq;
ALTER TABLE notifications.telegram_subscriptions RENAME CONSTRAINT bot_subscriptions_pkey TO telegram_subscriptions_pkey;
ALTER TABLE notifications.telegram_subscriptions RENAME CONSTRAINT bot_subscriptions_username_key TO telegram_subscriptions_username_key;

ALTER TABLE submissions.submissions ADD COLUMN event_type_code TEXT;
UPDATE submissions.submissions
SET event_type_code = CASE event_type_id
    WHEN 1 THEN 'political'
    WHEN 2 THEN 'military'
    ELSE 'legacy_' || event_type_id::TEXT
END;
ALTER TABLE submissions.submissions ALTER COLUMN event_type_code SET NOT NULL;
CREATE INDEX submissions_event_type_code_idx ON submissions.submissions (event_type_code);

ALTER TABLE events.events ADD COLUMN event_type_code TEXT;
UPDATE events.events
SET event_type_code = CASE event_type_id
    WHEN 1 THEN 'political'
    WHEN 2 THEN 'military'
    ELSE 'legacy_' || event_type_id::TEXT
END;
ALTER TABLE events.events ALTER COLUMN event_type_code SET NOT NULL;
CREATE INDEX events_event_type_code_idx ON events.events (event_type_code);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DO $$
BEGIN
    RAISE EXCEPTION 'V2 schema migration is forward-only';
END
$$;

-- +goose StatementEnd
