-- +goose Up
-- Leaderboards order by a CASE over coalesce("int_value", "real_value"), which
-- casts the integer column to double precision. That expression cannot match an
-- index on the plain column, so these were never used and only cost write time.
DROP INDEX "stat_values_int_leaderboard_idx";
DROP INDEX "stat_values_real_leaderboard_idx";
DROP INDEX "stat_records_int_leaderboard_idx";
DROP INDEX "stat_records_real_leaderboard_idx";

-- +goose Down
CREATE INDEX "stat_values_int_leaderboard_idx" ON "stat_values" ("stat_id", "int_value" DESC) WHERE "int_value" IS NOT NULL;
CREATE INDEX "stat_values_real_leaderboard_idx" ON "stat_values" ("stat_id", "real_value" DESC) WHERE "real_value" IS NOT NULL;
CREATE INDEX "stat_records_int_leaderboard_idx" ON "stat_records" ("stat_id", "int_value" DESC) WHERE "int_value" IS NOT NULL;
CREATE INDEX "stat_records_real_leaderboard_idx" ON "stat_records" ("stat_id", "real_value" DESC) WHERE "real_value" IS NOT NULL;
