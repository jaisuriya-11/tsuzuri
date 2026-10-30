package preview

import (
	"fmt"
	"strings"

	"github.com/jaisuriya-11/tsuzuri/internal/theme"
	"github.com/jaisuriya-11/tsuzuri/internal/ui"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type boardCard struct {
	title string
	desc  []string
	line  int
}

type boardColumn struct {
	name  string
	line  int
	cards []boardCard
}

// statusTone reads what a board column's name says about its cards:
// "done", "progress", "todo", "blocked", or "" for none of these.
func statusTone(name string) string {
	n := strings.ToLower(name)
	switch {
	case strings.Contains(n, "done") || strings.Contains(n, "complete") || strings.Contains(n, "shipped"):
		return "done"
	case strings.Contains(n, "progress") || strings.Contains(n, "doing") || strings.Contains(n, "review"):
		return "progress"
	case strings.Contains(n, "not") || strings.Contains(n, "todo") || strings.Contains(n, "backlog"):
		return "todo"
	case strings.Contains(n, "block") || strings.Contains(n, "stuck"):
		return "blocked"
	}
	return ""
}

func statusColor(name string, i int, th theme.Theme) lipgloss.Color {
	switch statusTone(name) {
	case "done":
		return th.Green
	case "progress":
		return th.Blue
	case "todo":
		return th.GreyFg2
	case "blocked":
		return th.Red
	}
	return palette(th)[(i+2)%len(palette(th))]
}

// parseBoard reads "## Column" headings and "- card" lines; other text
// under a card is its description.
func parseBoard(body []string) []boardColumn {
	var cols []boardColumn
	for i, l := range body {
		t := strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(t, "#"):
			cols = append(cols, boardColumn{name: strings.TrimSpace(strings.TrimLeft(t, "#")), line: i})
		case strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* "):
			if len(cols) == 0 {
				cols = append(cols, boardColumn{name: "Cards", line: -1})
			}
			card := strings.TrimSpace(t[2:])
			card = strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(card, "[ ] "), "[x] "), "[X] ")
			cols[len(cols)-1].cards = append(cols[len(cols)-1].cards, boardCard{title: card, line: i})
		case t != "" && t != "-" && len(cols) > 0 && len(cols[len(cols)-1].cards) > 0:
			// Any other text under a card is that card's description.
			c := &cols[len(cols)-1]
			last := &c.cards[len(c.cards)-1]
			last.desc = append(last.desc, t)
		}
	}
	return cols
}

func renderBoard(body []string, th theme.Theme, width int, hs *[]Hit) []string {
	cols := parseBoard(body)
	if len(cols) == 0 {
		return []string{lipgloss.NewStyle().Foreground(th.GreyFg).Render("(empty board: add \"## Column\" headings and \"- card\" lines)")}
	}

	gap := 2
	colW := min(30, (width-gap*(len(cols)-1))/len(cols))
	stacked := colW < 16
	if stacked {
		colW = min(width, 40)
	}

	var blocks []string
	type colHits struct{ hits []Hit }
	perCol := make([]colHits, len(cols))
	for i, c := range cols {
		color := statusColor(c.name, i, th)
		pill := lipgloss.NewStyle().Background(color).Foreground(th.Bg).Bold(true).Render(" ● " + ui.Truncate(c.name, colW-8) + " ")
		count := lipgloss.NewStyle().Foreground(color).Render(fmt.Sprintf(" %d", len(c.cards)))
		lines := []string{pill + count}
		perCol[i].hits = append(perCol[i].hits, Hit{Row: 0, H: 1, X1: colW, Kind: "board:col", Line: c.line, Index: i, Arg: c.name})
		card := lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(th.Line).
			Foreground(th.Fg).
			Width(colW-2).
			Padding(0, 1)
		inner := colW - 4
		for _, cd := range c.cards {
			body := lipgloss.NewStyle().Bold(true).Render(ui.Truncate(cd.title, inner*2))
			if len(cd.desc) > 0 {
				desc := strings.Split(ansi.Wrap(strings.Join(cd.desc, " "), inner, ""), "\n")
				if len(desc) > 4 {
					desc = append(desc[:3], ui.Truncate(desc[3], inner-1)+"…")
				}
				body += "\n" + lipgloss.NewStyle().Foreground(th.GreyFg2).Render(strings.Join(desc, "\n"))
			}
			rendered := strings.Split(card.Render(body), "\n")
			perCol[i].hits = append(perCol[i].hits, Hit{Row: len(lines), H: len(rendered), X1: colW, Kind: "board:card", Line: cd.line, Index: i, Arg: cd.title})
			lines = append(lines, rendered...)
		}
		perCol[i].hits = append(perCol[i].hits, Hit{Row: len(lines), H: 1, X1: colW, Kind: "board:add", Line: c.line, Index: i, Arg: c.name})
		lines = append(lines, lipgloss.NewStyle().Foreground(color).Render(" + New page"))
		for j := range lines {
			lines[j] = pad(lines[j], colW)
		}
		blocks = append(blocks, strings.Join(lines, "\n"))
	}
	if stacked {
		row := 0
		for i, b := range blocks {
			for _, h := range perCol[i].hits {
				h.Row += row
				addHit(hs, h)
			}
			row += strings.Count(b, "\n") + 2
		}
		return strings.Split(strings.Join(blocks, "\n\n"), "\n")
	}
	for i := range blocks {
		for _, h := range perCol[i].hits {
			h.X0 += i * (colW + gap)
			h.X1 += i * (colW + gap)
			addHit(hs, h)
		}
	}
	spacer := strings.Repeat(" ", gap)
	parts := []string{}
	for i, b := range blocks {
		if i > 0 {
			parts = append(parts, spacer)
		}
		parts = append(parts, b)
	}
	return strings.Split(lipgloss.JoinHorizontal(lipgloss.Top, parts...), "\n")
}
