-- +goose Up
CREATE TABLE roles (
    name text NOT NULL CHECK (name ~ '^[a-z][a-z0-9_]{0,62}$'),
    create_time timestamptz NOT NULL DEFAULT current_timestamp,

    PRIMARY KEY (name)
);

CREATE TABLE role_permissions (
    role_name text NOT NULL REFERENCES roles (name) ON UPDATE CASCADE ON DELETE CASCADE,
    permission text COLLATE "C" NOT NULL CHECK (
        length(permission) <= 255
        AND permission ~ '^(\*|[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*(\.\*|:\*|:[a-z0-9_-]+)?)$'
    ),

    PRIMARY KEY (role_name, permission)
);

CREATE TABLE api_keys (
    id uuid NOT NULL,
    name text NOT NULL,
    secret_hash bytea NOT NULL CHECK (length(secret_hash) = 32),
    create_time timestamptz NOT NULL DEFAULT current_timestamp,
    expire_time timestamptz,
    revoke_time timestamptz,
    last_used_time timestamptz,

    PRIMARY KEY (id)
);

CREATE TABLE api_key_roles (
    api_key_id uuid NOT NULL REFERENCES api_keys (id) ON DELETE CASCADE,
    role_name text NOT NULL REFERENCES roles (name) ON UPDATE CASCADE ON DELETE CASCADE,

    PRIMARY KEY (api_key_id, role_name)
);

CREATE INDEX api_key_roles_role_name_idx ON api_key_roles (role_name);

-- +goose Down
DROP TABLE api_key_roles;
DROP TABLE api_keys;
DROP TABLE role_permissions;
DROP TABLE roles;
