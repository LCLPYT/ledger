-- +goose Up
CREATE TYPE game_type AS ENUM ('game', 'minigame');

CREATE TABLE games (
    id text NOT NULL PRIMARY KEY CHECK (id ~ '^[a-z0-9/._-]{1,63}$'),
    create_time timestamptz NOT NULL DEFAULT current_timestamp,
    update_time timestamptz NOT NULL DEFAULT current_timestamp,
    delete_time timestamptz,
    type game_type NOT NULL,
    name_translations jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX games_create_time_id_idx ON games (create_time, id);

-- +goose StatementBegin
CREATE FUNCTION set_update_time() RETURNS trigger AS $$
BEGIN
    NEW.update_time = current_timestamp;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER games_set_update_time
    BEFORE UPDATE ON games
    FOR EACH ROW
    EXECUTE FUNCTION set_update_time();

-- +goose Down

DROP TABLE games;
DROP FUNCTION set_update_time();
DROP TYPE game_type;
