package preview

import (
	"fmt"
	"strings"

	"github.com/jaisuriya-11/tsuzuri/internal/core"
	"github.com/jaisuriya-11/tsuzuri/internal/theme"
	"github.com/jaisuriya-11/tsuzuri/internal/ui"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model represents the real-time compiled Markdown preview component.
type Model struct {
	theme      theme.Theme
	viewport   viewport.Model
	width      int
	height     int
	pageID     string
	title      string
	rawContent string
	baseDir    string
	cal        CalendarView
	notes      NoteLookup
	backlinks  []core.Backlink
	hits       []Hit
	pendingZ   bool
	focused    bool
	ready      bool
	margin     int // blank columns left of the page

	blocks   []Block
	hover    int  // block under the mouse, -1 if none
	drag     int  // block being dragged, -1 if none
	dropLine int  // document line the dragged block would move to
	dropRow  int  // content row of the drop marker
	dropGap  bool // the marker sits on a blank row between blocks
}

// pageWidth is the widest the preview text gets; wider panes centre the
// page with equal margins, like a sheet of paper.
const pageWidth = 88

// pageMargin returns the left margin for a pane w columns wide. It is
// wide enough for the block handles ("⠿" from 40 columns, "+ ⠿" from 70),
// and grows to centre the page once the pane is wider than pageWidth.
func pageMargin(w int) int {
	minMargin := 1
	switch {
	case w >= 70:
		minMargin = 4
	case w >= 40:
		minMargin = 2
	}
	return max((w-pageWidth)/2, minMargin)
}

// New constructs a Preview Model.
func New(th theme.Theme) Model {
	vp := viewport.New(0, 0)
	vp.MouseWheelEnabled = true

	return Model{
		theme:    th,
		viewport: vp,
		title:    "Untitled",
		hover:    -1,
		drag:     -1,
		dropRow:  -1,
	}
}

// SetSize updates the layout dimensions for the preview pane and viewport.
func (m *Model) SetSize(w, h int) {
	m.width = max(w, 0)
	m.height = max(h, 0)

	m.margin = pageMargin(m.width)
	m.viewport.Width = max(m.width-2*m.margin, 10)
	m.viewport.Height = max(h, 1)
	m.ready = true
	m.recompile()
}

// ScrollStatus returns a formatted indicator of the viewport scroll position.
func (m Model) ScrollStatus() string {
	if m.viewport.AtTop() {
		return "Top"
	}
	if m.viewport.AtBottom() {
		return "Bot"
	}
	pct := m.viewport.ScrollPercent() * 100
	return fmt.Sprintf("%3.0f%%", pct)
}

// SetPage updates both the title and content from a domain Page entity.
func (m *Model) SetPage(p core.Page) {
	m.pageID = p.ID
	m.title = p.Title
	if m.title == "" {
		m.title = "Untitled"
	}
	m.SetContent(p.Content)
}

// SetBaseDir sets the folder the note lives in (for relative image paths).
func (m *Model) SetBaseDir(dir string) {
	if dir != m.baseDir {
		m.baseDir = dir
		m.recompile()
	}
}

// SetTheme switches colours and re-renders.
func (m *Model) SetTheme(th theme.Theme) {
	m.theme = th
	m.recompile()
}

// SetContent recompiles the raw Markdown into styled ANSI text in real time.
func (m *Model) SetContent(content string) {
	m.rawContent = content
	m.recompile()
}

// SetTitle updates the preview pane title.
func (m *Model) SetTitle(title string) {
	if title == "" {
		title = "Untitled"
	}
	m.title = title
}

// SetFocused updates whether the preview pane has keyboard focus.
func (m *Model) SetFocused(focused bool) {
	m.focused = focused
}

// recompile parses the current rawContent with the compiler.
func (m *Model) recompile() {
	vpWidth := m.viewport.Width
	if vpWidth <= 0 {
		vpWidth = m.width - 2*m.margin
	}
	cal := m.cal
	if m.notes != nil {
		self := Note{ID: m.pageID, Title: m.title, Content: m.rawContent, Dir: m.baseDir}
		cal.Notes = func(target string) (Note, bool) {
			if target == "" {
				return self, true
			}
			return m.notes(target)
		}
	}
	compiled, hits, blocks := CompileBlocks(m.rawContent, m.theme, vpWidth, m.baseDir, cal)
	if len(m.backlinks) > 0 {
		compiled, hits = m.appendBacklinks(compiled, hits, vpWidth)
	}
	m.hits, m.blocks = hits, blocks
	if m.hover >= len(blocks) {
		m.hover = -1
	}
	if m.drag >= len(blocks) {
		m.drag, m.dropRow = -1, -1
	}
	m.viewport.SetContent(compiled)
}

// ShiftCalendars moves every calendar by delta months; today jumps to the
// current month.
func (m *Model) ShiftCalendars(delta int, today bool) {
	if today {
		m.cal = CalendarView{Today: true}
	} else {
		m.cal.Shift += delta
	}
	m.recompile()
}

// ToggleFold collapses or expands one heading section or code block.
func (m *Model) ToggleFold(key string) {
	if m.cal.Folded == nil {
		m.cal.Folded = map[string]bool{}
	}
	m.cal.Folded[key] = !m.cal.Folded[key]
	m.recompile()
}

// FoldAll collapses (true) or expands (false) every section and code block.
func (m *Model) FoldAll(fold bool) {
	m.cal.FoldAll = fold
	m.cal.Folded = map[string]bool{}
	m.recompile()
}

// HitMsg reports a click on an interactive part of a view block.
type HitMsg struct{ Hit Hit }

// HitAt returns the view element under pane coordinates (x, y).
func (m Model) HitAt(x, y int) (Hit, bool) {
	row := m.viewport.YOffset + y
	col := x - m.margin
	for i := len(m.hits) - 1; i >= 0; i-- {
		h := m.hits[i]
		if row >= h.Row && row < h.Row+h.H && col >= h.X0 && col < h.X1 {
			return h, true
		}
	}
	return Hit{}, false
}

// click handles calendar navigation itself and reports everything else.
func (m *Model) click(x, y int) tea.Cmd {
	h, ok := m.HitAt(x, y)
	if !ok {
		return nil
	}
	switch h.Kind {
	case "cal:prev":
		m.ShiftCalendars(-1, false)
	case "cal:next":
		m.ShiftCalendars(1, false)
	case "cal:today":
		m.ShiftCalendars(0, true)
	case "fold":
		m.ToggleFold(h.Arg)
	default:
		return func() tea.Msg { return HitMsg{Hit: h} }
	}
	return nil
}

// ScrollBy scrolls the preview by n lines (negative scrolls up).
func (m *Model) ScrollBy(n int) {
	if n > 0 {
		m.viewport.ScrollDown(n)
	} else {
		m.viewport.ScrollUp(-n)
	}
}

// Update processes key and mouse events for scrolling the preview.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		if cmd, ok := m.mouse(msg); ok {
			return m, cmd
		}
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.ScrollBy(-3)
		case tea.MouseButtonWheelDown:
			m.ScrollBy(3)
		case tea.MouseButtonLeft:
			if msg.Action == tea.MouseActionPress {
				return m, m.click(msg.X, msg.Y)
			}
		}
		return m, nil
	case tea.KeyMsg:
		if k := msg.String(); m.pendingZ && k != "M" && k != "R" {
			m.pendingZ = false
		}
		switch msg.String() {
		case "j", "down":
			m.viewport.ScrollDown(1)
		case "k", "up":
			m.viewport.ScrollUp(1)
		case "d", "ctrl+d":
			m.viewport.HalfPageDown()
		case "u", "ctrl+u":
			m.viewport.HalfPageUp()
		case "pgdown", "ctrl+f", " ":
			m.viewport.PageDown()
		case "pgup", "ctrl+b":
			m.viewport.PageUp()
		case "z":
			m.pendingZ = true
			return m, nil
		case "M", "R":
			if m.pendingZ {
				m.pendingZ = false
				m.FoldAll(msg.String() == "M")
				return m, nil
			}
		case "<", "H":
			m.ShiftCalendars(-1, false)
		case ">", "L":
			m.ShiftCalendars(1, false)
		case "T":
			m.ShiftCalendars(0, true)
		case "g", "home":
			m.viewport.GotoTop()
		case "G", "end":
			m.viewport.GotoBottom()
		}
	}
	return m, nil
}

// View renders the compiled Markdown preview at exactly width × height.
func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	th := m.theme
	plain := lipgloss.NewStyle()

	var body string
	if m.pageID == "" {
		msg := lipgloss.NewStyle().Foreground(th.GreyFg).Render("Nothing to preview")
		body = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, msg)
	} else {
		body = ui.Fit(m.viewport.View(), m.viewport.Width, m.height, plain)
		pad := strings.Repeat(" ", m.margin)
		lines := strings.Split(body, "\n")
		for i, l := range lines {
			lines[i] = pad + l
		}
		m.decorate(lines)
		body = strings.Join(lines, "\n")
	}
	return ui.Fit(body, m.width, m.height, plain)
}
