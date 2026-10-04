-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION set_update_time() RETURNS trigger AS $$
BEGIN
    NEW.update_time = current_timestamp;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION set_update_time();
