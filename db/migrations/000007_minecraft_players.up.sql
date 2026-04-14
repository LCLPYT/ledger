CREATE TABLE minecraft_players (
    id         BIGSERIAL PRIMARY KEY,
    uuid       UUID      NOT NULL UNIQUE,
    created_at TIMESTAMP NOT NULL DEFAULT now()
);
