-- +goose Up
CREATE TABLE users (
    id BIGSERIAL NOT NULL,
    name text NOT NULL,
    email text NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT current_timestamp,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT current_timestamp,
    deleted_at TIMESTAMPTZ,
    password_hash BYTEA NOT NULL,

    PRIMARY KEY (id),
    UNIQUE (email)
);

-- +goose Down
DROP TABLE users;
