package app

import (
	"fmt"

	"github.com/jaisuriya-11/tsuzuri/internal/ui"

	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// View

// View composes child component views into the complete terminal layout.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "loading…"
	}
	if m.width < MinTermWidth || m.height < MinTermHeight {
		return lipgloss.NewStyle().
			Width(m.width).Height(m.height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(m.theme.Yellow).
			Render(fmt.Sprintf("Terminal too small (%dx%d)\nResize to at least %dx%d", m.width, m.height, MinTermWidth, MinTermHeight))
	}

	var screen string
	if m.viewMode == viewModeDashboard {
		screen = lipgloss.JoinVertical(lipgloss.Left, m.dashboard.View(), m.bottomBar())
	} else {
		l := m.layout()
		div := func(d divider) string {
			c := m.theme.Line
			if m.resizing == d {
				c = m.theme.Blue
			}
			return ui.Column("│", l.BodyH, lipgloss.NewStyle().Foreground(c))
		}
		var cols []string
		if l.SidebarW > 0 {
			cols = append(cols, m.sidebar.View(), div(dividerSidebar))
		}
		if m.graphActive() {
			cols = append(cols, m.graph.View())
		} else {
			cols = append(cols, m.content.View())
		}
		if l.PreviewW > 0 {
			cols = append(cols, div(dividerPreview), m.preview.View())
		}
		tabs, _ := m.tabline()
		screen = lipgloss.JoinVertical(lipgloss.Left,
			tabs,
			lipgloss.JoinHorizontal(lipgloss.Top, cols...),
			m.bottomBar(),
		)
	}

	switch {
	case m.saveAs != nil:
		box, x, y := m.saveAs.view(m.theme, m.width, m.height)
		screen = ui.Overlay(screen, box, x, y)
	case m.confirm != nil:
		box, x, y := m.confirm.view(m.theme, m.width, m.height)
		screen = ui.Overlay(screen, box, x, y)
	case m.finder != nil:
		box, x, y := m.finder.view(&m)
		screen = ui.Overlay(screen, box, x, y)
	case m.themes != nil:
		box, x, y := m.themes.view(&m)
		screen = ui.Overlay(screen, box, x, y)
	case m.browser != nil:
		box, x, y := m.browser.view(&m)
		screen = ui.Overlay(screen, box, x, y)
	case m.datePick != nil:
		box, x, y := m.datePick.view(&m)
		screen = ui.Overlay(screen, box, x, y)
	case m.promptBox != nil:
		box, x, y := m.promptBox.view(&m)
		screen = ui.Overlay(screen, box, x, y)
	case m.menuBox != nil:
		box, x, y := m.menuBox.view(&m)
		screen = ui.Overlay(screen, box, x, y)
	case m.viewMode == viewModeWorkspace && m.focus == focusEditor && m.content.SlashOpen():
		if box, x, y, ok := m.content.SlashView(); ok {
			l := m.layout()
			screen = ui.Overlay(screen, box, l.EditorX+x, l.BodyY+y)
		}
	case m.showKeymap:
		box := RenderKeymapModal(m.theme, m.width, m.height)
		x, y := ui.Center(m.width, m.height, lipgloss.Width(box), lipgloss.Height(box))
		screen = ui.Overlay(screen, box, x, y)
	}
	screen = m.renderToast(screen)
	return ui.Paint(screen, lipgloss.NewStyle().Foreground(m.theme.Fg).Background(m.theme.Bg))
}
