package playersettings

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/aarondl/opt/omit"
	"github.com/gofrs/uuid/v5"
	"github.com/lclpyt/ledger/gen/go/bob/model"
	"github.com/quadrubo/golib/apperr"
	"github.com/quadrubo/golib/grpcerr"
	"github.com/quadrubo/golib/pagination"
	"github.com/quadrubo/golib/queryfield"
	"github.com/quadrubo/golib/resourcename"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dialect"
	"github.com/stephenafamo/bob/dialect/psql/im"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/stephenafamo/bob/types"
)

const (
	ResourceType = "ledger.lclpnet.work/PlayerSetting"

	namePattern   = "players/*/settings/*"
	parentPattern = "players/*"
	maxPathLength = 255
)

var (
	playerIDRegex = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	pathRegex     = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}(\.[a-z][a-z0-9_]{0,62}){0,15}$`)

	Paging = pagination.MustCompile(pagination.Spec[*model.PlayerSetting]{
		Tiebreak:        queryfield.Text("path", func(s *model.PlayerSetting) string { return s.Path }),
		DefaultPageSize: 50,
		MaxPageSize:     1000,
	})
)

func ValidPath(path string) bool {
	return len(path) <= maxPathLength && pathRegex.MatchString(path)
}

func FormatName(playerID uuid.UUID, path string) string {
	return FormatParent(playerID) + "/settings/" + path
}

func FormatParent(playerID uuid.UUID) string {
	return "players/" + playerID.String()
}

// ParseName returns the player id and setting path of a setting resource name.
func ParseName(name string) (uuid.UUID, string, error) {
	segments := strings.SplitN(name, "/", 4)

	if len(segments) != 4 || segments[0] != "players" || segments[2] != "settings" || !ValidPath(segments[3]) {
		return uuid.Nil, "", &resourcename.Error{Name: name, Pattern: namePattern}
	}

	playerID, ok := parsePlayerID(segments[1])

	if !ok {
		return uuid.Nil, "", &resourcename.Error{Name: name, Pattern: namePattern}
	}

	return playerID, segments[3], nil
}

func ParseParent(parent string) (uuid.UUID, error) {
	id, ok := strings.CutPrefix(parent, "players/")

	if !ok {
		return uuid.Nil, &resourcename.Error{Name: parent, Pattern: parentPattern}
	}

	playerID, ok := parsePlayerID(id)

	if !ok {
		return uuid.Nil, &resourcename.Error{Name: parent, Pattern: parentPattern}
	}

	return playerID, nil
}

func parsePlayerID(id string) (uuid.UUID, bool) {
	if !playerIDRegex.MatchString(id) {
		return uuid.Nil, false
	}

	playerID, err := uuid.FromString(id)

	return playerID, err == nil
}

func NewService(db bob.DB) *Service {
	return &Service{
		db: db,
	}
}

type Service struct {
	db bob.DB
}

func (s *Service) Get(
	ctx context.Context,
	playerID uuid.UUID,
	path string,
) (*model.PlayerSetting, error) {
	setting, err := model.PlayerSettings.Query(
		model.SelectWhere.PlayerSettings.PlayerID.EQ(playerID),
		model.SelectWhere.PlayerSettings.Path.EQ(path),
	).One(ctx, s.db)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, notFound(playerID, path)
		}

		return nil, fmt.Errorf("player settings service: failed to query setting: %w", err)
	}

	return setting, nil
}

// List returns the settings of a player. A non-empty prefix restricts the
// result to the setting at that path and its descendants.
func (s *Service) List(
	ctx context.Context,
	playerID uuid.UUID,
	prefix string,
	page *pagination.Page[*model.PlayerSetting],
) ([]*model.PlayerSetting, string, error) {
	mods := []bob.Mod[*dialect.SelectQuery]{
		model.SelectWhere.PlayerSettings.PlayerID.EQ(playerID),
		sm.Limit(page.Limit()),
	}

	for _, term := range page.Order() {
		mods = append(mods, sm.OrderBy(psql.Raw(term)))
	}

	if keyset, args, ok := page.Keyset(); ok {
		mods = append(mods, sm.Where(psql.Raw(keyset, args...)))
	}

	if prefix != "" {
		mods = append(mods, sm.Where(psql.Raw(
			"(path = ? OR (path >= ? AND path < ?))", prefix, prefix+".", prefix+"/",
		)))
	}

	rows, err := model.PlayerSettings.Query(mods...).All(ctx, s.db)

	if err != nil {
		return nil, "", fmt.Errorf("player settings service: failed to list settings: %w", err)
	}

	rows, nextPageToken := page.Result(rows)

	return rows, nextPageToken, nil
}

// Upsert creates or replaces a setting. Settings are leaves of the path tree,
// so a path that is an ancestor or descendant of an existing one is rejected.
func (s *Service) Upsert(
	ctx context.Context,
	playerID uuid.UUID,
	path string,
	value json.RawMessage,
) (*model.PlayerSetting, error) {
	var setting *model.PlayerSetting

	err := s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bob.Transaction) error {
		// Serializes writes per player, so concurrent overlap checks cannot both pass.
		if _, err := tx.ExecContext(ctx,
			"SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0))", playerID.String(),
		); err != nil {
			return fmt.Errorf("player settings service: failed to lock player: %w", err)
		}

		conflict, err := model.PlayerSettings.Query(
			model.SelectWhere.PlayerSettings.PlayerID.EQ(playerID),
			sm.Where(psql.Raw(
				"(path = ANY(?) OR (path >= ? AND path < ?))", ancestors(path), path+".", path+"/",
			)),
			sm.Limit(1),
		).One(ctx, tx)

		if err == nil {
			return pathConflict(playerID, path, conflict.Path)
		}

		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("player settings service: failed to check path conflicts: %w", err)
		}

		setting, err = model.PlayerSettings.Insert(
			&model.PlayerSettingSetter{
				PlayerID: omit.From(playerID),
				Path:     omit.From(path),
				Value:    omit.From(types.NewJSON(value)),
			},
			im.OnConflict("player_id", "path").DoUpdate(im.SetExcluded("value")),
		).One(ctx, tx)

		if err != nil {
			return fmt.Errorf("player settings service: failed to upsert setting: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return setting, nil
}

func (s *Service) Delete(
	ctx context.Context,
	playerID uuid.UUID,
	path string,
) error {
	_, err := model.PlayerSettings.Delete(
		model.DeleteWhere.PlayerSettings.PlayerID.EQ(playerID),
		model.DeleteWhere.PlayerSettings.Path.EQ(path),
	).One(ctx, s.db)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return notFound(playerID, path)
		}

		return fmt.Errorf("player settings service: failed to delete setting: %w", err)
	}

	return nil
}

func ancestors(path string) []string {
	var result []string

	for i := range len(path) {
		if path[i] == '.' {
			result = append(result, path[:i])
		}
	}

	return result
}

func notFound(playerID uuid.UUID, path string) error {
	return apperr.NotFound("PLAYER_SETTING_NOT_FOUND", ResourceType, FormatName(playerID, path))
}

func pathConflict(playerID uuid.UUID, path, existing string) error {
	return grpcerr.FailedPrecondition("PLAYER_SETTING_PATH_CONFLICT",
		"the setting {name} overlaps the existing setting {existing}",
		grpcerr.Arg("name", FormatName(playerID, path)),
		grpcerr.Arg("existing", FormatName(playerID, existing))).
		ForResource(ResourceType, FormatName(playerID, existing))
}
