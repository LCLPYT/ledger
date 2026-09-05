package stats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"ledger/model/enums"

	"github.com/aarondl/opt/null"
	"github.com/gofrs/uuid/v5"
)

type statValue struct {
	IntValue  null.Val[int64]
	RealValue null.Val[float64]
	TextValue null.Val[string]
}

type valuePlan struct {
	meta  statMeta
	value statValue
}

type roundPlan struct {
	input       *RoundInput
	schema      *versionSchema
	teamStats   map[string][]valuePlan
	playerStats map[uuid.UUID][]valuePlan
}

type matchPlan struct {
	input      *MatchInput
	game       *versionSchema
	status     enums.MatchStatus
	matchStats []valuePlan
	rounds     []roundPlan
}

// validateMatch checks a payload against the registered schemas and, on
// success, returns the coerced values ready to be written.
func (s *Service) validateMatch(ctx context.Context, in *MatchInput) (*matchPlan, error) {
	var probs problems

	plan := &matchPlan{input: in}

	if in.ID.IsNil() {
		probs.add("missing_id", "id", "a match id is required")
	}

	status := enums.MatchStatus(in.Status)

	if !status.Valid() {
		probs.add("invalid_status", "status", "unknown match status %q", in.Status)
	}

	plan.status = status

	if in.StartedAt.IsZero() || in.EndedAt.IsZero() {
		probs.add("missing_timestamps", "started_at", "started_at and ended_at are required")
	} else if in.EndedAt.Before(in.StartedAt) {
		probs.add("invalid_window", "ended_at", "ended_at is before started_at")
	}

	known := make(map[uuid.UUID]struct{}, len(in.Players))

	if len(in.Players) == 0 {
		probs.add("no_players", "players", "at least one player is required")
	}

	for i, player := range in.Players {
		path := fmt.Sprintf("players[%d]", i)

		if player.UUID.IsNil() {
			probs.add("missing_player_uuid", path+".uuid", "player uuid is required")
			continue
		}

		if _, duplicate := known[player.UUID]; duplicate {
			probs.add("duplicate_player", path+".uuid", "player %s is listed twice", player.UUID)
			continue
		}

		known[player.UUID] = struct{}{}
	}

	for i, presence := range in.Presence {
		path := fmt.Sprintf("presence[%d]", i)

		if _, ok := known[presence.Player]; !ok {
			probs.add("unknown_player", path+".player", "player %s is not listed in players", presence.Player)
		}

		if presence.LeftAt != nil && presence.LeftAt.Before(presence.JoinedAt) {
			probs.add("invalid_window", path+".left_at", "left_at is before joined_at")
		}
	}

	seenResults := make(map[uuid.UUID]struct{}, len(in.Results))

	for i, result := range in.Results {
		path := fmt.Sprintf("results[%d]", i)

		if _, ok := known[result.Player]; !ok {
			probs.add("unknown_player", path+".player", "player %s is not listed in players", result.Player)
			continue
		}

		if _, duplicate := seenResults[result.Player]; duplicate {
			probs.add("duplicate_result", path+".player", "player %s has two results", result.Player)
		}

		seenResults[result.Player] = struct{}{}
	}

	game, err := s.schema(ctx, enums.DefinitionKindGame, in.Game)

	switch {
	case errors.Is(err, errUnregistered):
		probs.add("unregistered_definition", "game",
			"game %s@%s is not registered", in.Game.Key, in.Game.Version)
	case err != nil:
		return nil, err
	default:
		plan.game = game
		plan.matchStats = appendStatValues(&probs, "stats", game, enums.StatScopeMatch, in.Stats)
	}

	seenRounds := make(map[int32]struct{}, len(in.Rounds))

	for i := range in.Rounds {
		round := &in.Rounds[i]
		path := fmt.Sprintf("rounds[%d]", i)

		if round.Index < 1 {
			probs.add("invalid_round_index", path+".index", "round index must be at least 1")
		}

		if _, duplicate := seenRounds[round.Index]; duplicate {
			probs.add("duplicate_round", path+".index", "round index %d appears twice", round.Index)
		}

		seenRounds[round.Index] = struct{}{}

		roundStatus := enums.RoundStatus(round.Status)

		if !roundStatus.Valid() {
			probs.add("invalid_status", path+".status", "unknown round status %q", round.Status)
		}

		if round.EndedAt.Before(round.StartedAt) {
			probs.add("invalid_window", path+".ended_at", "ended_at is before started_at")
		}

		if !in.StartedAt.IsZero() && !in.EndedAt.IsZero() {
			if round.StartedAt.Before(in.StartedAt) || round.EndedAt.After(in.EndedAt) {
				probs.add("round_outside_match", path, "round falls outside the match window")
			}
		}

		schema, err := s.schema(ctx, enums.DefinitionKindMinigame, round.Minigame)

		if errors.Is(err, errUnregistered) {
			probs.add("unregistered_definition", path+".minigame",
				"minigame %s@%s is not registered", round.Minigame.Key, round.Minigame.Version)

			continue
		}

		if err != nil {
			return nil, err
		}

		plan.rounds = append(plan.rounds, validateRound(&probs, path, round, schema, known))
	}

	if len(probs) > 0 {
		return nil, &ValidationError{Problems: probs}
	}

	return plan, nil
}

func validateRound(probs *problems, path string, round *RoundInput, schema *versionSchema, known map[uuid.UUID]struct{}) roundPlan {
	plan := roundPlan{
		input:       round,
		schema:      schema,
		teamStats:   make(map[string][]valuePlan, len(round.Teams)),
		playerStats: make(map[uuid.UUID][]valuePlan, len(round.Players)),
	}

	teams := make(map[string]struct{}, len(round.Teams))

	for i, team := range round.Teams {
		teamPath := fmt.Sprintf("%s.teams[%d]", path, i)

		if team.Key == "" {
			probs.add("missing_team_key", teamPath+".key", "team key must not be empty")
			continue
		}

		if _, duplicate := teams[team.Key]; duplicate {
			probs.add("duplicate_team", teamPath+".key", "team %q appears twice", team.Key)
			continue
		}

		teams[team.Key] = struct{}{}

		if len(team.Stats) > 0 && schema.GameType != enums.GameTypeTeam {
			probs.add("team_stats_not_allowed", teamPath+".stats",
				"minigame %s is not a team game", schema.Key)

			continue
		}

		plan.teamStats[team.Key] = appendStatValues(probs, teamPath+".stats", schema, enums.StatScopeTeam, team.Stats)
	}

	seen := make(map[uuid.UUID]struct{}, len(round.Players))

	for i, player := range round.Players {
		playerPath := fmt.Sprintf("%s.players[%d]", path, i)

		if _, ok := known[player.Player]; !ok {
			probs.add("unknown_player", playerPath+".player",
				"player %s is not listed in players", player.Player)

			continue
		}

		if _, duplicate := seen[player.Player]; duplicate {
			probs.add("duplicate_player", playerPath+".player",
				"player %s appears twice in this round", player.Player)

			continue
		}

		seen[player.Player] = struct{}{}

		if player.Rank != nil && *player.Rank < 1 {
			probs.add("invalid_rank", playerPath+".rank", "rank must be at least 1")
		}

		if player.Team != "" {
			if _, ok := teams[player.Team]; !ok {
				probs.add("unknown_team", playerPath+".team",
					"team %q is not declared for this round", player.Team)
			}
		}

		plan.playerStats[player.Player] = appendStatValues(probs, playerPath+".stats", schema, enums.StatScopePlayer, player.Stats)
	}

	return plan
}

func appendStatValues(probs *problems, path string, schema *versionSchema, scope enums.StatScope, raw map[string]any) []valuePlan {
	if len(raw) == 0 {
		return nil
	}

	values := make([]valuePlan, 0, len(raw))

	for key, value := range raw {
		meta, ok := schema.stat(scope, key)

		if !ok {
			probs.add("unknown_stat", path+"."+key,
				"stat %q is not declared by %s %s@%s in scope %s",
				key, schema.Kind, schema.Key, schema.Version, scope)

			continue
		}

		coerced, err := coerceValue(meta.ValueType, value)

		if err != nil {
			probs.add("stat_type_mismatch", path+"."+key,
				"stat %q is declared as %s: %v", key, meta.ValueType, err)

			continue
		}

		values = append(values, valuePlan{meta: meta, value: coerced})
	}

	return values
}

func coerceValue(valueType enums.StatValueType, raw any) (statValue, error) {
	switch valueType {
	case enums.StatValueTypeInt, enums.StatValueTypeDuration:
		number, err := toFloat(raw)

		if err != nil {
			return statValue{}, err
		}

		if number != math.Trunc(number) {
			return statValue{}, fmt.Errorf("got a fractional number")
		}

		return statValue{IntValue: null.From(int64(number))}, nil

	case enums.StatValueTypeFloat:
		number, err := toFloat(raw)

		if err != nil {
			return statValue{}, err
		}

		if math.IsNaN(number) || math.IsInf(number, 0) {
			return statValue{}, fmt.Errorf("got a non finite number")
		}

		return statValue{RealValue: null.From(number)}, nil

	case enums.StatValueTypeBool:
		flag, ok := raw.(bool)

		if !ok {
			return statValue{}, fmt.Errorf("got %T", raw)
		}

		value := int64(0)

		if flag {
			value = 1
		}

		return statValue{IntValue: null.From(value)}, nil

	case enums.StatValueTypeString:
		text, ok := raw.(string)

		if !ok {
			return statValue{}, fmt.Errorf("got %T", raw)
		}

		return statValue{TextValue: null.From(text)}, nil

	default:
		return statValue{}, fmt.Errorf("unsupported stat type %q", valueType)
	}
}

func toFloat(raw any) (float64, error) {
	switch v := raw.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case json.Number:
		return v.Float64()
	default:
		return 0, fmt.Errorf("got %T", raw)
	}
}
