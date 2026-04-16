ALTER TABLE minecraft_players
    ADD COLUMN data JSONB NOT NULL DEFAULT '{}';
