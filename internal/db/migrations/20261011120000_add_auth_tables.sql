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

CREATE TABLE user_roles (
    user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role_name text NOT NULL REFERENCES roles (name) ON UPDATE CASCADE ON DELETE CASCADE,

    PRIMARY KEY (user_id, role_name)
);

CREATE INDEX user_roles_role_name_idx ON user_roles (role_name);

CREATE TABLE api_keys (
    id uuid NOT NULL,
    user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name text NOT NULL,
    secret_hash bytea NOT NULL CHECK (length(secret_hash) = 32),
    create_time timestamptz NOT NULL DEFAULT current_timestamp,
    expire_time timestamptz,
    revoke_time timestamptz,
    last_used_time timestamptz,

    PRIMARY KEY (id)
);

CREATE INDEX api_keys_user_id_idx ON api_keys (user_id);

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
DROP TABLE api_keys;
DROP TABLE user_roles;
DROP TABLE role_permissions;
DROP TABLE roles;
