package games

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/aarondl/opt/omit"
	"github.com/aarondl/opt/omitnull"
	"github.com/lclpyt/ledger/gen/go/bob/model"
	"github.com/quadrubo/golib/apperr"
	"github.com/quadrubo/golib/fieldmask"
	"github.com/quadrubo/golib/pagination"
	"github.com/quadrubo/golib/postgres"
	"github.com/quadrubo/golib/queryfield"
	"github.com/quadrubo/golib/resourcename"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dialect"
	"github.com/stephenafamo/bob/dialect/psql/sm"
)

const (
	ResourceType = "ledger.lclpnet.work/Game"
	PurgeDelay   = 30 * 24 * time.Hour
)

var (
	Pattern = resourcename.MustCompile("games/*")

	UpdatePaths = fieldmask.Paths{
		Writable:  []string{"name_translations"},
		Immutable: []string{"type"},
	}

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

// Upsert updates the game, or inserts the setter create returns when no game
// carries the id. A nil create makes a missing game a NotFound.
func (s *Service) Upsert(
	ctx context.Context,
	id string,
	create func() (*model.GameSetter, error),
	update func(game *model.Game) (*model.GameSetter, error),
) (*model.Game, error) {
	return s.write(ctx, id, create, func(game *model.Game) (*model.GameSetter, error) {
		if game.DeleteTime.IsValue() {
			return nil, apperr.Deleted("GAME_DELETED", ResourceType, Pattern.Format(id))
		}

		return update(game)
	})
}

func (s *Service) Delete(
	ctx context.Context,
	id string,
) (*model.Game, error) {
	return s.modify(ctx, id, func(game *model.Game) (*model.GameSetter, error) {
		if game.DeleteTime.IsValue() {
			return nil, apperr.AlreadyDeleted("GAME_ALREADY_DELETED", ResourceType, Pattern.Format(id))
		}

		now := time.Now()

		return &model.GameSetter{
			DeleteTime: omitnull.From(now),
			PurgeTime:  omitnull.From(now.Add(PurgeDelay)),
		}, nil
	})
}

func (s *Service) Undelete(
	ctx context.Context,
	id string,
) (*model.Game, error) {
	return s.modify(ctx, id, func(game *model.Game) (*model.GameSetter, error) {
		if !game.DeleteTime.IsValue() {
			return nil, apperr.NotDeleted("GAME_NOT_DELETED", ResourceType, Pattern.Format(id))
		}

		return &model.GameSetter{
			DeleteTime: omitnull.FromPtr[time.Time](nil),
			PurgeTime:  omitnull.FromPtr[time.Time](nil),
		}, nil
	})
}

func (s *Service) modify(
	ctx context.Context,
	id string,
	modify func(game *model.Game) (*model.GameSetter, error),
) (*model.Game, error) {
	return s.write(ctx, id, nil, modify)
}

func (s *Service) write(
	ctx context.Context,
	id string,
	create func() (*model.GameSetter, error),
	modify func(game *model.Game) (*model.GameSetter, error),
) (*model.Game, error) {
	var game *model.Game

	err := s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bob.Transaction) error {
		row, err := model.Games.Query(
			model.SelectWhere.Games.ID.EQ(id),
			sm.ForUpdate(),
		).One(ctx, tx)

		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				if create == nil {
					return apperr.NotFound("GAME_NOT_FOUND", ResourceType, Pattern.Format(id))
				}

				game, err = insertMissing(ctx, tx, id, create)

				return err
			}

			return fmt.Errorf("game service: failed to lock game: %w", err)
		}

		setter, err := modify(row)

		if err != nil {
			return err
		}

		if err := row.Update(ctx, tx, setter); err != nil {
			return fmt.Errorf("game service: failed to update game: %w", err)
		}

		game = row

		return nil
	})

	if err != nil {
		return nil, err
	}

	return game, nil
}

func insertMissing(
	ctx context.Context,
	tx bob.Transaction,
	id string,
	create func() (*model.GameSetter, error),
) (*model.Game, error) {
	setter, err := create()

	if err != nil {
		return nil, err
	}

	setter.ID = omit.From(id)

	game, err := model.Games.Insert(setter).One(ctx, tx)

	if err != nil {
		if postgres.IsUniqueViolationOn(err, "games_pkey") {
			return nil, apperr.ConcurrentUpdate(ResourceType, Pattern.Format(id))
		}

		return nil, fmt.Errorf("game service: failed to insert game: %w", err)
	}

	return game, nil
}
