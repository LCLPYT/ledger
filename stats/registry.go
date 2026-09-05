package stats

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"ledger/model"
	"ledger/model/enums"

	"github.com/aarondl/opt/omit"
	"github.com/aarondl/opt/omitnull"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql/dialect"
	"github.com/stephenafamo/bob/dialect/psql/im"
)

type normalizedStat struct {
	Key            string              `json:"key"`
	Name           string              `json:"name"`
	Scope          enums.StatScope     `json:"scope"`
	ValueType      enums.StatValueType `json:"type"`
	Unit           string              `json:"unit"`
	HigherIsBetter bool                `json:"higher_is_better"`
	Display        bool                `json:"display"`
}

func (s normalizedStat) isRecord() bool {
	if s.Scope != enums.StatScopePlayer || !s.Display {
		return false
	}

	switch s.ValueType {
	case enums.StatValueTypeInt, enums.StatValueTypeFloat, enums.StatValueTypeDuration:
		return true
	default:
		return false
	}
}

type normalizedDefinition struct {
	Kind     enums.DefinitionKind `json:"kind"`
	Key      string               `json:"key"`
	Name     string               `json:"name"`
	Group    string               `json:"group"`
	Version  string               `json:"version"`
	GameType enums.GameType       `json:"game_type"`
	Stats    []normalizedStat     `json:"stats"`

	fingerprint []byte
}

func normalizeDefinition(index int, in DefinitionInput) (normalizedDefinition, problems) {
	var probs problems

	path := fmt.Sprintf("definitions[%d]", index)

	out := normalizedDefinition{
		Key:     strings.TrimSpace(in.Key),
		Name:    strings.TrimSpace(in.Name),
		Group:   strings.TrimSpace(in.Group),
		Version: strings.TrimSpace(in.Version),
	}

	kind := enums.DefinitionKind(in.Kind)

	if !kind.Valid() {
		probs.add("invalid_kind", path+".kind", "unknown definition kind %q", in.Kind)
	}

	out.Kind = kind

	if out.Key == "" {
		probs.add("missing_key", path+".key", "key must not be empty")
	}

	if out.Version == "" {
		probs.add("missing_version", path+".version", "version must not be empty")
	}

	if in.GameType != "" {
		gameType := enums.GameType(in.GameType)

		if !gameType.Valid() {
			probs.add("invalid_game_type", path+".game_type", "unknown game type %q", in.GameType)
		}

		out.GameType = gameType
	}

	seen := make(map[statKey]struct{}, len(in.Stats))

	for i, stat := range in.Stats {
		statPath := fmt.Sprintf("%s.stats[%d]", path, i)

		normalized, statProbs := normalizeStat(statPath, stat)
		probs = append(probs, statProbs...)

		sk := statKey{scope: normalized.Scope, key: normalized.Key}

		if _, duplicate := seen[sk]; duplicate {
			probs.add("duplicate_stat", statPath+".key", "stat %q is declared twice in scope %s", normalized.Key, normalized.Scope)
			continue
		}

		seen[sk] = struct{}{}

		out.Stats = append(out.Stats, normalized)
	}

	slices.SortFunc(out.Stats, func(a, b normalizedStat) int {
		if c := strings.Compare(string(a.Scope), string(b.Scope)); c != 0 {
			return c
		}

		return strings.Compare(a.Key, b.Key)
	})

	if len(probs) > 0 {
		return out, probs
	}

	fingerprint, err := fingerprintOf(out)

	if err != nil {
		probs.add("invalid_definition", path, "%v", err)
		return out, probs
	}

	out.fingerprint = fingerprint

	return out, nil
}

func normalizeStat(path string, in StatInput) (normalizedStat, problems) {
	var probs problems

	out := normalizedStat{
		Key:            strings.TrimSpace(in.Key),
		Name:           strings.TrimSpace(in.Name),
		Unit:           strings.TrimSpace(in.Unit),
		HigherIsBetter: true,
		Display:        true,
	}

	if out.Key == "" {
		probs.add("missing_stat_key", path+".key", "stat key must not be empty")
	}

	if out.Unit == "" {
		out.Unit = "plain"
	}

	scope := enums.StatScopePlayer

	if in.Scope != "" {
		scope = enums.StatScope(in.Scope)

		if !scope.Valid() {
			probs.add("invalid_scope", path+".scope", "unknown stat scope %q", in.Scope)
		}
	}

	out.Scope = scope

	valueType := enums.StatValueType(in.Type)

	if !valueType.Valid() {
		probs.add("invalid_stat_type", path+".type", "unknown stat type %q", in.Type)
	}

	out.ValueType = valueType

	if in.HigherIsBetter != nil {
		out.HigherIsBetter = *in.HigherIsBetter
	}

	if in.Display != nil {
		out.Display = *in.Display
	}

	return out, probs
}

// fingerprintOf hashes the full normalized declaration. Anything that differs
// between two registrations of the same version is a conflict, so the hash
// deliberately covers cosmetic fields too.
func fingerprintOf(d normalizedDefinition) ([]byte, error) {
	payload, err := json.Marshal(struct {
		Kind     enums.DefinitionKind `json:"kind"`
		Key      string               `json:"key"`
		Version  string               `json:"version"`
		GameType enums.GameType       `json:"game_type"`
		Stats    []normalizedStat     `json:"stats"`
	}{
		Kind:     d.Kind,
		Key:      d.Key,
		Version:  d.Version,
		GameType: d.GameType,
		Stats:    d.Stats,
	})

	if err != nil {
		return nil, err
	}

	sum := sha256.Sum256(payload)

	return sum[:], nil
}

// Register stores the declared schema of every definition in the manifest.
// It is idempotent: re-registering an identical version is a no-op, while a
// version whose declaration changed is rejected as a conflict.
func (s *Service) Register(ctx context.Context, manifest Manifest) (*RegisterResult, error) {
	if len(manifest.Definitions) == 0 {
		return nil, &ValidationError{Problems: []Problem{{
			Code:    "empty_manifest",
			Path:    "definitions",
			Message: "at least one definition is required",
		}}}
	}

	normalized := make([]normalizedDefinition, 0, len(manifest.Definitions))

	var probs problems

	seen := make(map[refKey]struct{}, len(manifest.Definitions))

	for i, input := range manifest.Definitions {
		definition, defProbs := normalizeDefinition(i, input)
		probs = append(probs, defProbs...)

		if len(defProbs) > 0 {
			continue
		}

		rk := refKey{kind: definition.Kind, key: definition.Key, version: definition.Version}

		if _, duplicate := seen[rk]; duplicate {
			probs.add("duplicate_definition", fmt.Sprintf("definitions[%d]", i),
				"%s %s@%s is declared twice in the manifest", definition.Kind, definition.Key, definition.Version)
			continue
		}

		seen[rk] = struct{}{}

		normalized = append(normalized, definition)
	}

	if len(probs) > 0 {
		return nil, &ValidationError{Problems: probs}
	}

	result := &RegisterResult{Definitions: make([]DefinitionResult, 0, len(normalized))}

	var conflicts problems

	err := s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bob.Transaction) error {
		for _, definition := range normalized {
			entry, defConflicts, err := registerDefinition(ctx, tx, definition)

			if err != nil {
				return err
			}

			if len(defConflicts) > 0 {
				conflicts = append(conflicts, defConflicts...)
				continue
			}

			result.Definitions = append(result.Definitions, entry)
		}

		if len(conflicts) > 0 {
			return &ConflictError{Problems: conflicts}
		}

		return nil
	})

	if err != nil {
		conflict := &ConflictError{}

		if errors.As(err, &conflict) {
			return nil, conflict
		}

		return nil, err
	}

	for _, definition := range normalized {
		s.invalidate(definition.Kind, definition.Key)
	}

	for _, entry := range result.Definitions {
		if entry.Status == StatusCreated {
			result.Created++
		} else {
			result.Unchanged++
		}
	}

	return result, nil
}

func registerDefinition(ctx context.Context, tx bob.Transaction, d normalizedDefinition) (DefinitionResult, problems, error) {
	entry := DefinitionResult{Kind: string(d.Kind), Key: d.Key, Version: d.Version}
	path := fmt.Sprintf("%s %s@%s", d.Kind, d.Key, d.Version)

	definition, err := upsertDefinition(ctx, tx, d)

	if err != nil {
		return entry, nil, err
	}

	existing, err := findVersion(ctx, tx, definition.ID, d.Version)

	if err != nil {
		return entry, nil, err
	}

	if existing != nil {
		return matchVersion(entry, path, existing, d)
	}

	statIDs := make([]int64, 0, len(d.Stats))

	var probs problems

	for _, stat := range d.Stats {
		id, statProbs, err := upsertStat(ctx, tx, definition.ID, path, stat)

		if err != nil {
			return entry, nil, err
		}

		probs = append(probs, statProbs...)

		if len(statProbs) == 0 {
			statIDs = append(statIDs, id)
		}
	}

	if len(probs) > 0 {
		return entry, probs, nil
	}

	version, err := model.DefinitionVersions.Insert(
		&model.DefinitionVersionSetter{
			DefinitionID: omit.From(definition.ID),
			Version:      omit.From(d.Version),
			GameType:     gameTypeSetter(d.GameType),
			Fingerprint:  omit.From(d.fingerprint),
		},
		im.OnConflict("definition_id", "version").DoNothing(),
	).One(ctx, tx)

	// Concurrent registrations of the same definition already serialise on the
	// row lock that upsertDefinition takes, so this is unreachable today. It
	// keeps the insert correct if that upsert ever stops writing the row.
	if errors.Is(err, sql.ErrNoRows) {
		concurrent, err := findVersion(ctx, tx, definition.ID, d.Version)

		if err != nil {
			return entry, nil, err
		}

		return matchVersion(entry, path, concurrent, d)
	}

	if err != nil {
		return entry, nil, err
	}

	if len(statIDs) > 0 {
		links := make([]bob.Mod[*dialect.InsertQuery], 0, len(statIDs))

		for _, statID := range statIDs {
			links = append(links, &model.DefinitionVersionStatSetter{
				DefinitionVersionID: omit.From(version.ID),
				StatID:              omit.From(statID),
			})
		}

		if _, err := model.DefinitionVersionStats.Insert(links...).All(ctx, tx); err != nil {
			return entry, nil, err
		}
	}

	entry.VersionID = version.ID
	entry.Status = StatusCreated

	return entry, nil, nil
}

func upsertDefinition(ctx context.Context, tx bob.Transaction, d normalizedDefinition) (*model.Definition, error) {
	return model.Definitions.Insert(
		&model.DefinitionSetter{
			Kind:     omit.From(d.Kind),
			Key:      omit.From(d.Key),
			Name:     nullText(d.Name),
			GroupKey: nullText(d.Group),
		},
		im.OnConflict("kind", "key").DoUpdate(
			im.SetExcluded("name", "group_key"),
			im.SetCol("updated_at").To("current_timestamp"),
		),
	).One(ctx, tx)
}

func upsertStat(ctx context.Context, tx bob.Transaction, definitionID int64, path string, s normalizedStat) (int64, problems, error) {
	existing, err := model.Stats.Query(
		model.SelectWhere.Stats.DefinitionID.EQ(definitionID),
		model.SelectWhere.Stats.Scope.EQ(s.Scope),
		model.SelectWhere.Stats.Key.EQ(s.Key),
	).One(ctx, tx)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, nil, err
	}

	if existing != nil {
		var probs problems

		if existing.ValueType != s.ValueType {
			probs.add("stat_conflict", path+"."+s.Key,
				"stat %q is already recorded as %s and cannot become %s; use a new stat key",
				s.Key, existing.ValueType, s.ValueType)
		}

		if existing.HigherIsBetter.GetOr(true) != s.HigherIsBetter {
			probs.add("stat_conflict", path+"."+s.Key,
				"stat %q changed direction (higher_is_better); use a new stat key", s.Key)
		}

		if len(probs) > 0 {
			return 0, probs, nil
		}
	}

	stat, err := model.Stats.Insert(
		&model.StatSetter{
			DefinitionID:   omit.From(definitionID),
			Scope:          omit.From(s.Scope),
			Key:            omit.From(s.Key),
			Name:           nullText(s.Name),
			ValueType:      omit.From(s.ValueType),
			HigherIsBetter: omitnull.From(s.HigherIsBetter),
			Unit:           omit.From(s.Unit),
			Display:        omit.From(s.Display),
			IsRecord:       omit.From(s.isRecord()),
		},
		im.OnConflict("definition_id", "scope", "key").DoUpdate(
			im.SetExcluded("name", "unit", "display", "is_record"),
			im.SetCol("updated_at").To("current_timestamp"),
		),
	).One(ctx, tx)

	if err != nil {
		return 0, nil, err
	}

	return stat.ID, nil, nil
}

func gameTypeSetter(t enums.GameType) omitnull.Val[enums.GameType] {
	if t == "" {
		return nullValue[enums.GameType]()
	}

	return omitnull.From(t)
}

func nullText(v string) omitnull.Val[string] {
	if v == "" {
		return nullValue[string]()
	}

	return omitnull.From(v)
}

func nullValue[T any]() omitnull.Val[T] {
	var v omitnull.Val[T]
	v.Null()

	return v
}

func findVersion(ctx context.Context, tx bob.Transaction, definitionID int64, version string) (*model.DefinitionVersion, error) {
	existing, err := model.DefinitionVersions.Query(
		model.SelectWhere.DefinitionVersions.DefinitionID.EQ(definitionID),
		model.SelectWhere.DefinitionVersions.Version.EQ(version),
	).One(ctx, tx)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	return existing, err
}

// matchVersion decides whether a stored version agrees with the declaration
// being registered.
func matchVersion(entry DefinitionResult, path string, existing *model.DefinitionVersion, d normalizedDefinition) (DefinitionResult, problems, error) {
	if existing == nil {
		return entry, nil, fmt.Errorf("version %s disappeared while registering", path)
	}

	if !bytes.Equal(existing.Fingerprint, d.fingerprint) {
		var probs problems

		probs.add("definition_conflict", path,
			"version %s is already registered with a different declaration", d.Version)

		return entry, probs, nil
	}

	entry.VersionID = existing.ID
	entry.Status = StatusUnchanged

	return entry, nil, nil
}
