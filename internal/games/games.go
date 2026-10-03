package games

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lclpyt/ledger/internal/model"
	"github.com/quadrubo/golib/apperr"
	"github.com/quadrubo/golib/pagination"
	"github.com/quadrubo/golib/postgres"
	"github.com/quadrubo/golib/queryfield"
	"github.com/quadrubo/golib/resourcename"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dialect"
	"github.com/stephenafamo/bob/dialect/psql/sm"
)

const ResourceType = "ledger.lclpnet.work/Game"

var (
	Pattern = resourcename.MustCompile("games/*")

	Paging = pagination.MustCompile(pagination.Spec[*model.Game]{
		Fields: map[string]queryfield.Field[*model.Game]{
			"create_time": queryfield.Time("create_time", func(g *model.Game) time.Time { return g.CreateTime }),
		},
		Tiebreak:        queryfield.Text("id", func(g *model.Game) string { return g.ID }),
		DefaultPageSize: 50,
		MaxPageSize:     1000,
	})
)

func NewService(db bob.DB) *Service {
	return &Service{
		db: db,
	}
}

type Service struct {
	db bob.DB
}

type Filters struct {
	ShowDeleted bool
}

func (s *Service) List(
	ctx context.Context,
	page *pagination.Page[*model.Game],
	filters Filters,
) ([]*model.Game, string, error) {
	mods := []bob.Mod[*dialect.SelectQuery]{
		sm.Limit(page.Limit()),
	}

	for _, term := range page.Order() {
		mods = append(mods, sm.OrderBy(psql.Raw(term)))
	}

	if keyset, args, ok := page.Keyset(); ok {
		mods = append(mods, sm.Where(psql.Raw(keyset, args...)))
	}

	if !filters.ShowDeleted {
		mods = append(mods, model.SelectWhere.Games.DeleteTime.IsNull())
	}

	rows, err := model.Games.Query(mods...).All(ctx, s.db)

	if err != nil {
		return nil, "", fmt.Errorf("game service: failed to list games: %w", err)
	}

	rows, nextPageToken := page.Result(rows)

	return rows, nextPageToken, nil
}

func (s *Service) Create(
	ctx context.Context,
	setter *model.GameSetter,
) (*model.Game, error) {
	game, err := model.Games.Insert(setter).One(ctx, s.db)

	if err != nil {
		if postgres.IsUniqueViolationOn(err, "games_pkey") {
			return nil, apperr.AlreadyExists("GAME_ALREADY_EXISTS", ResourceType,
				Pattern.Format(setter.ID.MustGet()))
		}

		return nil, fmt.Errorf("game service: failed to insert game: %w", err)
	}

	return game, nil
}

func (s *Service) GetById(
	ctx context.Context,
	id string,
) (*model.Game, error) {
	game, err := model.Games.Query(
		model.SelectWhere.Games.ID.EQ(id),
	).One(ctx, s.db)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.NotFound("GAME_NOT_FOUND", ResourceType, Pattern.Format(id))
		}

		return nil, fmt.Errorf("game service: failed to query game by id: %w", err)
	}

	return game, nil
}
