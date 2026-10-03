-- +goose Up
CREATE TYPE game_type AS ENUM ('game', 'minigame');

CREATE TABLE games (
    id text NOT NULL PRIMARY KEY CHECK (id ~ '^[a-z]([a-z0-9-]{0,61}[a-z0-9])?$'),
    create_time timestamptz NOT NULL DEFAULT current_timestamp,
    update_time timestamptz NOT NULL DEFAULT current_timestamp,
    delete_time timestamptz,
    purge_time timestamptz,
    type game_type NOT NULL,
    name_translations jsonb NOT NULL DEFAULT '{}'::jsonb,

    CHECK ((delete_time IS NULL) = (purge_time IS NULL))
);

CREATE INDEX games_create_time_id_idx ON games (create_time, id);

CREATE TRIGGER games_set_update_time
    BEFORE UPDATE ON games
    FOR EACH ROW
    EXECUTE FUNCTION set_update_time();

-- +goose Down
DROP TABLE games;
DROP TYPE game_type;
