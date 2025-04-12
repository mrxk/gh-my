package model

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/soft-serve/pkg/ui/common"
	"github.com/charmbracelet/soft-serve/pkg/ui/components/tabs"
	"github.com/mrxk/gh-my/internal/github"
	"github.com/mrxk/gh-my/internal/prtable"
)

// Ensure that Model implements tea.Model.
var _ tea.Model = (*Model)(nil)

// selectedWindowIndex indicates which window has focus.
type TabIndex int

// Possible selected window indexes.
const (
	MyPRsTab TabIndex = iota
	MyRequestsTab
	AllPRsTab
)

// Update the model with search results
type SearchResultsMsg struct {
	SearchResults github.SearchResults
}

type Model struct {
	commands      chan<- github.Command
	selectedTab   TabIndex
	topTabs       *tabs.Tabs
	myPRs         *prtable.PRTable
	myRequests    *prtable.PRTable
	allPRs        *prtable.PRTable
	height        int
	width         int
	error         string
	includeClosed bool
	includeDrafts bool
	prListUpdated time.Time
	interval      time.Duration
}

type Options struct {
	Context       context.Context
	Commands      chan<- github.Command
	IncludeClosed bool
	IncludeDrafts bool
	StartTab      TabIndex
	Interval      time.Duration
}

func New(opts Options) *Model {
	m := &Model{}
	m.commands = opts.Commands
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.SetSpacing(0) // compact lists
	m.topTabs = tabs.New(common.NewCommon(opts.Context, lipgloss.DefaultRenderer(), 0, 0), []string{"My PRs", "My Requests", "All PRs"})
	m.myPRs = prtable.New(m.fetchMyPullRequests)
	m.myRequests = prtable.New(m.fetchMyRequests)
	m.allPRs = prtable.New(m.fetchAllPullRequets)
	m.includeClosed = opts.IncludeClosed
	m.includeDrafts = opts.IncludeDrafts
	m.selectedTab = opts.StartTab
	m.interval = opts.Interval
	return m
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	var cmd tea.Cmd
	switch m.selectedTab {
	case MyPRsTab:
		cmd = m.myPRs.Focus()
	case MyRequestsTab:
		cmd = m.myRequests.Focus()
	case AllPRsTab:
		cmd = m.allPRs.Focus()
	}
	return tea.Batch(tea.SetWindowTitle("Github Pull Requests"), cmd, tabs.SelectTabCmd(int(m.selectedTab)))

}

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := make([]tea.Cmd, 0, 4)
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg)
	case tea.KeyMsg:
		newModel, cmd, handled := m.handleGlobalKey(msg)
		if handled {
			return newModel, cmd
		}
		cmds = append(cmds, cmd)
	case SearchResultsMsg:
		return m.handleSearchResults(msg)
	case tabs.ActiveTabMsg:
		cmd = m.activateTab(TabIndex(msg))
		return m, cmd
	}
	var newTabs tea.Model
	newTabs, cmd = m.topTabs.Update(msg)
	cmds = append(cmds, cmd)
	m.topTabs = newTabs.(*tabs.Tabs)

	m.myPRs, cmd = m.myPRs.Update(msg)
	cmds = append(cmds, cmd)

	m.myRequests, cmd = m.myRequests.Update(msg)
	cmds = append(cmds, cmd)

	m.allPRs, cmd = m.allPRs.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *Model) activateTab(idx TabIndex) tea.Cmd {
	var cmd tea.Cmd
	m.selectedTab = idx
	switch m.selectedTab {
	case MyPRsTab:
		cmd = m.myPRs.Focus()
		m.myRequests.Blur()
		m.allPRs.Blur()
	case MyRequestsTab:
		m.myPRs.Blur()
		cmd = m.myRequests.Focus()
		m.allPRs.Blur()
	case AllPRsTab:
		m.myPRs.Blur()
		m.myRequests.Blur()
		cmd = m.allPRs.Focus()
	}
	return cmd
}

// View implements tea.Model.
func (m *Model) View() string {
	border := lipgloss.NewStyle().Border(lipgloss.NormalBorder(), true, false, true, false).BorderForeground(lipgloss.Color("#6CB0D2"))
	var tableView string
	var footerStatus string
	switch m.selectedTab {
	case MyPRsTab:
		tableView = m.myPRs.View()
		footerStatus = m.myPRs.Status()
	case MyRequestsTab:
		tableView = m.myRequests.View()
		footerStatus = m.myRequests.Status()
	case AllPRsTab:
		tableView = m.allPRs.View()
		footerStatus = m.allPRs.Status()
	}
	footer := m.footerView(footerStatus)
	return strings.Join(
		[]string{
			lipgloss.JoinVertical(lipgloss.Top,
				lipgloss.JoinVertical(lipgloss.Top,
					m.topTabs.View(),
					border.Render(tableView),
				),
				footer,
			),
		}, "\n")
}

func (m *Model) footerView(status string) string {
	footer := status
	if m.includeClosed {
		footer += " [including closed]"
	}
	if m.includeDrafts {
		footer += " [including drafts]"
	}
	footer += " " + m.error
	timeFooter := m.prListUpdated.Format("03:04:05 PM")
	if m.interval != 0 {
		timeFooter += " (🔄" + m.interval.String() + ")"
	}
	infoWidth := m.myPRs.Width() - len(timeFooter)
	footerFormat := fmt.Sprintf("%%-%ds%%s", infoWidth)
	return fmt.Sprintf(footerFormat, footer, timeFooter)
}

func (m *Model) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.height = msg.Height
	m.width = msg.Width
	m.topTabs.SetSize(m.width, 2)
	m.myPRs.SetWidth(m.width - 2)
	m.myPRs.SetHeight(m.height - 4)
	m.myRequests.SetWidth(m.width - 2)
	m.myRequests.SetHeight(m.height - 4)
	m.allPRs.SetWidth(m.width - 2)
	m.allPRs.SetHeight(m.height - 4)
	return m, nil
}

func (m *Model) handleGlobalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	var cmd tea.Cmd
	var handled bool
	switch msg.String() {
	case "enter":
		m.openSelectedPullRequest()
		handled = true
	case "esc":
		fallthrough
	case "q":
		cmd = tea.Quit
		handled = true
	case "d":
		cmd = tea.Batch(m.toggleDrafts, m.reload)
		handled = false // let the other components see this message
	case "c":
		cmd = tea.Batch(m.toggleClosed, m.reload)
		handled = false // let the other components see this message
	}
	return m, cmd, handled
}

func (m *Model) handleSearchResults(msg SearchResultsMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.prListUpdated = time.Now()
	m.error = "" // clear any error
	switch typedResults := msg.SearchResults.(type) {
	case *github.MyPRs:
		m.myPRs, cmd = m.myPRs.Update(typedResults.SearchResults)
	case *github.MyRequets:
		m.myRequests, cmd = m.myRequests.Update(typedResults.SearchResults)
	case *github.AllPRs:
		m.allPRs, cmd = m.allPRs.Update(typedResults.SearchResults)
	}
	return m, cmd
}

func (m *Model) openSelectedPullRequest() {
	var url string
	switch m.selectedTab {
	case MyPRsTab:
		url = m.myPRs.GetSelectedPRURL()
	case MyRequestsTab:
		url = m.myRequests.GetSelectedPRURL()
	case AllPRsTab:
		url = m.allPRs.GetSelectedPRURL()
	}
	cmd := exec.Command("open", url)
	err := cmd.Start()
	if err != nil {
		m.error = err.Error()
	}
}

func (m *Model) fetchMyPullRequests() tea.Msg {
	m.commands <- github.FetchMyPRs{}
	return nil
}

func (m *Model) fetchMyRequests() tea.Msg {
	m.commands <- github.FetchMyRequests{}
	return nil
}

func (m *Model) fetchAllPullRequets() tea.Msg {
	m.commands <- github.FetchAllPRs{}
	return nil
}

func (m *Model) toggleDrafts() tea.Msg {
	m.includeDrafts = !m.includeDrafts
	m.commands <- github.IncludeDrafts(m.includeDrafts)
	return nil
}

func (m *Model) toggleClosed() tea.Msg {
	m.includeClosed = !m.includeClosed
	m.commands <- github.IncludeClosed(m.includeClosed)
	return nil
}

func (m *Model) reload() tea.Msg {
	key := tea.Key{
		Type:  tea.KeyRunes,
		Runes: []rune{'r'},
	}
	return tea.KeyMsg(key)
}
