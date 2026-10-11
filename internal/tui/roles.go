package tui

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/lclpyt/ledger/gen/go/bob/model"
	"github.com/lclpyt/ledger/internal/admin"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

var roleKeys = struct {
	create, rename, permissions, members, delete key.Binding
}{
	create:      key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new")),
	rename:      key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "rename")),
	permissions: key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "permissions")),
	members:     key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "members")),
	delete:      key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
}

var roleColumns = []column{
	{title: "Name", min: 12, flex: 1},
	{title: "Members", min: 7},
	{title: "Permissions", min: 20, flex: 4},
}

type rolesLoadedMsg struct {
	roles []admin.Role
	users []admin.User
	err   error
}

type rolesTab struct {
	env
	table  table.Model
	dialog dialog
	roles  []admin.Role
	users  []admin.User
	width  int
	height int
}

func newRolesTab(e env) *rolesTab {
	return &rolesTab{env: e, table: newTable()}
}

func (t *rolesTab) title() string { return "Roles" }

func (t *rolesTab) capturing() bool { return t.dialog.active() }

func (t *rolesTab) keys() []key.Binding {
	return []key.Binding{
		roleKeys.create,
		roleKeys.rename,
		roleKeys.permissions,
		roleKeys.members,
		roleKeys.delete,
	}
}

func (t *rolesTab) resize(width, height int) {
	t.width, t.height = width, height
	layoutTable(&t.table, roleColumns, width)
}

func (t *rolesTab) load() tea.Cmd {
	return func() tea.Msg {
		roles, err := t.svc.Roles(t.ctx)

		if err != nil {
			return rolesLoadedMsg{err: err}
		}

		users, err := t.svc.Users(t.ctx)

		return rolesLoadedMsg{roles: roles, users: users, err: err}
	}
}

func (t *rolesTab) update(msg tea.Msg) tea.Cmd {
	if msg, ok := msg.(rolesLoadedMsg); ok {
		if msg.err != nil {
			return func() tea.Msg { return statusMsg{err: msg.err} }
		}

		t.roles, t.users = msg.roles, msg.users
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

	if key.Matches(keyMsg, roleKeys.create) {
		return t.openCreate()
	}

	role, selected := t.selected()

	switch {
	case !selected:
	case key.Matches(keyMsg, roleKeys.rename):
		return t.openRename(role)
	case key.Matches(keyMsg, roleKeys.permissions):
		return t.openPermissions(role)
	case key.Matches(keyMsg, roleKeys.members):
		return t.openMembers(role)
	case key.Matches(keyMsg, roleKeys.delete):
		return t.openDelete(role)
	}

	var cmd tea.Cmd
	t.table, cmd = t.table.Update(msg)

	return cmd
}

func (t *rolesTab) rows() []table.Row {
	rows := make([]table.Row, len(t.roles))

	for i, role := range t.roles {
		rows[i] = table.Row{
			role.Name,
			strconv.Itoa(len(role.Members)),
			strings.Join(role.Permissions, ", "),
		}
	}

	return rows
}

func (t *rolesTab) selected() (admin.Role, bool) {
	i := t.table.Cursor()

	if i < 0 || i >= len(t.roles) {
		return admin.Role{}, false
	}

	return t.roles[i], true
}

func (t *rolesTab) view() string {
	if t.dialog.active() {
		return t.dialog.view()
	}

	if len(t.roles) == 0 {
		return emptyHint("roles")
	}

	return withDetail(&t.table, t.height, t.detail())
}

func (t *rolesTab) detail() string {
	role, ok := t.selected()

	if !ok {
		return ""
	}

	permissions := make([]string, len(role.Permissions))
	unknown := t.svc.UnknownPermissions(role.Permissions)

	for i, p := range role.Permissions {
		permissions[i] = p

		if slices.Contains(unknown, p) {
			permissions[i] += styles.warnText.Render(" (unchecked)")
		}
	}

	members := make([]string, len(role.Members))

	for i, member := range role.Members {
		members[i] = userLabel(member)
	}

	return renderDetail(t.width,
		styles.heading.Render(role.Name),
		field("Created", formatTime(role.CreateTime)),
		field("Permissions", listOrNone(permissions, ", ")),
		field("Members", listOrNone(members, ", ")),
	)
}

func (t *rolesTab) nameValidator(current string) func(string) error {
	return func(name string) error {
		if err := admin.ValidateRoleName(name); err != nil {
			return err
		}

		if name == current {
			return nil
		}

		exists, err := t.svc.RoleExists(t.ctx, name)

		if err != nil {
			return err
		}

		if exists {
			return fmt.Errorf("role %s already exists", name)
		}

		return nil
	}
}

// permissionFields lets the user pick from the checked permissions and type in any other one.
// selected is preselected and receives the picked permissions, extra the typed ones.
func (t *rolesTab) permissionFields(selected *[]string, extra *string) []huh.Field {
	known := t.svc.PermissionOptions()
	unknown := slices.DeleteFunc(slices.Clone(*selected), func(p string) bool { return slices.Contains(known, p) })
	options := huh.NewOptions(append(known, unknown...)...)

	for i := range unknown {
		options[len(known)+i].Key += " (unchecked)"
	}

	return []huh.Field{
		huh.NewMultiSelect[string]().
			Title("Permissions").
			Description("A wildcard grants every sub-permission. Press / to filter.").
			Options(options...).
			Filterable(true).
			Height(min(len(options)+2, max(t.height-10, 5))).
			Value(selected),
		huh.NewInput().
			Title("Other permissions").
			Description("Space separated, for other permissions unknown to the app.").
			Validate(func(value string) error { return admin.ValidatePermissions(strings.Fields(value)) }).
			Value(extra),
	}
}

func (t *rolesTab) openCreate() tea.Cmd {
	var name, extra string
	var permissions []string

	return t.dialog.open(t.width, func() tea.Cmd {
		return t.mutate("Created role "+name, func(ctx context.Context) error {
			return t.svc.CreateRole(ctx, name, append(permissions, strings.Fields(extra)...))
		})
	},
		huh.NewGroup(huh.NewInput().
			Title("Name").
			Description("Lower case letters, digits and underscores.").
			Validate(t.nameValidator("")).
			Value(&name),
		).Title("New role"),
		huh.NewGroup(t.permissionFields(&permissions, &extra)...),
	)
}

func (t *rolesTab) openRename(role admin.Role) tea.Cmd {
	if admin.IsProtectedRole(role.Name) {
		return func() tea.Msg {
			return statusMsg{err: fmt.Errorf("role %s is protected and cannot be renamed", role.Name)}
		}
	}

	name := role.Name

	return t.dialog.open(t.width, func() tea.Cmd {
		return t.mutate(fmt.Sprintf("Renamed role %s to %s", role.Name, name), func(ctx context.Context) error {
			return t.svc.RenameRole(ctx, role.Name, name)
		})
	}, huh.NewGroup(huh.NewInput().Title("Name").Validate(t.nameValidator(role.Name)).Value(&name)).
		Title("Rename "+role.Name))
}

func (t *rolesTab) openPermissions(role admin.Role) tea.Cmd {
	var extra string
	permissions := slices.Clone(role.Permissions)

	return t.dialog.open(t.width, func() tea.Cmd {
		return t.mutate("Updated the permissions of "+role.Name, func(ctx context.Context) error {
			return t.svc.SetRolePermissions(ctx, role.Name, append(permissions, strings.Fields(extra)...))
		})
	}, huh.NewGroup(t.permissionFields(&permissions, &extra)...).Title("Permissions of "+role.Name))
}

func (t *rolesTab) openMembers(role admin.Role) tea.Cmd {
	candidates := slices.DeleteFunc(slices.Clone(t.users), func(u admin.User) bool {
		return u.Deleted() && !isMember(role, u.ID)
	})

	if len(candidates) == 0 {
		return func() tea.Msg { return statusMsg{text: "No users exist yet"} }
	}

	options := make([]huh.Option[int64], len(candidates))
	members := make([]int64, len(role.Members))

	for i, user := range candidates {
		options[i] = huh.NewOption(userLabel(user.User), user.ID)
	}

	for i, member := range role.Members {
		members[i] = member.ID
	}

	return t.dialog.open(t.width, func() tea.Cmd {
		return t.mutate("Updated the members of "+role.Name, func(ctx context.Context) error {
			return t.svc.SetRoleMembers(ctx, role.Name, members)
		})
	}, huh.NewGroup(
		huh.NewMultiSelect[int64]().
			Title("Members").
			Description("Press / to filter.").
			Options(options...).
			Filterable(true).
			Height(min(len(options)+2, max(t.height-8, 4))).
			Value(&members),
	).Title("Members of "+role.Name))
}

func (t *rolesTab) openDelete(role admin.Role) tea.Cmd {
	if admin.IsProtectedRole(role.Name) {
		return func() tea.Msg {
			return statusMsg{err: fmt.Errorf("role %s is protected and cannot be deleted", role.Name)}
		}
	}

	var confirmed bool

	description := fmt.Sprintf("%d member(s) lose the permissions it grants.", len(role.Members))

	return t.dialog.open(t.width, func() tea.Cmd {
		if !confirmed {
			return nil
		}

		return t.mutate("Deleted role "+role.Name, func(ctx context.Context) error {
			return t.svc.DeleteRole(ctx, role.Name)
		})
	}, confirm("Delete "+role.Name+"?", description, "Delete", &confirmed))
}

func isMember(role admin.Role, userID int64) bool {
	return slices.ContainsFunc(role.Members, func(u *model.User) bool { return u.ID == userID })
}

func userLabel(user *model.User) string {
	label := user.Name

	if user.DeleteTime.IsValue() {
		label += " [deleted]"
	}

	return label
}
