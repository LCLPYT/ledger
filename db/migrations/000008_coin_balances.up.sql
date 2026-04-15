CREATE TABLE coin_balances (
    player_id  BIGINT    PRIMARY KEY REFERENCES minecraft_players(id) ON DELETE CASCADE,
    balance    BIGINT    NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT now()
);
