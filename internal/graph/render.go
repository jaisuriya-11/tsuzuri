package graph

import (
	"fmt"
	"strings"

	"github.com/jaisuriya-11/tsuzuri/internal/ui"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// headerRows is the title row above the map; one row of hints sits below.
const headerRows = 1

func (m Model) mapRows() int { return max(m.height-headerRows-1, 1) }

// geometry is where columns go: each is labelW cells of name plus three of
// line ("─", the vertical join, "─") before the next.
type geometry struct {
	labelW, pitch, minCol, width int
}

func (g geometry) x(col int) int { return 1 + (col-g.minCol)*g.pitch }

func (m Model) geometry(t tree) geometry {
	// Names share the pane between the columns, within reason; zoomed out
	// maps wider than that are panned.
	cols := t.maxLevel - t.minLevel + 1
	labelW := max(min((m.width-2)/max(cols, 3)-3, 24), 14)
	g := geometry{labelW: labelW, pitch: labelW + 3, minCol: t.minLevel}
	g.width = g.x(t.maxLevel) + labelW + 1
	return g
}

// viewFor is the map cell at the pane's top-left: where the user panned
// to, or, after the centre or zoom changed, the map centred (as far as
// keeps the centre note on screen).
func (m Model) viewFor(t tree, g geometry) [2]int {
	if m.placed {
		return m.view
	}
	rows := m.mapRows()
	v := [2]int{(g.width - m.width) / 2, (t.rows - rows) / 2}
	if g.width > m.width { // keep the centre note in view
		cx := g.x(0)
		v[0] = max(min(v[0], cx-1), cx+g.labelW+1-m.width)
	}
	if t.rows > rows {
		v[1] = t.nodes[0].row - rows/2
	}
	return v
}

// settle fixes the view before it is moved by hand.
func (m *Model) settle() {
	if !m.placed && m.centre != "" {
		t := m.tree()
		m.view, m.placed = m.viewFor(t, m.geometry(t)), true
	}
}

// cell is one character of the map.
type cell struct {
	r      rune
	fg, bg lipgloss.Color
	bold   bool
	tail   bool // right half of a wide character
}

type canvas struct {
	cells [][]cell
	w     int
}

func newCanvas(w, h int) *canvas {
	c := &canvas{cells: make([][]cell, h), w: w}
	for y := range c.cells {
		c.cells[y] = make([]cell, w)
	}
	return c
}

func (c *canvas) set(x, y int, r rune, fg lipgloss.Color) {
	if y >= 0 && y < len(c.cells) && x >= 0 && x < c.w {
		c.cells[y][x] = cell{r: r, fg: fg}
	}
}

// hline draws "─" over cells [x0, x1].
func (c *canvas) hline(x0, x1, y int, fg lipgloss.Color) {
	for x := x0; x <= x1; x++ {
		c.set(x, y, '─', fg)
	}
}

// text writes s from x, at most width cells; wide characters take two.
func (c *canvas) text(x, y, width int, s string, st cell) {
	if y < 0 || y >= len(c.cells) {
		return
	}
	end := min(x+width, c.w)
	for _, r := range s {
		w := ansi.StringWidth(string(r))
		if w == 0 || x+w > end {
			break
		}
		if x >= 0 {
			st.r = r
			c.cells[y][x] = st
			for t := 1; t < w; t++ {
				c.cells[y][x+t] = cell{tail: true}
			}
		}
		x += w
	}
}

// Line ends meeting in a cell.
const (
	up = 1 << iota
	down
	left
	right
)

var boxChars = map[int]rune{
	left | right: '─', up | down: '│', up: '│', down: '│', left: '─', right: '─',
	down | right: '╭', down | left: '╮', up | right: '╰', up | left: '╯',
	up | down | right: '├', up | down | left: '┤', left | right | down: '┬', left | right | up: '┴',
	up | down | left | right: '┼',
}

// join draws the vertical line at x linking a parent on row p (entering
// from parentSide) to children on rows kids (leaving the other way). The
// stretch from the lit child's row to p is drawn in hot.
func (c *canvas) join(x, p int, kids []int, parentSide, kidSide, lit int, dim, hot lipgloss.Color) {
	lo, hi := p, p
	has := map[int]bool{}
	for _, y := range kids {
		lo, hi = min(lo, y), max(hi, y)
		has[y] = true
	}
	for y := lo; y <= hi; y++ {
		mask := 0
		if y > lo {
			mask |= up
		}
		if y < hi {
			mask |= down
		}
		if has[y] {
			mask |= kidSide
		}
		if y == p {
			mask |= parentSide
		}
		fg := dim
		if lit >= 0 && y >= min(lit, p) && y <= max(lit, p) {
			fg = hot
		}
		c.set(x, y, boxChars[mask], fg)
	}
}

// labelSpan is where a node's dot and name go: right of the dot for the
// centre and links, left of it for backlinks.
func (g geometry) labelSpan(n gnode) (dotX, nameX, nameW int, name string) {
	name = ui.Truncate(n.title, g.labelW-2)
	nameW = ansi.StringWidth(name)
	x := g.x(n.col())
	if n.side < 0 {
		return x + g.labelW - 1, x + g.labelW - 2 - nameW, nameW, name
	}
	return x, x + 2, nameW, name
}

// draw paints the whole map.
func (m Model) draw(t tree, g geometry) *canvas {
	th := m.th
	cv := newCanvas(g.width, t.rows)
	dim, hot := th.GreyFg, th.Blue
	sel := t.index[m.sel]

	for _, p := range t.nodes {
		for _, s := range []int{1, -1} {
			var kids, rows []int
			lit := -1
			for _, k := range p.kids {
				if t.nodes[k].side == s {
					kids = append(kids, k)
					rows = append(rows, t.nodes[k].row)
					if t.onPath(k, sel) {
						lit = t.nodes[k].row
					}
				}
			}
			if len(kids) == 0 {
				continue
			}
			fg := dim
			if lit >= 0 {
				fg = hot
			}
			pDot, pName, pW, _ := g.labelSpan(p)
			if s > 0 {
				bus := g.x(p.col()) + g.labelW + 1
				cv.hline(pName+pW+1, bus-1, p.row, fg)
				cv.join(bus, p.row, rows, left, right, lit, dim, hot)
				for _, k := range kids {
					kfg := dim
					if t.onPath(k, sel) {
						kfg = hot
					}
					cv.set(bus+1, t.nodes[k].row, '─', kfg)
				}
			} else {
				bus := g.x(p.col()-1) + g.labelW + 1
				end := pName - 2 // a backlink's name is left of its dot
				if p.side == 0 {
					end = pDot - 1
				}
				cv.hline(bus+1, max(end, bus+1), p.row, fg)
				cv.join(bus, p.row, rows, right, left, lit, dim, hot)
				for _, k := range kids {
					kfg := dim
					if t.onPath(k, sel) {
						kfg = hot
					}
					cv.set(bus-1, t.nodes[k].row, '─', kfg)
				}
			}
		}
	}

	for i, n := range t.nodes {
		dotX, nameX, nameW, name := g.labelSpan(n)
		dot := th.Blue
		switch {
		case n.side == 0:
			dot = th.Green
		case n.side < 0:
			dot = th.Purple
		}
		st := cell{fg: th.Fg, bold: n.side == 0}
		if n.level > 1 {
			st.fg = th.GreyFg2
		}
		if i == sel {
			dot, st.fg, st.bold = th.Yellow, th.Fg, true
			if m.focused {
				st.bg = th.OneBg
			}
		}
		if st.bg != "" { // highlight the dot, the gap and the name
			for x := min(dotX, nameX); x <= max(dotX, nameX+nameW-1); x++ {
				cv.cells[n.row][x] = cell{r: ' ', bg: st.bg}
			}
		}
		cv.set(dotX, n.row, '●', dot)
		cv.cells[n.row][dotX].bg = st.bg
		cv.text(nameX, n.row, nameW, name, st)
	}
	return cv
}

// View renders the graph at exactly the pane's size.
func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	th := m.th
	plain := lipgloss.NewStyle()
	if m.centre == "" {
		msg := lipgloss.NewStyle().Foreground(th.GreyFg).Render("No notes yet")
		return ui.Fit(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, msg), m.width, m.height, plain)
	}
	t := m.tree()
	g := m.geometry(t)
	cv := m.draw(t, g)
	view := m.viewFor(t, g)

	lines := []string{m.titleLine(t)}
	for r := 0; r < m.mapRows(); r++ {
		y := view[1] + r
		row := make([]cell, m.width)
		if y >= 0 && y < t.rows {
			for x := range row {
				if sx := view[0] + x; sx >= 0 && sx < g.width {
					row[x] = cv.cells[y][sx]
				}
			}
			if row[0].tail { // a wide character cut in half by the edge
				row[0] = cell{}
			}
		}
		lines = append(lines, renderRow(row))
	}
	lines = append(lines, m.helpLine())
	return ui.Fit(strings.Join(lines, "\n"), m.width, m.height, plain)
}

// button is a clickable control on the title row.
type button struct {
	label, action string
	x0, x1        int
	enabled       bool
}

// buttons sit at the start of the bottom row.
func (m Model) buttons() []button {
	bs := []button{
		{label: " − ", action: "zoom-out", enabled: m.depth < maxDepth},
		{label: " + ", action: "zoom-in", enabled: m.depth > 1},
		{label: " centre ", action: "centre", enabled: m.sel != m.centre},
		{label: " back ", action: "back", enabled: len(m.history) > 0},
		{label: " reset ", action: "reset", enabled: true},
	}
	x := 1
	for i := range bs {
		bs[i].x0 = x
		x += ansi.StringWidth(bs[i].label)
		bs[i].x1 = x
		x++
	}
	return bs
}

// titleLine names the centre note and counts its links.
func (m Model) titleLine(t tree) string {
	th := m.th
	steps := "1 step"
	if m.depth > 1 {
		steps = fmt.Sprintf("%d steps", m.depth)
	}
	info := fmt.Sprintf(" · %s · %s · %s", plural(len(m.out[m.centre]), "link"), plural(len(m.in[m.centre]), "backlink"), steps)
	if m.depth > 1 {
		info += " · " + plural(len(t.nodes)-1, "note")
	}
	head := lipgloss.NewStyle().Foreground(th.Blue).Bold(true).Render("󰈙 "+ui.Truncate(m.titles[m.centre], 40)) +
		lipgloss.NewStyle().Foreground(th.GreyFg2).Render(info)
	return ui.FitLine(" "+head, m.width, lipgloss.NewStyle())
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// helpLine is the bottom row: the buttons, then what the mouse does.
func (m Model) helpLine() string {
	th := m.th
	var b strings.Builder
	b.WriteString(" ")
	for i, bt := range m.buttons() {
		st := lipgloss.NewStyle().Background(th.OneBg).Foreground(th.Fg)
		if !bt.enabled {
			st = st.Foreground(th.GreyFg)
		}
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(st.Render(bt.label))
	}
	b.WriteString(lipgloss.NewStyle().Foreground(th.GreyFg2).Render("  click: preview, again: open · scroll: zoom · drag: move"))
	return ui.FitLine(b.String(), m.width, lipgloss.NewStyle())
}

// renderRow styles runs of cells that look the same.
func renderRow(row []cell) string {
	var b, run strings.Builder
	var cur cell
	flush := func() {
		if run.Len() == 0 {
			return
		}
		st := lipgloss.NewStyle().Bold(cur.bold)
		if cur.fg != "" {
			st = st.Foreground(cur.fg)
		}
		if cur.bg != "" {
			st = st.Background(cur.bg)
		}
		b.WriteString(st.Render(run.String()))
		run.Reset()
	}
	for _, c := range row {
		if c.tail {
			continue
		}
		if c.r == 0 {
			c.r = ' '
		}
		if c.fg != cur.fg || c.bg != cur.bg || c.bold != cur.bold {
			flush()
			cur = c
		}
		run.WriteRune(c.r)
	}
	flush()
	return b.String()
}
