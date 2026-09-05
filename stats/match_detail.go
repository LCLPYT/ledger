package stats

import (
	"context"
	"time"

	"github.com/gofrs/uuid/v5"
)

type MatchDetail struct {
	ID               uuid.UUID           `json:"id"`
	Game             string              `json:"game"`
	GameVersion      string              `json:"game_version"`
	Status           string              `json:"status"`
	StartedAt        time.Time           `json:"started_at"`
	EndedAt          time.Time           `json:"ended_at"`
	MinecraftVersion *string             `json:"minecraft_version,omitempty"`
	ServerName       *string             `json:"server_name,omitempty"`
	Stats            map[string]any      `json:"stats"`
	Players          []MatchDetailPlayer `json:"players"`
	Rounds           []MatchDetailRound  `json:"rounds"`
}

type MatchDetailPlayer struct {
	UUID         uuid.UUID        `json:"uuid"`
	Name         string           `json:"name"`
	Score        int32            `json:"score"`
	Placement    *int32           `json:"placement,omitempty"`
	Winner       bool             `json:"winner"`
	RoundsPlayed int32            `json:"rounds_played"`
	Sessions     []PresenceWindow `json:"sessions"`
}

type PresenceWindow struct {
	JoinedAt time.Time  `json:"joined_at"`
	LeftAt   *time.Time `json:"left_at,omitempty"`
}

type MatchDetailRound struct {
	Index           int32                    `json:"index"`
	Minigame        string                   `json:"minigame"`
	MinigameVersion string                   `json:"minigame_version"`
	Status          string                   `json:"status"`
	StartedAt       time.Time                `json:"started_at"`
	EndedAt         time.Time                `json:"ended_at"`
	Map             *string                  `json:"map,omitempty"`
	Seed            *int64                   `json:"seed,omitempty"`
	StatsRef        *uuid.UUID               `json:"stats_ref,omitempty"`
	Teams           []MatchDetailTeam        `json:"teams,omitempty"`
	Players         []MatchDetailRoundPlayer `json:"players"`
}

type MatchDetailTeam struct {
	Key   string         `json:"key"`
	Rank  *int32         `json:"rank,omitempty"`
	Stats map[string]any `json:"stats,omitempty"`
}

type MatchDetailRoundPlayer struct {
	UUID   uuid.UUID      `json:"uuid"`
	Name   string         `json:"name"`
	Team   *string        `json:"team,omitempty"`
	Rank   *int32         `json:"rank,omitempty"`
	Points int32          `json:"points"`
	Stats  map[string]any `json:"stats,omitempty"`
}

// MatchDetail returns the full timeline of one match: participation windows,
// every round with its ranking, and all recorded stat values.
func (s *Service) MatchDetail(ctx context.Context, publicID uuid.UUID) (*MatchDetail, error) {
	detail := &MatchDetail{Stats: map[string]any{}}

	var matchID int64

	row := s.db.QueryRowContext(ctx, `
		SELECT m."id", m."public_id", d."key", dv."version", m."status", m."started_at", m."ended_at",
		    m."minecraft_version", m."server_name"
		FROM "matches" m
		JOIN "definition_versions" dv ON dv."id" = m."definition_version_id"
		JOIN "definitions" d ON d."id" = dv."definition_id"
		WHERE m."public_id" = $1
	`, publicID)

	err := row.Scan(&matchID, &detail.ID, &detail.Game, &detail.GameVersion, &detail.Status,
		&detail.StartedAt, &detail.EndedAt, &detail.MinecraftVersion, &detail.ServerName)

	if err != nil {
		return nil, notFoundOr(err)
	}

	players, playerIndex, err := s.matchDetailPlayers(ctx, matchID)

	if err != nil {
		return nil, err
	}

	detail.Players = players

	rounds, roundIndex, teamIndex, err := s.matchDetailRounds(ctx, matchID)

	if err != nil {
		return nil, err
	}

	detail.Rounds = rounds

	if err := s.applyStatValues(ctx, matchID, detail, playerIndex, roundIndex, teamIndex); err != nil {
		return nil, err
	}

	return detail, nil
}

func (s *Service) matchDetailPlayers(ctx context.Context, matchID int64) ([]MatchDetailPlayer, map[int64]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT mp."id", p."id", p."uuid", p."name", mp."score", mp."placement", mp."is_winner", mp."rounds_played"
		FROM "match_players" mp
		JOIN "players" p ON p."id" = mp."player_id"
		WHERE mp."match_id" = $1
		ORDER BY mp."placement" NULLS LAST, mp."score" DESC, p."name"
	`, matchID)

	if err != nil {
		return nil, nil, err
	}

	defer rows.Close()

	players := make([]MatchDetailPlayer, 0)
	byPlayerID := make(map[int64]int)
	byMatchPlayerID := make(map[int64]int)

	for rows.Next() {
		var matchPlayerID, playerID int64
		var player MatchDetailPlayer

		if err := rows.Scan(&matchPlayerID, &playerID, &player.UUID, &player.Name,
			&player.Score, &player.Placement, &player.Winner, &player.RoundsPlayed); err != nil {
			return nil, nil, err
		}

		player.Sessions = make([]PresenceWindow, 0)

		byPlayerID[playerID] = len(players)
		byMatchPlayerID[matchPlayerID] = len(players)

		players = append(players, player)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	sessions, err := s.db.QueryContext(ctx, `
		SELECT s."match_player_id", s."joined_at", s."left_at"
		FROM "match_player_sessions" s
		JOIN "match_players" mp ON mp."id" = s."match_player_id"
		WHERE mp."match_id" = $1
		ORDER BY s."joined_at"
	`, matchID)

	if err != nil {
		return nil, nil, err
	}

	defer sessions.Close()

	for sessions.Next() {
		var matchPlayerID int64
		var window PresenceWindow

		if err := sessions.Scan(&matchPlayerID, &window.JoinedAt, &window.LeftAt); err != nil {
			return nil, nil, err
		}

		if i, ok := byMatchPlayerID[matchPlayerID]; ok {
			players[i].Sessions = append(players[i].Sessions, window)
		}
	}

	return players, byPlayerID, sessions.Err()
}

type teamLocation struct {
	round int
	team  int
}

func (s *Service) matchDetailRounds(ctx context.Context, matchID int64) ([]MatchDetailRound, map[int64]int, map[int64]teamLocation, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r."id", r."index", d."key", dv."version", r."status", r."started_at", r."ended_at",
		    r."map_id", r."map_seed", r."stats_ref"
		FROM "rounds" r
		JOIN "definition_versions" dv ON dv."id" = r."definition_version_id"
		JOIN "definitions" d ON d."id" = dv."definition_id"
		WHERE r."match_id" = $1
		ORDER BY r."index"
	`, matchID)

	if err != nil {
		return nil, nil, nil, err
	}

	defer rows.Close()

	rounds := make([]MatchDetailRound, 0)
	roundIndex := make(map[int64]int)

	for rows.Next() {
		var roundID int64
		var round MatchDetailRound

		if err := rows.Scan(&roundID, &round.Index, &round.Minigame, &round.MinigameVersion, &round.Status,
			&round.StartedAt, &round.EndedAt, &round.Map, &round.Seed, &round.StatsRef); err != nil {
			return nil, nil, nil, err
		}

		round.Players = make([]MatchDetailRoundPlayer, 0)

		roundIndex[roundID] = len(rounds)

		rounds = append(rounds, round)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, nil, err
	}

	teamIndex, teamKeys, err := s.matchDetailTeams(ctx, matchID, rounds, roundIndex)

	if err != nil {
		return nil, nil, nil, err
	}

	if err := s.matchDetailRoundPlayers(ctx, matchID, rounds, roundIndex, teamKeys); err != nil {
		return nil, nil, nil, err
	}

	return rounds, roundIndex, teamIndex, nil
}

func (s *Service) matchDetailTeams(ctx context.Context, matchID int64, rounds []MatchDetailRound, roundIndex map[int64]int) (map[int64]teamLocation, map[int64]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT t."id", t."round_id", t."team_key", t."rank"
		FROM "round_teams" t
		JOIN "rounds" r ON r."id" = t."round_id"
		WHERE r."match_id" = $1
		ORDER BY t."rank" NULLS LAST, t."team_key"
	`, matchID)

	if err != nil {
		return nil, nil, err
	}

	defer rows.Close()

	teamIndex := make(map[int64]teamLocation)
	teamKeys := make(map[int64]string)

	for rows.Next() {
		var teamID, roundID int64
		var team MatchDetailTeam

		if err := rows.Scan(&teamID, &roundID, &team.Key, &team.Rank); err != nil {
			return nil, nil, err
		}

		i, ok := roundIndex[roundID]

		if !ok {
			continue
		}

		teamIndex[teamID] = teamLocation{round: i, team: len(rounds[i].Teams)}
		teamKeys[teamID] = team.Key

		rounds[i].Teams = append(rounds[i].Teams, team)
	}

	return teamIndex, teamKeys, rows.Err()
}

func (s *Service) matchDetailRoundPlayers(ctx context.Context, matchID int64, rounds []MatchDetailRound, roundIndex map[int64]int, teamKeys map[int64]string) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT rp."round_id", rp."player_id", p."uuid", p."name", rp."round_team_id", rp."rank", rp."points_awarded"
		FROM "round_players" rp
		JOIN "rounds" r ON r."id" = rp."round_id"
		JOIN "players" p ON p."id" = rp."player_id"
		WHERE r."match_id" = $1
		ORDER BY rp."rank" NULLS LAST, p."name"
	`, matchID)

	if err != nil {
		return err
	}

	defer rows.Close()

	for rows.Next() {
		var roundID, playerID int64
		var teamID *int64
		var player MatchDetailRoundPlayer

		if err := rows.Scan(&roundID, &playerID, &player.UUID, &player.Name, &teamID, &player.Rank, &player.Points); err != nil {
			return err
		}

		if teamID != nil {
			if key, ok := teamKeys[*teamID]; ok {
				player.Team = &key
			}
		}

		i, ok := roundIndex[roundID]

		if !ok {
			continue
		}

		rounds[i].Players = append(rounds[i].Players, player)
	}

	return rows.Err()
}

func (s *Service) applyStatValues(ctx context.Context, matchID int64, detail *MatchDetail, playerIndex map[int64]int, roundIndex map[int64]int, teamIndex map[int64]teamLocation) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT v."round_id", v."player_id", v."round_team_id", s."key", s."value_type",
		    v."int_value", v."real_value", v."text_value"
		FROM "stat_values" v
		JOIN "stats" s ON s."id" = v."stat_id"
		WHERE v."match_id" = $1
	`, matchID)

	if err != nil {
		return err
	}

	defer rows.Close()

	byUUID := make(map[uuid.UUID]int64, len(playerIndex))

	for playerID, i := range playerIndex {
		byUUID[detail.Players[i].UUID] = playerID
	}

	roundPlayers := make(map[int]map[int64]int, len(detail.Rounds))

	for i := range detail.Rounds {
		index := make(map[int64]int, len(detail.Rounds[i].Players))

		for j, player := range detail.Rounds[i].Players {
			index[byUUID[player.UUID]] = j
		}

		roundPlayers[i] = index
	}

	for rows.Next() {
		var roundID, playerID, teamID *int64
		var key, valueType string
		var intValue *int64
		var realValue *float64
		var textValue *string

		if err := rows.Scan(&roundID, &playerID, &teamID, &key, &valueType, &intValue, &realValue, &textValue); err != nil {
			return err
		}

		value := renderStatValue(valueType, intValue, realValue, textValue)

		switch {
		case roundID == nil:
			detail.Stats[key] = value

		case teamID != nil:
			location, ok := teamIndex[*teamID]

			if !ok {
				continue
			}

			team := &detail.Rounds[location.round].Teams[location.team]

			if team.Stats == nil {
				team.Stats = map[string]any{}
			}

			team.Stats[key] = value

		case playerID != nil:
			i, ok := roundIndex[*roundID]

			if !ok {
				continue
			}

			j, ok := roundPlayers[i][*playerID]

			if !ok {
				continue
			}

			player := &detail.Rounds[i].Players[j]

			if player.Stats == nil {
				player.Stats = map[string]any{}
			}

			player.Stats[key] = value
		}
	}

	return rows.Err()
}

func renderStatValue(valueType string, intValue *int64, realValue *float64, textValue *string) any {
	switch valueType {
	case "bool":
		if intValue == nil {
			return nil
		}

		return *intValue != 0

	case "int", "duration":
		if intValue == nil {
			return nil
		}

		return *intValue

	case "float":
		if realValue == nil {
			return nil
		}

		return *realValue

	default:
		if textValue == nil {
			return nil
		}

		return *textValue
	}
}
