package app

import (
	"fmt"
	"strings"

	"github.com/jaisuriya-11/tsuzuri/internal/config"
	"github.com/jaisuriya-11/tsuzuri/internal/theme"
	"github.com/jaisuriya-11/tsuzuri/internal/ui"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// applyTheme recolours every component. persist also saves the choice.
func (m *Model) applyTheme(th theme.Theme, persist bool) {
	m.theme = th
	m.dashboard.SetTheme(th)
	m.sidebar.SetTheme(th)
	m.content.SetTheme(th)
	m.preview.SetTheme(th)
	m.graph.SetTheme(th)
	if persist && m.configPath != "" {
		cfg, _ := config.Load(m.configPath)
		cfg.Theme = th.Name
		if err := config.Save(m.configPath, cfg); err != nil {
			m.setError("Could not save theme: " + err.Error())
		}
	}
}

// setThemeByName applies and saves a theme, reporting unknown names.
func (m *Model) setThemeByName(name string) {
	th, ok := theme.Get(strings.ToLower(strings.TrimSpace(name)))
	if !ok {
		m.setError(fmt.Sprintf("E185: Cannot find color scheme '%s'", name))
		return
	}
	m.applyTheme(th, true)
	m.setStatus("Theme: " + th.Name)
}

// themePicker is NvChad's theme switcher: the highlighted theme is applied
// live; Enter keeps it, Esc restores the previous one.
type themePicker struct {
	input    textinput.Model
	all      []string
	list     []string
	sel      int
	offset   int
	original theme.Theme
}

func (m *Model) openThemePicker() tea.Cmd {
	th := m.theme
	in := textinput.New()
	in.Prompt = ""
	in.Placeholder = "Filter themes…"
	in.CharLimit = 40
	p := &themePicker{input: in, all: theme.Names(), original: th}
	p.filter()
	for i, n := range p.list {
		if n == th.Name {
			p.sel = i
		}
	}
	p.scrollTo(themeRows(m.height))
	m.themes = p
	m.leaderPending = false
	return p.input.Focus()
}

func themeRows(H int) int { return max(min(H-10, 18), 3) }

func (p *themePicker) filter() {
	q := strings.ToLower(strings.TrimSpace(p.input.Value()))
	p.list = p.list[:0]
	for _, n := range p.all {
		if q == "" || strings.Contains(n, q) {
			p.list = append(p.list, n)
		}
	}
	p.sel, p.offset = 0, 0
}

func (p *themePicker) scrollTo(rows int) {
	if p.sel < p.offset {
		p.offset = p.sel
	}
	if p.sel >= p.offset+rows {
		p.offset = p.sel - rows + 1
	}
}

func (p *themePicker) preview(m *Model) {
	if p.sel < 0 || p.sel >= len(p.list) {
		return
	}
	if th, ok := theme.Get(p.list[p.sel]); ok {
		m.applyTheme(th, false)
	}
}

func (p *themePicker) move(m *Model, delta int) {
	if len(p.list) == 0 {
		return
	}
	p.sel = (p.sel + delta + len(p.list)) % len(p.list)
	p.scrollTo(themeRows(m.height))
	p.preview(m)
}

func (p *themePicker) close(m *Model, keep bool) {
	m.themes = nil
	if !keep {
		m.applyTheme(p.original, false)
		return
	}
	m.applyTheme(m.theme, true)
	m.setStatus("Theme: " + m.theme.Name)
}

type themeGeom struct{ x, y, w, h int }

func (m *Model) themeLayout() themeGeom {
	w := min(52, m.width-2)
	h := themeRows(m.height) + 6
	x, y := ui.Center(m.width, m.height, w, h)
	return themeGeom{x, y, w, h}
}

func (p *themePicker) update(m *Model, msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			p.close(m, false)
			return nil
		case "enter":
			p.close(m, true)
			return nil
		case "down", "ctrl+n", "ctrl+j", "tab":
			p.move(m, 1)
			return nil
		case "up", "ctrl+p", "ctrl+k", "shift+tab":
			p.move(m, -1)
			return nil
		}
		before := p.input.Value()
		var cmd tea.Cmd
		p.input, cmd = p.input.Update(msg)
		if p.input.Value() != before {
			p.filter()
			p.preview(m)
		}
		return cmd

	case tea.MouseMsg:
		if msg.Action != tea.MouseActionPress {
			return nil
		}
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			p.move(m, -1)
		case tea.MouseButtonWheelDown:
			p.move(m, 1)
		case tea.MouseButtonLeft:
			g := m.themeLayout()
			if msg.X < g.x || msg.X >= g.x+g.w || msg.Y < g.y || msg.Y >= g.y+g.h {
				p.close(m, false)
				return nil
			}
			row := msg.Y - g.y - 1 - 3
			if idx := p.offset + row; row >= 0 && row < themeRows(m.height) && idx < len(p.list) {
				p.sel = idx
				p.preview(m)
				p.close(m, true)
			}
		}
		return nil

	default:
		var cmd tea.Cmd
		p.input, cmd = p.input.Update(msg)
		return cmd
	}
}

func (p *themePicker) view(m *Model) (string, int, int) {
	th := m.theme
	g := m.themeLayout()
	inner := g.w - 2
	bg := lipgloss.NewStyle().Background(th.DarkerBg)
	field := lipgloss.NewStyle().Background(th.Bg2)
	p.input.TextStyle = field.Foreground(th.Fg)
	p.input.PlaceholderStyle = field.Foreground(th.GreyFg)
	p.input.Cursor.Style = lipgloss.NewStyle().Foreground(th.Blue)
	p.input.Cursor.TextStyle = field.Foreground(th.Fg)
	p.input.Width = max(inner-8, 1)

	title := bg.Foreground(th.Blue).Render(" \U000f03d8 ") + bg.Foreground(th.Fg).Bold(true).Render("Themes")
	count := bg.Foreground(th.GreyFg2).Render(fmt.Sprintf("%d ", len(p.list)))
	rows := []string{
		title + bg.Render(strings.Repeat(" ", max(inner-lipgloss.Width(title)-lipgloss.Width(count), 0))) + count,
		bg.Render(" ") + ui.FitLine(field.Foreground(th.Blue).Bold(true).Render(" \uf002 ")+p.input.View(), inner-2, field) + bg.Render(" "),
		bg.Foreground(th.Line).Render(strings.Repeat("─", inner)),
	}

	n := themeRows(m.height)
	for r := 0; r < n; r++ {
		idx := p.offset + r
		if idx >= len(p.list) {
			rows = append(rows, "")
			continue
		}
		name := p.list[idx]
		t, _ := theme.Get(name)
		rowBg := th.DarkerBg
		if idx == p.sel {
			rowBg = th.OneBg2
		}
		base := lipgloss.NewStyle().Background(rowBg)
		marker := base.Render("  ")
		if idx == p.sel {
			marker = base.Foreground(th.Blue).Render("▌ ")
		}
		var sw strings.Builder
		chip := lipgloss.NewStyle().Background(t.Bg)
		sw.WriteString(chip.Render(" "))
		for _, c := range []lipgloss.Color{t.Red, t.Yellow, t.Green, t.Blue, t.Purple} {
			sw.WriteString(chip.Foreground(c).Render("●"))
		}
		sw.WriteString(chip.Render(" "))
		kind := "dark"
		if t.Light {
			kind = "light"
		}
		label := base.Foreground(th.Fg).Bold(idx == p.sel).Render(name)
		right := base.Foreground(th.GreyFg).Render(kind+" ") + sw.String() + base.Render(" ")
		gap := inner - lipgloss.Width(marker) - lipgloss.Width(label) - lipgloss.Width(right)
		rows = append(rows, marker+label+base.Render(strings.Repeat(" ", max(gap, 1)))+right)
	}
	rows = append(rows, bg.Foreground(th.GreyFg).Render(" ↑↓ preview · Enter apply · Esc revert"))
	return panel(th, rows, inner), g.x, g.y
}
