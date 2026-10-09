package app

import (
	tea "github.com/charmbracelet/bubbletea"
)

// ---------------------------------------------------------------------------
// Mouse

func (m *Model) handleMouse(msg tea.MouseMsg) tea.Cmd {
	if m.viewMode == viewModeDashboard {
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			if a, id, ok := m.dashboard.Click(msg.X, msg.Y); ok {
				return m.runDashboardAction(a, id)
			}
		}
		return nil
	}
	l := m.layout()
	if m.resizing != dividerNone {
		switch msg.Action {
		case tea.MouseActionMotion:
			m.dragDivider(msg.X)
		case tea.MouseActionRelease:
			m.resizing = dividerNone
		}
		return nil
	}
	// Drags that start in the editor keep going there (text selection).
	if msg.Action == tea.MouseActionMotion || msg.Action == tea.MouseActionRelease {
		if !m.dragging {
			// Hover shows the preview's block handles; a block drag keeps
			// going wherever the mouse goes.
			overPreview := l.PreviewW > 0 && msg.X >= l.PreviewX && msg.Y >= l.BodyY && msg.Y < l.BodyY+l.BodyH
			if !overPreview && !m.preview.Dragging() {
				m.preview.ClearHover()
				return nil
			}
			local := msg
			local.X -= l.PreviewX
			local.Y -= l.BodyY
			var cmd tea.Cmd
			m.preview, cmd = m.preview.Update(local)
			return cmd
		}
		if msg.Action == tea.MouseActionRelease {
			m.dragging = false
		}
		local := msg
		local.X -= l.EditorX
		local.Y -= l.BodyY
		return m.editorPaneUpdate(local)
	}
	if msg.Action != tea.MouseActionPress {
		return nil
	}

	if msg.Y == 0 {
		return m.handleTabClick(msg)
	}
	if msg.Y < l.BodyY || msg.Y >= l.BodyY+l.BodyH {
		return nil
	}

	wheel := msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown
	if !wheel && msg.Button != tea.MouseButtonLeft {
		return nil
	}

	local := msg
	local.Y -= l.BodyY
	var target focus
	switch {
	case msg.X < l.SidebarW:
		target = focusSidebar
	case msg.X >= l.EditorX && msg.X < l.EditorX+l.EditorW:
		target, local.X = focusEditor, msg.X-l.EditorX
	case l.PreviewW > 0 && msg.X >= l.PreviewX:
		target, local.X = focusPreview, msg.X-l.PreviewX
	default: // a divider
		if msg.Button == tea.MouseButtonLeft {
			m.resizing = dividerAt(l, msg.X)
		}
		return nil
	}

	var cmds []tea.Cmd
	if !wheel && m.focus != target {
		cmds = append(cmds, m.focusPane(target))
	}
	var cmd tea.Cmd
	switch target {
	case focusSidebar:
		m.sidebar, cmd = m.sidebar.Update(local)
	case focusEditor:
		m.dragging = msg.Button == tea.MouseButtonLeft
		cmd = m.editorPaneUpdate(local)
	case focusPreview:
		m.preview, cmd = m.preview.Update(local)
	}
	return tea.Batch(append(cmds, cmd)...)
}

// divider names a draggable pane divider.
type divider int

const (
	dividerNone    divider = iota
	dividerSidebar         // between the explorer and the editor
	dividerPreview         // between the editor and the preview
)

// dividerAt returns the divider in screen column x.
func dividerAt(l Layout, x int) divider {
	switch {
	case l.SidebarW > 0 && x == l.SidebarW:
		return dividerSidebar
	case l.PreviewW > 0 && x == l.PreviewX-1:
		return dividerPreview
	}
	return dividerNone
}

// dragDivider moves the divider being dragged to screen column x.
func (m *Model) dragDivider(x int) {
	l := m.layout()
	switch m.resizing {
	case dividerSidebar:
		m.split.SidebarW = x
	case dividerPreview:
		if avail := l.EditorW + l.PreviewW; avail > 0 {
			m.split.EditorFrac = float64(x-l.EditorX) / float64(avail)
		}
	}
	m.updateLayout()
	// Keep the stored position inside the limits the layout applied, so a
	// drag past the edge doesn't need to travel back before it moves again.
	l = m.layout()
	if m.resizing == dividerSidebar && l.SidebarW > 0 {
		m.split.SidebarW = l.SidebarW
	}
	if m.resizing == dividerPreview && l.EditorW+l.PreviewW > 0 {
		m.split.EditorFrac = float64(l.EditorW) / float64(l.EditorW+l.PreviewW)
	}
}

func (m *Model) handleTabClick(msg tea.MouseMsg) tea.Cmd {
	hit := m.tabAt(msg.X)
	switch msg.Button {
	case tea.MouseButtonMiddle:
		if hit.kind == hitTab || hit.kind == hitClose {
			return m.closeBuffer(hit.id, false)
		}
		return nil
	case tea.MouseButtonWheelUp:
		return m.cycleBuffer(-1)
	case tea.MouseButtonWheelDown:
		return m.cycleBuffer(1)
	case tea.MouseButtonLeft:
	default:
		return nil
	}
	switch hit.kind {
	case hitTab:
		if i := m.bufferIndex(hit.id); i >= 0 {
			m.showBuffer(m.buffers[i])
			return m.focusPane(focusEditor)
		}
	case hitClose:
		return m.closeBuffer(hit.id, false)
	case hitNew:
		return m.newDraft(m.sidebar.ContextParentID())
	case hitExplorer:
		return m.focusPane(focusSidebar)
	}
	return nil
}
