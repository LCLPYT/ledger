-- +goose Up
CREATE TABLE "stat_values" (
    "id" BIGSERIAL NOT NULL,
    "stat_id" BIGINT NOT NULL,
    "match_id" BIGINT NOT NULL,
    "round_id" BIGINT,
    "player_id" BIGINT,
    "round_team_id" BIGINT,
    "int_value" BIGINT,
    "real_value" DOUBLE PRECISION,
    "text_value" text,

    PRIMARY KEY ("id"),
    UNIQUE NULLS NOT DISTINCT ("match_id", "round_id", "stat_id", "player_id", "round_team_id"),
    CHECK (num_nonnulls("int_value", "real_value", "text_value") = 1),
    FOREIGN KEY ("stat_id") REFERENCES "stats" ("id"),
    FOREIGN KEY ("match_id") REFERENCES "matches" ("id") ON DELETE CASCADE,
    FOREIGN KEY ("round_id") REFERENCES "rounds" ("id") ON DELETE CASCADE,
    FOREIGN KEY ("player_id") REFERENCES "players" ("id"),
    FOREIGN KEY ("round_team_id") REFERENCES "round_teams" ("id") ON DELETE CASCADE
);

CREATE INDEX "stat_values_int_leaderboard_idx" ON "stat_values" ("stat_id", "int_value" DESC) WHERE "int_value" IS NOT NULL;
CREATE INDEX "stat_values_real_leaderboard_idx" ON "stat_values" ("stat_id", "real_value" DESC) WHERE "real_value" IS NOT NULL;
CREATE INDEX "stat_values_player_id_stat_id_idx" ON "stat_values" ("player_id", "stat_id");

CREATE TABLE "stat_records" (
    "id" BIGSERIAL NOT NULL,
    "stat_id" BIGINT NOT NULL,
    "player_id" BIGINT NOT NULL,
    "int_value" BIGINT,
    "real_value" DOUBLE PRECISION,
    "match_id" BIGINT NOT NULL,
    "round_id" BIGINT,
    "achieved_at" TIMESTAMPTZ NOT NULL,

    PRIMARY KEY ("id"),
    UNIQUE ("stat_id", "player_id"),
    CHECK (num_nonnulls("int_value", "real_value") = 1),
    FOREIGN KEY ("stat_id") REFERENCES "stats" ("id") ON DELETE CASCADE,
    FOREIGN KEY ("player_id") REFERENCES "players" ("id"),
    FOREIGN KEY ("match_id") REFERENCES "matches" ("id") ON DELETE CASCADE,
    FOREIGN KEY ("round_id") REFERENCES "rounds" ("id") ON DELETE CASCADE
);

CREATE INDEX "stat_records_int_leaderboard_idx" ON "stat_records" ("stat_id", "int_value" DESC) WHERE "int_value" IS NOT NULL;
CREATE INDEX "stat_records_real_leaderboard_idx" ON "stat_records" ("stat_id", "real_value" DESC) WHERE "real_value" IS NOT NULL;
CREATE INDEX "stat_records_player_id_idx" ON "stat_records" ("player_id");

CREATE TABLE "player_game_stats" (
    "id" BIGSERIAL NOT NULL,
    "player_id" BIGINT NOT NULL,
    "definition_id" BIGINT NOT NULL,
    "matches_played" INTEGER NOT NULL DEFAULT 0,
    "wins" INTEGER NOT NULL DEFAULT 0,
    "rounds_played" INTEGER NOT NULL DEFAULT 0,
    "points_total" BIGINT NOT NULL DEFAULT 0,
    "placement_sum" BIGINT NOT NULL DEFAULT 0,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT current_timestamp,

    PRIMARY KEY ("id"),
    UNIQUE ("player_id", "definition_id"),
    FOREIGN KEY ("player_id") REFERENCES "players" ("id"),
    FOREIGN KEY ("definition_id") REFERENCES "definitions" ("id") ON DELETE CASCADE
);

CREATE INDEX "player_game_stats_wins_idx" ON "player_game_stats" ("definition_id", "wins" DESC);

CREATE TABLE "player_minigame_stats" (
    "id" BIGSERIAL NOT NULL,
    "player_id" BIGINT NOT NULL,
    "definition_id" BIGINT NOT NULL,
    "rounds_played" INTEGER NOT NULL DEFAULT 0,
    "wins" INTEGER NOT NULL DEFAULT 0,
    "rank_sum" BIGINT NOT NULL DEFAULT 0,
    "points_total" BIGINT NOT NULL DEFAULT 0,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT current_timestamp,

    PRIMARY KEY ("id"),
    UNIQUE ("player_id", "definition_id"),
    FOREIGN KEY ("player_id") REFERENCES "players" ("id"),
    FOREIGN KEY ("definition_id") REFERENCES "definitions" ("id") ON DELETE CASCADE
);

CREATE INDEX "player_minigame_stats_wins_idx" ON "player_minigame_stats" ("definition_id", "wins" DESC);

-- +goose Down
DROP TABLE "player_minigame_stats";
DROP TABLE "player_game_stats";
DROP TABLE "stat_records";
DROP TABLE "stat_values";
