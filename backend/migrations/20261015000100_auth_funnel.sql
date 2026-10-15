-- +goose Up
-- +goose StatementBegin

-- auth_step - гость нажал "Предложить событие" и прошёл вход перед формой: 0 - открыл вход
-- или регистрацию, 1 - запросил код, 2 - вошёл. Показывает, сколько уходит до формы.
ALTER TABLE metrics.actions DROP CONSTRAINT actions_action_check;
ALTER TABLE metrics.actions ADD CONSTRAINT actions_action_check
    CHECK (action IN ('card_open', 'card_click', 'form_step', 'auth_step'));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DELETE FROM metrics.actions WHERE action = 'auth_step';
ALTER TABLE metrics.actions DROP CONSTRAINT actions_action_check;
ALTER TABLE metrics.actions ADD CONSTRAINT actions_action_check
    CHECK (action IN ('card_open', 'card_click', 'form_step'));

-- +goose StatementEnd
