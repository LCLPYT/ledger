-- +goose Up
CREATE TYPE definition_kind AS ENUM ('game', 'minigame');
CREATE TYPE stat_scope AS ENUM ('match', 'player', 'team');
CREATE TYPE stat_value_type AS ENUM ('int', 'float', 'bool', 'duration', 'string');
CREATE TYPE game_type AS ENUM ('ffa', 'team');
CREATE TYPE match_status AS ENUM ('completed', 'cancelled', 'aborted');
CREATE TYPE round_status AS ENUM ('completed', 'cancelled');

-- +goose Down
DROP TYPE round_status;
DROP TYPE match_status;
DROP TYPE game_type;
DROP TYPE stat_value_type;
DROP TYPE stat_scope;
DROP TYPE definition_kind;
