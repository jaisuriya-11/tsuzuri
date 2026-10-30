package app

import (
	"strings"

	"github.com/jaisuriya-11/tsuzuri/internal/note"
	"github.com/jaisuriya-11/tsuzuri/internal/preview"

	tea "github.com/charmbracelet/bubbletea"
)

// Board

type boardCol struct {
	name     string
	line     int // heading line, -1 if implicit
	endLine  int // where new cards go (before the next heading / fence)
	cardLine []int
}

func boardColumns(lines []string, h preview.Hit) []boardCol {
	var cols []boardCol
	for _, c := range note.BoardColumns(lines, h.Block, h.End) {
		cols = append(cols, boardCol{name: c.Name, line: c.Line, endLine: c.EndLine, cardLine: c.Cards})
	}
	return cols
}

// cardExtent is the card line plus its description lines.
func cardExtent(lines []string, at, end int) int { return note.CardExtent(lines, at, end) }

func (m *Model) boardHit(h preview.Hit) tea.Cmd {
	lines := m.docLines()
	cols := boardColumns(lines, h)
	switch h.Kind {
	case "board:add":
		return m.prompt("New card in "+h.Arg, "", "Lines you add under the card in the editor become its description", func(m *Model, v string) tea.Cmd {
			if v == "" {
				return nil
			}
			lines := m.docLines()
			cols := boardColumns(lines, h)
			at := h.End
			if h.Index < len(cols) {
				at = cols[h.Index].endLine
			}
			m.setDocLines(insertAt(lines, at, "- "+v))
			return nil
		})

	case "board:card":
		items := []string{"Rename"}
		var targets []int
		for i, c := range cols {
			if i != h.Index {
				items = append(items, "Move to → "+c.name)
				targets = append(targets, i)
			}
		}
		items = append(items, "Delete")
		m.menu(h.Arg, items, func(m *Model, choice int) tea.Cmd {
			lines := m.docLines()
			n := cardExtent(lines, h.Line, h.End)
			switch {
			case choice == 0:
				return m.prompt("Rename card", h.Arg, "", func(m *Model, v string) tea.Cmd {
					if v == "" {
						return nil
					}
					lines := m.docLines()
					indent := lines[h.Line][:len(lines[h.Line])-len(strings.TrimLeft(lines[h.Line], " "))]
					m.setDocLines(append(append(lines[:h.Line:h.Line], indent+"- "+v), lines[h.Line+1:]...))
					return nil
				})
			case choice == len(items)-1:
				m.setDocLines(removeAt(lines, h.Line, n))
				m.setStatus("Card deleted")
			default:
				card := append([]string{}, lines[h.Line:h.Line+n]...)
				lines = removeAt(lines, h.Line, n)
				hh := h
				hh.End -= n
				cols := boardColumns(lines, hh)
				to := targets[choice-1]
				if to < len(cols) {
					m.setDocLines(insertAt(lines, cols[to].endLine, card...))
					m.setStatus("Moved to " + cols[to].name)
				}
			}
			return nil
		})

	case "board:col":
		if h.Line < 0 {
			return nil
		}
		m.menu(h.Arg, []string{"Add card", "Rename column", "Delete column and its cards"}, func(m *Model, choice int) tea.Cmd {
			switch choice {
			case 0:
				hh := h
				hh.Kind = "board:add"
				return m.boardHit(hh)
			case 1:
				return m.prompt("Rename column", h.Arg, "", func(m *Model, v string) tea.Cmd {
					if v != "" {
						lines := m.docLines()
						hashes := strings.TrimSpace(lines[h.Line])
						hashes = hashes[:len(hashes)-len(strings.TrimLeft(hashes, "#"))]
						lines[h.Line] = hashes + " " + v
						m.setDocLines(lines)
					}
					return nil
				})
			default:
				lines := m.docLines()
				cols := boardColumns(lines, h)
				if h.Index < len(cols) {
					end := h.End
					if h.Index+1 < len(cols) {
						end = cols[h.Index+1].line
					}
					m.setDocLines(removeAt(lines, h.Line, end-h.Line))
				}
			}
			return nil
		})
	}
	return nil
}
