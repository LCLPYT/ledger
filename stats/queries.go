package stats

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/stephenafamo/scan"
)

// ErrNotFound reports a read for something that does not exist.
var ErrNotFound = errors.New("not found")

// ErrUnknownMetric reports a leaderboard request for a metric that is not supported.
var ErrUnknownMetric = errors.New("unknown metric")

type PlayerProfile struct {
	UUID        uuid.UUID           `json:"uuid"`
	Name        string              `json:"name"`
	FirstSeenAt time.Time           `json:"first_seen_at"`
	LastSeenAt  time.Time           `json:"last_seen_at"`
	Games       []GameAggregate     `json:"games"`
	Minigames   []MinigameAggregate `json:"minigames"`
}

type GameAggregate struct {
	Key              string   `db:"key" json:"key"`
	Name             *string  `db:"name" json:"name,omitempty"`
	MatchesPlayed    int      `db:"matches_played" json:"matches_played"`
	Wins             int      `db:"wins" json:"wins"`
	RoundsPlayed     int      `db:"rounds_played" json:"rounds_played"`
	PointsTotal      int64    `db:"points_total" json:"points_total"`
	AveragePlacement *float64 `db:"average_placement" json:"average_placement,omitempty"`
}

type MinigameAggregate struct {
	Key          string   `db:"key" json:"key"`
	Name         *string  `db:"name" json:"name,omitempty"`
	RoundsPlayed int      `db:"rounds_played" json:"rounds_played"`
	Wins         int      `db:"wins" json:"wins"`
	PointsTotal  int64    `db:"points_total" json:"points_total"`
	AverageRank  *float64 `db:"average_rank" json:"average_rank,omitempty"`
}

const playerGameAggregatesSQL = `
SELECT d."key", d."name", g."matches_played", g."wins", g."rounds_played", g."points_total",
    CASE WHEN g."matches_played" > 0 AND g."placement_sum" > 0
        THEN g."placement_sum"::float8 / g."matches_played" END AS "average_placement"
FROM "player_game_stats" g
JOIN "definitions" d ON d."id" = g."definition_id"
WHERE g."player_id" = $1
ORDER BY g."matches_played" DESC, d."key"
`

const playerMinigameAggregatesSQL = `
SELECT d."key", d."name", m."rounds_played", m."wins", m."points_total",
    CASE WHEN m."rounds_played" > 0 AND m."rank_sum" > 0
        THEN m."rank_sum"::float8 / m."rounds_played" END AS "average_rank"
FROM "player_minigame_stats" m
JOIN "definitions" d ON d."id" = m."definition_id"
WHERE m."player_id" = $1
ORDER BY m."rounds_played" DESC, d."key"
`

func (s *Service) PlayerProfile(ctx context.Context, playerUUID uuid.UUID) (*PlayerProfile, error) {
	player, err := s.playerByUUID(ctx, playerUUID)

	if err != nil {
		return nil, err
	}

	profile := &PlayerProfile{
		UUID:        player.UUID,
		Name:        player.Name,
		FirstSeenAt: player.FirstSeenAt,
		LastSeenAt:  player.LastSeenAt,
	}

	profile.Games, err = scan.All(ctx, s.db, scan.StructMapper[GameAggregate](), playerGameAggregatesSQL, player.ID)

	if err != nil {
		return nil, err
	}

	profile.Minigames, err = scan.All(ctx, s.db, scan.StructMapper[MinigameAggregate](), playerMinigameAggregatesSQL, player.ID)

	if err != nil {
		return nil, err
	}

	return profile, nil
}

type MatchSummary struct {
	PublicID  uuid.UUID `db:"public_id" json:"id"`
	GameKey   string    `db:"game_key" json:"game"`
	Status    string    `db:"status" json:"status"`
	StartedAt time.Time `db:"started_at" json:"started_at"`
	EndedAt   time.Time `db:"ended_at" json:"ended_at"`
	Rounds    int       `db:"rounds" json:"rounds_played"`
	Score     int32     `db:"score" json:"score"`
	Placement *int32    `db:"placement" json:"placement,omitempty"`
	Winner    bool      `db:"is_winner" json:"winner"`
}

const playerMatchesSQL = `
SELECT m."public_id", d."key" AS "game_key", m."status", m."started_at", m."ended_at",
    mp."rounds_played" AS "rounds", mp."score", mp."placement", mp."is_winner"
FROM "match_players" mp
JOIN "matches" m ON m."id" = mp."match_id"
JOIN "definition_versions" dv ON dv."id" = m."definition_version_id"
JOIN "definitions" d ON d."id" = dv."definition_id"
WHERE mp."player_id" = $1
ORDER BY m."started_at" DESC, m."id" DESC
LIMIT $2 OFFSET $3
`

func (s *Service) PlayerMatches(ctx context.Context, playerUUID uuid.UUID, limit, offset int) ([]MatchSummary, int, error) {
	player, err := s.playerByUUID(ctx, playerUUID)

	if err != nil {
		return nil, 0, err
	}

	var total int

	row := s.db.QueryRowContext(ctx, `SELECT count(*) FROM "match_players" WHERE "player_id" = $1`, player.ID)

	if err := row.Scan(&total); err != nil {
		return nil, 0, err
	}

	matches, err := scan.All(ctx, s.db, scan.StructMapper[MatchSummary](), playerMatchesSQL, player.ID, limit, offset)

	if err != nil {
		return nil, 0, err
	}

	return matches, total, nil
}

type PlayerRecord struct {
	Definition     string    `db:"definition_key" json:"definition"`
	Stat           string    `db:"stat_key" json:"stat"`
	Unit           string    `db:"unit" json:"unit"`
	ValueType      string    `db:"value_type" json:"type"`
	HigherIsBetter bool      `db:"higher_is_better" json:"higher_is_better"`
	Value          float64   `db:"value" json:"value"`
	GlobalRank     int       `db:"global_rank" json:"global_rank"`
	AchievedAt     time.Time `db:"achieved_at" json:"achieved_at"`
	Match          uuid.UUID `db:"match_public_id" json:"match"`
}

const playerRecordsSQL = `
WITH "ranked" AS (
    SELECT r."stat_id", r."player_id", r."match_id", r."achieved_at",
        coalesce(r."int_value", r."real_value") AS "value",
        rank() OVER (
            PARTITION BY r."stat_id"
            ORDER BY CASE WHEN coalesce(s."higher_is_better", true)
                THEN coalesce(r."int_value", r."real_value") END DESC NULLS LAST,
            CASE WHEN NOT coalesce(s."higher_is_better", true)
                THEN coalesce(r."int_value", r."real_value") END ASC NULLS LAST
        ) AS "global_rank"
    FROM "stat_records" r
    JOIN "stats" s ON s."id" = r."stat_id"
    WHERE r."stat_id" IN (SELECT "stat_id" FROM "stat_records" WHERE "player_id" = $1)
)
SELECT d."key" AS "definition_key", s."key" AS "stat_key", s."unit", s."value_type",
    coalesce(s."higher_is_better", true) AS "higher_is_better",
    ranked."value", ranked."global_rank"::int, ranked."achieved_at", m."public_id" AS "match_public_id"
FROM "ranked"
JOIN "stats" s ON s."id" = ranked."stat_id"
JOIN "definitions" d ON d."id" = s."definition_id"
JOIN "matches" m ON m."id" = ranked."match_id"
WHERE ranked."player_id" = $1
ORDER BY d."key", s."key"
`

func (s *Service) PlayerRecords(ctx context.Context, playerUUID uuid.UUID) ([]PlayerRecord, error) {
	player, err := s.playerByUUID(ctx, playerUUID)

	if err != nil {
		return nil, err
	}

	return scan.All(ctx, s.db, scan.StructMapper[PlayerRecord](), playerRecordsSQL, player.ID)
}

type LeaderboardEntry struct {
	Rank       int       `db:"rank" json:"rank"`
	PlayerUUID uuid.UUID `db:"uuid" json:"player"`
	PlayerName string    `db:"name" json:"name"`
	Value      float64   `db:"value" json:"value"`
	AchievedAt time.Time `db:"achieved_at" json:"achieved_at"`
}

const statLeaderboardSQL = `
SELECT
    rank() OVER (
        ORDER BY CASE WHEN coalesce(s."higher_is_better", true)
            THEN coalesce(r."int_value", r."real_value") END DESC NULLS LAST,
        CASE WHEN NOT coalesce(s."higher_is_better", true)
            THEN coalesce(r."int_value", r."real_value") END ASC NULLS LAST
    )::int AS "rank",
    p."uuid", p."name",
    coalesce(r."int_value", r."real_value") AS "value",
    r."achieved_at"
FROM "stat_records" r
JOIN "stats" s ON s."id" = r."stat_id"
JOIN "players" p ON p."id" = r."player_id"
WHERE s."id" = $1
ORDER BY "rank"
LIMIT $2
`

// StatLeaderboard returns the global best per player for one stat, best first.
func (s *Service) StatLeaderboard(ctx context.Context, definitionKey, statKey string, limit int) ([]LeaderboardEntry, error) {
	var statID int64

	row := s.db.QueryRowContext(ctx, `
		SELECT s."id"
		FROM "stats" s
		JOIN "definitions" d ON d."id" = s."definition_id"
		WHERE d."key" = $1 AND s."key" = $2 AND s."scope" = 'player'
	`, definitionKey, statKey)

	if err := row.Scan(&statID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	return scan.All(ctx, s.db, scan.StructMapper[LeaderboardEntry](), statLeaderboardSQL, statID, limit)
}

type GameLeaderboardEntry struct {
	Rank       int       `db:"rank" json:"rank"`
	PlayerUUID uuid.UUID `db:"uuid" json:"player"`
	PlayerName string    `db:"name" json:"name"`
	Value      float64   `db:"value" json:"value"`
}

// gameMetrics maps a metric name to its expression and sort direction.
var gameMetrics = map[string]struct {
	expr string
	desc bool
}{
	"wins":          {`g."wins"::float8`, true},
	"matches":       {`g."matches_played"::float8`, true},
	"points":        {`g."points_total"::float8`, true},
	"rounds":        {`g."rounds_played"::float8`, true},
	"avg_placement": {`CASE WHEN g."placement_sum" > 0 THEN g."placement_sum"::float8 / g."matches_played" END`, false},
}

func GameMetrics() []string {
	names := make([]string, 0, len(gameMetrics))

	for name := range gameMetrics {
		names = append(names, name)
	}

	return names
}

func (s *Service) GameLeaderboard(ctx context.Context, gameKey, metric string, limit int) ([]GameLeaderboardEntry, error) {
	definition, ok := gameMetrics[metric]

	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownMetric, metric)
	}

	direction := "DESC"

	if !definition.desc {
		direction = "ASC"
	}

	// Only expressions from the gameMetrics literal reach the format string;
	// an unknown metric is rejected above and the request values stay bound.
	query := fmt.Sprintf(`
		SELECT rank() OVER (ORDER BY %s %s NULLS LAST)::int AS "rank",
		    p."uuid", p."name", %s AS "value"
		FROM "player_game_stats" g
		JOIN "definitions" d ON d."id" = g."definition_id"
		JOIN "players" p ON p."id" = g."player_id"
		WHERE d."key" = $1 AND %s IS NOT NULL
		ORDER BY "rank"
		LIMIT $2
	`, definition.expr, direction, definition.expr, definition.expr)

	return scan.All(ctx, s.db, scan.StructMapper[GameLeaderboardEntry](), query, gameKey, limit)
}

type DefinitionSummary struct {
	Kind     string   `db:"kind" json:"kind"`
	Key      string   `db:"key" json:"key"`
	Name     *string  `db:"name" json:"name,omitempty"`
	Group    *string  `db:"group_key" json:"group,omitempty"`
	Versions []string `json:"versions"`
	Stats    int      `db:"stats" json:"stats"`
}

const listDefinitionsSQL = `
SELECT d."kind", d."key", d."name", d."group_key",
    (SELECT count(*) FROM "stats" s WHERE s."definition_id" = d."id")::int AS "stats",
    coalesce(json_agg(dv."version" ORDER BY dv."registered_at") FILTER (WHERE dv."id" IS NOT NULL), '[]')::text AS "versions"
FROM "definitions" d
LEFT JOIN "definition_versions" dv ON dv."definition_id" = d."id"
WHERE ($1 = '' OR d."kind"::text = $1) AND ($2 = '' OR d."key" = $2)
GROUP BY d."id"
ORDER BY d."kind", d."key"
`

func (s *Service) ListDefinitions(ctx context.Context, kind, key string) ([]DefinitionSummary, error) {
	rows, err := s.db.QueryContext(ctx, listDefinitionsSQL, kind, key)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	definitions := make([]DefinitionSummary, 0)

	for rows.Next() {
		var summary DefinitionSummary
		var versions string

		if err := rows.Scan(&summary.Kind, &summary.Key, &summary.Name, &summary.Group, &summary.Stats, &versions); err != nil {
			return nil, err
		}

		if err := json.Unmarshal([]byte(versions), &summary.Versions); err != nil {
			return nil, err
		}

		definitions = append(definitions, summary)
	}

	return definitions, rows.Err()
}

func (s *Service) playerByUUID(ctx context.Context, playerUUID uuid.UUID) (*playerRow, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT "id", "uuid", "name", "first_seen_at", "last_seen_at"
		FROM "players" WHERE "uuid" = $1
	`, playerUUID)

	var player playerRow

	if err := row.Scan(&player.ID, &player.UUID, &player.Name, &player.FirstSeenAt, &player.LastSeenAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	return &player, nil
}

func notFoundOr(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}

	return err
}

type playerRow struct {
	ID          int64
	UUID        uuid.UUID
	Name        string
	FirstSeenAt time.Time
	LastSeenAt  time.Time
}
