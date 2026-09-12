-- +goose Up
CREATE TYPE game_type AS ENUM ('game', 'minigame');

CREATE TABLE games (
    id text NOT NULL PRIMARY KEY,
    create_time timestamptz NOT NULL DEFAULT current_timestamp,
    update_time timestamptz NOT NULL DEFAULT current_timestamp,
    delete_time timestamptz,
    type game_type NOT NULL,
    name_translations jsonb NOT NULL DEFAULT '{}'::json
);

-- +goose Down

DROP TABLE games;
DROP TYPE game_type;