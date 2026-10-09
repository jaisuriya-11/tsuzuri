// Package highlight colours Markdown and fenced code (via chroma) with the
// active theme. Results are per-rune colours so both the editor and the
// preview can paint them.
package highlight

import (
	"regexp"
	"strings"

	"github.com/jaisuriya-11/tsuzuri/internal/theme"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/charmbracelet/lipgloss"
)

// Colors holds one colour per rune for every line ("" = default colour).
type Colors [][]lipgloss.Color

// lexerFor finds a chroma lexer by fence name, falling back to content
// analysis and then plain text.
func lexerFor(lang, code string) chroma.Lexer {
	var l chroma.Lexer
	switch strings.ToLower(lang) {
	case "board", "kanban":
		lang = "markdown"
	case "calendar", "timeline", "gantt", "chart", "form":
		lang = "yaml"
	case "math", "katex":
		lang = "latex"
	}
	if lang != "" {
		l = lexers.Get(lang)
	}
	if l == nil && strings.TrimSpace(code) != "" {
		l = lexers.Analyse(code)
	}
	if l == nil {
		l = lexers.Fallback
	}
	return chroma.Coalesce(l)
}

func tokenColor(tt chroma.TokenType, th theme.Theme) lipgloss.Color {
	switch {
	case tt == chroma.CommentPreproc || tt == chroma.CommentPreprocFile:
		return th.Purple
	case tt.InCategory(chroma.Comment):
		return th.GreyFg2
	case tt == chroma.KeywordType:
		return th.Yellow
	case tt == chroma.KeywordConstant:
		return th.Orange
	case tt.InCategory(chroma.Keyword):
		return th.Purple
	case tt == chroma.NameFunction || tt == chroma.NameFunctionMagic:
		return th.Blue
	case tt == chroma.NameBuiltin || tt == chroma.NameBuiltinPseudo:
		return th.Cyan
	case tt == chroma.NameClass || tt == chroma.NameNamespace || tt == chroma.NameDecorator ||
		tt == chroma.NameAttribute || tt == chroma.NameException:
		return th.Yellow
	case tt == chroma.NameTag:
		return th.Red
	case tt == chroma.NameConstant || tt == chroma.NameEntity:
		return th.Orange
	case tt == chroma.NameVariable || tt == chroma.NameVariableInstance || tt == chroma.NameProperty:
		return th.Red
	case tt == chroma.LiteralStringEscape || tt == chroma.LiteralStringRegex:
		return th.Cyan
	case tt.InSubCategory(chroma.LiteralString):
		return th.Green
	case tt.InSubCategory(chroma.LiteralNumber) || tt == chroma.Literal:
		return th.Orange
	case tt.InCategory(chroma.Operator):
		return th.Cyan
	case tt == chroma.GenericHeading || tt == chroma.GenericSubheading:
		return th.Blue
	case tt == chroma.GenericDeleted:
		return th.Red
	case tt == chroma.GenericInserted:
		return th.Green
	}
	return ""
}

// Code returns per-rune colours for a block of source code.
func Code(lang, code string, th theme.Theme) Colors {
	lines := strings.Split(code, "\n")
	out := make(Colors, len(lines))
	for i, l := range lines {
		out[i] = make([]lipgloss.Color, len([]rune(l)))
	}
	it, err := lexerFor(lang, code).Tokenise(nil, code)
	if err != nil {
		return out
	}
	row, col := 0, 0
	for tok := it(); tok != chroma.EOF; tok = it() {
		c := tokenColor(tok.Type, th)
		for _, r := range tok.Value {
			if r == '\n' {
				row++
				col = 0
				continue
			}
			if row < len(out) && col < len(out[row]) {
				out[row][col] = c
			}
			col++
		}
	}
	return out
}

var (
	headingRe = regexp.MustCompile(`^(#{1,6})\s`)
	listRe    = regexp.MustCompile(`^(\s*)([-*+]|\d+[.)])(\s+)(\[[ xX]\]\s)?`)
	quoteRe   = regexp.MustCompile(`^\s*>`)
	ruleRe    = regexp.MustCompile(`^\s*(-{3,}|\*{3,}|_{3,})\s*$`)
	codeSpan  = regexp.MustCompile("`[^`]+`")
	linkRe    = regexp.MustCompile(`!?\[[^\]]*\]\([^)]*\)`)
	wikiRe    = regexp.MustCompile(`!?\[\[[^\[\]\n]+\]\]`)
	blockIDRe = regexp.MustCompile(`\s\^[A-Za-z0-9-]+\s*$`)
	tagRe     = regexp.MustCompile(`</?[a-zA-Z][^>]*>`)
	emphRe    = regexp.MustCompile(`\*\*|__|~~`)
)

// headingColor matches the preview's heading colours.
func headingColor(level int, th theme.Theme) lipgloss.Color {
	return []lipgloss.Color{th.Blue, th.Purple, th.Green, th.Yellow, th.Cyan, th.GreyFg2}[level-1]
}

// Markdown returns per-rune colours for a whole Markdown document, with
// fenced code blocks highlighted for their language.
func Markdown(text string, th theme.Theme) Colors {
	lines := strings.Split(text, "\n")
	out := make(Colors, len(lines))
	first := 0
	if len(lines) > 1 && strings.TrimRight(lines[0], "\r") == "---" {
		for end := 1; end < len(lines); end++ {
			if t := strings.TrimRight(lines[end], "\r"); t == "---" || t == "..." {
				for j := 0; j <= end; j++ {
					out[j] = frontMatterColors(lines[j], j == 0 || j == end, th)
				}
				first = end + 1
				break
			}
		}
	}
	for i := first; i < len(lines); i++ {
		line := lines[i]
		cols := make([]lipgloss.Color, len([]rune(line)))
		out[i] = cols
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			fence := trimmed[:3]
			lang := strings.TrimSpace(trimmed[3:])
			fill(cols, 0, len(cols), th.GreyFg2)
			start := i + 1
			end := start
			for end < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[end]), fence) {
				end++
			}
			code := Code(lang, strings.Join(lines[start:end], "\n"), th)
			for j := start; j < end; j++ {
				out[j] = code[j-start]
			}
			if end < len(lines) {
				closing := make([]lipgloss.Color, len([]rune(lines[end])))
				fill(closing, 0, len(closing), th.GreyFg2)
				out[end] = closing
			}
			i = end
			continue
		}

		switch {
		case headingRe.MatchString(line):
			level := len(headingRe.FindStringSubmatch(line)[1])
			fill(cols, 0, len(cols), headingColor(level, th))
			continue
		case ruleRe.MatchString(line):
			fill(cols, 0, len(cols), th.GreyFg)
			continue
		case quoteRe.MatchString(line):
			idx := len([]rune(line[:strings.Index(line, ">")]))
			fill(cols, idx, idx+1, th.Purple)
		}
		if m := listRe.FindStringSubmatchIndex(line); m != nil {
			fill(cols, runeIdx(line, m[4]), runeIdx(line, m[5]), th.Red)
			if m[8] >= 0 {
				box := th.GreyFg2
				if strings.ContainsAny(line[m[8]:m[9]], "xX") {
					box = th.Green
				}
				fill(cols, runeIdx(line, m[8]), runeIdx(line, m[9]), box)
			}
		}
		inline(line, cols, th)
	}
	return out
}

func frontMatterColors(line string, delimiter bool, th theme.Theme) []lipgloss.Color {
	cols := make([]lipgloss.Color, len([]rune(line)))
	if delimiter {
		fill(cols, 0, len(cols), th.GreyFg)
		return cols
	}
	if i := strings.Index(line, ":"); i > 0 {
		k := runeIdx(line, i)
		fill(cols, 0, k, th.Red)
		fill(cols, k, k+1, th.GreyFg)
		fill(cols, k+1, len(cols), th.Green)
	}
	return cols
}

func inline(line string, cols []lipgloss.Color, th theme.Theme) {
	for _, m := range tagRe.FindAllStringIndex(line, -1) {
		fill(cols, runeIdx(line, m[0]), runeIdx(line, m[1]), th.Red)
	}
	for _, m := range emphRe.FindAllStringIndex(line, -1) {
		fill(cols, runeIdx(line, m[0]), runeIdx(line, m[1]), th.GreyFg)
	}
	for _, m := range linkRe.FindAllStringIndex(line, -1) {
		s := line[m[0]:m[1]]
		mid := strings.Index(s, "](")
		fill(cols, runeIdx(line, m[0]), runeIdx(line, m[0]+mid+1), th.Blue)
		fill(cols, runeIdx(line, m[0]+mid+1), runeIdx(line, m[1]), th.GreyFg2)
	}
	for _, m := range wikiRe.FindAllStringIndex(line, -1) {
		open := strings.Index(line[m[0]:m[1]], "[[") + 2
		fill(cols, runeIdx(line, m[0]), runeIdx(line, m[0]+open), th.GreyFg)
		fill(cols, runeIdx(line, m[0]+open), runeIdx(line, m[1]-2), th.Blue)
		fill(cols, runeIdx(line, m[1]-2), runeIdx(line, m[1]), th.GreyFg)
	}
	for _, m := range blockIDRe.FindAllStringIndex(line, -1) {
		fill(cols, runeIdx(line, m[0]), runeIdx(line, m[1]), th.GreyFg)
	}
	for _, m := range codeSpan.FindAllStringIndex(line, -1) {
		fill(cols, runeIdx(line, m[0]), runeIdx(line, m[1]), th.Orange)
	}
}

func runeIdx(s string, byteIdx int) int { return len([]rune(s[:byteIdx])) }

func fill(cols []lipgloss.Color, from, to int, c lipgloss.Color) {
	for i := max(from, 0); i < min(to, len(cols)); i++ {
		cols[i] = c
	}
}

// Render paints runes using per-rune colours on top of base.
func Render(runes []rune, colors []lipgloss.Color, base lipgloss.Style) string {
	var b strings.Builder
	start := 0
	color := func(i int) lipgloss.Color {
		if i < len(colors) {
			return colors[i]
		}
		return ""
	}
	for i := 1; i <= len(runes); i++ {
		if i < len(runes) && color(i) == color(start) {
			continue
		}
		st := base
		if c := color(start); c != "" {
			st = st.Foreground(c)
		}
		b.WriteString(st.Render(string(runes[start:i])))
		start = i
	}
	return b.String()
}
