package content

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Markdown pipe-table editing: add/delete rows and columns, move between
// cells with Tab, and keep columns aligned.

// tableCommands maps ex commands to table operations.
var tableCommands = map[string]string{
	"addrow": "addrow", "tr": "addrow",
	"addcol": "addcol", "tc": "addcol",
	"delrow": "delrow", "delcol": "delcol",
	"tablefmt": "format", "tf": "format",
}

func isTableLine(l string) bool { return strings.HasPrefix(strings.TrimSpace(l), "|") }

func isSeparator(cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	for _, c := range cells {
		t := strings.Trim(strings.TrimSpace(c), ":")
		if len(t) < 1 || strings.Trim(t, "-") != "" {
			return false
		}
	}
	return true
}

func splitCells(line string) []string {
	t := strings.TrimSpace(line)
	t = strings.TrimPrefix(t, "|")
	t = strings.TrimSuffix(t, "|")
	cells := strings.Split(t, "|")
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	return cells
}

// tableAt returns the line range [start, end) of the table holding row.
func tableAt(lines []string, row int) (int, int, bool) {
	if row < 0 || row >= len(lines) || !isTableLine(lines[row]) {
		return 0, 0, false
	}
	start, end := row, row+1
	for start > 0 && isTableLine(lines[start-1]) {
		start--
	}
	for end < len(lines) && isTableLine(lines[end]) {
		end++
	}
	return start, end, true
}

// formatTable pads every cell so the pipes line up.
func formatTable(rows [][]string) []string {
	cols := 0
	for _, r := range rows {
		cols = max(cols, len(r))
	}
	widths := make([]int, cols)
	for _, r := range rows {
		if isSeparator(r) {
			continue
		}
		for j, c := range r {
			widths[j] = max(widths[j], ansi.StringWidth(c))
		}
	}
	for j := range widths {
		widths[j] = max(widths[j], 3)
	}
	out := make([]string, len(rows))
	for i, r := range rows {
		var b strings.Builder
		b.WriteString("|")
		sep := isSeparator(r)
		for j := 0; j < cols; j++ {
			c := ""
			if j < len(r) {
				c = r[j]
			}
			if sep {
				left, right := strings.HasPrefix(c, ":"), strings.HasSuffix(c, ":") && len(c) > 1
				dash := strings.Repeat("-", widths[j])
				if left {
					dash = ":" + dash[1:]
				}
				if right {
					dash = dash[:len(dash)-1] + ":"
				}
				b.WriteString(" " + dash + " |")
				continue
			}
			b.WriteString(" " + c + strings.Repeat(" ", widths[j]-ansi.StringWidth(c)) + " |")
		}
		out[i] = b.String()
	}
	return out
}

// cellIndex returns which cell the rune column col falls in.
func cellIndex(line string, col int) int {
	runes := []rune(line)
	n := -1
	for i := 0; i < min(col, len(runes)); i++ {
		if runes[i] == '|' {
			n++
		}
	}
	return max(n, 0)
}

// cellStart returns the rune column where cell idx's text begins.
func cellStart(line string, idx int) int {
	n := -1
	for i, r := range []rune(line) {
		if r == '|' {
			n++
			if n == idx {
				return i + 2
			}
		}
	}
	return len([]rune(line))
}

// InTable reports whether the cursor is on a table line.
func (m Model) InTable() bool {
	row, _ := m.textarea.RowCol()
	_, _, ok := tableAt(strings.Split(m.textarea.Value(), "\n"), row)
	return ok
}

// TableEdit is the result of EditTable: lines [Start, End) of the note are
// replaced by Lines, and the cursor moves to Row, Col (a rune column).
type TableEdit struct {
	Start int      `json:"start"`
	End   int      `json:"end"`
	Lines []string `json:"lines"`
	Row   int      `json:"row"`
	Col   int      `json:"col"`
}

// EditTable applies a table operation ("addrow", "addcol", "delrow",
// "delcol" or "format") to the table holding row, with the cursor at rune
// column col, and re-aligns it. ok is false when row is not in a table. An
// edit that would remove the header, the separator or the last column
// leaves the table alone and returns an edit with no Lines.
func EditTable(lines []string, row, col int, op string) (edit TableEdit, ok bool) {
	start, end, ok := tableAt(lines, row)
	if !ok {
		return TableEdit{}, false
	}
	rows := make([][]string, 0, end-start)
	cols := 0
	for _, l := range lines[start:end] {
		cells := splitCells(l)
		rows = append(rows, cells)
		cols = max(cols, len(cells))
	}
	r := row - start
	// A cursor after the closing pipe still belongs to the last cell.
	c := min(cellIndex(lines[row], col), cols-1)
	sepIdx := -1
	if len(rows) > 1 && isSeparator(rows[1]) {
		sepIdx = 1
	}

	targetRow, targetCell := r, c
	switch op {
	case "addrow":
		at := max(r+1, sepIdx+1)
		rows = append(rows[:at], append([][]string{make([]string, cols)}, rows[at:]...)...)
		targetRow, targetCell = at, 0
	case "addcol":
		for i := range rows {
			for len(rows[i]) < cols {
				rows[i] = append(rows[i], "")
			}
			cell := ""
			if i == sepIdx {
				cell = "---"
			} else if i == 0 {
				cell = "Column"
			}
			rows[i] = append(rows[i][:c+1], append([]string{cell}, rows[i][c+1:]...)...)
		}
		targetCell = c + 1
	case "delrow":
		if r == sepIdx || (r == 0 && sepIdx >= 0) || len(rows) <= 1 {
			return TableEdit{Start: start, End: end}, true // keep the header and separator
		}
		rows = append(rows[:r], rows[r+1:]...)
		targetRow = min(r, len(rows)-1)
	case "delcol":
		if cols <= 1 {
			return TableEdit{Start: start, End: end}, true
		}
		for i := range rows {
			if c < len(rows[i]) {
				rows[i] = append(rows[i][:c], rows[i][c+1:]...)
			}
		}
		targetCell = min(c, cols-2)
	}

	formatted := formatTable(rows)
	if targetRow == sepIdx {
		targetRow++
	}
	targetRow = min(targetRow, len(formatted)-1)
	return TableEdit{
		Start: start,
		End:   end,
		Lines: formatted,
		Row:   start + targetRow,
		Col:   cellStart(formatted[targetRow], targetCell),
	}, true
}

// tableOp applies a table edit at the cursor. It returns false when the
// cursor is not in a table.
func (m *Model) tableOp(op string) bool {
	lines := strings.Split(m.textarea.Value(), "\n")
	row, col := m.textarea.RowCol()
	edit, ok := EditTable(lines, row, col, op)
	if !ok || edit.Lines == nil {
		return ok
	}
	out := append(append(append([]string{}, lines[:edit.Start]...), edit.Lines...), lines[edit.End:]...)
	m.textarea.SetValue(strings.Join(out, "\n"))
	m.textarea.SetRowCol(edit.Row, edit.Col)
	return true
}

// nextCell moves to the next (or previous) cell, adding a row after the
// last cell, as in most table editors.
func (m *Model) nextCell(forward bool) bool {
	lines := strings.Split(m.textarea.Value(), "\n")
	row, col := m.textarea.RowCol()
	start, end, ok := tableAt(lines, row)
	if !ok {
		return false
	}
	m.tableOp("format")
	lines = strings.Split(m.textarea.Value(), "\n")
	row, col = m.textarea.RowCol()
	cols := len(splitCells(lines[row]))
	c := cellIndex(lines[row], col)
	step := func(r, c int) (int, int) {
		if forward {
			c++
			if c >= cols {
				r, c = r+1, 0
			}
		} else {
			c--
			if c < 0 {
				r, c = r-1, cols-1
			}
		}
		return r, c
	}
	r, c := step(row, c)
	for r >= start && r < end && isSeparator(splitCells(lines[r])) {
		r, c = step(r, c)
	}
	switch {
	case r >= end:
		m.textarea.SetRowCol(end-1, 0)
		m.tableOp("addrow")
	case r < start:
		m.textarea.SetRowCol(start, cellStart(lines[start], 0))
	default:
		m.textarea.SetRowCol(r, cellStart(lines[r], c))
	}
	return true
}

// TableOpAt runs a table operation ("addrow", "addcol", "delrow", "delcol",
// "format") as if the cursor were in cell idx of document line row.
func (m *Model) TableOpAt(row, idx int, op string) bool {
	lines := strings.Split(m.textarea.Value(), "\n")
	if row < 0 || row >= len(lines) {
		return false
	}
	m.textarea.SetRowCol(row, cellStart(lines[row], idx))
	return m.tableOp(op)
}

// SetTableCell replaces the text of cell idx on document line row and
// re-aligns the table.
func (m *Model) SetTableCell(row, idx int, text string) bool {
	lines := strings.Split(m.textarea.Value(), "\n")
	if row < 0 || row >= len(lines) || !isTableLine(lines[row]) {
		return false
	}
	cells := splitCells(lines[row])
	for len(cells) <= idx {
		cells = append(cells, "")
	}
	cells[idx] = strings.ReplaceAll(strings.TrimSpace(text), "|", "\\|")
	lines[row] = "| " + strings.Join(cells, " | ") + " |"
	m.textarea.SetValue(strings.Join(lines, "\n"))
	return m.TableOpAt(row, idx, "format")
}
