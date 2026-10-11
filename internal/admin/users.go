package admin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aarondl/opt/omit"
	"github.com/aarondl/opt/omitnull"
	"github.com/lclpyt/ledger/gen/go/bob/model"
	"github.com/lclpyt/ledger/internal/passwords"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql/dialect"
	"github.com/stephenafamo/bob/dialect/psql/sm"
)

type User struct {
	*model.User
	Roles []string
}

func (u User) Deleted() bool {
	return u.DeleteTime.IsValue()
}

func (s *Service) Users(ctx context.Context) ([]User, error) {
	users, err := model.Users.Query(
		sm.OrderBy(model.Users.Columns.Name),
		sm.OrderBy(model.Users.Columns.ID),
	).All(ctx, s.db)

	if err != nil {
		return nil, err
	}

	memberships, err := model.UserRoles.Query(sm.OrderBy(model.UserRoles.Columns.RoleName)).All(ctx, s.db)

	if err != nil {
		return nil, err
	}

	roles := make(map[int64][]string)

	for _, m := range memberships {
		roles[m.UserID] = append(roles[m.UserID], m.RoleName)
	}

	result := make([]User, len(users))

	for i, user := range users {
		result[i] = User{User: user, Roles: roles[user.ID]}
	}

	return result, nil
}

// UserNameTaken reports whether a user other than exceptID uses the name.
func (s *Service) UserNameTaken(ctx context.Context, name string, exceptID int64) (bool, error) {
	return model.Users.Query(
		model.SelectWhere.Users.Name.EQ(strings.TrimSpace(name)),
		model.SelectWhere.Users.ID.NE(exceptID),
	).Exists(ctx, s.db)
}

func (s *Service) CreateUser(ctx context.Context, name string, password []byte) (*model.User, error) {
	if err := s.checkUserName(ctx, name, 0); err != nil {
		return nil, err
	}

	hash, err := hashPassword(password)

	if err != nil {
		return nil, err
	}

	return model.Users.Insert(&model.UserSetter{
		Name:         omit.From(strings.TrimSpace(name)),
		PasswordHash: omit.From(hash),
	}).One(ctx, s.db)
}

func (s *Service) UpdateUser(ctx context.Context, id int64, name string) error {
	if err := s.checkUserName(ctx, name, id); err != nil {
		return err
	}

	return s.updateUser(ctx, id, &model.UserSetter{Name: omit.From(strings.TrimSpace(name))})
}

func (s *Service) SetPassword(ctx context.Context, id int64, password []byte) error {
	hash, err := hashPassword(password)

	if err != nil {
		return err
	}

	return s.updateUser(ctx, id, &model.UserSetter{PasswordHash: omit.From(hash)})
}

// DeleteUser soft deletes the user, which also invalidates every access token the user owns.
func (s *Service) DeleteUser(ctx context.Context, id int64) error {
	return s.updateUser(ctx, id, &model.UserSetter{DeleteTime: omitnull.From(time.Now())})
}

func (s *Service) RestoreUser(ctx context.Context, id int64) error {
	return s.updateUser(ctx, id, &model.UserSetter{DeleteTime: omitnull.FromPtr[time.Time](nil)})
}

// SetUserRoles replaces the roles of the user.
func (s *Service) SetUserRoles(ctx context.Context, id int64, roles []string) error {
	return s.inTx(ctx, func(tx bob.Executor) error {
		if _, err := findUser(ctx, tx, id); err != nil {
			return err
		}

		_, err := model.UserRoles.Delete(model.DeleteWhere.UserRoles.UserID.EQ(id)).Exec(ctx, tx)

		if err != nil || len(roles) == 0 {
			return err
		}

		setters := make([]bob.Mod[*dialect.InsertQuery], len(roles))

		for i, role := range roles {
			setters[i] = &model.UserRoleSetter{UserID: omit.From(id), RoleName: omit.From(role)}
		}

		_, err = model.UserRoles.Insert(setters...).Exec(ctx, tx)

		return err
	})
}

func (s *Service) checkUserName(ctx context.Context, name string, id int64) error {
	if err := ValidateUserName(name); err != nil {
		return err
	}

	taken, err := s.UserNameTaken(ctx, name, id)

	if err != nil {
		return err
	}

	if taken {
		return errors.New("another user already uses that name")
	}

	return nil
}

func (s *Service) updateUser(ctx context.Context, id int64, setter *model.UserSetter) error {
	user, err := findUser(ctx, s.db, id)

	if err != nil {
		return err
	}

	setter.UpdateTime = omit.From(time.Now())

	return user.Update(ctx, s.db, setter)
}

func findUser(ctx context.Context, exec bob.Executor, id int64) (*model.User, error) {
	user, err := model.FindUser(ctx, exec, id)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("user %d: %w", id, ErrNotFound)
	}

	return user, err
}

func hashPassword(password []byte) ([]byte, error) {
	if err := ValidatePassword(password); err != nil {
		return nil, err
	}

	return passwords.Hash(password)
}
