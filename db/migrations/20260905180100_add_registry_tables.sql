-- +goose Up
CREATE TABLE "definitions" (
    "id" BIGSERIAL NOT NULL,
    "kind" definition_kind NOT NULL,
    "key" text NOT NULL,
    "name" text,
    "group_key" text,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT current_timestamp,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT current_timestamp,

    PRIMARY KEY ("id"),
    UNIQUE ("kind", "key")
);

CREATE TABLE "definition_versions" (
    "id" BIGSERIAL NOT NULL,
    "definition_id" BIGINT NOT NULL,
    "version" text NOT NULL,
    "game_type" game_type,
    "fingerprint" BYTEA NOT NULL,
    "registered_at" TIMESTAMPTZ NOT NULL DEFAULT current_timestamp,

    PRIMARY KEY ("id"),
    UNIQUE ("definition_id", "version"),
    FOREIGN KEY ("definition_id") REFERENCES "definitions" ("id") ON DELETE CASCADE
);

CREATE TABLE "stats" (
    "id" BIGSERIAL NOT NULL,
    "definition_id" BIGINT NOT NULL,
    "scope" stat_scope NOT NULL,
    "key" text NOT NULL,
    "name" text,
    "value_type" stat_value_type NOT NULL,
    "higher_is_better" BOOLEAN,
    "unit" text NOT NULL DEFAULT 'plain',
    "display" BOOLEAN NOT NULL DEFAULT true,
    "is_record" BOOLEAN NOT NULL DEFAULT false,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT current_timestamp,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT current_timestamp,

    PRIMARY KEY ("id"),
    UNIQUE ("definition_id", "scope", "key"),
    FOREIGN KEY ("definition_id") REFERENCES "definitions" ("id") ON DELETE CASCADE
);

CREATE TABLE "definition_version_stats" (
    "definition_version_id" BIGINT NOT NULL,
    "stat_id" BIGINT NOT NULL,

    PRIMARY KEY ("definition_version_id", "stat_id"),
    FOREIGN KEY ("definition_version_id") REFERENCES "definition_versions" ("id") ON DELETE CASCADE,
    FOREIGN KEY ("stat_id") REFERENCES "stats" ("id") ON DELETE CASCADE
);

-- +goose Down
DROP TABLE "definition_version_stats";
DROP TABLE "stats";
DROP TABLE "definition_versions";
DROP TABLE "definitions";
