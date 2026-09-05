-- +goose Up
CREATE TABLE "players" (
    "id" BIGSERIAL NOT NULL,
    "uuid" UUID NOT NULL,
    "name" text NOT NULL,
    "first_seen_at" TIMESTAMPTZ NOT NULL DEFAULT current_timestamp,
    "last_seen_at" TIMESTAMPTZ NOT NULL DEFAULT current_timestamp,

    PRIMARY KEY ("id"),
    UNIQUE ("uuid")
);

CREATE TABLE "matches" (
    "id" BIGSERIAL NOT NULL,
    "public_id" UUID NOT NULL,
    "definition_version_id" BIGINT NOT NULL,
    "status" match_status NOT NULL,
    "started_at" TIMESTAMPTZ NOT NULL,
    "ended_at" TIMESTAMPTZ NOT NULL,
    "minecraft_version" text,
    "server_name" text,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT current_timestamp,

    PRIMARY KEY ("id"),
    UNIQUE ("public_id"),
    FOREIGN KEY ("definition_version_id") REFERENCES "definition_versions" ("id")
);

CREATE INDEX "matches_started_at_idx" ON "matches" ("started_at" DESC);

CREATE TABLE "match_players" (
    "id" BIGSERIAL NOT NULL,
    "match_id" BIGINT NOT NULL,
    "player_id" BIGINT NOT NULL,
    "score" INTEGER NOT NULL DEFAULT 0,
    "placement" INTEGER,
    "is_winner" BOOLEAN NOT NULL DEFAULT false,
    "rounds_played" INTEGER NOT NULL DEFAULT 0,

    PRIMARY KEY ("id"),
    UNIQUE ("match_id", "player_id"),
    FOREIGN KEY ("match_id") REFERENCES "matches" ("id") ON DELETE CASCADE,
    FOREIGN KEY ("player_id") REFERENCES "players" ("id")
);

CREATE INDEX "match_players_player_id_idx" ON "match_players" ("player_id");

CREATE TABLE "match_player_sessions" (
    "id" BIGSERIAL NOT NULL,
    "match_player_id" BIGINT NOT NULL,
    "joined_at" TIMESTAMPTZ NOT NULL,
    "left_at" TIMESTAMPTZ,

    PRIMARY KEY ("id"),
    FOREIGN KEY ("match_player_id") REFERENCES "match_players" ("id") ON DELETE CASCADE
);

CREATE INDEX "match_player_sessions_match_player_id_idx" ON "match_player_sessions" ("match_player_id");

CREATE TABLE "rounds" (
    "id" BIGSERIAL NOT NULL,
    "match_id" BIGINT NOT NULL,
    "index" INTEGER NOT NULL,
    "definition_version_id" BIGINT NOT NULL,
    "status" round_status NOT NULL,
    "started_at" TIMESTAMPTZ NOT NULL,
    "ended_at" TIMESTAMPTZ NOT NULL,
    "map_id" text,
    "map_seed" BIGINT,
    "stats_ref" UUID,

    PRIMARY KEY ("id"),
    UNIQUE ("match_id", "index"),
    FOREIGN KEY ("match_id") REFERENCES "matches" ("id") ON DELETE CASCADE,
    FOREIGN KEY ("definition_version_id") REFERENCES "definition_versions" ("id")
);

CREATE INDEX "rounds_definition_version_id_idx" ON "rounds" ("definition_version_id");

CREATE TABLE "round_teams" (
    "id" BIGSERIAL NOT NULL,
    "round_id" BIGINT NOT NULL,
    "team_key" text NOT NULL,
    "rank" INTEGER,

    PRIMARY KEY ("id"),
    UNIQUE ("round_id", "team_key"),
    FOREIGN KEY ("round_id") REFERENCES "rounds" ("id") ON DELETE CASCADE
);

CREATE TABLE "round_players" (
    "id" BIGSERIAL NOT NULL,
    "round_id" BIGINT NOT NULL,
    "player_id" BIGINT NOT NULL,
    "round_team_id" BIGINT,
    "rank" INTEGER,
    "points_awarded" INTEGER NOT NULL DEFAULT 0,

    PRIMARY KEY ("id"),
    UNIQUE ("round_id", "player_id"),
    FOREIGN KEY ("round_id") REFERENCES "rounds" ("id") ON DELETE CASCADE,
    FOREIGN KEY ("player_id") REFERENCES "players" ("id"),
    FOREIGN KEY ("round_team_id") REFERENCES "round_teams" ("id") ON DELETE CASCADE
);

CREATE INDEX "round_players_player_id_idx" ON "round_players" ("player_id");

-- +goose Down
DROP TABLE "round_players";
DROP TABLE "round_teams";
DROP TABLE "rounds";
DROP TABLE "match_player_sessions";
DROP TABLE "match_players";
DROP TABLE "matches";
DROP TABLE "players";
