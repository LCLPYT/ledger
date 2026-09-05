package stats

import (
	"errors"
	"testing"
)

func aimMaster(version string) DefinitionInput {
	return DefinitionInput{
		Kind:     "minigame",
		Key:      "ap2:aim_master",
		Name:     "Aim Master",
		Version:  version,
		GameType: "ffa",
		Stats: []StatInput{
			{Key: "score", Type: "int", Display: boolPtr(false)},
			{Key: "clicks", Type: "int"},
			{Key: "accuracy", Type: "float", Unit: "percent"},
			{Key: "avg_advance_time", Type: "float", Unit: "seconds", HigherIsBetter: boolPtr(false)},
		},
	}
}

func arcadeParty(version string) DefinitionInput {
	return DefinitionInput{
		Kind:    "game",
		Key:     "ap2",
		Name:    "Arcade Party",
		Group:   "arcade_party",
		Version: version,
		Stats: []StatInput{
			{Key: "rounds_played", Scope: "match", Type: "int"},
		},
	}
}

func boolPtr(v bool) *bool {
	return &v
}

func TestRegisterIsIdempotent(t *testing.T) {
	service := newService(t)

	manifest := Manifest{Definitions: []DefinitionInput{arcadeParty("1.0.0"), aimMaster("1.0.0")}}

	first, err := service.Register(t.Context(), manifest)

	if err != nil {
		t.Fatalf("first register: %v", err)
	}

	if first.Created != 2 || first.Unchanged != 0 {
		t.Fatalf("expected 2 created, got %d created and %d unchanged", first.Created, first.Unchanged)
	}

	second, err := service.Register(t.Context(), manifest)

	if err != nil {
		t.Fatalf("second register: %v", err)
	}

	if second.Created != 0 || second.Unchanged != 2 {
		t.Fatalf("expected 2 unchanged, got %d created and %d unchanged", second.Created, second.Unchanged)
	}

	for i, entry := range second.Definitions {
		if entry.VersionID != first.Definitions[i].VersionID {
			t.Errorf("version id changed for %s: %d then %d", entry.Key, first.Definitions[i].VersionID, entry.VersionID)
		}
	}
}

func TestRegisterRejectsChangedDeclarationAtSameVersion(t *testing.T) {
	service := newService(t)

	if _, err := service.Register(t.Context(), Manifest{Definitions: []DefinitionInput{aimMaster("1.0.0")}}); err != nil {
		t.Fatalf("register: %v", err)
	}

	changed := aimMaster("1.0.0")
	changed.Stats = append(changed.Stats, StatInput{Key: "misses", Type: "int"})

	_, err := service.Register(t.Context(), Manifest{Definitions: []DefinitionInput{changed}})

	conflict := &ConflictError{}

	if !errors.As(err, &conflict) {
		t.Fatalf("expected a conflict, got %v", err)
	}

	if conflict.Problems[0].Code != "definition_conflict" {
		t.Fatalf("expected definition_conflict, got %q", conflict.Problems[0].Code)
	}

	// The rejected registration must not have added the new stat.
	definitions, err := service.ListDefinitions(t.Context(), "minigame", "ap2:aim_master")

	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if definitions[0].Stats != 4 {
		t.Fatalf("expected the declaration to be unchanged, got %d stats", definitions[0].Stats)
	}
}

func TestRegisterRejectsChangedStatTypeAcrossVersions(t *testing.T) {
	service := newService(t)

	if _, err := service.Register(t.Context(), Manifest{Definitions: []DefinitionInput{aimMaster("1.0.0")}}); err != nil {
		t.Fatalf("register: %v", err)
	}

	next := aimMaster("1.1.0")
	next.Stats[1].Type = "float"

	_, err := service.Register(t.Context(), Manifest{Definitions: []DefinitionInput{next}})

	conflict := &ConflictError{}

	if !errors.As(err, &conflict) {
		t.Fatalf("expected a conflict, got %v", err)
	}

	if conflict.Problems[0].Code != "stat_conflict" {
		t.Fatalf("expected stat_conflict, got %q", conflict.Problems[0].Code)
	}
}

func TestRegisterAcceptsNewVersionWithAddedStat(t *testing.T) {
	service := newService(t)

	if _, err := service.Register(t.Context(), Manifest{Definitions: []DefinitionInput{aimMaster("1.0.0")}}); err != nil {
		t.Fatalf("register: %v", err)
	}

	next := aimMaster("1.1.0")
	next.Stats = append(next.Stats, StatInput{Key: "misses", Type: "int"})

	result, err := service.Register(t.Context(), Manifest{Definitions: []DefinitionInput{next}})

	if err != nil {
		t.Fatalf("register next version: %v", err)
	}

	if result.Created != 1 {
		t.Fatalf("expected the new version to be created, got %+v", result)
	}

	schema, err := service.schema(t.Context(), "minigame", Ref{Key: "ap2:aim_master", Version: "1.0.0"})

	if err != nil {
		t.Fatalf("load old schema: %v", err)
	}

	if _, ok := schema.stat("player", "misses"); ok {
		t.Error("version 1.0.0 must not declare a stat added in 1.1.0")
	}
}

func TestRegisterRejectsUnknownStatType(t *testing.T) {
	service := newService(t)

	broken := aimMaster("1.0.0")
	broken.Stats[0].Type = "complex"

	_, err := service.Register(t.Context(), Manifest{Definitions: []DefinitionInput{broken}})

	validation := &ValidationError{}

	if !errors.As(err, &validation) {
		t.Fatalf("expected a validation error, got %v", err)
	}

	if validation.Problems[0].Code != "invalid_stat_type" {
		t.Fatalf("expected invalid_stat_type, got %q", validation.Problems[0].Code)
	}
}

func TestRegisterMarksRecordStats(t *testing.T) {
	service := newService(t)

	if _, err := service.Register(t.Context(), Manifest{Definitions: []DefinitionInput{aimMaster("1.0.0")}}); err != nil {
		t.Fatalf("register: %v", err)
	}

	schema, err := service.schema(t.Context(), "minigame", Ref{Key: "ap2:aim_master", Version: "1.0.0"})

	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	clicks, _ := schema.stat("player", "clicks")

	if !clicks.IsRecord {
		t.Error("a displayed numeric player stat should be a record stat")
	}

	score, _ := schema.stat("player", "score")

	if score.IsRecord {
		t.Error("a hidden stat should not be a record stat")
	}
}
