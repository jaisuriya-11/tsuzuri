package app

import (
	"strings"
	"time"

	"github.com/jaisuriya-11/tsuzuri/internal/note"
	"github.com/jaisuriya-11/tsuzuri/internal/preview"

	tea "github.com/charmbracelet/bubbletea"
)

// questionTypes are the choices offered when adding a form question:
// label shown, type tag written in the Markdown, whether it has options.
var questionTypes = []struct {
	label, tag string
	options    bool
}{
	{"Short answer", "", false},
	{"Long answer", "long", false},
	{"Single choice  ○", "choice", true},
	{"Multiple choice  ☐", "multi", true},
	{"Rating  ☆☆☆☆☆", "rating", false},
	{"Date", "date", false},
	{"Email", "email", false},
}

// chooseQuestionType asks for a question type (and options when needed) and
// hands the finished "? …" line to done.
func (m *Model) chooseQuestionType(text, options string, done func(*Model, string)) tea.Cmd {
	labels := make([]string, len(questionTypes))
	for i, t := range questionTypes {
		labels[i] = t.label
	}
	m.menu("Type of \""+text+"\"", labels, func(m *Model, i int) tea.Cmd {
		t := questionTypes[i]
		line := "? " + text
		if t.tag != "" {
			line += " (" + t.tag + ")"
		}
		if !t.options {
			done(m, line)
			return nil
		}
		if options == "" {
			options = "Option 1 | Option 2 | Option 3"
		}
		return m.prompt("Options for \""+text+"\"", options, "Separate options with | or commas", func(m *Model, v string) tea.Cmd {
			var opts []string
			for _, o := range strings.FieldsFunc(v, func(r rune) bool { return r == '|' || r == ',' }) {
				if o = strings.TrimSpace(o); o != "" {
					opts = append(opts, o)
				}
			}
			if len(opts) == 0 {
				opts = []string{"Option 1"}
			}
			done(m, line+": "+strings.Join(opts, " | "))
			return nil
		})
	})
	return nil
}

func answerLine(lines []string, q int) int { return note.AnswerLine(lines, q) }

func (m *Model) setAnswer(q int, value string) { m.setDocLines(note.SetAnswer(m.docLines(), q, value)) }

func (m *Model) currentAnswer(q int) string { return note.Answer(m.docLines(), q) }

func (m *Model) formHit(h preview.Hit) tea.Cmd {
	lines := m.docLines()
	switch h.Kind {
	case "form:title", "form:desc":
		key, label := "title", "Form title"
		if h.Kind == "form:desc" {
			key, label = "description", "Description"
		}
		return m.prompt(label, h.Arg, "", func(m *Model, v string) tea.Cmd {
			lines := m.docLines()
			if h.Line >= 0 {
				lines[h.Line] = key + ": " + v
			} else {
				lines = insertAt(lines, h.Block+1, key+": "+v)
			}
			m.setDocLines(lines)
			return nil
		})

	case "form:text":
		if strings.Contains(strings.ToLower(lines[h.Line]), "(date)") {
			current, _ := time.Parse("2006-01-02", m.currentAnswer(h.Line))
			m.pickDate(h.Arg, current, time.Time{}, func(m *Model, d time.Time) tea.Cmd {
				m.setAnswer(h.Line, d.Format("2006-01-02"))
				return nil
			})
			return nil
		}
		return m.prompt(h.Arg, m.currentAnswer(h.Line), "Your answer (saved in the note as \"= answer\")", func(m *Model, v string) tea.Cmd {
			m.setAnswer(h.Line, v)
			return nil
		})

	case "form:option":
		m.setDocLines(note.PickAnswer(lines, h.Line, "pick", h.Index))

	case "form:multi":
		m.setDocLines(note.PickAnswer(lines, h.Line, "toggle", h.Index))

	case "form:rating":
		m.setDocLines(note.PickAnswer(lines, h.Line, "rate", h.Index))

	case "form:addopt":
		return m.prompt("New option for "+h.Arg, "", "", func(m *Model, v string) tea.Cmd {
			if v != "" {
				m.setDocLines(note.AddOption(m.docLines(), h.Line, v))
			}
			return nil
		})

	case "form:question":
		required := strings.HasPrefix(strings.TrimSpace(lines[h.Line]), "?*")
		toggle := "Make required"
		if required {
			toggle = "Make optional"
		}
		const (
			rename = iota
			changeType
			toggleRequired
			clearAnswer
			deleteQuestion
		)
		m.menu(h.Arg, []string{"Rename", "Change type", toggle, "Clear answer", "Delete question"}, func(m *Model, choice int) tea.Cmd {
			lines := m.docLines()
			switch choice {
			case rename:
				return m.prompt("Rename question", h.Arg, "", func(m *Model, v string) tea.Cmd {
					if v == "" {
						return nil
					}
					lines := m.docLines()
					lines[h.Line] = strings.Replace(lines[h.Line], h.Arg, v, 1)
					m.setDocLines(lines)
					return nil
				})
			case changeType:
				prefix := "? "
				if required {
					prefix = "?* "
				}
				opts := strings.Join(note.ParseQuestion(lines[h.Line]).Options, " | ")
				return m.chooseQuestionType(h.Arg, opts, func(m *Model, line string) {
					lines := m.docLines()
					lines[h.Line] = prefix + strings.TrimPrefix(line, "? ")
					if at := answerLine(lines, h.Line); at >= 0 {
						lines = removeAt(lines, at, 1) // the old answer may not fit the new type
					}
					m.setDocLines(lines)
				})
			case toggleRequired:
				t := strings.TrimSpace(lines[h.Line])
				if required {
					lines[h.Line] = "?" + strings.TrimPrefix(t, "?*")
				} else {
					lines[h.Line] = "?*" + strings.TrimPrefix(t, "?")
				}
				m.setDocLines(lines)
			case clearAnswer:
				m.setAnswer(h.Line, "")
			case deleteQuestion:
				n := 1
				if answerLine(lines, h.Line) >= 0 {
					n = 2
				}
				m.setDocLines(removeAt(lines, h.Line, n))
			}
			return nil
		})

	case "form:addq":
		return m.prompt("New question", "", "Next you'll pick its type (text, choice, rating…)", func(m *Model, v string) tea.Cmd {
			if v == "" {
				return nil
			}
			return m.chooseQuestionType(v, "", func(m *Model, line string) {
				m.setDocLines(insertAt(m.docLines(), h.End, line))
			})
		})
	}
	return nil
}
