package tui

import (
	"context"
	"strings"

	"github.com/lclpyt/ledger/internal/admin"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tab interface {
	title() string
	load() tea.Cmd
	update(msg tea.Msg) tea.Cmd
	view() string
	resize(width, height int)
	// capturing reports whether the tab handles every key itself, e.g. while a form is open.
	capturing() bool
	keys() []key.Binding
}

// statusMsg is shown in the status line.
type statusMsg struct {
	text string
	err  error
}

// changedMsg reports a successful change, after which the active tab reloads.
type changedMsg struct {
	text string
}

var globalKeys = struct {
	next, prev, refresh, quit key.Binding
}{
	next:    key.NewBinding(key.WithKeys("tab", "right", "l"), key.WithHelp("tab", "next tab")),
	prev:    key.NewBinding(key.WithKeys("shift+tab", "left", "h"), key.WithHelp("shift+tab", "prev tab")),
	refresh: key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("ctrl+r", "refresh")),
	quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}

type app struct {
	tabs   []tab
	active int
	status statusMsg
	help   help.Model
	width  int
	height int
}

func Run(ctx context.Context, svc *admin.Service) error {
	e := env{ctx: ctx, svc: svc}

	m := &app{
		tabs: []tab{newUsersTab(e), newRolesTab(e), newTokensTab(e)},
		help: help.New(),
	}

	_, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithContext(ctx)).Run()

	return err
}

func (a *app) Init() tea.Cmd {
	return a.tabs[a.active].load()
}

func (a *app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		a.help.Width = msg.Width

		for _, t := range a.tabs {
			t.resize(msg.Width-2, a.bodyHeight())
		}

		return a, nil

	case statusMsg:
		a.status = msg
		return a, nil

	case changedMsg:
		a.status = statusMsg{text: msg.text}
		return a, a.tabs[a.active].load()

	case tea.KeyMsg:
		return a, a.handleKey(msg)
	}

	cmds := make([]tea.Cmd, len(a.tabs))

	for i, t := range a.tabs {
		cmds[i] = t.update(msg)
	}

	return a, tea.Batch(cmds...)
}

func (a *app) handleKey(msg tea.KeyMsg) tea.Cmd {
	if msg.String() == "ctrl+c" {
		return tea.Quit
	}

	current := a.tabs[a.active]

	if current.capturing() {
		return current.update(msg)
	}

	a.status = statusMsg{}

	switch {
	case key.Matches(msg, globalKeys.quit):
		return tea.Quit
	case key.Matches(msg, globalKeys.next):
		return a.switchTo((a.active + 1) % len(a.tabs))
	case key.Matches(msg, globalKeys.prev):
		return a.switchTo((a.active + len(a.tabs) - 1) % len(a.tabs))
	case key.Matches(msg, globalKeys.refresh):
		return current.load()
	}

	if s := msg.String(); len(s) == 1 && s[0] >= '1' && int(s[0]-'1') < len(a.tabs) {
		return a.switchTo(int(s[0] - '1'))
	}

	return current.update(msg)
}

func (a *app) switchTo(i int) tea.Cmd {
	a.active = i

	return a.tabs[i].load()
}

func (a *app) bodyHeight() int {
	return max(a.height-lipgloss.Height(a.header())-3, 1)
}

func (a *app) header() string {
	titles := make([]string, len(a.tabs))

	for i, t := range a.tabs {
		label := string(rune('1'+i)) + " " + t.title()

		if i == a.active {
			titles[i] = styles.activeTab.Render(label)
		} else {
			titles[i] = styles.tab.Render(label)
		}
	}

	bar := lipgloss.JoinHorizontal(lipgloss.Bottom, titles...)
	gap := styles.tabGap.Render(strings.Repeat(" ", max(a.width-lipgloss.Width(bar), 0)))

	return lipgloss.JoinHorizontal(lipgloss.Bottom, bar, gap)
}

func (a *app) footer() string {
	var status string

	switch {
	case a.status.err != nil:
		status = styles.errorText.Render("Error: " + a.status.err.Error())
	case a.status.text != "":
		status = styles.successText.Render(a.status.text)
	}

	current := a.tabs[a.active]

	if current.capturing() {
		return status
	}

	bindings := append(current.keys(), globalKeys.next, globalKeys.refresh, globalKeys.quit)

	return lipgloss.JoinVertical(lipgloss.Left, status, a.help.ShortHelpView(bindings))
}

func (a *app) View() string {
	if a.width == 0 {
		return ""
	}

	body := lipgloss.NewStyle().
		Height(a.bodyHeight()).
		MaxHeight(a.bodyHeight()).
		Padding(0, 1).
		Render(a.tabs[a.active].view())

	return lipgloss.JoinVertical(lipgloss.Left, a.header(), body, a.footer())
}
