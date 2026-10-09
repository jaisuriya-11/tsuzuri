package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/jaisuriya-11/tsuzuri/internal/content"
	"github.com/jaisuriya-11/tsuzuri/internal/ui"

	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Tabufline (top row): buffer tabs and a "+" button.

type tabHit int

const (
	hitNone tabHit = iota
	hitTab
	hitClose
	hitNew
	hitExplorer
)

type tabSpan struct {
	kind       tabHit
	id         string
	start, end int // absolute columns [start, end)
}

const tabMaxName = 20

func (m *Model) renderTab(b *buffer) (string, int) {
	th := m.theme
	active := b.id == m.active
	bg, fg, iconFg := th.Bg2, th.LightGrey, th.GreyFg
	if active {
		bg, fg, iconFg = th.Bg, th.Fg, th.NordBlue
	}
	base := lipgloss.NewStyle().Background(bg)
	name := ui.Truncate(b.fileName(), tabMaxName)

	closeGlyph, closeFg := "󰅖", th.GreyFg
	if active {
		closeFg = th.Red
	}
	if m.isDirty(b) {
		closeGlyph, closeFg = "●", th.Green
	}

	left := base.Foreground(iconFg).Render("  ") + base.Foreground(fg).Bold(active).Render(name)
	if b.draft() {
		left += base.Foreground(th.GreyFg).Italic(true).Render(" new")
	}
	s := left + base.Render("  ") + base.Foreground(closeFg).Render(closeGlyph) + base.Render(" ")
	if active {
		s = lipgloss.NewStyle().Background(bg).Foreground(th.Blue).Render("▎") + s
	} else {
		s = base.Render(" ") + s
	}
	closeOffset := lipgloss.Width(s) - 2
	return s, closeOffset
}

// tabline renders the top bar and returns clickable spans.
func (m *Model) tabline() (string, []tabSpan) {
	th := m.theme
	fill := lipgloss.NewStyle().Background(th.DarkerBg)
	l := m.layout()
	var spans []tabSpan
	var b strings.Builder
	x := 0

	if l.SidebarW > 0 {
		w := l.SidebarW + 1
		b.WriteString(ui.FitLine("", w, fill))
		spans = append(spans, tabSpan{kind: hitExplorer, start: 0, end: w})
		x = w
	}

	plus := fill.Foreground(th.GreyFg2).Render("  ")
	avail := m.width - x - lipgloss.Width(plus)

	// Lay out tabs, sliding the window so the active tab is visible.
	type rendered struct {
		s        string
		w, close int
		id       string
	}
	tabs := make([]rendered, len(m.buffers))
	activeIdx := 0
	for i, buf := range m.buffers {
		s, c := m.renderTab(buf)
		tabs[i] = rendered{s: s, w: lipgloss.Width(s), close: c, id: buf.id}
		if buf.id == m.active {
			activeIdx = i
		}
	}
	start := 0
	for {
		used := 0
		end := start
		for end < len(tabs) && used+tabs[end].w <= avail {
			used += tabs[end].w
			end++
		}
		if activeIdx < end || start >= activeIdx {
			break
		}
		start++
	}
	if start > 0 {
		b.WriteString(fill.Foreground(th.GreyFg).Render("‹"))
		x++
		avail--
	}
	used := 0
	for i := start; i < len(tabs); i++ {
		t := tabs[i]
		if used+t.w > avail {
			break
		}
		b.WriteString(t.s)
		spans = append(spans,
			tabSpan{kind: hitTab, id: t.id, start: x, end: x + t.w},
			tabSpan{kind: hitClose, id: t.id, start: x + t.close - 1, end: x + t.close + 2},
		)
		x += t.w
		used += t.w
	}
	b.WriteString(plus)
	spans = append(spans, tabSpan{kind: hitNew, start: x, end: x + lipgloss.Width(plus)})
	x += lipgloss.Width(plus)

	return ui.FitLine(b.String(), m.width, fill), spans
}

// tabAt finds what lies under column x of the tab bar. Close buttons win
// over the tab they sit on.
func (m *Model) tabAt(x int) tabSpan {
	_, spans := m.tabline()
	hit := tabSpan{kind: hitNone}
	for _, s := range spans {
		if x >= s.start && x < s.end {
			if s.kind == hitClose {
				return s
			}
			hit = s
		}
	}
	return hit
}

// ---------------------------------------------------------------------------
// Statusline (NvChad "default" style) and command line.

func (m *Model) modeBlock() (string, lipgloss.Color) {
	th := m.theme
	switch {
	case m.viewMode == viewModeDashboard:
		return "  HOME ", th.Blue
	case m.focus == focusSidebar:
		return " 󰙅 EXPLORER ", th.Teal
	case m.focus == focusPreview:
		return " 󰈈 PREVIEW ", th.Yellow
	}
	switch m.content.Mode() {
	case content.ModeInsert:
		return "  INSERT ", th.DarkPurple
	case content.ModeVisual:
		return " \U000f0489 VISUAL ", th.Orange
	case content.ModeVisualLine:
		return " \U000f0489 V-LINE ", th.Orange
	case content.ModeCommand:
		return "  COMMAND ", th.Green
	}
	return "  NORMAL ", th.Blue
}

func (m *Model) statusline() string {
	th := m.theme
	fill := lipgloss.NewStyle().Background(th.StatusBg)
	light := lipgloss.NewStyle().Background(th.LightBg)

	label, color := m.modeBlock()
	left := lipgloss.NewStyle().Background(color).Foreground(th.Bg).Bold(true).Render(label)

	b := m.activeBuffer()
	if m.viewMode == viewModeDashboard {
		b = nil
	}
	file := "No file"
	fileFg := th.GreyFg2
	if b != nil {
		file = b.fileName()
		fileFg = th.Fg
	}
	if m.viewMode == viewModeDashboard {
		left += lipgloss.NewStyle().Background(th.StatusBg).Foreground(color).Render("\ue0b4")
	} else {
		left += lipgloss.NewStyle().Background(th.LightBg).Foreground(color).Render("")
		left += light.Foreground(th.NordBlue).Render(" 󰈙 ") + light.Foreground(fileFg).Render(ui.Truncate(file, 32)+" ")
		if b != nil && m.isDirty(b) {
			left += light.Foreground(th.Green).Render("● ")
		}
		if b != nil && b.draft() && !b.isGraph() {
			left += light.Foreground(th.Yellow).Render("unsaved ")
		}
		left += lipgloss.NewStyle().Background(th.StatusBg).Foreground(th.LightBg).Render("")
	}
	if b != nil && !b.draft() {
		if dir := filepath.Dir(b.id); dir != "." {
			left += fill.Foreground(th.GreyFg).Render("  󰉋 " + ui.Truncate(dir, 30))
		}
	}

	// Right side, dropped piece by piece when the terminal is narrow.
	var parts []string
	if b != nil {
		words := len(strings.Fields(m.liveText(b)))
		unit := "words"
		if words == 1 {
			unit = "word"
		}
		parts = append(parts, fill.Foreground(th.GreyFg2).Render(fmt.Sprintf("󰈭 %d %s  ", words, unit)))
	}
	if b != nil {
		parts = append(parts, fill.Foreground(th.GreyFg2).Render(" Markdown  "))
	}
	cwd := lipgloss.NewStyle().Background(th.StatusBg).Foreground(th.Red).Render("") +
		lipgloss.NewStyle().Background(th.Red).Foreground(th.Bg).Render("󰉋 ") +
		light.Foreground(th.Fg).Render(" "+ui.Truncate(filepath.Base(m.store.Root()), 20)+" ")
	parts = append(parts, cwd)
	pos := "  "
	if b != nil {
		ln, col := m.content.CursorPosition()
		pos = fmt.Sprintf(" Ln %d, Col %d ", ln, col)
	}
	parts = append(parts,
		lipgloss.NewStyle().Background(th.LightBg).Foreground(th.Green).Render("")+
			lipgloss.NewStyle().Background(th.Green).Foreground(th.Bg).Render(" ")+
			light.Foreground(th.Green).Render(pos))

	right := strings.Join(parts, "")
	for len(parts) > 1 && lipgloss.Width(left)+lipgloss.Width(right)+1 > m.width {
		parts = parts[1:]
		right = strings.Join(parts, "")
	}
	if msg := m.message(); msg != "" {
		room := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 1
		if lipgloss.Width(msg) > room {
			msg = ui.FitLine(msg, max(room, 0), fill)
		}
		left += msg
	}
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	line := left + fill.Render(strings.Repeat(" ", max(gap, 0))) + right
	return ui.FitLine(line, m.width, fill)
}

// bottomBar is the single bottom row: the ":" prompt while typing a
// command (like Vim with cmdheight=0), otherwise the statusline.
func (m *Model) bottomBar() string {
	if m.viewMode == viewModeWorkspace && m.focus == focusEditor && m.content.Mode() == content.ModeCommand {
		bar := lipgloss.NewStyle().Background(m.theme.StatusBg)
		return ui.FitLine(m.content.CommandView(m.width), m.width, bar)
	}
	return m.statusline()
}

// message is the transient text shown in the middle of the statusline.
func (m *Model) message() string {
	th := m.theme
	fill := lipgloss.NewStyle().Background(th.StatusBg)
	switch {
	case m.status != "" && m.statusErr:
		return fill.Foreground(th.Red).Render("  " + m.status)
	case m.status != "":
		return fill.Foreground(th.Fg).Render("  " + m.status)
	case m.leaderPending:
		return fill.Foreground(th.Blue).Bold(true).Render("  <Space>")
	}
	return ""
}
