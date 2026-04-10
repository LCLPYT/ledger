CREATE TABLE roles (
    id         BIGSERIAL PRIMARY KEY,
    name       VARCHAR(127) NOT NULL UNIQUE,
    created_at TIMESTAMP NOT NULL DEFAULT now()
);
