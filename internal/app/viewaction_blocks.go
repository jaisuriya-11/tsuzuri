package app

import (
	"strings"

	"github.com/jaisuriya-11/tsuzuri/internal/note"
	"github.com/jaisuriya-11/tsuzuri/internal/preview"

	tea "github.com/charmbracelet/bubbletea"
)

// Editing the note's lines

func (m *Model) docLines() []string { return strings.Split(m.content.Value(), "\n") }

func (m *Model) setDocLines(lines []string) {
	m.content.ReplaceText(strings.Join(lines, "\n"))
	m.preview.SetContent(m.content.Value())
	m.refreshModified()
}

// moveBlock moves lines [from, end) so they start where line to was; see
// note.MoveBlock.
func moveBlock(lines []string, from, end, to int) []string {
	return note.MoveBlock(lines, from, end, to)
}

// addBlockAfter opens an empty line after document line after (Notion's
// "+"), switches to the editor there and opens the "/" block menu.
func (m *Model) addBlockAfter(after int) tea.Cmd {
	if m.activeBuffer() == nil {
		return nil
	}
	lines := m.docLines()
	after = max(0, min(after, len(lines)))
	add := []string{"", ""}
	if after < len(lines) && strings.TrimSpace(lines[after]) == "" {
		add = []string{""}
	}
	m.setDocLines(insertAt(lines, after, add...))
	cmds := []tea.Cmd{m.focusPane(focusEditor)}
	m.content.GotoLine(after + len(add))
	cmds = append(cmds, m.content.EnterInsert())
	var cmd tea.Cmd
	m.content, cmd = m.content.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	return tea.Batch(append(cmds, cmd)...)
}

func insertAt(lines []string, at int, add ...string) []string { return note.Insert(lines, at, add...) }

func removeAt(lines []string, at, n int) []string { return note.Remove(lines, at, n) }

// ---------------------------------------------------------------------------
// Dispatch

func (m *Model) handleViewHit(h preview.Hit) tea.Cmd {
	if m.activeBuffer() == nil {
		return nil
	}
	switch {
	case strings.HasPrefix(h.Kind, "board:"):
		return m.boardHit(h)
	case strings.HasPrefix(h.Kind, "cal:"):
		return m.calendarHit(h)
	case strings.HasPrefix(h.Kind, "tl:"):
		return m.timelineHit(h)
	case strings.HasPrefix(h.Kind, "chart:"):
		return m.chartHit(h)
	case strings.HasPrefix(h.Kind, "form:"):
		return m.formHit(h)
	case strings.HasPrefix(h.Kind, "table:"):
		return m.tableHit(h)
	case strings.HasPrefix(h.Kind, "link:"):
		return m.followHit(h)
	}
	return nil
}
