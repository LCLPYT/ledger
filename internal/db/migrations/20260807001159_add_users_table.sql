-- +goose Up
CREATE TABLE users (
    id BIGSERIAL NOT NULL,
    name text NOT NULL,
    email text NOT NULL,
    create_time TIMESTAMPTZ NOT NULL DEFAULT current_timestamp,
    update_time TIMESTAMPTZ NOT NULL DEFAULT current_timestamp,
    delete_time TIMESTAMPTZ,
    password_hash BYTEA NOT NULL,

    PRIMARY KEY (id),
    UNIQUE (email)
);

-- +goose Down
DROP TABLE users;
