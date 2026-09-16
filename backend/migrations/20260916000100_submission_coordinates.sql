-- +goose Up
-- +goose StatementBegin

-- Точку ставит автор заявки. У заявок, созданных раньше, координат нет.
ALTER TABLE submissions.submissions
    ADD COLUMN latitude DOUBLE PRECISION CHECK (latitude BETWEEN -90 AND 90),
    ADD COLUMN longitude DOUBLE PRECISION CHECK (longitude BETWEEN -180 AND 180),
    ADD CONSTRAINT submissions_coordinates_pair CHECK ((latitude IS NULL) = (longitude IS NULL));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE submissions.submissions
    DROP CONSTRAINT submissions_coordinates_pair,
    DROP COLUMN longitude,
    DROP COLUMN latitude;

-- +goose StatementEnd
