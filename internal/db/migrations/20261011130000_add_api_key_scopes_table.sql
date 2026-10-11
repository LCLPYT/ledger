-- +goose Up
CREATE TABLE api_key_scopes (
    api_key_id uuid NOT NULL REFERENCES api_keys (id) ON DELETE CASCADE,
    permission text COLLATE "C" NOT NULL CHECK (
        length(permission) <= 255
        AND permission ~ '^(\*|[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*(\.\*|:\*|:[a-z0-9_-]+)?)$'
    ),

    PRIMARY KEY (api_key_id, permission)
);

-- +goose Down
DROP TABLE api_key_scopes;
