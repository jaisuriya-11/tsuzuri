// Package graph draws a map of the links around one note: the notes it
// links to on the right, the notes linking to it on the left, and, zoomed
// out, their links in turn. Lines are box-drawing characters, so they stay
// crisp in any terminal.
package graph

import (
	"github.com/jaisuriya-11/tsuzuri/internal/core"
	"github.com/jaisuriya-11/tsuzuri/internal/theme"

	tea "github.com/charmbracelet/bubbletea"
)

// OpenMsg asks the app to open a note picked in the graph.
type OpenMsg struct{ ID string }

// SelectMsg reports the selected note, so the app can preview it.
type SelectMsg struct{ ID string }

// maxDepth is the furthest the map reaches when zoomed all the way out.
const maxDepth = 4

// Model is the graph view.
type Model struct {
	th theme.Theme

	titles map[string]string
	out    map[string][]string // links from each note, in the order written
	in     map[string][]string // notes linking to each note
	byLink []string            // notes, most linked first

	centre  string
	history []string // earlier centres, for going back
	sel     string
	depth   int

	width, height int
	focused       bool
	view          [2]int // map cell at the top-left of the pane
	placed        bool   // view has been set for this centre and size

	dragging bool
	dragAt   [2]int
}

// New makes an empty graph; SetLinks and SetCentre fill it.
func New(th theme.Theme) Model { return Model{th: th, depth: 1} }

// SetTheme switches colours.
func (m *Model) SetTheme(th theme.Theme) { m.th = th }

// SetFocused shows whether keys go to the graph.
func (m *Model) SetFocused(f bool) { m.focused = f }

// SetSize sets the pane size, keeping the centre in view.
func (m *Model) SetSize(w, h int) {
	if w != m.width || h != m.height {
		m.width, m.height = w, h
		m.placed = false
	}
}

// SetLinks replaces the workspace's notes and links.
func (m *Model) SetLinks(notes []core.Page, edges []core.Edge) {
	m.titles = map[string]string{}
	for _, p := range notes {
		m.titles[p.ID] = p.Title
	}
	m.out, m.in = map[string][]string{}, map[string][]string{}
	seen := map[core.Edge]bool{}
	degree := map[string]int{}
	for _, e := range edges {
		_, okF := m.titles[e.From]
		_, okT := m.titles[e.To]
		if !okF || !okT || e.From == e.To || seen[e] {
			continue
		}
		seen[e] = true
		m.out[e.From] = append(m.out[e.From], e.To)
		m.in[e.To] = append(m.in[e.To], e.From)
		degree[e.From]++
		degree[e.To]++
	}
	m.byLink = m.byLink[:0]
	for _, p := range notes {
		m.byLink = append(m.byLink, p.ID)
	}
	for i := 1; i < len(m.byLink); i++ { // stable insertion sort by links
		for j := i; j > 0 && degree[m.byLink[j]] > degree[m.byLink[j-1]]; j-- {
			m.byLink[j], m.byLink[j-1] = m.byLink[j-1], m.byLink[j]
		}
	}
	if _, ok := m.titles[m.centre]; !ok {
		m.centre = ""
	}
	if m.centre == "" && len(m.byLink) > 0 {
		m.centre = m.byLink[0]
	}
	if _, ok := m.titles[m.sel]; !ok {
		m.sel = m.centre
	}
}

// SetCentre starts the map from a note (the one open when the graph was
// opened) and selects it. It starts a fresh "back" history: only moves made
// inside the graph can be gone back over.
func (m *Model) SetCentre(id string) {
	m.history = nil
	if _, ok := m.titles[id]; !ok {
		return
	}
	if id != m.centre {
		m.centre, m.sel, m.placed = id, id, false
	}
}

// recentre moves the map to a note picked in the graph, remembering where
// it was for "back".
func (m *Model) recentre(id string) {
	if _, ok := m.titles[id]; !ok || id == m.centre {
		return
	}
	m.history = append(m.history, m.centre)
	m.centre, m.sel, m.placed = id, id, false
}

// Centre returns the note in the middle of the map.
func (m Model) Centre() string { return m.centre }

// Selected returns the selected note's ID.
func (m Model) Selected() string { return m.sel }

// tree lays out the map, with a blank row between notes when it fits.
func (m Model) tree() tree {
	t := buildTree(m.centre, m.titles, m.out, m.in, m.depth, 2)
	if t.rows > m.mapRows() {
		t = buildTree(m.centre, m.titles, m.out, m.in, m.depth, 1)
	}
	return t
}

// Update handles mouse events (coordinates relative to the pane). The
// graph is driven by the mouse alone; keys are ignored.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	mouse, ok := msg.(tea.MouseMsg)
	if !ok || m.centre == "" {
		return m, nil
	}
	m.settle()
	before := m.sel
	cmd := m.mouse(mouse)
	if cmd == nil && m.sel != before {
		id := m.sel
		cmd = func() tea.Msg { return SelectMsg{ID: id} }
	}
	return m, cmd
}

// act runs a header button.
func (m *Model) act(action string) {
	switch action {
	case "zoom-in":
		m.zoom(-1)
	case "zoom-out":
		m.zoom(1)
	case "centre":
		m.recentre(m.sel)
	case "back":
		for n := len(m.history); n > 0; n = len(m.history) {
			prev := m.history[n-1]
			m.history = m.history[:n-1]
			if _, ok := m.titles[prev]; ok { // skip notes deleted since
				m.centre, m.sel, m.placed = prev, prev, false
				break
			}
		}
	case "reset":
		m.placed = false
	}
}

// zoom shows d more (or fewer) levels of links.
func (m *Model) zoom(d int) {
	depth := max(min(m.depth+d, maxDepth), 1)
	if depth != m.depth {
		m.depth, m.placed = depth, false
		if _, ok := m.tree().index[m.sel]; !ok {
			m.sel = m.centre
		}
	}
}

func (m *Model) mouse(msg tea.MouseMsg) tea.Cmd {
	switch {
	case msg.Button == tea.MouseButtonWheelUp:
		m.zoom(-1)
	case msg.Button == tea.MouseButtonWheelDown:
		m.zoom(1)
	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft && msg.Y == m.height-1:
		for _, b := range m.buttons() {
			if msg.X >= b.x0 && msg.X < b.x1 && b.enabled {
				m.act(b.action)
			}
		}
	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft:
		if id := m.noteAt(msg.X, msg.Y); id != "" {
			if id == m.sel {
				return func() tea.Msg { return OpenMsg{ID: id} }
			}
			m.sel = id
			return nil
		}
		m.dragging, m.dragAt = true, [2]int{msg.X, msg.Y}
	case msg.Action == tea.MouseActionMotion && m.dragging:
		m.view[0] -= msg.X - m.dragAt[0]
		m.view[1] -= msg.Y - m.dragAt[1]
		m.dragAt = [2]int{msg.X, msg.Y}
	case msg.Action == tea.MouseActionRelease:
		m.dragging = false
	}
	return nil
}

// noteAt finds the note whose name is drawn at pane cell (x, y).
func (m Model) noteAt(x, y int) string {
	t := m.tree()
	g := m.geometry(t)
	view := m.viewFor(t, g)
	for _, n := range t.nodes {
		x0 := g.x(n.col()) - view[0]
		if y == headerRows+n.row-view[1] && x >= x0 && x < x0+g.labelW {
			return n.id
		}
	}
	return ""
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
