-- +goose Up
CREATE TABLE user_roles (
    user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role_name text NOT NULL REFERENCES roles (name) ON UPDATE CASCADE ON DELETE CASCADE,

    PRIMARY KEY (user_id, role_name)
);

CREATE INDEX user_roles_role_name_idx ON user_roles (role_name);

DROP TABLE api_key_roles;

ALTER TABLE api_keys ADD COLUMN user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE;

CREATE INDEX api_keys_user_id_idx ON api_keys (user_id);

-- +goose Down
ALTER TABLE api_keys DROP COLUMN user_id;

CREATE TABLE api_key_roles (
    api_key_id uuid NOT NULL REFERENCES api_keys (id) ON DELETE CASCADE,
    role_name text NOT NULL REFERENCES roles (name) ON UPDATE CASCADE ON DELETE CASCADE,

    PRIMARY KEY (api_key_id, role_name)
);

CREATE INDEX api_key_roles_role_name_idx ON api_key_roles (role_name);

DROP TABLE user_roles;
