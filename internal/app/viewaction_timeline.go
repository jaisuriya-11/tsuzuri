package app

import (
	"time"

	"github.com/jaisuriya-11/tsuzuri/internal/note"
	"github.com/jaisuriya-11/tsuzuri/internal/preview"

	tea "github.com/charmbracelet/bubbletea"
)

// Timeline

func parseSpan(line string) (name string, start, end time.Time, ok bool) { return note.ParseSpan(line) }

func spanLine(name string, start, end time.Time) string { return note.SpanLine(name, start, end) }

func (m *Model) timelineHit(h preview.Hit) tea.Cmd {
	switch h.Kind {
	case "tl:add":
		return m.prompt("New timeline item", "", "Next you'll pick the start and end dates", func(m *Model, v string) tea.Cmd {
			if v == "" {
				return nil
			}
			m.pickDates(v, time.Time{}, time.Time{}, func(m *Model, start, end time.Time) {
				m.setDocLines(insertAt(m.docLines(), h.End, spanLine(v, start, end)))
			})
			return nil
		})
	case "tl:item":
		items := []string{"Move 1 day later", "Move 1 day earlier", "Move 1 week later", "Move 1 week earlier",
			"Extend by 1 day", "Shorten by 1 day", "Set dates…", "Rename", "Delete"}
		m.menu(h.Arg, items, func(m *Model, choice int) tea.Cmd {
			lines := m.docLines()
			name, start, end, ok := parseSpan(lines[h.Line])
			if !ok {
				return nil
			}
			shift := func(a, b int) {
				start, end = start.AddDate(0, 0, a), end.AddDate(0, 0, b)
				if end.Before(start) {
					end = start
				}
				lines[h.Line] = spanLine(name, start, end)
				m.setDocLines(lines)
			}
			switch choice {
			case 0:
				shift(1, 1)
			case 1:
				shift(-1, -1)
			case 2:
				shift(7, 7)
			case 3:
				shift(-7, -7)
			case 4:
				shift(0, 1)
			case 5:
				shift(0, -1)
			case 6:
				m.pickDates(name, start, end, func(m *Model, s, e time.Time) {
					lines := m.docLines()
					lines[h.Line] = spanLine(name, s, e)
					m.setDocLines(lines)
				})
			case 7:
				return m.prompt("Rename", name, "", func(m *Model, v string) tea.Cmd {
					if v != "" {
						lines := m.docLines()
						lines[h.Line] = spanLine(v, start, end)
						m.setDocLines(lines)
					}
					return nil
				})
			case 8:
				m.setDocLines(removeAt(lines, h.Line, 1))
			}
			return nil
		})
	}
	return nil
}

// pickDates asks for a start date, then an end date on or after it.
func (m *Model) pickDates(name string, start, end time.Time, done func(*Model, time.Time, time.Time)) {
	m.pickDate("Start date · "+name, start, time.Time{}, func(m *Model, s time.Time) tea.Cmd {
		initial := end
		if initial.IsZero() || initial.Before(s) {
			initial = s
		}
		m.pickDate("End date · "+name, initial, s, func(m *Model, e time.Time) tea.Cmd {
			done(m, s, e)
			return nil
		})
		return nil
	})
}
