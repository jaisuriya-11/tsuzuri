package preview

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/jaisuriya-11/tsuzuri/internal/theme"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var (
	checkboxRegex   = regexp.MustCompile(`^(\s*)[-*+]\s+\[([ xX])\]\s*(.*)$`)
	bulletRegex     = regexp.MustCompile(`^(\s*)[-*+]\s+(.*)$`)
	numberedRegex   = regexp.MustCompile(`^(\s*)(\d+)[.)]\s+(.*)$`)
	blockquoteRegex = regexp.MustCompile(`^\s*>\s?(.*)$`)
	dividerRegex    = regexp.MustCompile(`^(\-{3,}|\*{3,}|_{3,})$`)
	headingRegex    = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*#*\s*$`)
	tableSepRegex   = regexp.MustCompile(`^\s*\|?\s*:?-{2,}:?\s*(\|\s*:?-{2,}:?\s*)*\|?\s*$`)

	htmlCommentRegex = regexp.MustCompile(`<!--.*?-->`)
	htmlAttrRegex    = func(name string) *regexp.Regexp {
		return regexp.MustCompile(`(?i)\b` + name + `\s*=\s*("([^"]*)"|'([^']*)'|([^\s>]+))`)
	}
	altAttr = htmlAttrRegex("alt")
)

// styles bundles the per-theme styles used while compiling.
type styles struct {
	th        theme.Theme
	text      lipgloss.Style
	muted     lipgloss.Style
	code      lipgloss.Style
	link      lipgloss.Style
	headings  [6]lipgloss.Style
	quoteBar  lipgloss.Style
	calloutBr lipgloss.Style
	bullet    lipgloss.Style
	codeBox   lipgloss.Style
	codeLang  lipgloss.Style
	rule      lipgloss.Style
}

func newStyles(th theme.Theme) styles {
	return styles{
		th:    th,
		text:  lipgloss.NewStyle().Foreground(th.Fg),
		muted: lipgloss.NewStyle().Foreground(th.GreyFg2),
		code:  lipgloss.NewStyle().Foreground(th.Orange).Background(th.OneBg),
		link:  lipgloss.NewStyle().Foreground(th.Blue).Underline(true),
		headings: [6]lipgloss.Style{
			lipgloss.NewStyle().Bold(true).Foreground(th.Blue),
			lipgloss.NewStyle().Bold(true).Foreground(th.Purple),
			lipgloss.NewStyle().Bold(true).Foreground(th.Green),
			lipgloss.NewStyle().Bold(true).Foreground(th.Yellow),
			lipgloss.NewStyle().Bold(true).Foreground(th.Cyan),
			lipgloss.NewStyle().Bold(true).Foreground(th.GreyFg2),
		},
		quoteBar:  lipgloss.NewStyle().Foreground(th.GreyFg),
		calloutBr: lipgloss.NewStyle().Foreground(th.Blue),
		bullet:    lipgloss.NewStyle().Foreground(th.Blue),
		codeBox: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(th.Line).
			Foreground(th.Fg).
			Padding(0, 1),
		codeLang: lipgloss.NewStyle().Foreground(th.GreyFg2).Italic(true),
		rule:     lipgloss.NewStyle().Foreground(th.Line),
	}
}

// compiler accumulates rendered lines, collapsing runs of blank lines.
type compiler struct {
	st      styles
	width   int
	baseDir string // folder relative image paths resolve against
	hits    []Hit
	seen    map[string]int // occurrence counters for fold keys
	// Document positions of the block being rendered.
	lineOffset, fence, fenceEnd int
	cal                         CalendarView
	depth                       int // embed nesting: 0 for the note itself
	out                         []string
	para                        []string

	// Draggable blocks: finished ones, ones still being drawn, and where the
	// current paragraph started.
	blocks            []Block
	pending           []Block
	paraLine, paraEnd int
	paraRow           int
}

// Block is one top-level piece of the note (paragraph, heading, list item,
// table, fenced block …) that can be dragged to a new place.
type Block struct {
	Row, H    int // rows in the compiled output
	Line, End int // document lines [Line, End)
}

// openBlock starts a block covering document lines [line, end) of the
// current input; it is closed once the loop passes end.
func (c *compiler) openBlock(line, end int) {
	c.pending = append(c.pending, Block{Row: len(c.out), Line: c.lineOffset + line, End: c.lineOffset + end})
}

// closeBlocks finishes the pending blocks that end at or before line.
func (c *compiler) closeBlocks(line int) {
	keep := c.pending[:0]
	for _, b := range c.pending {
		if b.End <= c.lineOffset+line {
			c.addBlock(b.Line, b.End, b.Row, len(c.out))
		} else {
			keep = append(keep, b)
		}
	}
	c.pending = keep
}

// addBlock records a block drawn on rows [r0, r1), without blank edge rows.
func (c *compiler) addBlock(line, end, r0, r1 int) {
	for r0 < r1 && c.out[r0] == "" {
		r0++
	}
	for r1 > r0 && c.out[r1-1] == "" {
		r1--
	}
	if r1 > r0 {
		c.blocks = append(c.blocks, Block{Row: r0, H: r1 - r0, Line: line, End: end})
	}
}

func (c *compiler) emit(lines ...string) {
	for _, l := range lines {
		if l == "" && (len(c.out) == 0 || c.out[len(c.out)-1] == "") {
			continue
		}
		c.out = append(c.out, l)
	}
}

func (c *compiler) blank() { c.emit("") }

// wrapIndent word-wraps an already styled string to the pane width, prefixing
// the first line with first and continuation lines with rest.
func (c *compiler) wrapIndent(s, first, rest string) []string {
	w := max(c.width-ansi.StringWidth(first), 8)
	lines := strings.Split(ansi.Wrap(s, w, ""), "\n")
	for i := range lines {
		if i == 0 {
			lines[i] = first + lines[i]
		} else {
			lines[i] = rest + lines[i]
		}
	}
	return lines
}

func (c *compiler) flushPara() {
	if len(c.para) == 0 {
		return
	}
	text := strings.Join(c.para, " ")
	c.para = nil
	row := len(c.out)
	c.emit(c.wrapIndent(c.inline(text, c.st.text), "", "")...)
	c.addBlock(c.lineOffset+c.paraLine, c.lineOffset+c.paraEnd, row, len(c.out))
}

// Compile parses raw Markdown text and returns a styled ANSI string using the
// given Theme, wrapped to contentWidth columns.
func Compile(input string, th theme.Theme, contentWidth int) string {
	return CompileIn(input, th, contentWidth, "")
}

// CompileIn is Compile for a note stored in baseDir, so local images can be
// found and drawn.
func CompileIn(input string, th theme.Theme, contentWidth int, baseDir string) string {
	return CompileWith(input, th, contentWidth, baseDir, CalendarView{})
}

// CompileWith is CompileIn with a calendar navigation state.
func CompileWith(input string, th theme.Theme, contentWidth int, baseDir string, cal CalendarView) string {
	out, _ := CompileHits(input, th, contentWidth, baseDir, cal)
	return out
}

// CompileHits compiles and also returns the clickable regions of view
// blocks (rows in the output, lines in the original document).
func CompileHits(input string, th theme.Theme, contentWidth int, baseDir string, cal CalendarView) (out string, hits []Hit) {
	out, hits, _ = CompileBlocks(input, th, contentWidth, baseDir, cal)
	return out, hits
}

// CompileBlocks is CompileHits that also returns the note's draggable
// blocks, sorted by row (a list item's children follow it).
func CompileBlocks(input string, th theme.Theme, contentWidth int, baseDir string, cal CalendarView) (out string, hits []Hit, blocks []Block) {
	defer func() {
		if r := recover(); r != nil {
			out = fmt.Sprintf("Error rendering preview: %v", r)
			hits, blocks = nil, nil
		}
	}()
	if contentWidth < 10 {
		contentWidth = 40
	}
	c := &compiler{st: newStyles(th), width: contentWidth, baseDir: baseDir, cal: cal}
	c.compile(input)

	for len(c.out) > 0 && c.out[len(c.out)-1] == "" {
		c.out = c.out[:len(c.out)-1]
	}
	c.collectLinks()
	sort.SliceStable(c.blocks, func(a, b int) bool {
		if c.blocks[a].Row != c.blocks[b].Row {
			return c.blocks[a].Row < c.blocks[b].Row
		}
		return c.blocks[a].H > c.blocks[b].H
	})
	return strings.Join(c.out, "\n"), c.hits, c.blocks
}

// compile renders a whole note (or, for an embed, part of one) into c.out.
func (c *compiler) compile(input string) {
	meta, body := SplitFrontMatter(input)
	c.lineOffset = strings.Count(input, "\n") - strings.Count(body, "\n")
	input = body
	if c.depth == 0 {
		c.header(meta)
	}

	input = htmlCommentRegex.ReplaceAllString(strings.ReplaceAll(input, "\t", "    "), "")
	lines := strings.Split(input, "\n")

	skipLevel := 0 // >0 while inside a collapsed heading's section
	skipFence := ""
	for i := 0; i < len(lines); i++ {
		c.closeBlocks(i)
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if skipLevel > 0 {
			switch {
			case skipFence != "":
				if strings.HasPrefix(trimmed, skipFence) {
					skipFence = ""
				}
				continue
			case strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~"):
				skipFence = trimmed[:3]
				continue
			case headingRegex.MatchString(trimmed) && len(headingRegex.FindStringSubmatch(trimmed)[1]) <= skipLevel:
				skipLevel = 0
			default:
				continue
			}
		}

		// Fenced code blocks.
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			c.flushPara()
			fence := trimmed[:3]
			lang := strings.TrimSpace(trimmed[3:])
			var code []string
			open := i
			for i++; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), fence); i++ {
				code = append(code, lines[i])
			}
			c.openBlock(open, min(i+1, len(lines)))
			c.fence, c.fenceEnd = c.lineOffset+open, c.lineOffset+i
			c.codeBlock(lang, code)
			continue
		}

		// $$ … $$ equations, on one line or spread over several.
		if strings.HasPrefix(trimmed, "$$") {
			c.flushPara()
			open := i
			rest := strings.TrimSpace(trimmed[2:])
			var body []string
			if end := strings.Index(rest, "$$"); end >= 0 {
				body = []string{rest[:end]}
			} else {
				if rest != "" {
					body = append(body, rest)
				}
				for i++; i < len(lines); i++ {
					t := strings.TrimSpace(lines[i])
					if end := strings.Index(t, "$$"); end >= 0 {
						body = append(body, t[:end])
						break
					}
					body = append(body, lines[i])
				}
			}
			c.openBlock(open, min(i+1, len(lines)))
			c.fence, c.fenceEnd = c.lineOffset+open, c.lineOffset+i
			c.codeBlock("math", body)
			continue
		}

		// Tables: a header row followed by a separator row.
		if strings.Contains(line, "|") && i+1 < len(lines) && tableSepRegex.MatchString(lines[i+1]) {
			c.flushPara()
			start := c.lineOffset + i
			rows := [][]string{splitRow(line)}
			for i += 2; i < len(lines) && strings.Contains(lines[i], "|") && strings.TrimSpace(lines[i]) != ""; i++ {
				rows = append(rows, splitRow(lines[i]))
			}
			c.openBlock(start-c.lineOffset, i)
			i--
			c.table(rows, start)
			continue
		}

		// A line holding only an image is drawn as a picture.
		if alt, src, ok := standaloneImage(line); ok {
			c.flushPara()
			c.openBlock(i, i+1)
			c.image(alt, src)
			continue
		}

		// A line holding only "![[Note]]" shows that note (or image) here.
		if l, ok := standaloneEmbed(trimmed); ok {
			c.flushPara()
			c.openBlock(i, i+1)
			c.embed(l)
			continue
		}

		// Block ids ("^id") are link anchors, not text.
		line = stripBlockID(line)
		trimmed = strings.TrimSpace(line)

		// Lines made only of HTML layout tags (<div>, </p>, <br> …) vanish;
		// inline HTML elsewhere is handled by the inline parser.
		if isTagOnly(trimmed) {
			if inner := strings.TrimSpace(stripLayoutTags(trimmed)); inner == "" {
				continue
			}
		}

		switch {
		case trimmed == "":
			c.flushPara()
			c.blank()

		case dividerRegex.MatchString(trimmed):
			c.flushPara()
			c.openBlock(i, i+1)
			c.blank()
			c.emit(c.st.rule.Render(strings.Repeat("─", c.width)))
			c.blank()

		case headingRegex.MatchString(trimmed):
			c.flushPara()
			m := headingRegex.FindStringSubmatch(trimmed)
			level := len(m[1])
			st := c.st.headings[level-1]
			key := c.foldKey("h", m[1]+m[2])
			folded := c.folded(key)
			arrow := "▾ "
			switch {
			case c.depth > 0:
				arrow = "" // embedded sections don't fold
			case folded:
				arrow = "▸ "
			}
			prefix := arrow + []string{"󰉫 ", "󰉬 ", "󰉭 ", "󰉮 ", "󰉯 ", "󰉰 "}[level-1]
			c.blank()
			if c.depth == 0 {
				c.hits = append(c.hits, Hit{Row: len(c.out), H: 1, X1: c.width, Kind: "fold", Arg: key, Line: -1})
			}
			heading := c.wrapIndent(c.inline(m[2], st), st.Render(prefix), "    ")
			end := i + 1
			if folded {
				end += sectionLength(lines, i, level)
			}
			c.openBlock(i, min(end, len(lines)))
			if folded {
				hidden := sectionLength(lines, i, level)
				heading[len(heading)-1] += c.st.muted.Render(fmt.Sprintf("  … %d lines", hidden))
				c.emit(heading...)
				skipLevel = level
				continue
			}
			c.emit(heading...)
			if level == 1 {
				c.emit(c.st.headings[0].Render(strings.Repeat("━", min(c.width, max(ansi.StringWidth(m[2])+2, 8)))))
			}
			c.blank()

		case checkboxRegex.MatchString(line):
			c.flushPara()
			c.openBlock(i, listItemEnd(lines, i))
			m := checkboxRegex.FindStringSubmatch(line)
			indent := strings.Repeat(" ", len(m[1]))
			if m[2] == "x" || m[2] == "X" {
				box := lipgloss.NewStyle().Foreground(c.st.th.Green).Render("󰄲 ")
				text := c.inline(m[3], c.st.muted.Strikethrough(true))
				c.emit(c.wrapIndent(text, indent+box, indent+"  ")...)
			} else {
				box := lipgloss.NewStyle().Foreground(c.st.th.GreyFg2).Render("󰄱 ")
				c.emit(c.wrapIndent(c.inline(m[3], c.st.text), indent+box, indent+"  ")...)
			}

		case bulletRegex.MatchString(line):
			c.flushPara()
			c.openBlock(i, listItemEnd(lines, i))
			m := bulletRegex.FindStringSubmatch(line)
			depth := len(m[1]) / 2
			indent := strings.Repeat(" ", len(m[1]))
			glyph := []string{"●", "○", "◆", "◇"}[depth%4]
			c.emit(c.wrapIndent(c.inline(m[2], c.st.text), indent+c.st.bullet.Render(glyph)+" ", indent+"  ")...)

		case numberedRegex.MatchString(line):
			c.flushPara()
			c.openBlock(i, listItemEnd(lines, i))
			m := numberedRegex.FindStringSubmatch(line)
			indent := strings.Repeat(" ", len(m[1]))
			num := c.st.bullet.Bold(true).Render(m[2] + ".")
			pad := strings.Repeat(" ", len(m[2])+2)
			c.emit(c.wrapIndent(c.inline(m[3], c.st.text), indent+num+" ", indent+pad)...)

		case blockquoteRegex.MatchString(line):
			c.flushPara()
			c.openBlock(i, i+1)
			m := blockquoteRegex.FindStringSubmatch(line)
			body := m[1]
			bar := c.st.quoteBar.Render("▎ ")
			st := c.st.muted.Italic(true)
			if kind, rest, ok := callout(body); ok {
				bar = c.st.calloutBr.Render("▎ ")
				c.emit(bar + c.st.calloutBr.Bold(true).Render(kind))
				body = rest
				st = c.st.text
				if strings.TrimSpace(body) == "" {
					continue
				}
			}
			c.emit(c.wrapIndent(c.inline(body, st), bar, bar)...)

		default:
			if len(c.para) == 0 {
				c.paraLine = i
			}
			c.paraEnd = i + 1
			c.para = append(c.para, trimmed)
		}
	}
	c.flushPara()
	c.closeBlocks(len(lines))
}

// listItemEnd is the line after list item i and the more-indented lines
// (its children) under it.
func listItemEnd(lines []string, i int) int {
	indent := len(lines[i]) - len(strings.TrimLeft(lines[i], " "))
	end := i + 1
	for end < len(lines) {
		l := lines[end]
		if strings.TrimSpace(l) == "" || len(l)-len(strings.TrimLeft(l, " ")) <= indent {
			break
		}
		end++
	}
	return end
}
