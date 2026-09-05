package stats

import (
	"context"
	"database/sql"
	"errors"
	"sync"

	"ledger/model"
	"ledger/model/enums"

	"github.com/stephenafamo/bob"
)

// Service registers game schemas and ingests matches validated against them.
type Service struct {
	db bob.DB

	mu      sync.RWMutex
	schemas map[refKey]*versionSchema
}

func NewService(db bob.DB) *Service {
	return &Service{db: db, schemas: make(map[refKey]*versionSchema)}
}

type refKey struct {
	kind    enums.DefinitionKind
	key     string
	version string
}

type statMeta struct {
	ID             int64
	Scope          enums.StatScope
	Key            string
	ValueType      enums.StatValueType
	HigherIsBetter bool
	IsRecord       bool
}

type versionSchema struct {
	VersionID    int64
	DefinitionID int64
	Kind         enums.DefinitionKind
	Key          string
	Version      string
	GameType     enums.GameType
	stats        map[statKey]statMeta
}

type statKey struct {
	scope enums.StatScope
	key   string
}

func (v *versionSchema) stat(scope enums.StatScope, key string) (statMeta, bool) {
	m, ok := v.stats[statKey{scope: scope, key: key}]
	return m, ok
}

// errUnregistered reports a definition version that was never registered.
var errUnregistered = errors.New("definition version is not registered")

// schema returns the declared schema of a definition version, loading and
// caching it on first use.
func (s *Service) schema(ctx context.Context, kind enums.DefinitionKind, ref Ref) (*versionSchema, error) {
	rk := refKey{kind: kind, key: ref.Key, version: ref.Version}

	s.mu.RLock()
	cached, ok := s.schemas[rk]
	s.mu.RUnlock()

	if ok {
		return cached, nil
	}

	loaded, err := s.loadSchema(ctx, rk)

	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.schemas[rk] = loaded
	s.mu.Unlock()

	return loaded, nil
}

func (s *Service) loadSchema(ctx context.Context, rk refKey) (*versionSchema, error) {
	definition, err := model.Definitions.Query(
		model.SelectWhere.Definitions.Kind.EQ(rk.kind),
		model.SelectWhere.Definitions.Key.EQ(rk.key),
	).One(ctx, s.db)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, errUnregistered
	}

	if err != nil {
		return nil, err
	}

	version, err := model.DefinitionVersions.Query(
		model.SelectWhere.DefinitionVersions.DefinitionID.EQ(definition.ID),
		model.SelectWhere.DefinitionVersions.Version.EQ(rk.version),
	).One(ctx, s.db)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, errUnregistered
	}

	if err != nil {
		return nil, err
	}

	links, err := model.DefinitionVersionStats.Query(
		model.SelectWhere.DefinitionVersionStats.DefinitionVersionID.EQ(version.ID),
	).All(ctx, s.db)

	if err != nil {
		return nil, err
	}

	schema := &versionSchema{
		VersionID:    version.ID,
		DefinitionID: definition.ID,
		Kind:         definition.Kind,
		Key:          definition.Key,
		Version:      version.Version,
		GameType:     version.GameType.GetOrZero(),
		stats:        make(map[statKey]statMeta, len(links)),
	}

	if len(links) == 0 {
		return schema, nil
	}

	ids := make([]int64, 0, len(links))

	for _, link := range links {
		ids = append(ids, link.StatID)
	}

	declared, err := model.Stats.Query(model.SelectWhere.Stats.ID.In(ids...)).All(ctx, s.db)

	if err != nil {
		return nil, err
	}

	for _, stat := range declared {
		schema.stats[statKey{scope: stat.Scope, key: stat.Key}] = statMeta{
			ID:             stat.ID,
			Scope:          stat.Scope,
			Key:            stat.Key,
			ValueType:      stat.ValueType,
			HigherIsBetter: stat.HigherIsBetter.GetOr(true),
			IsRecord:       stat.IsRecord,
		}
	}

	return schema, nil
}

func (s *Service) invalidate(kind enums.DefinitionKind, key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for rk := range s.schemas {
		if rk.kind == kind && rk.key == key {
			delete(s.schemas, rk)
		}
	}
}
