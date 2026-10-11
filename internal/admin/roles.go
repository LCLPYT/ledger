package admin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"

	"github.com/lclpyt/ledger/gen/go/bob/model"

	"github.com/aarondl/opt/omit"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql/dialect"
	"github.com/stephenafamo/bob/dialect/psql/sm"
)

const (
	// DefaultUserRole is preselected for new users
	DefaultUserRole = "user"
	AdminRole       = "admin"
)

// IsProtectedRole reports whether the role is seeded by the migrations and thus must not be renamed or deleted.
func IsProtectedRole(name string) bool {
	return name == DefaultUserRole || name == AdminRole
}

type Role struct {
	*model.Role
	Permissions []string
	Members     []*model.User
}

func (s *Service) Roles(ctx context.Context) ([]Role, error) {
	roles, err := model.Roles.Query(sm.OrderBy(model.Roles.Columns.Name)).All(ctx, s.db)

	if err != nil {
		return nil, err
	}

	permissions, err := model.RolePermissions.Query(sm.OrderBy(model.RolePermissions.Columns.Permission)).All(ctx, s.db)

	if err != nil {
		return nil, err
	}

	memberships, err := model.UserRoles.Query().All(ctx, s.db)

	if err != nil {
		return nil, err
	}

	users, err := model.Users.Query(
		sm.OrderBy(model.Users.Columns.Name),
		sm.OrderBy(model.Users.Columns.ID),
	).All(ctx, s.db)

	if err != nil {
		return nil, err
	}

	permissionsByRole := make(map[string][]string)

	for _, p := range permissions {
		permissionsByRole[p.RoleName] = append(permissionsByRole[p.RoleName], p.Permission)
	}

	rolesByUser := make(map[int64]map[string]bool)

	for _, m := range memberships {
		if rolesByUser[m.UserID] == nil {
			rolesByUser[m.UserID] = make(map[string]bool)
		}

		rolesByUser[m.UserID][m.RoleName] = true
	}

	result := make([]Role, len(roles))

	for i, role := range roles {
		result[i] = Role{Role: role, Permissions: permissionsByRole[role.Name]}

		for _, user := range users {
			if rolesByUser[user.ID][role.Name] {
				result[i].Members = append(result[i].Members, user)
			}
		}
	}

	return result, nil
}

func (s *Service) RoleExists(ctx context.Context, name string) (bool, error) {
	return model.RoleExists(ctx, s.db, name)
}

func (s *Service) CreateRole(ctx context.Context, name string, permissions []string) error {
	if err := ValidateRoleName(name); err != nil {
		return err
	}

	if err := ValidatePermissions(permissions); err != nil {
		return err
	}

	exists, err := s.RoleExists(ctx, name)

	if err != nil {
		return err
	}

	if exists {
		return fmt.Errorf("role %s already exists", name)
	}

	return s.inTx(ctx, func(tx bob.Executor) error {
		if _, err := model.Roles.Insert(&model.RoleSetter{Name: omit.From(name)}).One(ctx, tx); err != nil {
			return err
		}

		return insertRolePermissions(ctx, tx, name, permissions)
	})
}

func (s *Service) RenameRole(ctx context.Context, name, newName string) error {
	if IsProtectedRole(name) {
		return fmt.Errorf("role %s is protected and cannot be renamed", name)
	}

	if err := ValidateRoleName(newName); err != nil {
		return err
	}

	if name == newName {
		return nil
	}

	exists, err := s.RoleExists(ctx, newName)

	if err != nil {
		return err
	}

	if exists {
		return fmt.Errorf("role %s already exists", newName)
	}

	role, err := findRole(ctx, s.db, name)

	if err != nil {
		return err
	}

	return role.Update(ctx, s.db, &model.RoleSetter{Name: omit.From(newName)})
}

// DeleteRole deletes the role, removing it from every member.
func (s *Service) DeleteRole(ctx context.Context, name string) error {
	if IsProtectedRole(name) {
		return fmt.Errorf("role %s is protected and cannot be deleted", name)
	}

	role, err := findRole(ctx, s.db, name)

	if err != nil {
		return err
	}

	return role.Delete(ctx, s.db)
}

// SetRolePermissions replaces the permissions of the role.
func (s *Service) SetRolePermissions(ctx context.Context, name string, permissions []string) error {
	if err := ValidatePermissions(permissions); err != nil {
		return err
	}

	return s.inTx(ctx, func(tx bob.Executor) error {
		if _, err := findRole(ctx, tx, name); err != nil {
			return err
		}

		_, err := model.RolePermissions.Delete(model.DeleteWhere.RolePermissions.RoleName.EQ(name)).Exec(ctx, tx)

		if err != nil {
			return err
		}

		return insertRolePermissions(ctx, tx, name, permissions)
	})
}

// SetRoleMembers replaces the users holding the role.
func (s *Service) SetRoleMembers(ctx context.Context, name string, userIDs []int64) error {
	return s.inTx(ctx, func(tx bob.Executor) error {
		if _, err := findRole(ctx, tx, name); err != nil {
			return err
		}

		_, err := model.UserRoles.Delete(model.DeleteWhere.UserRoles.RoleName.EQ(name)).Exec(ctx, tx)

		if err != nil || len(userIDs) == 0 {
			return err
		}

		setters := make([]bob.Mod[*dialect.InsertQuery], len(userIDs))

		for i, id := range userIDs {
			setters[i] = &model.UserRoleSetter{UserID: omit.From(id), RoleName: omit.From(name)}
		}

		_, err = model.UserRoles.Insert(setters...).Exec(ctx, tx)

		return err
	})
}

func findRole(ctx context.Context, exec bob.Executor, name string) (*model.Role, error) {
	role, err := model.FindRole(ctx, exec, name)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("role %s: %w", name, ErrNotFound)
	}

	return role, err
}

func insertRolePermissions(ctx context.Context, exec bob.Executor, role string, permissions []string) error {
	if len(permissions) == 0 {
		return nil
	}

	permissions = slices.Compact(slices.Sorted(slices.Values(permissions)))
	setters := make([]bob.Mod[*dialect.InsertQuery], len(permissions))

	for i, permission := range permissions {
		setters[i] = &model.RolePermissionSetter{RoleName: omit.From(role), Permission: omit.From(permission)}
	}

	_, err := model.RolePermissions.Insert(setters...).Exec(ctx, exec)

	return err
}
