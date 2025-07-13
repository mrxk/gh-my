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

type PRTable struct {
	table.Model
	reloadCommand  tea.Cmd
	needReload     bool
	loading        bool
	wideView       bool
	err            error
	defaultColumns []Column
	wideColumns    []Column
	currentResults *page
	urls           []string
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

func New(reloadCommand tea.Cmd, defaultColumns []Column, wideColumns []Column) *PRTable {
	if len(defaultColumns) == 0 {
		defaultColumns = defaultDefaultColumns
	}
	if len(wideColumns) == 0 {
		wideColumns = defaultWideColumns
	}
	return &PRTable{
		Model: table.New(
			table.WithKeyMap(keyMap),
			table.WithColumns(asTableColumns(defaultColumns)),
		),
		needReload:     true,
		reloadCommand:  reloadCommand,
		defaultColumns: defaultColumns,
		wideColumns:    wideColumns,
		urls:           []string{},
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
	t.updateModel(t.currentResults)
	t.Model.UpdateViewport()
}

// handleSearchResults updates the model with the given search results.
func (t *PRTable) handleSearchResults(searchResults result.Result[github.PullRequestSearchResults]) {
	t.loading = false
	if searchResults.IsError() {
		t.err = searchResults.Error()
		return
	}
	page := result.MapNoError(asPage, searchResults)
	page = result.MapNoError(t.updateModel, page)
	t.currentResults = page.MustGet()
}

type page struct {
	columnWidths map[Column]int
	rows         []map[Column]string
}

func asPage(prs github.PullRequestSearchResults) *page {
	p := &page{
		columnWidths: map[Column]int{},
		rows:         []map[Column]string{},
	}
	for _, issue := range prs.Data.Search.Edges {
		row := map[Column]string{
			checksColumn:     checkEmoji(issue.Node.StatusCheckRollup.State),
			mergeableColumn:  mergeableEmoji(issue.Node.Mergeable, issue.Node.MergeStateStatus),
			approvedColumn:   reviewEmoji(issue.Node.ReviewDecision),
			draftColumn:      draftEmoji(issue.Node.IsDraft),
			titleColumn:      issue.Node.Title,
			urlColumn:        issue.Node.URL,
			authorColumn:     issue.Node.Author.Login,
			repositoryColumn: shortenRepository(issue.Node.Repository.NameWithOwner),
			changeColumn:     fmt.Sprintf("%4s (+%d/-%d)", fmt.Sprintf("%d", issue.Node.ChangedFiles), issue.Node.Additions, issue.Node.Deletions),
			stateColumn:      stateEmoji(issue.Node.State),
			commentsColumn:   fmt.Sprintf("%d", issue.Node.TotalCommentsCount),
			updatedAtColumn:  timeAgo(issue.Node.UpdatedAt),
		}
		for columnIndex, columnValue := range row {
			p.columnWidths[columnIndex] = max(p.columnWidths[columnIndex], len(columnValue))
		}
		p.rows = append(p.rows, row)
	}
	return p
}

func (t *PRTable) updateModel(prs *page) *page {
	t.Model.SetRows(nil)
	t.setColumns(prs)
	t.setRows(prs)
	return prs
}

func (t *PRTable) setRows(prs *page) *page {
	if prs == nil {
		return prs
	}
	selectedColumns := t.defaultColumns
	if t.wideView {
		selectedColumns = t.wideColumns
	}
	t.urls = make([]string, 0, len(prs.rows))
	rows := make([]table.Row, 0, len(prs.rows))
	for _, inputRow := range prs.rows {
		t.urls = append(t.urls, inputRow[urlColumn])
		row := make([]string, 0, len(selectedColumns))
		for _, col := range selectedColumns {
			row = append(row, inputRow[col])
		}
		rows = append(rows, row)
	}
	t.Model.SetRows(rows)
	return prs
}

func (t *PRTable) setColumns(prs *page) *page {
	if prs == nil {
		return prs
	}
	selectedColumns := t.defaultColumns
	if t.wideView {
		selectedColumns = t.wideColumns
	}
	columns := make([]table.Column, 0, len(selectedColumns))
	for _, col := range selectedColumns {
		width := min(columnIndex_maxWidth[col], prs.columnWidths[col])
		width = max(columnIndex_minWidth[col], width)
		columns = append(columns, table.Column{
			Title: columnIndex_title[col],
			Width: width,
		})
	}
	t.Model.SetColumns(columns)
	return prs
}

func (t *PRTable) GetSelectedPRURL() string {
	row := t.Cursor()
	if row >= 0 && row < len(t.urls) {
		return t.urls[row]
	}
	return ""
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

func shortenRepository(value string) string {
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

func asTableColumns(src []Column) []table.Column {
	dst := make([]table.Column, 0, len(src))
	for _, col := range src {
		title := columnIndex_title[col]
		width := columnIndex_minWidth[col]
		dst = append(dst, table.Column{
			Title: title,
			Width: width,
		})
	}
	return dst
}
