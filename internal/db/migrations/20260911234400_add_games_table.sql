-- +goose Up
CREATE TYPE game_type AS ENUM ('game', 'minigame');

CREATE TABLE games (
    id text NOT NULL PRIMARY KEY,
    create_time timestamptz NOT NULL default current_timestamp,
    update_time timestamptz NOT NULL default current_timestamp,
    type game_type NOT NULL,
    name_translations json NOT NULL DEFAULT '{}'::json
);

-- +goose Down

DROP TABLE games;