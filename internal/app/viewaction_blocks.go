package app

import (
	"regexp"
	"strings"

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

// moveBlock moves lines [from, end) so they start where line to was,
// keeping one blank line between the block and its new neighbours (so a
// paragraph doesn't merge into the text next to it) and not leaving a double
// blank line behind.
func moveBlock(lines []string, from, end, to int) []string {
	from, end = max(0, from), min(end, len(lines))
	if from >= end || (to >= from && to <= end) {
		return lines
	}
	chunk := append([]string{}, lines[from:end]...)
	rest := removeAt(lines, from, end-from)
	if to > end {
		to -= end - from
	}
	to = max(0, min(to, len(rest)))
	// Drop the blank line left where the block was: a doubled one in the
	// middle, or one now at the very start or end of the note.
	blank := func(i int) bool { return i >= 0 && i < len(rest) && strings.TrimSpace(rest[i]) == "" }
	gone := -1
	switch {
	case blank(from-1) && blank(from):
		gone = from
	case from == len(rest) && blank(from-1):
		gone = from - 1
	case from == 0 && blank(0):
		gone = 0
	}
	if gone >= 0 {
		rest = removeAt(rest, gone, 1)
		if to > gone {
			to--
		}
	}

	add := chunk
	if to > 0 && needsGap(rest[to-1], chunk[0]) {
		add = append([]string{""}, add...)
	}
	if to < len(rest) && needsGap(chunk[len(chunk)-1], rest[to]) {
		add = append(add, "")
	}
	return insertAt(rest, to, add...)
}

var listLine = regexp.MustCompile(`^\s*([-*+]|\d+[.)])\s`)

// needsGap reports whether two neighbouring lines need a blank line between
// them to stay separate blocks. List items and quotes stack without one.
func needsGap(above, below string) bool {
	a, b := strings.TrimSpace(above), strings.TrimSpace(below)
	if a == "" || b == "" {
		return false
	}
	if listLine.MatchString(above) && listLine.MatchString(below) {
		return false
	}
	return !(strings.HasPrefix(a, ">") && strings.HasPrefix(b, ">"))
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

func insertAt(lines []string, at int, add ...string) []string {
	at = max(0, min(at, len(lines)))
	out := append([]string{}, lines[:at]...)
	out = append(out, add...)
	return append(out, lines[at:]...)
}

func removeAt(lines []string, at, n int) []string {
	if at < 0 || at >= len(lines) {
		return lines
	}
	end := min(at+n, len(lines))
	return append(append([]string{}, lines[:at]...), lines[end:]...)
}

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
