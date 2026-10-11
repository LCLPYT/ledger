package tui

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/lclpyt/ledger/internal/admin"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

var userKeys = struct {
	create, edit, password, roles, delete key.Binding
}{
	create:   key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new")),
	edit:     key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
	password: key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "password")),
	roles:    key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "roles")),
	delete:   key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete/restore")),
}

var userColumns = []column{
	{title: "ID", min: 4},
	{title: "Name", min: 12, flex: 2},
	{title: "Roles", min: 10, flex: 3},
	{title: "Status", min: 8},
}

type usersLoadedMsg struct {
	users []admin.User
	roles []string
	err   error
}

type usersTab struct {
	env
	table  table.Model
	dialog dialog
	users  []admin.User
	roles  []string
	width  int
	height int
}

func newUsersTab(e env) *usersTab {
	return &usersTab{env: e, table: newTable()}
}

func (t *usersTab) title() string { return "Users" }

func (t *usersTab) capturing() bool { return t.dialog.active() }

func (t *usersTab) keys() []key.Binding {
	return []key.Binding{userKeys.create, userKeys.edit, userKeys.password, userKeys.roles, userKeys.delete}
}

func (t *usersTab) resize(width, height int) {
	t.width, t.height = width, height
	layoutTable(&t.table, userColumns, width)
}

func (t *usersTab) load() tea.Cmd {
	return func() tea.Msg {
		users, err := t.svc.Users(t.ctx)

		if err != nil {
			return usersLoadedMsg{err: err}
		}

		roles, err := t.svc.Roles(t.ctx)

		if err != nil {
			return usersLoadedMsg{err: err}
		}

		names := make([]string, len(roles))

		for i, role := range roles {
			names[i] = role.Name
		}

		return usersLoadedMsg{users: users, roles: names}
	}
}

func (t *usersTab) update(msg tea.Msg) tea.Cmd {
	if msg, ok := msg.(usersLoadedMsg); ok {
		if msg.err != nil {
			return func() tea.Msg { return statusMsg{err: msg.err} }
		}

		t.users, t.roles = msg.users, msg.roles
		setRows(&t.table, t.rows())

		return nil
	}

	if t.dialog.active() {
		return t.dialog.update(msg)
	}

	keyMsg, ok := msg.(tea.KeyMsg)

	if !ok {
		return nil
	}

	if key.Matches(keyMsg, userKeys.create) {
		return t.openCreate()
	}

	user, selected := t.selected()

	switch {
	case !selected:
	case key.Matches(keyMsg, userKeys.edit):
		return t.openEdit(user)
	case key.Matches(keyMsg, userKeys.password):
		return t.openPassword(user)
	case key.Matches(keyMsg, userKeys.roles):
		return t.openRoles(user)
	case key.Matches(keyMsg, userKeys.delete):
		return t.openDelete(user)
	}

	var cmd tea.Cmd
	t.table, cmd = t.table.Update(msg)

	return cmd
}

func (t *usersTab) rows() []table.Row {
	rows := make([]table.Row, len(t.users))

	for i, user := range t.users {
		status := "active"

		if user.Deleted() {
			status = "deleted"
		}

		rows[i] = table.Row{
			strconv.FormatInt(user.ID, 10),
			user.Name,
			strings.Join(user.Roles, ", "),
			status,
		}
	}

	return rows
}

func (t *usersTab) selected() (admin.User, bool) {
	i := t.table.Cursor()

	if i < 0 || i >= len(t.users) {
		return admin.User{}, false
	}

	return t.users[i], true
}

func (t *usersTab) view() string {
	if t.dialog.active() {
		return t.dialog.view()
	}

	if len(t.users) == 0 {
		return emptyHint("users")
	}

	return withDetail(&t.table, t.height, t.detail())
}

func (t *usersTab) detail() string {
	user, ok := t.selected()

	if !ok {
		return ""
	}

	lines := []string{
		styles.heading.Render(user.Name),
		field("Roles", listOrNone(user.Roles, ", ")),
		field("Created", formatTime(user.CreateTime)),
		field("Updated", formatTime(user.UpdateTime)),
	}

	if deleted, ok := user.DeleteTime.Get(); ok {
		lines = append(lines, field("Deleted", styles.warnText.Render(formatTime(deleted))))
	}

	return renderDetail(t.width, lines...)
}

func (t *usersTab) nameValidator(exceptID int64) func(string) error {
	return func(value string) error {
		if err := admin.ValidateUserName(value); err != nil {
			return err
		}

		taken, err := t.svc.UserNameTaken(t.ctx, value, exceptID)

		if err != nil {
			return err
		}

		if taken {
			return errors.New("another user already uses that name")
		}

		return nil
	}
}

func passwordInputs(password, confirmation *string) []huh.Field {
	return []huh.Field{
		huh.NewInput().
			Title("Password").
			Description(fmt.Sprintf("%d to %d bytes.", admin.MinPasswordLength, admin.MaxPasswordLength)).
			EchoMode(huh.EchoModePassword).
			Validate(func(value string) error { return admin.ValidatePassword([]byte(value)) }).
			Value(password),
		huh.NewInput().
			Title("Confirm password").
			EchoMode(huh.EchoModePassword).
			Validate(func(value string) error {
				if value != *password {
					return errors.New("passwords do not match")
				}

				return nil
			}).
			Value(confirmation),
	}
}

// roleSelect preselects the roles value holds.
func (t *usersTab) roleSelect(value *[]string) *huh.MultiSelect[string] {
	options := huh.NewOptions(t.roles...)

	return huh.NewMultiSelect[string]().
		Title("Roles").
		Description("The user and every access token of the user hold the permissions of these roles.").
		Options(options...).
		Filterable(true).
		Height(min(len(options)+2, max(t.height-8, 4))).
		Value(value)
}

func (t *usersTab) openCreate() tea.Cmd {
	var name, password, confirmation string
	var roles []string

	fields := append([]huh.Field{
		huh.NewInput().Title("Name").Validate(t.nameValidator(0)).Value(&name),
	}, passwordInputs(&password, &confirmation)...)

	groups := []*huh.Group{huh.NewGroup(fields...).Title("New user")}

	if len(t.roles) > 0 {
		groups = append(groups, huh.NewGroup(t.roleSelect(&roles)))
	}

	return t.dialog.open(t.width, func() tea.Cmd {
		return t.mutate("Created user "+name, func(ctx context.Context) error {
			user, err := t.svc.CreateUser(ctx, name, []byte(password))

			if err != nil || len(roles) == 0 {
				return err
			}

			return t.svc.SetUserRoles(ctx, user.ID, roles)
		})
	}, groups...)
}

func (t *usersTab) openEdit(user admin.User) tea.Cmd {
	name := user.Name

	return t.dialog.open(t.width, func() tea.Cmd {
		return t.mutate("Updated user "+name, func(ctx context.Context) error {
			return t.svc.UpdateUser(ctx, user.ID, name)
		})
	}, huh.NewGroup(
		huh.NewInput().Title("Name").Validate(t.nameValidator(user.ID)).Value(&name),
	).Title("Edit "+user.Name))
}

func (t *usersTab) openPassword(user admin.User) tea.Cmd {
	var password, confirmation string

	return t.dialog.open(t.width, func() tea.Cmd {
		return t.mutate("Changed the password of "+user.Name, func(ctx context.Context) error {
			return t.svc.SetPassword(ctx, user.ID, []byte(password))
		})
	}, huh.NewGroup(passwordInputs(&password, &confirmation)...).Title("New password for "+user.Name))
}

func (t *usersTab) openRoles(user admin.User) tea.Cmd {
	if len(t.roles) == 0 {
		return func() tea.Msg { return statusMsg{err: errors.New("no roles exist, create one in the roles tab")} }
	}

	roles := slices.Clone(user.Roles)

	return t.dialog.open(t.width, func() tea.Cmd {
		return t.mutate("Updated the roles of "+user.Name, func(ctx context.Context) error {
			return t.svc.SetUserRoles(ctx, user.ID, roles)
		})
	}, huh.NewGroup(t.roleSelect(&roles)).Title("Roles of "+user.Name))
}

func (t *usersTab) openDelete(user admin.User) tea.Cmd {
	var confirmed bool

	if user.Deleted() {
		return t.dialog.open(t.width, func() tea.Cmd {
			if !confirmed {
				return nil
			}

			return t.mutate("Restored user "+user.Name, func(ctx context.Context) error {
				return t.svc.RestoreUser(ctx, user.ID)
			})
		}, confirm("Restore "+user.Name+"?", "The access tokens of the user that are neither revoked nor expired work again.", "Restore", &confirmed))
	}

	return t.dialog.open(t.width, func() tea.Cmd {
		if !confirmed {
			return nil
		}

		return t.mutate("Deleted user "+user.Name, func(ctx context.Context) error {
			return t.svc.DeleteUser(ctx, user.ID)
		})
	}, confirm("Delete "+user.Name+"?", "Every access token of the user stops working. The user can be restored later on.", "Delete", &confirmed))
}
