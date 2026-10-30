package preview

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jaisuriya-11/tsuzuri/internal/highlight"
	"github.com/jaisuriya-11/tsuzuri/internal/ui"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// callout recognises GitHub-style "[!NOTE]" admonitions.
func callout(s string) (string, string, bool) {
	t := strings.TrimSpace(s)
	for _, k := range []string{"NOTE", "TIP", "IMPORTANT", "WARNING", "CAUTION"} {
		tag := "[!" + k + "]"
		if strings.HasPrefix(strings.ToUpper(t), tag) {
			icons := map[string]string{"NOTE": "󰋽 ", "TIP": "󰌶 ", "IMPORTANT": "󰅾 ", "WARNING": "󰀪 ", "CAUTION": "󰳦 "}
			return icons[k] + k[:1] + strings.ToLower(k[1:]), t[len(tag):], true
		}
	}
	return "", "", false
}

func (c *compiler) codeBlock(lang string, code []string) {
	if l := strings.ToLower(strings.TrimSpace(lang)); l == "columns" || l == "cols" {
		c.columns(code)
		return
	}
	if lines, hits, ok := renderBlock(lang, code, c.st.th, c.width, c.cal); ok {
		c.blank()
		base := len(c.out)
		for _, h := range hits {
			h.Row += base
			if h.Line >= 0 {
				h.Line += c.fence + 1
			}
			h.Block, h.End = c.fence, c.fenceEnd
			c.hits = append(c.hits, h)
		}
		for _, l := range lines {
			if ansi.StringWidth(l) > c.width {
				l = ansi.Truncate(l, c.width, "")
			}
			c.out = append(c.out, l) // keep blank rows inside the block
		}
		c.blank()
		return
	}
	c.blank()
	key := c.foldKey("c", lang)
	label := lang
	if label == "" {
		label = "code"
	}
	plural := "s"
	if len(code) == 1 {
		plural = ""
	}
	info := fmt.Sprintf("%s · %d line%s", label, len(code), plural)
	c.hits = append(c.hits, Hit{Row: len(c.out), H: 1, X1: c.width, Kind: "fold", Arg: key, Line: -1})
	if c.folded(key) {
		c.emit(c.st.codeLang.Render("▸ " + info))
		c.blank()
		return
	}
	c.emit(c.st.codeLang.Render("▾ " + info))
	inner := max(c.width-4, 6)
	colors := highlight.Code(lang, strings.Join(code, "\n"), c.st.th)
	text := lipgloss.NewStyle().Foreground(c.st.th.Fg)
	for i, l := range code {
		painted := highlight.Render([]rune(l), colors[i], text)
		if ansi.StringWidth(painted) > inner {
			painted = ansi.Truncate(painted, inner-1, "…")
		}
		code[i] = painted
	}
	box := c.st.codeBox.Width(inner + 2).Render(strings.Join(code, "\n"))
	c.emit(strings.Split(box, "\n")...)
	c.blank()
}

// columnSep splits a columns block into its columns.
const columnSep = "+++"

// columnGap is the space between neighbouring columns.
const columnGap = 3

// columns lays out a columns block: each "+++"-separated part is compiled as
// its own Markdown and drawn side by side. Panes too narrow for the columns
// stack them instead.
func (c *compiler) columns(code []string) {
	type part struct {
		start int // index in code of the part's first line
		lines []string
	}
	parts := []part{{}}
	for i, l := range code {
		if strings.TrimSpace(l) == columnSep {
			parts = append(parts, part{start: i + 1})
			continue
		}
		cur := &parts[len(parts)-1]
		cur.lines = append(cur.lines, l)
	}

	n := len(parts)
	colW := (c.width - columnGap*(n-1)) / n
	stacked := colW < 16
	if stacked {
		colW = c.width
	}

	c.blank()
	base := len(c.out)
	var cols [][]string
	for i, p := range parts {
		out, hits := CompileHits(strings.Join(p.lines, "\n"), c.st.th, colW, c.baseDir, c.cal)
		x := i * (colW + columnGap)
		if stacked {
			x = 0
		}
		off := c.fence + 1 + p.start // document line of this part's first line
		for _, h := range hits {
			h.Row += base
			if !stacked {
				h.X0 += x
				h.X1 += x
			}
			if h.Line >= 0 {
				h.Line += off
			}
			if h.Kind != "fold" {
				h.Block += off
				h.End += off
			}
			if stacked {
				h.Row += len(c.out) - base
			}
			c.hits = append(c.hits, h)
		}
		lines := strings.Split(out, "\n")
		if stacked {
			c.out = append(c.out, lines...)
			if i < n-1 {
				c.out = append(c.out, "")
			}
			continue
		}
		cols = append(cols, lines)
	}

	rows := 0
	for _, col := range cols {
		rows = max(rows, len(col))
	}
	gap := strings.Repeat(" ", columnGap)
	for r := 0; r < rows; r++ {
		var b strings.Builder
		for i, col := range cols {
			l := ""
			if r < len(col) {
				l = col[r]
			}
			if ansi.StringWidth(l) > colW {
				l = ansi.Truncate(l, colW, "")
			}
			if i > 0 {
				b.WriteString(gap)
			}
			if i < len(cols)-1 {
				l = pad(l, colW)
			}
			b.WriteString(l)
		}
		c.out = append(c.out, strings.TrimRight(b.String(), " "))
	}
	c.blank()
}

// header draws the page cover banner and icon from front matter.
func (c *compiler) header(meta map[string]string) {
	if src := meta["cover"]; src != "" {
		path, err := resolveImage(strings.Trim(src, "<>"), c.baseDir)
		var lines []string
		if err == nil {
			lines, err = renderCover(path, c.width, c.st.th.Bg)
		}
		if err != nil {
			c.emit(lipgloss.NewStyle().Foreground(c.st.th.Purple).Render("󰋩 cover") + c.st.muted.Render(" ("+err.Error()+")"))
		} else {
			c.emit(lines...)
		}
		c.blank()
	}
	if icon := meta["icon"]; icon != "" {
		c.emit(" " + icon)
		c.blank()
	}
}

func (c *compiler) image(alt, src string) {
	label := lipgloss.NewStyle().Foreground(c.st.th.Purple)
	path, err := resolveImage(src, c.baseDir)
	var lines []string
	if err == nil {
		lines, err = renderImage(path, c.width, c.st.th.Bg)
	}
	if err != nil {
		name := orDefault(alt, filepath.Base(src))
		c.emit(label.Render("󰋩 "+name) + c.st.muted.Render(" ("+err.Error()+")"))
		return
	}
	c.blank()
	c.emit(lines...)
	if strings.TrimSpace(alt) != "" {
		c.emit(c.st.muted.Italic(true).Render(ui.Truncate(alt, c.width)))
	}
	c.blank()
}

// foldKey identifies a heading or code block by kind, text and occurrence,
// so folds survive edits elsewhere in the note.
func (c *compiler) foldKey(kind, text string) string {
	if c.seen == nil {
		c.seen = map[string]int{}
	}
	k := kind + ":" + text
	c.seen[k]++
	return fmt.Sprintf("%s#%d", k, c.seen[k])
}

func (c *compiler) folded(key string) bool {
	if c.cal.FoldAll {
		return !c.cal.Folded[key] // in fold-all mode the map lists the opened ones
	}
	return c.cal.Folded[key]
}

// sectionLength counts the lines under heading i until the next heading of
// the same or a higher level.
func sectionLength(lines []string, i, level int) int {
	n := 0
	fence := ""
	for j := i + 1; j < len(lines); j++ {
		t := strings.TrimSpace(lines[j])
		if fence == "" && headingRegex.MatchString(t) && len(headingRegex.FindStringSubmatch(t)[1]) <= level {
			break
		}
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			if fence == "" {
				fence = t[:3]
			} else if strings.HasPrefix(t, fence) {
				fence = ""
			}
		}
		if t != "" {
			n++
		}
	}
	return n
}

func splitRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	cells := strings.Split(line, "|")
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	return cells
}

// cell renders a table cell; one starting with "[ ]" or "[x]" shows a
// checkbox, like a to-do.
func (c *compiler) cell(text string, st lipgloss.Style) string {
	switch {
	case strings.HasPrefix(text, "[ ] ") || text == "[ ]":
		return lipgloss.NewStyle().Foreground(c.st.th.GreyFg2).Render("󰄱 ") + c.inline(strings.TrimPrefix(text[3:], " "), st)
	case strings.HasPrefix(strings.ToLower(text), "[x] ") || strings.ToLower(text) == "[x]":
		return lipgloss.NewStyle().Foreground(c.st.th.Green).Render("󰄲 ") + c.inline(strings.TrimPrefix(text[3:], " "), c.st.muted.Strikethrough(true))
	}
	return c.inline(text, st)
}

// table draws a pipe table whose header is on document line docStart and
// records clickable cells plus "add row / add column" buttons.
func (c *compiler) table(rows [][]string, docStart int) {
	cols := 0
	for _, r := range rows {
		cols = max(cols, len(r))
	}
	rendered := make([][]string, len(rows))
	widths := make([]int, cols)
	for i, r := range rows {
		rendered[i] = make([]string, cols)
		for j := 0; j < cols; j++ {
			cell := ""
			if j < len(r) {
				st := c.st.text
				if i == 0 {
					st = st.Bold(true).Foreground(c.st.th.Blue)
				}
				cell = c.cell(r[j], st)
			}
			rendered[i][j] = cell
			widths[j] = max(widths[j], ansi.StringWidth(cell))
		}
	}
	// Shrink the widest columns until the table fits the pane.
	avail := c.width - (3*cols + 1)
	for sum(widths) > avail && avail > cols {
		wi := 0
		for j := range widths {
			if widths[j] > widths[wi] {
				wi = j
			}
		}
		widths[wi]--
	}

	line := c.st.rule
	border := func(l, mid, r string) string {
		parts := make([]string, cols)
		for j, w := range widths {
			parts[j] = strings.Repeat("─", w+2)
		}
		return line.Render(l + strings.Join(parts, mid) + r)
	}
	c.blank()
	c.emit(border("┌", "┬", "┐"))
	lastLine := docStart
	for i, r := range rendered {
		docLine := docStart
		if i > 0 {
			docLine = docStart + 1 + i // skip the separator line
		}
		lastLine = docLine
		x := 1
		for j, w := range widths {
			text := ""
			if j < len(rows[i]) {
				text = rows[i][j]
			}
			c.hits = append(c.hits, Hit{Row: len(c.out), H: 1, X0: x, X1: x + w + 2, Kind: "table:cell", Line: docLine, Index: j, Arg: text})
			x += w + 3
		}
		var b strings.Builder
		b.WriteString(line.Render("│"))
		for j, cell := range r {
			if ansi.StringWidth(cell) > widths[j] {
				cell = ansi.Truncate(cell, widths[j], "…")
			}
			b.WriteString(" ")
			b.WriteString(cell)
			b.WriteString(strings.Repeat(" ", widths[j]-ansi.StringWidth(cell)))
			b.WriteString(" ")
			b.WriteString(line.Render("│"))
		}
		c.emit(b.String())
		if i < len(rendered)-1 {
			c.emit(border("├", "┼", "┤")) // divider between every row
		}
	}
	c.emit(border("└", "┴", "┘"))
	addRow := lipgloss.NewStyle().Foreground(c.st.th.Blue).Render("+ Add row")
	addCol := lipgloss.NewStyle().Foreground(c.st.th.Blue).Render("+ Add column")
	c.hits = append(c.hits,
		Hit{Row: len(c.out), H: 1, X0: 0, X1: 9, Kind: "table:addrow", Line: lastLine, Index: cols - 1},
		Hit{Row: len(c.out), H: 1, X0: 12, X1: 24, Kind: "table:addcol", Line: lastLine, Index: cols - 1},
	)
	c.emit(addRow + "   " + addCol)
	c.blank()
}

func sum(xs []int) int {
	t := 0
	for _, x := range xs {
		t += x
	}
	return t
}

var layoutTag = regexp.MustCompile(`(?i)</?(div|p|center|span|br|hr|details|summary|picture|source|section|table|tr|td|th|tbody|thead|h[1-6])\b[^>]*>`)

func isTagOnly(s string) bool { return strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">") }

func stripLayoutTags(s string) string { return layoutTag.ReplaceAllString(s, "") }

func attr(re *regexp.Regexp, tag string) string {
	m := re.FindStringSubmatch(tag)
	if m == nil {
		return ""
	}
	for _, v := range m[2:] {
		if v != "" {
			return v
		}
	}
	return ""
}
