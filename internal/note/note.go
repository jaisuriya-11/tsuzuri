// Package note edits a note's Markdown line by line: moving blocks and
// board cards. The app and editor integrations (through the bridge) share
// it, so a drag does the same thing wherever it happens.
package note

import (
	"regexp"
	"strings"
)

// Insert returns lines with add inserted before line at.
func Insert(lines []string, at int, add ...string) []string {
	at = max(0, min(at, len(lines)))
	out := append([]string{}, lines[:at]...)
	out = append(out, add...)
	return append(out, lines[at:]...)
}

// Remove returns lines without the n lines starting at at.
func Remove(lines []string, at, n int) []string {
	if at < 0 || at >= len(lines) {
		return lines
	}
	end := min(at+n, len(lines))
	return append(append([]string{}, lines[:at]...), lines[end:]...)
}

// MoveBlock moves lines [from, end) so they start where line to was,
// keeping one blank line between the block and its new neighbours (so a
// paragraph doesn't merge into the text next to it) and not leaving a double
// blank line behind.
func MoveBlock(lines []string, from, end, to int) []string {
	from, end = max(0, from), min(end, len(lines))
	if from >= end || (to >= from && to <= end) {
		return lines
	}
	chunk := append([]string{}, lines[from:end]...)
	rest := Remove(lines, from, end-from)
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
		rest = Remove(rest, gone, 1)
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
	return Insert(rest, to, add...)
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

// BoardColumn is one column of a board block, by note line.
type BoardColumn struct {
	Name    string
	Line    int   // heading line, -1 if implicit
	EndLine int   // where new cards go (before the next heading / fence)
	Cards   []int // the line of each card
}

// BoardColumns reads the columns of the board between its opening fence
// line block and its closing fence line end.
func BoardColumns(lines []string, block, end int) []BoardColumn {
	var cols []BoardColumn
	for i := block + 1; i < end && i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		switch {
		case strings.HasPrefix(t, "#"):
			cols = append(cols, BoardColumn{Name: strings.TrimSpace(strings.TrimLeft(t, "#")), Line: i})
		case strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* "):
			if len(cols) == 0 {
				cols = append(cols, BoardColumn{Name: "Cards", Line: -1})
			}
			cols[len(cols)-1].Cards = append(cols[len(cols)-1].Cards, i)
		}
	}
	for c := range cols {
		stop := end
		if c+1 < len(cols) {
			stop = cols[c+1].Line
		}
		// New cards go after the last non-blank line of the column.
		for stop-1 > cols[c].Line && stop-1 > block && strings.TrimSpace(lines[stop-1]) == "" {
			stop--
		}
		cols[c].EndLine = stop
	}
	return cols
}

// CardExtent is the number of lines a card takes: its line plus the
// description lines under it.
func CardExtent(lines []string, at, end int) int {
	n := 1
	for i := at + 1; i < end && i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if strings.HasPrefix(t, "#") || strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ") || t == "" {
			break
		}
		n++
	}
	return n
}

// MoveCard moves the card on line card of the board [block, end) to
// column col, before the card now at position index there (or last when
// index is past the end). ok is false when the card or column isn't there.
func MoveCard(lines []string, block, end, card, col, index int) (out []string, ok bool) {
	cols := BoardColumns(lines, block, end)
	found := false
	for _, c := range cols {
		for _, l := range c.Cards {
			found = found || l == card
		}
	}
	if !found || col < 0 || col >= len(cols) {
		return lines, false
	}

	n := CardExtent(lines, card, end)
	moved := append([]string{}, lines[card:card+n]...)
	rest := Remove(lines, card, n)
	cols = BoardColumns(rest, block, end-n)
	target := cols[col]
	at := target.EndLine
	if index >= 0 && index < len(target.Cards) {
		at = target.Cards[index]
	}
	return Insert(rest, at, moved...), true
}
