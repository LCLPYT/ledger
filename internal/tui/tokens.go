package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lclpyt/ledger/internal/admin"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

var tokenKeys = struct {
	create, revoke, delete, dismiss key.Binding
}{
	create:  key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new")),
	revoke:  key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "revoke")),
	delete:  key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
	dismiss: key.NewBinding(key.WithKeys("enter", "esc"), key.WithHelp("enter", "done")),
}

var tokenColumns = []column{
	{title: "Name", min: 12, flex: 2},
	{title: "Owner", min: 12, flex: 2},
	{title: "Scopes", min: 12, flex: 3},
	{title: "Status", min: 13},
	{title: "Expires", min: 16},
	{title: "Last used", min: 16},
}

var expiryOptions = []huh.Option[time.Duration]{
	huh.NewOption("Never", time.Duration(0)),
	huh.NewOption("7 days", 7*24*time.Hour),
	huh.NewOption("30 days", 30*24*time.Hour),
	huh.NewOption("90 days", 90*24*time.Hour),
	huh.NewOption("1 year", 365*24*time.Hour),
}

type tokensLoadedMsg struct {
	tokens      []admin.Token
	users       []admin.User
	permissions map[int64][]string
	err         error
}

type tokenCreatedMsg struct {
	name  string
	token string
	err   error
}

type tokensTab struct {
	env
	table       table.Model
	dialog      dialog
	tokens      []admin.Token
	users       []admin.User
	permissions map[int64][]string
	// secret holds a freshly created token until the user dismisses it.
	secret tokenCreatedMsg
	width  int
	height int
}

func newTokensTab(e env) *tokensTab {
	return &tokensTab{env: e, table: newTable()}
}

func (t *tokensTab) title() string { return "Access tokens" }

func (t *tokensTab) capturing() bool { return t.dialog.active() || t.secret.token != "" }

func (t *tokensTab) keys() []key.Binding {
	return []key.Binding{tokenKeys.create, tokenKeys.revoke, tokenKeys.delete}
}

func (t *tokensTab) resize(width, height int) {
	t.width, t.height = width, height
	layoutTable(&t.table, tokenColumns, width)
}

func (t *tokensTab) load() tea.Cmd {
	return func() tea.Msg {
		tokens, err := t.svc.Tokens(t.ctx)

		if err != nil {
			return tokensLoadedMsg{err: err}
		}

		users, err := t.svc.Users(t.ctx)

		if err != nil {
			return tokensLoadedMsg{err: err}
		}

		permissions, err := t.svc.UserPermissions(t.ctx)

		return tokensLoadedMsg{tokens: tokens, users: users, permissions: permissions, err: err}
	}
}

func (t *tokensTab) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tokensLoadedMsg:
		if msg.err != nil {
			return func() tea.Msg { return statusMsg{err: msg.err} }
		}

		t.tokens, t.users, t.permissions = msg.tokens, msg.users, msg.permissions
		setRows(&t.table, t.rows())

		return nil

	case tokenCreatedMsg:
		if msg.err != nil {
			return func() tea.Msg { return statusMsg{err: msg.err} }
		}

		t.secret = msg

		return func() tea.Msg { return changedMsg{text: "Created token " + msg.name} }
	}

	if t.dialog.active() {
		return t.dialog.update(msg)
	}

	keyMsg, ok := msg.(tea.KeyMsg)

	if !ok {
		return nil
	}

	if t.secret.token != "" {
		if key.Matches(keyMsg, tokenKeys.dismiss) {
			t.secret = tokenCreatedMsg{}
		}

		return nil
	}

	if key.Matches(keyMsg, tokenKeys.create) {
		return t.openCreate()
	}

	token, selected := t.selected()

	switch {
	case !selected:
	case key.Matches(keyMsg, tokenKeys.revoke):
		return t.openRevoke(token)
	case key.Matches(keyMsg, tokenKeys.delete):
		return t.openDelete(token)
	}

	var cmd tea.Cmd
	t.table, cmd = t.table.Update(msg)

	return cmd
}

func (t *tokensTab) rows() []table.Row {
	now := time.Now()
	rows := make([]table.Row, len(t.tokens))

	for i, token := range t.tokens {
		rows[i] = table.Row{
			token.Name,
			token.Owner.Name,
			scopeSummary(token.Scopes),
			string(token.Status(now)),
			optionalTime(token.ExpireTime.Ptr(), "never"),
			optionalTime(token.LastUsedTime.Ptr(), "never"),
		}
	}

	return rows
}

func (t *tokensTab) selected() (admin.Token, bool) {
	i := t.table.Cursor()

	if i < 0 || i >= len(t.tokens) {
		return admin.Token{}, false
	}

	return t.tokens[i], true
}

func (t *tokensTab) view() string {
	switch {
	case t.dialog.active():
		return t.dialog.view()
	case t.secret.token != "":
		return t.secretView()
	case len(t.tokens) == 0:
		return emptyHint("access tokens")
	}

	return withDetail(&t.table, t.height, t.detail())
}

func (t *tokensTab) secretView() string {
	return strings.Join([]string{
		styles.heading.Render("Created token " + t.secret.name),
		"",
		"Store the token now, it cannot be shown again:",
		"",
		styles.secret.Render(t.secret.token),
		"",
		styles.muted.Render("Press enter when done."),
	}, "\n")
}

func (t *tokensTab) detail() string {
	token, ok := t.selected()

	if !ok {
		return ""
	}

	status := string(token.Status(time.Now()))

	if token.Status(time.Now()) != admin.TokenActive {
		status = styles.warnText.Render(status)
	}

	lines := []string{
		styles.heading.Render(token.Name),
		field("ID", token.ID.String()),
		field("Owner", userLabel(token.Owner)),
		field("Status", status),
		field("Scopes", scopeSummary(token.Scopes)),
		field("Created", formatTime(token.CreateTime)),
		field("Expires", optionalTime(token.ExpireTime.Ptr(), "never")),
		field("Last used", optionalTime(token.LastUsedTime.Ptr(), "never")),
	}

	if revoked, ok := token.RevokeTime.Get(); ok {
		lines = append(lines, field("Revoked", formatTime(revoked)))
	}

	return renderDetail(t.width, lines...)
}

func (t *tokensTab) openCreate() tea.Cmd {
	var owners []huh.Option[int64]

	for _, user := range t.users {
		if !user.Deleted() {
			owners = append(owners, huh.NewOption(userLabel(user.User), user.ID))
		}
	}

	if len(owners) == 0 {
		return func() tea.Msg { return statusMsg{text: "Create a user in the users tab first"} }
	}

	var (
		owner  = owners[0].Value
		name   string
		expiry time.Duration
		scopes []string
	)

	scopeSelect := huh.NewMultiSelect[string]().
		Title("Scopes").
		DescriptionFunc(func() string {
			if len(t.permissions[owner]) == 0 {
				return "The owner holds no permissions, the token is denied everything until the owner gets a role."
			}

			return "Restricts the token to the selected permissions. Select none to use full owner permissions for the token."
		}, &owner).
		OptionsFunc(func() []huh.Option[string] {
			return huh.NewOptions(t.svc.Scopes(t.permissions[owner])...)
		}, &owner).
		Filterable(true).
		Height(max(t.height-8, 5)).
		Value(&scopes)

	return t.dialog.open(t.width, func() tea.Cmd {
		var expire *time.Time

		if expiry > 0 {
			at := time.Now().Add(expiry)
			expire = &at
		}

		return func() tea.Msg {
			token, err := t.svc.CreateToken(t.ctx, owner, name, expire, scopes)

			return tokenCreatedMsg{name: strings.TrimSpace(name), token: token, err: err}
		}
	},
		huh.NewGroup(
			huh.NewSelect[int64]().
				Title("Owner").
				Description("The token acts with the permissions of this user's roles.").
				Options(owners...).
				Value(&owner),
			huh.NewInput().Title("Name").Validate(notBlank).Value(&name),
			huh.NewSelect[time.Duration]().Title("Expires after").Options(expiryOptions...).Value(&expiry),
		).Title("New access token"),
		huh.NewGroup(scopeSelect),
	)
}

func (t *tokensTab) openRevoke(token admin.Token) tea.Cmd {
	if token.RevokeTime.IsValue() {
		return func() tea.Msg { return statusMsg{text: "Token " + token.Name + " is already revoked"} }
	}

	var confirmed bool

	return t.dialog.open(t.width, func() tea.Cmd {
		if !confirmed {
			return nil
		}

		return t.mutate("Revoked token "+token.Name, func(ctx context.Context) error {
			return t.svc.RevokeToken(ctx, token.ID)
		})
	}, confirm(
		"Revoke "+token.Name+"?",
		fmt.Sprintf("Requests with the token of %s fail from now on. The token stays listed.", token.Owner.Name),
		"Revoke",
		&confirmed,
	))
}

func (t *tokensTab) openDelete(token admin.Token) tea.Cmd {
	var confirmed bool

	return t.dialog.open(t.width, func() tea.Cmd {
		if !confirmed {
			return nil
		}

		return t.mutate("Deleted token "+token.Name, func(ctx context.Context) error {
			return t.svc.DeleteToken(ctx, token.ID)
		})
	}, confirm(
		"Delete "+token.Name+"?",
		"The token stops working and is removed for good.",
		"Delete",
		&confirmed,
	))
}

func scopeSummary(scopes []string) string {
	if len(scopes) == 0 {
		return "unrestricted"
	}

	return strings.Join(scopes, ", ")
}

func optionalTime(t *time.Time, fallback string) string {
	if t == nil {
		return fallback
	}

	return formatTime(*t)
}
