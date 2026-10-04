package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"buf.build/go/protovalidate"
	"github.com/aarondl/opt/omit"
	ledgerv1 "github.com/lclpyt/ledger/gen/go/apis/ledger/v1"
	"github.com/lclpyt/ledger/gen/go/bob/enums"
	"github.com/lclpyt/ledger/gen/go/bob/model"
	"github.com/lclpyt/ledger/internal/games"
	"github.com/quadrubo/golib/apperr"
	"github.com/quadrubo/golib/fieldmask"
	"github.com/quadrubo/golib/grpcinterceptor/validate"
	"github.com/quadrubo/golib/protoconv"
	"github.com/stephenafamo/bob/types"
)

func newGamesApi(gamesService *games.Service, validator protovalidate.Validator) *gamesApi {
	return &gamesApi{
		gamesService: gamesService,
		validator:    validator,
	}
}

type gamesApi struct {
	ledgerv1.UnimplementedGamesServiceServer

	gamesService *games.Service
	validator    protovalidate.Validator
}

func (g *gamesApi) GetGame(ctx context.Context, request *ledgerv1.GetGameRequest) (*ledgerv1.Game, error) {
	id, err := parseGameId(request.GetName())

	if err != nil {
		return nil, apperr.InvalidResourceName("name", err, request.GetName())
	}

	game, err := g.gamesService.GetById(ctx, id)

	if err != nil {
		return nil, fmt.Errorf("games: failed to get game %q: %w", request.GetName(), err)
	}

	return gameToProto(game)
}

func (g *gamesApi) ListGames(ctx context.Context, request *ledgerv1.ListGamesRequest) (*ledgerv1.ListGamesResponse, error) {
	order, err := games.Paging.ParseOrderBy("create_time")

	if err != nil {
		return nil, fmt.Errorf("games: failed to parse order: %w", err)
	}

	scope := []string{strconv.FormatBool(request.GetShowDeleted())}

	page, err := order.Page(int(request.GetPageSize()), request.GetPageToken(), scope)

	if err != nil {
		return nil, apperr.InvalidPageToken(err, request.GetPageToken())
	}

	filters := games.Filters{
		ShowDeleted: request.GetShowDeleted(),
	}

	gameModels, nextPageToken, err := g.gamesService.List(ctx, page, filters)

	if err != nil {
		return nil, fmt.Errorf("games: failed to list games: %w", err)
	}

	gameList := make([]*ledgerv1.Game, 0, len(gameModels))

	for _, row := range gameModels {
		game, err := gameToProto(row)

		if err != nil {
			return nil, err
		}

		gameList = append(gameList, game)
	}

	return &ledgerv1.ListGamesResponse{
		Games:         gameList,
		NextPageToken: nextPageToken,
	}, nil
}

func (g *gamesApi) CreateGame(ctx context.Context, request *ledgerv1.CreateGameRequest) (*ledgerv1.Game, error) {
	translations, err := json.Marshal(request.GetGame().GetNameTranslations())

	if err != nil {
		return nil, fmt.Errorf("games: failed to encode name translations: %w", err)
	}

	game, err := g.gamesService.Create(ctx, &model.GameSetter{
		ID:               omit.From(request.GetGameId()),
		Type:             omit.From(gameTypeFromProto(request.GetGame().GetType())),
		NameTranslations: omit.From(types.NewJSON(json.RawMessage(translations))),
	})

	if err != nil {
		return nil, fmt.Errorf("games: failed to create game: %w", err)
	}

	return gameToProto(game)
}

func (g *gamesApi) UpdateGame(ctx context.Context, request *ledgerv1.UpdateGameRequest) (*ledgerv1.Game, error) {
	id, err := parseGameId(request.GetGame().GetName())

	if err != nil {
		return nil, apperr.InvalidResourceName("game.name", err, request.GetGame().GetName())
	}

	mask, err := fieldmask.Parse(request.GetUpdateMask(), games.UpdatePaths)

	if err != nil {
		return nil, apperr.InvalidUpdateMask(err, request.GetUpdateMask())
	}

	if err := validate.Masked(g.validator, request.GetGame(), mask, "game"); err != nil {
		return nil, err
	}

	game, err := g.gamesService.Update(ctx, id, func(game *model.Game) (*model.GameSetter, error) {
		if mask.Has("type") && gameTypeFromProto(request.GetGame().GetType()) != game.Type {
			return nil, apperr.ImmutableField("game.type", "type")
		}

		setter := &model.GameSetter{}

		if mask.Has("name_translations") {
			translations, err := json.Marshal(request.GetGame().GetNameTranslations())

			if err != nil {
				return nil, fmt.Errorf("games: failed to encode name translations: %w", err)
			}

			setter.NameTranslations = omit.From(types.NewJSON(json.RawMessage(translations)))
		}

		return setter, nil
	})

	if err != nil {
		return nil, fmt.Errorf("games: failed to update game %q: %w", request.GetGame().GetName(), err)
	}

	return gameToProto(game)
}

func (g *gamesApi) DeleteGame(ctx context.Context, request *ledgerv1.DeleteGameRequest) (*ledgerv1.Game, error) {
	id, err := parseGameId(request.GetName())

	if err != nil {
		return nil, apperr.InvalidResourceName("name", err, request.GetName())
	}

	game, err := g.gamesService.Delete(ctx, id)

	if err != nil {
		return nil, fmt.Errorf("games: failed to delete game %q: %w", request.GetName(), err)
	}

	return gameToProto(game)
}

func (g *gamesApi) UndeleteGame(ctx context.Context, request *ledgerv1.UndeleteGameRequest) (*ledgerv1.Game, error) {
	id, err := parseGameId(request.GetName())

	if err != nil {
		return nil, apperr.InvalidResourceName("name", err, request.GetName())
	}

	game, err := g.gamesService.Undelete(ctx, id)

	if err != nil {
		return nil, fmt.Errorf("games: failed to undelete game %q: %w", request.GetName(), err)
	}

	return gameToProto(game)
}

func gameToProto(row *model.Game) (*ledgerv1.Game, error) {
	var translations map[string]string
	if err := json.Unmarshal(row.NameTranslations.Val, &translations); err != nil {
		return nil, fmt.Errorf("games: failed to decode name translations of %q: %w", row.ID, err)
	}

	return &ledgerv1.Game{
		Name:             games.Pattern.Format(row.ID),
		Type:             gameTypeToProto(row.Type),
		NameTranslations: translations,
		CreateTime:       protoconv.TimeToProto(row.CreateTime),
		UpdateTime:       protoconv.TimeToProto(row.UpdateTime),
		DeleteTime:       protoconv.TimeToProto(row.DeleteTime.GetOrZero()),
		PurgeTime:        protoconv.TimeToProto(row.PurgeTime.GetOrZero()),
	}, nil
}

func gameTypeToProto(t enums.GameType) ledgerv1.Game_Type {
	switch t {
	case enums.GameTypeGame:
		return ledgerv1.Game_TYPE_GAME
	case enums.GameTypeMinigame:
		return ledgerv1.Game_TYPE_MINIGAME
	default:
		return ledgerv1.Game_TYPE_UNSPECIFIED
	}
}

func gameTypeFromProto(t ledgerv1.Game_Type) enums.GameType {
	switch t {
	case ledgerv1.Game_TYPE_MINIGAME:
		return enums.GameTypeMinigame
	default:
		return enums.GameTypeGame
	}
}

func parseGameId(name string) (string, error) {
	ids, err := games.Pattern.Parse(name)

	if err != nil {
		return "", err
	}

	return ids[0], nil
}
