-- +goose Up
-- +goose StatementBegin

ALTER TYPE submissions.submission_status RENAME VALUE 'in_review' TO 'review';

DO $$
BEGIN
    IF EXISTS (
        SELECT submission_id
        FROM events.events
        GROUP BY submission_id
        HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'events.events contains duplicate submission_id values';
    END IF;
END
$$;

DROP INDEX events.events_submission_id_idx;
CREATE UNIQUE INDEX events_submission_id_key ON events.events (submission_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DO $$
BEGIN
    RAISE EXCEPTION 'submission contract migration is forward-only';
END
$$;

-- +goose StatementEnd
