-- +goose Up
-- +goose StatementBegin

-- Третий тип события - личность.
ALTER TABLE submissions.submissions DROP CONSTRAINT submissions_event_type_code_check;
ALTER TABLE submissions.submissions ADD CONSTRAINT submissions_event_type_code_check
    CHECK (event_type_code IN ('political', 'military', 'personality'));
ALTER TABLE events.events DROP CONSTRAINT events_event_type_code_check;
ALTER TABLE events.events ADD CONSTRAINT events_event_type_code_check
    CHECK (event_type_code IN ('political', 'military', 'personality'));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE events.events DROP CONSTRAINT events_event_type_code_check;
ALTER TABLE events.events ADD CONSTRAINT events_event_type_code_check
    CHECK (event_type_code IN ('political', 'military'));
ALTER TABLE submissions.submissions DROP CONSTRAINT submissions_event_type_code_check;
ALTER TABLE submissions.submissions ADD CONSTRAINT submissions_event_type_code_check
    CHECK (event_type_code IN ('political', 'military'));

-- +goose StatementEnd
