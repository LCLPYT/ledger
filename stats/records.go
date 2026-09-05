package stats

import (
	"context"

	"github.com/stephenafamo/bob"
)

const rebuildRecordsSQL = `
INSERT INTO "stat_records"
    ("stat_id", "player_id", "int_value", "real_value", "match_id", "round_id", "achieved_at")
SELECT DISTINCT ON (sv."stat_id", sv."player_id")
    sv."stat_id", sv."player_id", sv."int_value", sv."real_value", sv."match_id", sv."round_id", m."ended_at"
FROM "stat_values" sv
JOIN "stats" s ON s."id" = sv."stat_id"
JOIN "matches" m ON m."id" = sv."match_id"
WHERE s."is_record" AND sv."player_id" IS NOT NULL
ORDER BY
    sv."stat_id",
    sv."player_id",
    CASE WHEN coalesce(s."higher_is_better", true)
        THEN coalesce(sv."int_value", sv."real_value") END DESC NULLS LAST,
    CASE WHEN NOT coalesce(s."higher_is_better", true)
        THEN coalesce(sv."int_value", sv."real_value") END ASC NULLS LAST,
    m."ended_at" ASC,
    sv."id" ASC
`

const rebuildGameStatsSQL = `
INSERT INTO "player_game_stats"
    ("player_id", "definition_id", "matches_played", "wins", "rounds_played", "points_total", "placement_sum")
SELECT
    mp."player_id",
    dv."definition_id",
    count(*)::int,
    coalesce(sum(mp."is_winner"::int), 0)::int,
    coalesce(sum(mp."rounds_played"), 0)::int,
    coalesce(sum(pts."points"), 0),
    coalesce(sum(mp."placement"), 0)
FROM "match_players" mp
JOIN "matches" m ON m."id" = mp."match_id"
JOIN "definition_versions" dv ON dv."id" = m."definition_version_id"
LEFT JOIN LATERAL (
    SELECT sum(rp."points_awarded") AS "points"
    FROM "round_players" rp
    JOIN "rounds" r ON r."id" = rp."round_id"
    WHERE r."match_id" = mp."match_id" AND rp."player_id" = mp."player_id"
) pts ON true
GROUP BY mp."player_id", dv."definition_id"
`

const rebuildMinigameStatsSQL = `
INSERT INTO "player_minigame_stats"
    ("player_id", "definition_id", "rounds_played", "wins", "rank_sum", "points_total")
SELECT
    rp."player_id",
    dv."definition_id",
    count(*)::int,
    coalesce(sum((rp."rank" = 1)::int), 0)::int,
    coalesce(sum(rp."rank"), 0),
    coalesce(sum(rp."points_awarded"), 0)
FROM "round_players" rp
JOIN "rounds" r ON r."id" = rp."round_id"
JOIN "definition_versions" dv ON dv."id" = r."definition_version_id"
GROUP BY rp."player_id", dv."definition_id"
`

// Rebuild recomputes every derived table from the raw match records. The
// derived tables are maintained during ingest, so this only needs to run after
// a manual repair or a change to the aggregation rules. Ties are broken towards
// the value recorded first, matching what ingest keeps.
func (s *Service) Rebuild(ctx context.Context) error {
	return s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bob.Transaction) error {
		steps := []struct {
			clear string
			fill  string
		}{
			{`DELETE FROM "stat_records"`, rebuildRecordsSQL},
			{`DELETE FROM "player_game_stats"`, rebuildGameStatsSQL},
			{`DELETE FROM "player_minigame_stats"`, rebuildMinigameStatsSQL},
		}

		for _, step := range steps {
			if _, err := tx.ExecContext(ctx, step.clear); err != nil {
				return err
			}

			if _, err := tx.ExecContext(ctx, step.fill); err != nil {
				return err
			}
		}

		return nil
	})
}
