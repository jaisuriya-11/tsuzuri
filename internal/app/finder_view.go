package app

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/jaisuriya-11/tsuzuri/internal/theme"
	"github.com/jaisuriya-11/tsuzuri/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (f *finder) view(m *Model) (string, int, int) {
	th := m.theme
	g := finderLayout(m.width, m.height)
	inner := g.w - 2
	bg := lipgloss.NewStyle().Background(th.DarkerBg)
	field := lipgloss.NewStyle().Background(th.Bg2)

	rows := make([]string, 0, g.innerHeight)

	// Title + count.
	title := bg.Foreground(th.Blue).Render(" 󰍉 ") + bg.Foreground(th.Fg).Bold(true).Render(f.title()) + bg.Foreground(th.GreyFg2).Render("  in "+f.root+"/")
	count := bg.Foreground(th.GreyFg2).Render(fmt.Sprintf("%d results · %d notes ", len(f.matches), len(f.notes)))
	rows = append(rows, title+bg.Render(strings.Repeat(" ", max(inner-lipgloss.Width(title)-lipgloss.Width(count), 0)))+count)

	// Prompt.
	f.input.Width = max(inner-8, 1)
	prompt := field.Foreground(th.Blue).Bold(true).Render(" \uf002 ") + f.input.View()
	rows = append(rows, bg.Render(" ")+ui.FitLine(prompt, inner-2, field)+bg.Render(" "))
	rows = append(rows, bg.Foreground(th.Line).Render(strings.Repeat("─", inner)))

	// Results and preview side by side.
	var prevLines []string
	var prevTitle string
	prevStart, hitLine := 0, 0
	if p, ok := f.selected(); ok && g.previewW > 0 {
		prevLines = f.lines[p.ID]
		prevTitle = p.ID
		if hitLine = f.matches[f.sel].line; hitLine > 0 {
			prevTitle = fmt.Sprintf("%s:%d", p.ID, hitLine)
			prevStart = max(hitLine-1-(g.listRows-1)/3, 0)
		}
	}
	sep := bg.Foreground(th.Line).Render("│")
	for r := 0; r < g.listRows; r++ {
		left := f.resultRow(th, f.offset+r, g.listW)
		if r == 0 && len(f.matches) == 0 {
			left = ui.FitLine(bg.Foreground(th.GreyFg).Italic(true).Render("   No matching notes"), g.listW, bg)
		}
		if g.previewW == 0 {
			rows = append(rows, left)
			continue
		}
		var right string
		switch {
		case r == 0 && prevTitle != "":
			right = bg.Foreground(th.Yellow).Render(" 󰈈 ") + bg.Foreground(th.GreyFg2).Render(ui.Truncate(prevTitle, g.previewW-5))
		case r >= 1 && prevStart+r-1 < len(prevLines):
			n := prevStart + r - 1
			right = previewLine(th, prevLines[n], g.previewW)
			if n+1 == hitLine {
				right = ui.FitLine(lipgloss.NewStyle().Background(th.OneBg2).Foreground(th.Fg).Render(" "+ui.Truncate(prevLines[n], g.previewW-2)), g.previewW, lipgloss.NewStyle().Background(th.OneBg2))
			}
		}
		rows = append(rows, left+sep+ui.FitLine(right, g.previewW, bg))
	}

	help := "↑↓ move · Enter open here (jumps to the line) · Ctrl+T new tab · Esc close"
	rows = append(rows, bg.Foreground(th.GreyFg).Render(" "+ui.Truncate(help, inner-2)))
	return panel(th, rows, inner), g.x, g.y
}

func (f *finder) resultRow(th theme.Theme, idx, width int) string {
	bg := lipgloss.NewStyle().Background(th.DarkerBg)
	if idx >= len(f.matches) {
		return ui.FitLine("", width, bg)
	}
	mt := f.matches[idx]
	selected := idx == f.sel
	rowBg := th.DarkerBg
	if selected {
		rowBg = th.OneBg2
	}
	base := lipgloss.NewStyle().Background(rowBg)
	hit := base.Foreground(th.Blue).Bold(true)

	marker := base.Render("  ")
	if selected {
		marker = base.Foreground(th.Blue).Render("▌ ")
	}
	icon := base.Foreground(th.NordBlue).Render(" ")

	if mt.line > 0 {
		return f.textRow(th, mt, base, hit, marker, width)
	}

	id := mt.page.ID
	nameStart := strings.LastIndex(id, "/") + 1
	matched := map[int]bool{}
	for _, p := range mt.pos {
		matched[p] = true
	}
	render := func(from, to int, normal lipgloss.Style) string {
		var b strings.Builder
		for i, r := range id[from:to] {
			st := normal
			if matched[from+i] {
				st = hit
			}
			b.WriteString(st.Render(string(r)))
		}
		return b.String()
	}

	name := render(nameStart, len(id), base.Foreground(th.Fg).Bold(selected))
	var dir string
	if nameStart > 0 {
		dir = base.Render("  ") + render(0, nameStart-1, base.Foreground(th.GreyFg))
	}
	line := marker + icon + name + dir
	if lipgloss.Width(line) > width {
		line = ui.FitLine(line, width-1, base) + base.Foreground(th.GreyFg).Render("…")
	}
	return ui.FitLine(line, width, base)
}

func previewLine(th theme.Theme, l string, width int) string {
	bg := lipgloss.NewStyle().Background(th.DarkerBg)
	trim := strings.TrimSpace(l)
	st := bg.Foreground(th.GreyFg2)
	switch {
	case strings.HasPrefix(trim, "#"):
		st = bg.Foreground(th.Blue).Bold(true)
	case strings.HasPrefix(trim, "- [x]"), strings.HasPrefix(trim, "- [X]"):
		st = bg.Foreground(th.Green)
	case strings.HasPrefix(trim, "- "), strings.HasPrefix(trim, "* "):
		st = bg.Foreground(th.Fg)
	case strings.HasPrefix(trim, ">"):
		st = bg.Foreground(th.Purple)
	case strings.HasPrefix(trim, "```"):
		st = bg.Foreground(th.Yellow)
	}
	return bg.Render(" ") + st.Render(ui.Truncate(l, width-2))
}

// openInCurrentBuffer shows id in the active tab instead of adding a new one
// (Telescope / ":e" behaviour). An already-open file is simply focused, and a
// tab with unsaved changes is never replaced — the file opens next to it.
func (m *Model) openInCurrentBuffer(id string) tea.Cmd {
	if i := m.bufferIndex(id); i >= 0 {
		m.showBuffer(m.buffers[i])
		return m.focusPane(focusEditor)
	}
	cur := m.activeBuffer()
	if cur == nil || m.isDirty(cur) || cur.isGraph() {
		cmd := m.openFile(id)
		if cur != nil {
			m.setStatus(fmt.Sprintf("Opened in a new tab — %s has unsaved changes", cur.fileName()))
		}
		return cmd
	}
	p, err := m.store.Get(id)
	if err != nil || p.IsFolder {
		m.setError("Cannot open " + id)
		return nil
	}
	cur.id, cur.title, cur.saved, cur.text, cur.dir = p.ID, p.Title, p.Content, p.Content, ""
	m.active = ""
	m.showBuffer(cur)
	m.refreshModified()
	return m.focusPane(focusEditor)
}

// textRow renders a text hit: "path:line  …matched text…".
func (f *finder) textRow(th theme.Theme, mt finderMatch, base, hit lipgloss.Style, marker string, width int) string {
	loc := base.Foreground(th.GreyFg2).Render(fmt.Sprintf("%s:%d", mt.page.ID, mt.line))
	icon := base.Foreground(th.Yellow).Render("\uf002 ")
	head := marker + icon + loc + base.Render("  ")
	room := width - lipgloss.Width(head)

	text := mt.text
	start := 0
	if len(mt.pos) > 0 && mt.pos[0] > room/2 {
		start = mt.pos[0] - room/3 // keep the match visible on long lines
		for start > 0 && !utf8.RuneStart(text[start]) {
			start--
		}
	}
	matched := map[int]bool{}
	for _, p := range mt.pos {
		matched[p] = true
	}
	var b strings.Builder
	if start > 0 {
		b.WriteString(base.Foreground(th.GreyFg).Render("…"))
	}
	normal := base.Foreground(th.Fg)
	for i, r := range text[start:] {
		st := normal
		if matched[start+i] {
			st = hit
		}
		b.WriteString(st.Render(string(r)))
	}
	line := head + b.String()
	if lipgloss.Width(line) > width {
		line = ui.FitLine(line, width-1, base) + base.Foreground(th.GreyFg).Render("…")
	}
	return ui.FitLine(line, width, base)
}

func (f *finder) title() string {
	if f.link {
		return "Link to Note"
	}
	return "Find Note"
}
