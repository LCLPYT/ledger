package tui

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/lclpyt/ledger/internal/admin"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

const (
	timeFormat   = "2006-01-02 15:04"
	maxFormWidth = 80
)

var (
	accent = lipgloss.AdaptiveColor{Light: "#5A56E0", Dark: "#7571F9"}
	subtle = lipgloss.AdaptiveColor{Light: "#9B9B9B", Dark: "#5C5C5C"}

	tabBorder = lipgloss.Border{Bottom: "─", BottomLeft: "─", BottomRight: "─"}
)

var styles = struct {
	tab, activeTab, tabGap                  lipgloss.Style
	errorText, successText, warnText, muted lipgloss.Style
	heading, label, detail, secret          lipgloss.Style
}{
	tab:         lipgloss.NewStyle().Padding(0, 2).Foreground(subtle).Border(tabBorder, false, false, true).BorderForeground(subtle),
	activeTab:   lipgloss.NewStyle().Padding(0, 2).Bold(true).Foreground(accent).Border(tabBorder, false, false, true).BorderForeground(accent),
	tabGap:      lipgloss.NewStyle().Border(tabBorder, false, false, true).BorderForeground(subtle),
	errorText:   lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
	successText: lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
	warnText:    lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
	muted:       lipgloss.NewStyle().Foreground(subtle),
	heading:     lipgloss.NewStyle().Bold(true).Foreground(accent),
	label:       lipgloss.NewStyle().Bold(true).Width(13),
	detail:      lipgloss.NewStyle().MarginTop(1).Padding(0, 1).Border(lipgloss.RoundedBorder()).BorderForeground(subtle),
	secret:      lipgloss.NewStyle().Bold(true).Padding(0, 1).Border(lipgloss.RoundedBorder()).BorderForeground(accent),
}

type env struct {
	ctx context.Context
	svc *admin.Service
}

// mutate runs fn in the background and reports success, which reloads the active tab.
func (e env) mutate(success string, fn func(context.Context) error) tea.Cmd {
	return func() tea.Msg {
		if err := fn(e.ctx); err != nil {
			return statusMsg{err: err}
		}

		return changedMsg{text: success}
	}
}

// dialog hosts a form on top of a tab.
type dialog struct {
	form     *huh.Form
	onSubmit func() tea.Cmd
}

func (d *dialog) open(width int, onSubmit func() tea.Cmd, groups ...*huh.Group) tea.Cmd {
	keymap := huh.NewDefaultKeyMap()
	keymap.Quit = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel"))

	d.form = huh.NewForm(groups...).
		WithKeyMap(keymap).
		WithWidth(min(width, maxFormWidth)).
		WithShowHelp(true)
	d.onSubmit = onSubmit

	return d.form.Init()
}

func (d *dialog) active() bool {
	return d.form != nil
}

func (d *dialog) update(msg tea.Msg) tea.Cmd {
	model, cmd := d.form.Update(msg)
	d.form = model.(*huh.Form)

	switch d.form.State {
	case huh.StateCompleted:
		submit := d.onSubmit
		d.form, d.onSubmit = nil, nil

		return tea.Batch(cmd, submit())
	case huh.StateAborted:
		d.form, d.onSubmit = nil, nil
	case huh.StateNormal:
	}

	return cmd
}

func (d *dialog) view() string {
	return d.form.View()
}

func confirm(title, description, affirmative string, value *bool) *huh.Group {
	return huh.NewGroup(
		huh.NewConfirm().
			Title(title).
			Description(description).
			Affirmative(affirmative).
			Negative("Cancel").
			Value(value),
	)
}

type column struct {
	title string
	min   int
	flex  int
}

func newTable() table.Model {
	keymap := table.KeyMap{
		LineUp:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		LineDown:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		PageUp:       key.NewBinding(key.WithKeys("pgup")),
		PageDown:     key.NewBinding(key.WithKeys("pgdown")),
		HalfPageUp:   key.NewBinding(key.WithKeys("ctrl+u")),
		HalfPageDown: key.NewBinding(key.WithKeys("ctrl+d")),
		GotoTop:      key.NewBinding(key.WithKeys("home", "g")),
		GotoBottom:   key.NewBinding(key.WithKeys("end", "G")),
	}

	s := table.DefaultStyles()

	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(subtle).
		BorderBottom(true).
		Bold(true)

	s.Selected = s.Selected.Foreground(lipgloss.Color("229")).Background(accent).Bold(false)

	return table.New(table.WithFocused(true), table.WithKeyMap(keymap), table.WithStyles(s))
}

// fitColumns hands every column its minimum width and splits the rest by flex.
func fitColumns(columns []column, width int) []table.Column {
	free := width - 2*len(columns)
	flex := 0

	for _, c := range columns {
		free -= c.min
		flex += c.flex
	}

	result := make([]table.Column, len(columns))

	for i, c := range columns {
		w := c.min

		if free > 0 && flex > 0 {
			w += free * c.flex / flex
		}

		result[i] = table.Column{Title: c.title, Width: w}
	}

	return result
}

func layoutTable(t *table.Model, columns []column, width int) {
	t.SetColumns(fitColumns(columns, width))
	t.SetWidth(width)
}

// withDetail renders the table above the detail pane, giving the table the height the pane leaves.
func withDetail(t *table.Model, height int, detail string) string {
	t.SetHeight(max(height-lipgloss.Height(detail), 3))

	return lipgloss.JoinVertical(lipgloss.Left, t.View(), detail)
}

// setRows keeps a row selected, which the table drops once it held no rows.
func setRows(t *table.Model, rows []table.Row) {
	t.SetRows(rows)

	if t.Cursor() < 0 && len(rows) > 0 {
		t.SetCursor(0)
	}
}

func renderDetail(width int, lines ...string) string {
	return styles.detail.Width(max(width-2, 10)).Render(strings.Join(lines, "\n"))
}

func field(label, value string) string {
	return styles.label.Render(label) + value
}

func listOrNone(items []string, sep string) string {
	if len(items) == 0 {
		return styles.muted.Render("none")
	}

	return strings.Join(items, sep)
}

func formatTime(t time.Time) string {
	return t.Local().Format(timeFormat)
}

func notBlank(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("must not be empty")
	}

	return nil
}

func emptyHint(what string) string {
	return styles.muted.Render("No " + what + " yet, press n to create one.")
}
