-- +goose Up
CREATE TABLE player_settings (
    player_id uuid NOT NULL,
    path text COLLATE "C" NOT NULL CHECK (
        length(path) <= 255 AND path ~ '^[a-z][a-z0-9_]{0,62}(\.[a-z][a-z0-9_]{0,62}){0,15}$'
    ),
    create_time timestamptz NOT NULL DEFAULT current_timestamp,
    update_time timestamptz NOT NULL DEFAULT current_timestamp,
    value jsonb NOT NULL,

    PRIMARY KEY (player_id, path)
);

CREATE TRIGGER player_settings_set_update_time
    BEFORE UPDATE ON player_settings
    FOR EACH ROW
    EXECUTE FUNCTION set_update_time();

-- +goose Down
DROP TABLE player_settings;
