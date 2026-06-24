package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type Worktree interface {
	GetRepoName() string
	GetBranch() string
	GetPath() string
	GetSymlinkStatus() string
}

type Actions interface {
	Delete(repoName, branch string) error
	Push(repoName, branch string) error
	Create(repoName, branch string) (Worktree, error) // returns new item for list append
	OpenTmux(name, dir string) error
	Symlink(repoName, branch string) error
	RepoNames() []string // for repo selection when no current repo
}

type state int

const (
	stateList state = iota
	stateConfirmDelete
	stateCreateRepo  // select repo when currentRepo unknown
	stateCreateInput // type branch name
)

type Model struct {
	items       []Worktree
	cursor      int
	currentRepo string
	state       state
	input       string
	createRepo  string
	repoList    []string
	repoCursor  int
	actions     Actions
	err         string
	status      string
}

func New(items []Worktree, currentRepo string, actions Actions) Model {
	return Model{
		items:       sortItems(items, currentRepo),
		currentRepo: currentRepo,
		actions:     actions,
		repoList:    actions.RepoNames(),
	}
}

func sortItems(items []Worktree, currentRepo string) []Worktree {
	if currentRepo == "" {
		return items
	}
	var mine, others []Worktree
	for _, item := range items {
		if item.GetRepoName() == currentRepo {
			mine = append(mine, item)
		} else {
			others = append(others, item)
		}
	}
	return append(mine, others...)
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.state {
		case stateList:
			return m.updateList(msg)
		case stateConfirmDelete:
			return m.updateConfirm(msg)
		case stateCreateRepo:
			return m.updateRepoSelect(msg)
		case stateCreateInput:
			return m.updateCreate(msg)
		}
	}
	return m, nil
}

func (m Model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
	case "enter":
		if len(m.items) > 0 {
			wt := m.items[m.cursor]
			if err := m.actions.OpenTmux(wt.GetRepoName()+"-"+wt.GetBranch(), wt.GetPath()); err != nil {
				m.err = err.Error()
			} else {
				return m, tea.Quit
			}
		}
	case "d":
		if len(m.items) > 0 {
			m.state = stateConfirmDelete
			m.err = ""
		}
	case "p":
		if len(m.items) > 0 {
			wt := m.items[m.cursor]
			if err := m.actions.Push(wt.GetRepoName(), wt.GetBranch()); err != nil {
				m.err = err.Error()
			} else {
				m.status = "pushed " + wt.GetBranch()
				m.err = ""
			}
		}
	case "s":
		if len(m.items) > 0 {
			wt := m.items[m.cursor]
			if err := m.actions.Symlink(wt.GetRepoName(), wt.GetBranch()); err != nil {
				m.err = err.Error()
			} else {
				m.status = "symlink → " + wt.GetBranch()
				m.err = ""
			}
		}
	case "c":
		m.input = ""
		m.err = ""
		if m.currentRepo != "" {
			m.createRepo = m.currentRepo
			m.state = stateCreateInput
		} else if len(m.repoList) == 1 {
			m.createRepo = m.repoList[0]
			m.state = stateCreateInput
		} else {
			m.repoCursor = 0
			m.state = stateCreateRepo
		}
	}
	return m, nil
}

func (m Model) updateRepoSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.repoCursor > 0 {
			m.repoCursor--
		}
	case "down", "j":
		if m.repoCursor < len(m.repoList)-1 {
			m.repoCursor++
		}
	case "enter":
		if len(m.repoList) > 0 {
			m.createRepo = m.repoList[m.repoCursor]
			m.state = stateCreateInput
		}
	case "esc":
		m.state = stateList
	}
	return m, nil
}

func (m Model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		wt := m.items[m.cursor]
		if err := m.actions.Delete(wt.GetRepoName(), wt.GetBranch()); err != nil {
			m.err = err.Error()
		} else {
			m.items = append(m.items[:m.cursor], m.items[m.cursor+1:]...)
			if m.cursor >= len(m.items) && m.cursor > 0 {
				m.cursor--
			}
			m.status = "deleted"
			m.err = ""
		}
		m.state = stateList
	case "n", "N", "esc":
		m.state = stateList
	}
	return m, nil
}

func (m Model) updateCreate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if m.input != "" {
			wt, err := m.actions.Create(m.createRepo, m.input)
			if err != nil {
				m.err = err.Error()
			} else {
				m.items = append(m.items, wt)
				m.cursor = len(m.items) - 1
				m.status = "created " + m.input
				m.err = ""
			}
		}
		m.state = stateList
		m.input = ""
	case "esc":
		m.state = stateList
		m.input = ""
	case "backspace":
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
	default:
		if len(msg.String()) == 1 {
			m.input += msg.String()
		}
	}
	return m, nil
}

func (m Model) View() string {
	var sb strings.Builder
	sb.WriteString("  gwt — worktree manager\n")
	sb.WriteString("  ─────────────────────────────\n")

	switch m.state {
	case stateCreateRepo:
		sb.WriteString("  select repo:\n")
		for i, name := range m.repoList {
			cursor := "  "
			if i == m.repoCursor {
				cursor = "▶ "
			}
			sb.WriteString(fmt.Sprintf("%s%s\n", cursor, name))
		}
		sb.WriteString("\n  ↑/↓:select  enter:confirm  esc:cancel\n")
	default:
		if len(m.items) == 0 {
			sb.WriteString("  no worktrees. press c to create.\n")
		} else {
			for i, wt := range m.items {
				cursor := "  "
				if i == m.cursor {
					cursor = "▶ "
				}
				if i > 0 && m.currentRepo != "" && wt.GetRepoName() != m.currentRepo &&
					m.items[i-1].GetRepoName() == m.currentRepo {
					sb.WriteString("  ·····\n")
				}
				symIndicator := ""
				if s := wt.GetSymlinkStatus(); s != "" && s == wt.GetPath() {
					symIndicator = " ⇒"
				}
				sb.WriteString(fmt.Sprintf("%s%-20s  %s%s\n",
					cursor,
					wt.GetRepoName()+"/"+wt.GetBranch(),
					shortenPath(wt.GetPath()),
					symIndicator,
				))
			}
		}

		sb.WriteString("\n")
		switch m.state {
		case stateConfirmDelete:
			if len(m.items) > 0 {
				wt := m.items[m.cursor]
				sb.WriteString(fmt.Sprintf("  delete %s/%s? [y/n] ", wt.GetRepoName(), wt.GetBranch()))
			}
		case stateCreateInput:
			sb.WriteString(fmt.Sprintf("  branch [%s]: %s█\n", m.createRepo, m.input))
		default:
			sb.WriteString("  enter:open  c:create  d:delete  p:push  s:symlink  q:quit\n")
		}
	}

	if m.err != "" {
		sb.WriteString("  err: " + m.err + "\n")
	} else if m.status != "" {
		sb.WriteString("  " + m.status + "\n")
	}
	return sb.String()
}

func shortenPath(p string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	if strings.HasPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return filepath.Base(p)
}
