package prtable

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cli/go-gh/pkg/text"
	"github.com/mrxk/gh-my/internal/github"
	"github.com/sassoftware/sas-ggdk/pkg/result"
)

// columnIndex identifies colums in the output table
type columnIndex int

const (
	checksColumn columnIndex = iota
	mergeableColumn
	approvedColumn
	titleColumn
	urlColumn
	authorColumn
	repositoryColumn
	changeColumn
	draftColumn
	stateColumn
	commentsColumn
	updatedAtColumn
)

var columns = []table.Column{
	{Title: "C", Width: 2},
	{Title: "M", Width: 2},
	{Title: "A", Width: 2},
	{Title: "Title", Width: 5},
	{Title: "Url", Width: 0},
	{Title: "Author", Width: 6},
	{Title: "Repository", Width: 10},
	{Title: "Change", Width: 6},
	{Title: "Draft", Width: 0},
	{Title: "State", Width: 0},
	{Title: "Comments", Width: 0},
	{Title: "UpdatedAt", Width: 10},
}

var defaultColumnWidths = []int{2, 2, 2, 5, 3, 6, 10, 6, 5, 5, 8, 9}

type PRTable struct {
	table.Model
	prs           github.PullRequestSearchResults
	reloadCommand tea.Cmd
	needReload    bool
	loading       bool
	wideView      bool
	urlWidth      int
	draftWidth    int
	stateWidth    int
	commentsWidth int
	err           error
}

var keyMap = table.KeyMap{
	LineUp: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	LineDown: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	PageUp: key.NewBinding(
		key.WithKeys("pgup"),
		key.WithHelp("pgup", "page up"),
	),
	PageDown: key.NewBinding(
		key.WithKeys("pgdown", " "),
		key.WithHelp("pgdn", "page down"),
	),
	GotoTop: key.NewBinding(
		key.WithKeys("home", "g"),
		key.WithHelp("g/home", "go to start"),
	),
	GotoBottom: key.NewBinding(
		key.WithKeys("end", "G"),
		key.WithHelp("G/end", "go to end"),
	),
}

func New(reloadCommand tea.Cmd) *PRTable {
	return &PRTable{
		Model: table.New(
			table.WithKeyMap(keyMap),
			table.WithColumns(duplicate(columns)),
		),
		needReload:    true,
		reloadCommand: reloadCommand,
		urlWidth:      3,
		commentsWidth: 8,
		stateWidth:    5,
		draftWidth:    5,
	}
}

func (t *PRTable) Status() string {
	if t.needReload || t.loading {
		return "Loading ..."
	}
	if t.err != nil {
		return t.err.Error()
	}
	return fmt.Sprintf("%d issues", len(t.Model.Rows()))
}

// View implements tea.Model.
func (t *PRTable) View() string {
	return t.Model.View()
}

// Update implements tea.Model.
func (t *PRTable) Update(msg tea.Msg) (*PRTable, tea.Cmd) {
	if !t.Model.Focused() {
		return t.unfocusedUpdate(msg)
	}
	return t.focusedUpdate(msg)
}

// unfocusedUpdate handles updates to the model when not focused. This is for
// things like remembering that a reload is necessary the next time this model
// gains focus.
func (t *PRTable) unfocusedUpdate(msg tea.Msg) (*PRTable, tea.Cmd) {
	switch typedMsg := msg.(type) {
	case tea.KeyMsg:
		switch typedMsg.String() {
		case "c":
			fallthrough
		case "d":
			fallthrough
		case "r":
			t.needReload = true
			t.Model.SetRows(nil)
		case "u":
			fallthrough
		case "w":
			t.toggleWideView()
		}
	}
	return t, nil
}

// focusedUpdate handles updates to the model when focused.
func (t *PRTable) focusedUpdate(msg tea.Msg) (*PRTable, tea.Cmd) {
	cmds := []tea.Cmd{}
	switch typedMsg := msg.(type) {
	case result.Result[github.PullRequestSearchResults]:
		t.handleSearchResults(typedMsg)
	case tea.KeyMsg:
		switch typedMsg.String() {
		case "r":
			t.loading = true
			cmds = append(cmds, t.reloadCommand)
		case "u":
			fallthrough
		case "w":
			t.toggleWideView()
		}
	}
	newTable, tableCmd := t.Model.Update(msg)
	t.Model = newTable
	cmds = append(cmds, tableCmd)
	return t, tea.Batch(cmds...)
}

// Focus sets the focus on this model. If a reload is needed then returns a
// reload command.
func (t *PRTable) Focus() tea.Cmd {
	t.Model.Focus()
	if t.needReload {
		t.needReload = false
		t.loading = true
		return t.reloadCommand
	}
	return nil
}

func (t *PRTable) toggleWideView() {
	t.wideView = !t.wideView
	cols := t.Model.Columns()
	if len(cols) == 0 {
		return
	}
	if t.wideView {
		t.Model.Columns()[urlColumn].Width = t.urlWidth
		t.Model.Columns()[draftColumn].Width = t.draftWidth
		t.Model.Columns()[stateColumn].Width = t.stateWidth
		t.Model.Columns()[commentsColumn].Width = t.commentsWidth
	} else {
		t.Model.Columns()[urlColumn].Width = 0
		t.Model.Columns()[draftColumn].Width = 0
		t.Model.Columns()[stateColumn].Width = 0
		t.Model.Columns()[commentsColumn].Width = 0
	}
	t.Model.UpdateViewport()
}

// handleSearchResults updates the model with the given search results.
func (t *PRTable) handleSearchResults(searchResults result.Result[github.PullRequestSearchResults]) {
	t.loading = false
	if searchResults.IsError() {
		t.err = searchResults.Error()
		return
	}
	columnWidths := duplicate(defaultColumnWidths)
	t.prs = searchResults.MustGet()
	rows := make([]table.Row, 0, t.prs.Data.Search.IssueCount)
	for _, issue := range t.prs.Data.Search.Edges {
		row := []string{
			checkEmoji(issue.Node.StatusCheckRollup.State),
			mergeableEmoji(issue.Node.Mergeable, issue.Node.MergeStateStatus),
			reviewEmoji(issue.Node.ReviewDecision),
			issue.Node.Title,
			issue.Node.URL,
			issue.Node.Author.Login,
			t.shortenRepository(issue.Node.Repository.NameWithOwner),
			fmt.Sprintf("%4s (+%d/-%d)", fmt.Sprintf("%d", issue.Node.ChangedFiles), issue.Node.Additions, issue.Node.Deletions),
			draftEmoji(issue.Node.IsDraft),
			stateEmoji(issue.Node.State),
			fmt.Sprintf("%d", issue.Node.TotalCommentsCount),
			timeAgo(issue.Node.UpdatedAt),
		}
		for i, columnValue := range row {
			columnWidths[i] = max(columnWidths[i], len(columnValue))
		}
		rows = append(rows, row)
	}
	t.Model.SetRows(rows)
	cols := t.Model.Columns()
	for i, columnWidth := range columnWidths {
		switch columnIndex(i) {
		case checksColumn:
			fallthrough
		case mergeableColumn:
			fallthrough
		case approvedColumn:
			continue
		case urlColumn:
			t.urlWidth = columnWidth
			if !t.wideView {
				continue
			}
		case draftColumn:
			t.draftWidth = columnWidth
			if !t.wideView {
				continue
			}
		case stateColumn:
			t.stateWidth = columnWidth
			if !t.wideView {
				continue
			}
		case commentsColumn:
			t.commentsWidth = columnWidth
			if !t.wideView {
				continue
			}
		}
		cols[i].Width = columnWidth
	}
	t.Model.SetColumns(cols)
}

func (t *PRTable) GetSelectedPRURL() string {
	row := t.SelectedRow()
	return row[urlColumn]
}

func checkEmoji(value string) string {
	switch value {
	case "SUCCESS":
		return "✅"
	case "FAILURE":
		return "❌"
	case "PENDING":
		return "⏳"
	default:
		return " "
	}
}

func reviewEmoji(value string) string {
	switch value {
	case "APPROVED":
		return "✅"
	default:
		return " "
	}
}

func draftEmoji(value bool) string {
	if value {
		return "📝"
	}
	return " "

}

func mergeableEmoji(mergeable, status string) string {
	switch mergeable {
	case "CONFLICTING":
		return "❌"
	case "MERGEABLE":
		if status == "BEHIND" {
			return "⬆️"
		}
		return "✅"
	default:
		return " "
	}
}

func stateEmoji(value string) string {
	switch value {
	case "MERGED":
		return "🚀"
	case "CLOSED":
		return "🗑️"
	default:
		return " "
	}
}

func (t *PRTable) shortenRepository(value string) string {
	parts := strings.Split(value, "/")
	if len(parts) == 1 {
		return parts[0]
	}
	return strings.Join(parts[1:], "/")
}

func timeAgo(timeSpec string) string {
	if timeSpec == "" {
		return ""
	}
	timeAgo, err := time.Parse(time.RFC3339, timeSpec)
	if err != nil {
		panic(err)
	}
	ago := time.Since(timeAgo)
	if ago < time.Minute {
		return "just now"
	}
	if ago < time.Hour {
		return text.Pluralize(int(ago.Minutes()), "minute") + " ago"
	}
	if ago < 24*time.Hour {
		return text.Pluralize(int(ago.Hours()), "hour") + " ago"
	}
	if ago < 30*24*time.Hour {
		return text.Pluralize(int(ago.Hours())/24, "day") + " ago"
	}
	if ago < 365*24*time.Hour {
		return text.Pluralize(int(ago.Hours())/24/30, "month") + " ago"
	}
	return text.Pluralize(int(ago.Hours()/24/365), "year") + " ago"
}

func duplicate[T any](src []T) []T {
	dst := make([]T, len(src))
	copy(dst, src)
	return dst
}
