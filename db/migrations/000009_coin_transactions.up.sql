CREATE TYPE coin_source AS ENUM ('minigame', 'admin', 'purchase', 'system');

CREATE TABLE coin_transactions (
    id             BIGSERIAL   PRIMARY KEY,
    player_id      BIGINT      NOT NULL REFERENCES minecraft_players(id) ON DELETE CASCADE,
    amount         BIGINT      NOT NULL,
    source         coin_source NOT NULL,
    description    TEXT,
    created_at     TIMESTAMP   NOT NULL DEFAULT now(),
    actor_user_id  BIGINT      REFERENCES users(id) ON DELETE SET NULL,
    actor_token_id BIGINT      REFERENCES access_tokens(id) ON DELETE SET NULL
);

CREATE INDEX coin_transactions_player_id_idx ON coin_transactions(player_id);
