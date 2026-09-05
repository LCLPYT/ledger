package stats

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/gofrs/uuid/v5"
)

type fixture struct {
	service *Service
	alice   uuid.UUID
	bob     uuid.UUID
	base    time.Time
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	service := newService(t)

	manifest := Manifest{Definitions: []DefinitionInput{arcadeParty("1.0.0"), aimMaster("1.0.0")}}

	if _, err := service.Register(t.Context(), manifest); err != nil {
		t.Fatalf("register: %v", err)
	}

	return &fixture{
		service: service,
		alice:   uuid.Must(uuid.NewV4()),
		bob:     uuid.Must(uuid.NewV4()),
		base:    time.Now().UTC().Truncate(time.Second).Add(-time.Hour),
	}
}

// match builds a two round match that alice wins.
func (f *fixture) match() *MatchInput {
	one := int32(1)
	two := int32(2)

	return &MatchInput{
		ID:               uuid.Must(uuid.NewV4()),
		Status:           "completed",
		Game:             Ref{Key: "ap2", Version: "1.0.0"},
		MinecraftVersion: "26.2",
		ServerName:       "ap-1",
		StartedAt:        f.base,
		EndedAt:          f.base.Add(30 * time.Minute),
		Players: []PlayerInput{
			{UUID: f.alice, Name: "Alice"},
			{UUID: f.bob, Name: "Bob"},
		},
		Presence: []PresenceInput{
			{Player: f.alice, JoinedAt: f.base},
			{Player: f.bob, JoinedAt: f.base, LeftAt: timePtr(f.base.Add(10 * time.Minute))},
			{Player: f.bob, JoinedAt: f.base.Add(12 * time.Minute)},
		},
		Results: []MatchResultInput{
			{Player: f.alice, Score: 31, Placement: &one, Winner: true},
			{Player: f.bob, Score: 18, Placement: &two},
		},
		Stats: map[string]any{"rounds_played": 2},
		Rounds: []RoundInput{
			f.round(1, f.base.Add(time.Minute), map[uuid.UUID]roundEntry{
				f.alice: {rank: 1, points: 3, clicks: 40, accuracy: 0.8, advance: 1.5},
				f.bob:   {rank: 2, points: 2, clicks: 30, accuracy: 0.6, advance: 2.5},
			}),
			f.round(2, f.base.Add(10*time.Minute), map[uuid.UUID]roundEntry{
				f.alice: {rank: 2, points: 2, clicks: 20, accuracy: 0.5, advance: 3.0},
				f.bob:   {rank: 1, points: 3, clicks: 50, accuracy: 0.9, advance: 1.0},
			}),
		},
	}
}

type roundEntry struct {
	rank     int32
	points   int32
	clicks   int
	accuracy float64
	advance  float64
}

func (f *fixture) round(index int32, start time.Time, entries map[uuid.UUID]roundEntry) RoundInput {
	round := RoundInput{
		Index:     index,
		Minigame:  Ref{Key: "ap2:aim_master", Version: "1.0.0"},
		Status:    "completed",
		StartedAt: start,
		EndedAt:   start.Add(2 * time.Minute),
		Map:       &MapInput{ID: "aim_master/classic"},
	}

	for _, player := range []uuid.UUID{f.alice, f.bob} {
		entry, ok := entries[player]

		if !ok {
			continue
		}

		rank := entry.rank

		round.Players = append(round.Players, RoundPlayerInput{
			Player: player,
			Rank:   &rank,
			Points: entry.points,
			Stats: map[string]any{
				"score":            float64(entry.points),
				"clicks":           float64(entry.clicks),
				"accuracy":         entry.accuracy,
				"avg_advance_time": entry.advance,
			},
		})
	}

	return round
}

func timePtr(v time.Time) *time.Time {
	return &v
}

func TestIngestRejectsUndeclaredStat(t *testing.T) {
	f := newFixture(t)

	match := f.match()
	match.Rounds[0].Players[0].Stats["blocks_broken"] = float64(12)

	_, err := f.service.IngestMatch(t.Context(), match)

	validation := &ValidationError{}

	if !errors.As(err, &validation) {
		t.Fatalf("expected a validation error, got %v", err)
	}

	if validation.Problems[0].Code != "unknown_stat" {
		t.Fatalf("expected unknown_stat, got %q", validation.Problems[0].Code)
	}
}

func TestIngestRejectsWrongStatType(t *testing.T) {
	f := newFixture(t)

	match := f.match()
	match.Rounds[0].Players[0].Stats["clicks"] = 1.5

	_, err := f.service.IngestMatch(t.Context(), match)

	validation := &ValidationError{}

	if !errors.As(err, &validation) {
		t.Fatalf("expected a validation error, got %v", err)
	}

	if validation.Problems[0].Code != "stat_type_mismatch" {
		t.Fatalf("expected stat_type_mismatch, got %q", validation.Problems[0].Code)
	}
}

func TestIngestRejectsUnregisteredMinigame(t *testing.T) {
	f := newFixture(t)

	match := f.match()
	match.Rounds[0].Minigame.Version = "9.9.9"

	_, err := f.service.IngestMatch(t.Context(), match)

	validation := &ValidationError{}

	if !errors.As(err, &validation) {
		t.Fatalf("expected a validation error, got %v", err)
	}

	if validation.Problems[0].Code != "unregistered_definition" {
		t.Fatalf("expected unregistered_definition, got %q", validation.Problems[0].Code)
	}
}

func TestIngestIsIdempotent(t *testing.T) {
	f := newFixture(t)

	match := f.match()

	first, err := f.service.IngestMatch(t.Context(), match)

	if err != nil {
		t.Fatalf("first ingest: %v", err)
	}

	if !first.Created {
		t.Fatal("expected the first ingest to create the match")
	}

	second, err := f.service.IngestMatch(t.Context(), match)

	if err != nil {
		t.Fatalf("second ingest: %v", err)
	}

	if second.Created {
		t.Fatal("expected the second ingest to be a no-op")
	}

	if second.MatchID != first.MatchID {
		t.Fatalf("expected match %d, got %d", first.MatchID, second.MatchID)
	}

	assertCount(t, f.service, `SELECT count(*) FROM "matches"`, 1)
	assertCount(t, f.service, `SELECT count(*) FROM "rounds"`, 2)
	assertCount(t, f.service, `SELECT count(*) FROM "round_players"`, 4)
	assertCount(t, f.service, `SELECT count(*) FROM "match_player_sessions"`, 3)

	// 4 stats per player per round, plus the match scoped stat.
	assertCount(t, f.service, `SELECT count(*) FROM "stat_values"`, 17)
}

func TestIngestTracksPersonalBests(t *testing.T) {
	f := newFixture(t)

	if _, err := f.service.IngestMatch(t.Context(), f.match()); err != nil {
		t.Fatalf("ingest: %v", err)
	}

	records, err := f.service.PlayerRecords(t.Context(), f.alice)

	if err != nil {
		t.Fatalf("records: %v", err)
	}

	byStat := make(map[string]PlayerRecord, len(records))

	for _, record := range records {
		byStat[record.Stat] = record
	}

	if got := byStat["clicks"].Value; got != 40 {
		t.Errorf("expected alice's best clicks to be 40, got %v", got)
	}

	// avg_advance_time is lower_is_better, so the best is the smaller value.
	if got := byStat["avg_advance_time"].Value; got != 1.5 {
		t.Errorf("expected alice's best avg_advance_time to be 1.5, got %v", got)
	}

	if _, ok := byStat["score"]; ok {
		t.Error("a hidden stat should not produce a record")
	}

	if got := byStat["clicks"].GlobalRank; got != 2 {
		t.Errorf("expected alice to rank 2 globally for clicks, got %d", got)
	}
}

func TestIngestBuildsAggregates(t *testing.T) {
	f := newFixture(t)

	if _, err := f.service.IngestMatch(t.Context(), f.match()); err != nil {
		t.Fatalf("ingest: %v", err)
	}

	profile, err := f.service.PlayerProfile(t.Context(), f.alice)

	if err != nil {
		t.Fatalf("profile: %v", err)
	}

	if len(profile.Games) != 1 {
		t.Fatalf("expected one game aggregate, got %d", len(profile.Games))
	}

	game := profile.Games[0]

	if game.MatchesPlayed != 1 || game.Wins != 1 || game.RoundsPlayed != 2 || game.PointsTotal != 5 {
		t.Errorf("unexpected game aggregate: %+v", game)
	}

	if game.AveragePlacement == nil || *game.AveragePlacement != 1 {
		t.Errorf("expected an average placement of 1, got %v", game.AveragePlacement)
	}

	if len(profile.Minigames) != 1 {
		t.Fatalf("expected one minigame aggregate, got %d", len(profile.Minigames))
	}

	minigame := profile.Minigames[0]

	if minigame.RoundsPlayed != 2 || minigame.Wins != 1 || minigame.PointsTotal != 5 {
		t.Errorf("unexpected minigame aggregate: %+v", minigame)
	}

	if minigame.AverageRank == nil || *minigame.AverageRank != 1.5 {
		t.Errorf("expected an average rank of 1.5, got %v", minigame.AverageRank)
	}
}

func TestRebuildReproducesIncrementalState(t *testing.T) {
	f := newFixture(t)

	if _, err := f.service.IngestMatch(t.Context(), f.match()); err != nil {
		t.Fatalf("first ingest: %v", err)
	}

	second := f.match()
	second.ID = uuid.Must(uuid.NewV4())

	if _, err := f.service.IngestMatch(t.Context(), second); err != nil {
		t.Fatalf("second ingest: %v", err)
	}

	before := derivedSnapshot(t, f.service)

	if err := f.service.Rebuild(t.Context()); err != nil {
		t.Fatalf("rebuild: %v", err)
	}

	after := derivedSnapshot(t, f.service)

	if before != after {
		t.Errorf("rebuild changed the derived state:\nbefore: %s\nafter:  %s", before, after)
	}
}

func TestMatchDetailRoundTrips(t *testing.T) {
	f := newFixture(t)

	match := f.match()

	if _, err := f.service.IngestMatch(t.Context(), match); err != nil {
		t.Fatalf("ingest: %v", err)
	}

	detail, err := f.service.MatchDetail(t.Context(), match.ID)

	if err != nil {
		t.Fatalf("detail: %v", err)
	}

	if detail.Game != "ap2" || detail.GameVersion != "1.0.0" {
		t.Errorf("unexpected game: %s@%s", detail.Game, detail.GameVersion)
	}

	if got := detail.Stats["rounds_played"]; got != int64(2) {
		t.Errorf("expected the match scoped stat to survive, got %v", got)
	}

	if len(detail.Rounds) != 2 {
		t.Fatalf("expected two rounds, got %d", len(detail.Rounds))
	}

	winner := detail.Rounds[0].Players[0]

	if winner.UUID != f.alice || winner.Points != 3 {
		t.Errorf("unexpected round winner: %+v", winner)
	}

	if got := winner.Stats["clicks"]; got != int64(40) {
		t.Errorf("expected 40 clicks, got %v", got)
	}

	if got := winner.Stats["accuracy"]; got != 0.8 {
		t.Errorf("expected an accuracy of 0.8, got %v", got)
	}

	alice := detail.Players[0]

	if alice.UUID != f.alice || len(alice.Sessions) != 1 {
		t.Errorf("unexpected first player: %+v", alice)
	}

	if len(detail.Players[1].Sessions) != 2 {
		t.Errorf("expected bob to have two presence windows, got %d", len(detail.Players[1].Sessions))
	}
}

// TestIngestHandlesLargeMatch guards the bound-parameter ceiling: postgres
// allows 65535 per statement and each stat value binds eight, so a long match
// used to fail outright instead of inserting in batches.
// Regression: one INSERT per stat value batch exceeded postgres' 65535
// bound-parameter limit; 8801 rows failed the whole ingest.

func TestIngestHandlesLargeMatch(t *testing.T) {
	service := newService(t)

	manifest := Manifest{Definitions: []DefinitionInput{
		{Kind: "game", Key: "ap2", Version: "1.0.0",
			Stats: []StatInput{{Key: "rounds_played", Scope: "match", Type: "int"}}},
		{Kind: "minigame", Key: "ap2:big", Version: "1.0.0", GameType: "ffa", Stats: []StatInput{
			{Key: "a", Type: "int"}, {Key: "b", Type: "int"},
			{Key: "c", Type: "float"}, {Key: "d", Type: "float"},
		}},
	}}

	if _, err := service.Register(t.Context(), manifest); err != nil {
		t.Fatalf("register: %v", err)
	}

	const (
		rounds  = 110
		players = 20
		stats   = 4
	)

	base := time.Now().UTC().Truncate(time.Second).Add(-24 * time.Hour)

	match := &MatchInput{
		ID:        uuid.Must(uuid.NewV4()),
		Status:    "completed",
		Game:      Ref{Key: "ap2", Version: "1.0.0"},
		StartedAt: base,
		EndedAt:   base.Add(6 * time.Hour),
		Stats:     map[string]any{"rounds_played": float64(rounds)},
	}

	ids := make([]uuid.UUID, players)

	for i := range players {
		ids[i] = uuid.Must(uuid.NewV4())
		match.Players = append(match.Players, PlayerInput{UUID: ids[i], Name: fmt.Sprintf("p%d", i)})
	}

	for r := range rounds {
		start := base.Add(time.Duration(r) * time.Minute)

		round := RoundInput{
			Index:     int32(r + 1),
			Minigame:  Ref{Key: "ap2:big", Version: "1.0.0"},
			Status:    "completed",
			StartedAt: start,
			EndedAt:   start.Add(30 * time.Second),
		}

		for i := range players {
			rank := int32(i + 1)

			round.Players = append(round.Players, RoundPlayerInput{
				Player: ids[i],
				Rank:   &rank,
				Stats:  map[string]any{"a": float64(i), "b": float64(r), "c": 0.5, "d": 1.5},
			})
		}

		match.Rounds = append(match.Rounds, round)
	}

	if _, err := service.IngestMatch(t.Context(), match); err != nil {
		t.Fatalf("ingest failed: %v", err)
	}

	want := rounds*players*stats + 1

	assertCount(t, service, `SELECT count(*) FROM "stat_values"`, want)
}

func assertCount(t *testing.T, service *Service, query string, want int) {
	t.Helper()

	var got int

	if err := service.db.QueryRowContext(t.Context(), query).Scan(&got); err != nil {
		t.Fatalf("%s: %v", query, err)
	}

	if got != want {
		t.Errorf("%s: expected %d, got %d", query, want, got)
	}
}

const derivedSnapshotSQL = `
SELECT
    (SELECT coalesce(json_agg(r ORDER BY r."stat_id", r."player_id"), '[]')::text
     FROM (SELECT "stat_id", "player_id", "int_value", "real_value", "match_id", "round_id" FROM "stat_records") r)
    || (SELECT coalesce(json_agg(g ORDER BY g."player_id", g."definition_id"), '[]')::text
     FROM (SELECT "player_id", "definition_id", "matches_played", "wins", "rounds_played", "points_total", "placement_sum" FROM "player_game_stats") g)
    || (SELECT coalesce(json_agg(m ORDER BY m."player_id", m."definition_id"), '[]')::text
     FROM (SELECT "player_id", "definition_id", "rounds_played", "wins", "rank_sum", "points_total" FROM "player_minigame_stats") m)
`

func derivedSnapshot(t *testing.T, service *Service) string {
	t.Helper()

	var snapshot string

	if err := service.db.QueryRowContext(t.Context(), derivedSnapshotSQL).Scan(&snapshot); err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	return snapshot
}
