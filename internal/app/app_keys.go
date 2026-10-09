package app

import (
	"github.com/jaisuriya-11/tsuzuri/internal/content"
	"github.com/jaisuriya-11/tsuzuri/internal/dashboard"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// ---------------------------------------------------------------------------
// Keyboard

func (m *Model) handleKey(k tea.KeyMsg) tea.Cmd {
	s := k.String()
	if key.Matches(k, m.keys.Quit) {
		return m.requestQuit(false)
	}
	if m.viewMode == viewModeDashboard {
		return m.handleDashboardKey(k)
	}

	sidebarBusy := m.focus == focusSidebar && m.sidebar.IsBusy()
	editorBusy := m.focus == focusEditor && m.content.Mode() != content.ModeNormal && !m.graphActive()

	// VSCode-style shortcuts work in every mode.
	switch {
	case key.Matches(k, m.keys.Save):
		if b := m.activeBuffer(); b != nil {
			return m.saveBuffer(b, nil)
		}
		return nil
	case key.Matches(k, m.keys.NewPage) && !sidebarBusy:
		return m.newDraft(m.sidebar.ContextParentID())
	case key.Matches(k, m.keys.ToggleSidebar):
		return m.toggleSidebar()
	case key.Matches(k, m.keys.Find):
		return m.startFind()
	}

	if !sidebarBusy && !editorBusy {
		if m.leaderPending {
			m.leaderPending = false
			return m.handleLeader(s)
		}
		switch s {
		case " ":
			m.leaderPending = true
			return nil
		case "?":
			m.showKeymap = true
			return nil
		case "tab":
			return m.movePane(1, true)
		case "shift+tab":
			return m.movePane(-1, true)
		case "ctrl+h":
			return m.movePane(-1, false)
		case "ctrl+l":
			return m.movePane(1, false)
		case "\\":
			return m.openFinder()
		case "[":
			return m.cycleBuffer(-1)
		case "]":
			return m.cycleBuffer(1)
		case "esc":
			if m.focus != focusEditor && m.activeBuffer() != nil {
				return m.focusPane(focusEditor)
			}
		}
	}
	m.leaderPending = false

	var cmd tea.Cmd
	switch m.focus {
	case focusSidebar:
		m.sidebar, cmd = m.sidebar.Update(k)
	case focusPreview:
		m.preview, cmd = m.preview.Update(k)
	default:
		if m.graphActive() {
			m.graph, cmd = m.graph.Update(k)
			return cmd
		}
		if m.activeBuffer() == nil && s == "i" {
			return m.newDraft(m.sidebar.ContextParentID())
		}
		m.content, cmd = m.content.Update(k)
		if b := m.activeBuffer(); b != nil {
			m.preview.SetContent(m.content.Value())
			m.refreshModified()
			if m.bufferIndex(graphID) >= 0 {
				m.liveGraph()
			}
		}
	}
	return cmd
}

// handleLeader runs NvChad-style <Space> mappings.
func (m *Model) handleLeader(s string) tea.Cmd {
	switch s {
	case "e":
		return m.focusPane(focusSidebar)
	case "f":
		return m.startFind()
	case "n":
		return m.newDraft(m.sidebar.ContextParentID())
	case "x":
		return m.closeBuffer(m.active, false)
	case "p":
		return m.togglePreview()
	case "tab":
		return m.cycleBuffer(1)
	case "shift+tab":
		return m.cycleBuffer(-1)
	case "t":
		return m.openThemePicker()
	case "g":
		return m.openGraph()
	case "d":
		m.goHome()
	case "h", "?":
		m.showKeymap = true
	case "w":
		if b := m.activeBuffer(); b != nil {
			return m.saveBuffer(b, nil)
		}
	}
	return nil
}

func (m *Model) handleDashboardKey(k tea.KeyMsg) tea.Cmd {
	s := k.String()
	switch s {
	case "n", "ctrl+n":
		return m.newDraft("")
	case "f", "\\", "ctrl+p":
		return m.startFind()
	case "e", "ctrl+b":
		return m.focusPane(focusSidebar)
	case "?":
		m.showKeymap = true
		return nil
	case "t":
		return m.openThemePicker()
	case "g":
		return m.openGraph()
	case "q":
		return m.requestQuit(false)
	case "esc":
		if m.activeBuffer() != nil {
			return m.focusPane(focusEditor)
		}
		return nil
	case "enter":
		a, id := m.dashboard.SelectedAction()
		return m.runDashboardAction(a, id)
	}
	if len(s) == 1 && s[0] >= '1' && s[0] <= '9' {
		if i := int(s[0] - '1'); i < len(m.dashboard.RecentPages()) {
			return m.openFile(m.dashboard.RecentPages()[i].ID)
		}
		return nil
	}
	m.dashboard, _ = m.dashboard.Update(k)
	return nil
}

func (m *Model) runDashboardAction(a dashboard.Action, id string) tea.Cmd {
	switch a {
	case dashboard.ActionNewPage:
		return m.newDraft("")
	case dashboard.ActionFind:
		return m.startFind()
	case dashboard.ActionBrowse:
		return m.focusPane(focusSidebar)
	case dashboard.ActionKeymap:
		m.showKeymap = true
	case dashboard.ActionQuit:
		return m.requestQuit(false)
	case dashboard.ActionOpenRecent:
		return m.openFile(id)
	case dashboard.ActionThemes:
		return m.openThemePicker()
	}
	return nil
}
