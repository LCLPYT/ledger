package stats

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"time"

	"ledger/model"
	"ledger/model/enums"

	"github.com/aarondl/opt/null"
	"github.com/aarondl/opt/omit"
	"github.com/aarondl/opt/omitnull"
	"github.com/gofrs/uuid/v5"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql/dialect"
	"github.com/stephenafamo/bob/dialect/psql/im"
)

type insertMods = []bob.Mod[*dialect.InsertQuery]

// IngestMatch stores a complete match. It is idempotent on the payload id:
// re-submitting a match that was already stored reports the existing record
// instead of writing a duplicate.
func (s *Service) IngestMatch(ctx context.Context, in *MatchInput) (*IngestResult, error) {
	plan, err := s.validateMatch(ctx, in)

	if err != nil {
		return nil, err
	}

	existing, err := s.findMatch(ctx, s.db, in.ID)

	if err != nil {
		return nil, err
	}

	if existing != nil {
		return &IngestResult{MatchID: existing.ID, PublicID: existing.PublicID}, nil
	}

	result := &IngestResult{PublicID: in.ID, Created: true}

	err = s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bob.Transaction) error {
		matchID, created, err := writeMatch(ctx, tx, plan)

		if err != nil {
			return err
		}

		result.MatchID = matchID
		result.Created = created

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *Service) findMatch(ctx context.Context, exec bob.Executor, publicID uuid.UUID) (*model.Match, error) {
	match, err := model.Matches.Query(
		model.SelectWhere.Matches.PublicID.EQ(publicID),
	).One(ctx, exec)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return match, nil
}

func writeMatch(ctx context.Context, tx bob.Transaction, plan *matchPlan) (int64, bool, error) {
	in := plan.input

	players, err := upsertPlayers(ctx, tx, in.Players)

	if err != nil {
		return 0, false, err
	}

	match, err := model.Matches.Insert(
		&model.MatchSetter{
			PublicID:            omit.From(in.ID),
			DefinitionVersionID: omit.From(plan.game.VersionID),
			Status:              omit.From(plan.status),
			StartedAt:           omit.From(in.StartedAt),
			EndedAt:             omit.From(in.EndedAt),
			MinecraftVersion:    nullText(in.MinecraftVersion),
			ServerName:          nullText(in.ServerName),
		},
		im.OnConflict("public_id").DoNothing(),
	).One(ctx, tx)

	if errors.Is(err, sql.ErrNoRows) {
		existing, err := model.Matches.Query(
			model.SelectWhere.Matches.PublicID.EQ(in.ID),
		).One(ctx, tx)

		if err != nil {
			return 0, false, err
		}

		return existing.ID, false, nil
	}

	if err != nil {
		return 0, false, err
	}

	tally := tallyRounds(plan, players)

	matchPlayers, err := writeMatchPlayers(ctx, tx, match.ID, in, players, tally)

	if err != nil {
		return 0, false, err
	}

	if err := writePresence(ctx, tx, in, players, matchPlayers); err != nil {
		return 0, false, err
	}

	values := make([]statValueRow, 0, 64)

	for _, value := range plan.matchStats {
		values = append(values, statValueRow{
			statID:  value.meta.ID,
			matchID: match.ID,
			value:   value.value,
		})
	}

	for i := range plan.rounds {
		rows, err := writeRound(ctx, tx, match.ID, &plan.rounds[i], players)

		if err != nil {
			return 0, false, err
		}

		values = append(values, rows...)
	}

	if err := insertStatValues(ctx, tx, values); err != nil {
		return 0, false, err
	}

	if err := updateRecords(ctx, tx, values, in.EndedAt); err != nil {
		return 0, false, err
	}

	if err := updateAggregates(ctx, tx, plan, players, tally); err != nil {
		return 0, false, err
	}

	return match.ID, true, nil
}

func upsertPlayers(ctx context.Context, tx bob.Transaction, in []PlayerInput) (map[uuid.UUID]int64, error) {
	setters := make(insertMods, 0, len(in)+1)

	for _, player := range in {
		setters = append(setters, &model.PlayerSetter{
			UUID: omit.From(player.UUID),
			Name: omit.From(player.Name),
		})
	}

	setters = append(setters, im.OnConflict("uuid").DoUpdate(
		im.SetExcluded("name"),
		im.SetCol("last_seen_at").To("current_timestamp"),
	))

	rows, err := model.Players.Insert(setters...).All(ctx, tx)

	if err != nil {
		return nil, err
	}

	players := make(map[uuid.UUID]int64, len(rows))

	for _, row := range rows {
		players[row.UUID] = row.ID
	}

	return players, nil
}

// playerTally holds the per player totals derived from the rounds of a match.
type playerTally struct {
	rounds int32
	points int64
}

func tallyRounds(plan *matchPlan, players map[uuid.UUID]int64) map[int64]*playerTally {
	tally := make(map[int64]*playerTally, len(players))

	for i := range plan.rounds {
		for _, player := range plan.rounds[i].input.Players {
			id := players[player.Player]

			entry, ok := tally[id]

			if !ok {
				entry = &playerTally{}
				tally[id] = entry
			}

			entry.rounds++
			entry.points += int64(player.Points)
		}
	}

	return tally
}

func writeMatchPlayers(ctx context.Context, tx bob.Transaction, matchID int64, in *MatchInput, players map[uuid.UUID]int64, tally map[int64]*playerTally) (map[uuid.UUID]int64, error) {
	results := make(map[uuid.UUID]MatchResultInput, len(in.Results))

	for _, result := range in.Results {
		results[result.Player] = result
	}

	setters := make(insertMods, 0, len(in.Players))

	for _, player := range in.Players {
		id := players[player.UUID]
		result := results[player.UUID]

		rounds := int32(0)

		if entry, ok := tally[id]; ok {
			rounds = entry.rounds
		}

		setters = append(setters, &model.MatchPlayerSetter{
			MatchID:      omit.From(matchID),
			PlayerID:     omit.From(id),
			Score:        omit.From(result.Score),
			Placement:    nullInt32(result.Placement),
			IsWinner:     omit.From(result.Winner),
			RoundsPlayed: omit.From(rounds),
		})
	}

	rows, err := model.MatchPlayers.Insert(setters...).All(ctx, tx)

	if err != nil {
		return nil, err
	}

	byPlayerID := make(map[int64]int64, len(rows))

	for _, row := range rows {
		byPlayerID[row.PlayerID] = row.ID
	}

	matchPlayers := make(map[uuid.UUID]int64, len(rows))

	for playerUUID, id := range players {
		matchPlayers[playerUUID] = byPlayerID[id]
	}

	return matchPlayers, nil
}

func writePresence(ctx context.Context, tx bob.Transaction, in *MatchInput, players map[uuid.UUID]int64, matchPlayers map[uuid.UUID]int64) error {
	if len(in.Presence) == 0 {
		return nil
	}

	setters := make(insertMods, 0, len(in.Presence))

	for _, presence := range in.Presence {
		setters = append(setters, &model.MatchPlayerSessionSetter{
			MatchPlayerID: omit.From(matchPlayers[presence.Player]),
			JoinedAt:      omit.From(presence.JoinedAt),
			LeftAt:        nullTime(presence.LeftAt),
		})
	}

	_, err := model.MatchPlayerSessions.Insert(setters...).All(ctx, tx)

	return err
}

type statValueRow struct {
	statID      int64
	matchID     int64
	roundID     null.Val[int64]
	playerID    null.Val[int64]
	roundTeamID null.Val[int64]
	value       statValue
	meta        statMeta
}

func writeRound(ctx context.Context, tx bob.Transaction, matchID int64, plan *roundPlan, players map[uuid.UUID]int64) ([]statValueRow, error) {
	in := plan.input

	setter := &model.RoundSetter{
		MatchID:             omit.From(matchID),
		Index:               omit.From(in.Index),
		DefinitionVersionID: omit.From(plan.schema.VersionID),
		Status:              omit.From(enums.RoundStatus(in.Status)),
		StartedAt:           omit.From(in.StartedAt),
		EndedAt:             omit.From(in.EndedAt),
		StatsRef:            nullUUID(in.StatsRef),
	}

	if in.Map != nil {
		setter.MapID = nullText(in.Map.ID)
		setter.MapSeed = nullInt64(in.Map.Seed)
	}

	round, err := model.Rounds.Insert(setter).One(ctx, tx)

	if err != nil {
		return nil, err
	}

	teams := make(map[string]int64, len(in.Teams))

	if len(in.Teams) > 0 {
		setters := make(insertMods, 0, len(in.Teams))

		for _, team := range in.Teams {
			setters = append(setters, &model.RoundTeamSetter{
				RoundID: omit.From(round.ID),
				TeamKey: omit.From(team.Key),
				Rank:    nullInt32(team.Rank),
			})
		}

		rows, err := model.RoundTeams.Insert(setters...).All(ctx, tx)

		if err != nil {
			return nil, err
		}

		for _, row := range rows {
			teams[row.TeamKey] = row.ID
		}
	}

	if len(in.Players) > 0 {
		setters := make(insertMods, 0, len(in.Players))

		for _, player := range in.Players {
			roundTeamID := omitnull.Val[int64]{}

			if id, ok := teams[player.Team]; ok {
				roundTeamID = omitnull.From(id)
			}

			setters = append(setters, &model.RoundPlayerSetter{
				RoundID:       omit.From(round.ID),
				PlayerID:      omit.From(players[player.Player]),
				RoundTeamID:   roundTeamID,
				Rank:          nullInt32(player.Rank),
				PointsAwarded: omit.From(player.Points),
			})
		}

		if _, err := model.RoundPlayers.Insert(setters...).All(ctx, tx); err != nil {
			return nil, err
		}
	}

	rows := make([]statValueRow, 0, len(plan.playerStats)+len(plan.teamStats))

	for playerUUID, values := range plan.playerStats {
		for _, value := range values {
			rows = append(rows, statValueRow{
				statID:   value.meta.ID,
				matchID:  matchID,
				roundID:  null.From(round.ID),
				playerID: null.From(players[playerUUID]),
				value:    value.value,
				meta:     value.meta,
			})
		}
	}

	for teamKey, values := range plan.teamStats {
		teamID, ok := teams[teamKey]

		if !ok {
			continue
		}

		for _, value := range values {
			rows = append(rows, statValueRow{
				statID:      value.meta.ID,
				matchID:     matchID,
				roundID:     null.From(round.ID),
				roundTeamID: null.From(teamID),
				value:       value.value,
				meta:        value.meta,
			})
		}
	}

	return rows, nil
}

// statValueBatch caps how many rows go into one INSERT. Each row binds eight
// parameters and postgres allows 65535 per statement, so a long match would
// otherwise be rejected outright.
const statValueBatch = 1000

func insertStatValues(ctx context.Context, tx bob.Transaction, rows []statValueRow) error {
	for chunk := range slices.Chunk(rows, statValueBatch) {
		setters := make(insertMods, 0, len(chunk))

		for _, row := range chunk {
			setters = append(setters, &model.StatValueSetter{
				StatID:      omit.From(row.statID),
				MatchID:     omit.From(row.matchID),
				RoundID:     omitnull.FromNull(row.roundID),
				PlayerID:    omitnull.FromNull(row.playerID),
				RoundTeamID: omitnull.FromNull(row.roundTeamID),
				IntValue:    omitnull.FromNull(row.value.IntValue),
				RealValue:   omitnull.FromNull(row.value.RealValue),
				TextValue:   omitnull.FromNull(row.value.TextValue),
			})
		}

		if _, err := model.StatValues.Insert(setters...).All(ctx, tx); err != nil {
			return err
		}
	}

	return nil
}

const upsertRecordSQL = `
INSERT INTO "stat_records"
    ("stat_id", "player_id", "int_value", "real_value", "match_id", "round_id", "achieved_at")
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT ("stat_id", "player_id") DO UPDATE SET
    "int_value" = EXCLUDED."int_value",
    "real_value" = EXCLUDED."real_value",
    "match_id" = EXCLUDED."match_id",
    "round_id" = EXCLUDED."round_id",
    "achieved_at" = EXCLUDED."achieved_at"
WHERE
    ($8 AND coalesce(EXCLUDED."int_value", EXCLUDED."real_value") > coalesce("stat_records"."int_value", "stat_records"."real_value"))
    OR (NOT $8 AND coalesce(EXCLUDED."int_value", EXCLUDED."real_value") < coalesce("stat_records"."int_value", "stat_records"."real_value"))
`

func updateRecords(ctx context.Context, tx bob.Transaction, rows []statValueRow, achievedAt time.Time) error {
	for _, row := range rows {
		if !row.meta.IsRecord || row.playerID.IsNull() {
			continue
		}

		_, err := tx.ExecContext(ctx, upsertRecordSQL,
			row.statID,
			row.playerID.GetOrZero(),
			nullableInt64(row.value.IntValue),
			nullableFloat64(row.value.RealValue),
			row.matchID,
			nullableInt64(row.roundID),
			achievedAt,
			row.meta.HigherIsBetter,
		)

		if err != nil {
			return err
		}
	}

	return nil
}

const upsertGameStatsSQL = `
INSERT INTO "player_game_stats"
    ("player_id", "definition_id", "matches_played", "wins", "rounds_played", "points_total", "placement_sum")
VALUES ($1, $2, 1, $3, $4, $5, $6)
ON CONFLICT ("player_id", "definition_id") DO UPDATE SET
    "matches_played" = "player_game_stats"."matches_played" + 1,
    "wins" = "player_game_stats"."wins" + EXCLUDED."wins",
    "rounds_played" = "player_game_stats"."rounds_played" + EXCLUDED."rounds_played",
    "points_total" = "player_game_stats"."points_total" + EXCLUDED."points_total",
    "placement_sum" = "player_game_stats"."placement_sum" + EXCLUDED."placement_sum",
    "updated_at" = current_timestamp
`

const upsertMinigameStatsSQL = `
INSERT INTO "player_minigame_stats"
    ("player_id", "definition_id", "rounds_played", "wins", "rank_sum", "points_total")
VALUES ($1, $2, 1, $3, $4, $5)
ON CONFLICT ("player_id", "definition_id") DO UPDATE SET
    "rounds_played" = "player_minigame_stats"."rounds_played" + 1,
    "wins" = "player_minigame_stats"."wins" + EXCLUDED."wins",
    "rank_sum" = "player_minigame_stats"."rank_sum" + EXCLUDED."rank_sum",
    "points_total" = "player_minigame_stats"."points_total" + EXCLUDED."points_total",
    "updated_at" = current_timestamp
`

func updateAggregates(ctx context.Context, tx bob.Transaction, plan *matchPlan, players map[uuid.UUID]int64, tally map[int64]*playerTally) error {
	winners := make(map[uuid.UUID]bool, len(plan.input.Results))
	placements := make(map[uuid.UUID]int64, len(plan.input.Results))

	for _, result := range plan.input.Results {
		winners[result.Player] = result.Winner

		if result.Placement != nil {
			placements[result.Player] = int64(*result.Placement)
		}
	}

	for _, player := range plan.input.Players {
		id := players[player.UUID]

		entry, ok := tally[id]

		if !ok {
			entry = &playerTally{}
		}

		wins := 0

		if winners[player.UUID] {
			wins = 1
		}

		_, err := tx.ExecContext(ctx, upsertGameStatsSQL,
			id, plan.game.DefinitionID, wins, entry.rounds, entry.points, placements[player.UUID])

		if err != nil {
			return err
		}
	}

	for i := range plan.rounds {
		round := &plan.rounds[i]

		for _, player := range round.input.Players {
			wins := 0
			rank := int64(0)

			if player.Rank != nil {
				rank = int64(*player.Rank)

				if *player.Rank == 1 {
					wins = 1
				}
			}

			_, err := tx.ExecContext(ctx, upsertMinigameStatsSQL,
				players[player.Player], round.schema.DefinitionID, wins, rank, int64(player.Points))

			if err != nil {
				return err
			}
		}
	}

	return nil
}

func nullInt32(v *int32) omitnull.Val[int32] {
	if v == nil {
		return nullValue[int32]()
	}

	return omitnull.From(*v)
}

func nullInt64(v *int64) omitnull.Val[int64] {
	if v == nil {
		return nullValue[int64]()
	}

	return omitnull.From(*v)
}

func nullTime(v *time.Time) omitnull.Val[time.Time] {
	if v == nil {
		return nullValue[time.Time]()
	}

	return omitnull.From(*v)
}

func nullUUID(v *uuid.UUID) omitnull.Val[uuid.UUID] {
	if v == nil {
		return nullValue[uuid.UUID]()
	}

	return omitnull.From(*v)
}

func nullableInt64(v null.Val[int64]) any {
	if value, ok := v.Get(); ok {
		return value
	}

	return nil
}

func nullableFloat64(v null.Val[float64]) any {
	if value, ok := v.Get(); ok {
		return value
	}

	return nil
}
