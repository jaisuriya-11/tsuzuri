// Package export writes notes out as printable documents (PDF).
//
// The PDF is laid out like a printed page rather than a terminal: proportional
// fonts, wrapped paragraphs, headings, lists, tables and boxed code. Views the
// preview draws as text art (boards, charts, flowcharts …) are drawn the same
// way here, in a monospace font with their colours.
package export

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/go-pdf/fpdf"
	"github.com/muesli/termenv"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gobolditalic"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/sfnt"

	"github.com/jaisuriya-11/tsuzuri/internal/core"
	"github.com/jaisuriya-11/tsuzuri/internal/highlight"
	"github.com/jaisuriya-11/tsuzuri/internal/preview"
	"github.com/jaisuriya-11/tsuzuri/internal/theme"
)

// Page layout, in millimetres and points.
const (
	margin     = 20.0
	bodySize   = 11.0
	bodyLine   = 5.6
	codeSize   = 9.0
	codeLine   = 4.6
	blockCols  = 96 // terminal cells across for boards, charts …
	listIndent = 6.0
)

// printTheme colours syntax highlighting and the text-art views.
const printTheme = "one_light"

// Ink colours for the printed page.
var (
	inkText   = rgb{33, 37, 41}
	inkMuted  = rgb{108, 117, 125}
	inkLink   = rgb{13, 110, 253}
	inkCode   = rgb{173, 20, 87}
	inkRule   = rgb{222, 226, 230}
	paperCode = rgb{246, 248, 250}
)

type rgb struct{ r, g, b int }

// Options describes the document being exported.
type Options struct {
	Title   string // PDF title metadata
	BaseDir string // folder relative image paths resolve against
}

// PDF renders Markdown as a PDF document and writes it to w.
func PDF(markdown string, opts Options, w io.Writer) error {
	th, ok := theme.Get(printTheme)
	if !ok {
		th = theme.DefaultTheme()
	}
	// The text-art views are drawn through lipgloss; force full colour so
	// their escape codes carry exact colours even with no terminal attached.
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prev)

	d := newDoc(opts, th)
	_, body := preview.SplitFrontMatter(strings.ReplaceAll(markdown, "\r\n", "\n"))
	d.blocks(strings.Split(body, "\n"))
	if err := d.pdf.Error(); err != nil {
		return err
	}
	return d.pdf.Output(w)
}

// WriteFile renders Markdown as a PDF file at path.
func WriteFile(markdown string, opts Options, path string) error {
	var buf bytes.Buffer
	if err := PDF(markdown, opts, &buf); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

type doc struct {
	pdf    *fpdf.Fpdf
	opts   Options
	th     theme.Theme
	glyphs map[string]*sfnt.Font // family+style → font, to spot missing glyphs
	cur    string                // family+style in use
	images int
}

func newDoc(opts Options, th theme.Theme) *doc {
	p := fpdf.New("P", "mm", "A4", "")
	p.SetMargins(margin, margin, margin)
	p.SetAutoPageBreak(true, margin)
	p.SetCellMargin(0)
	p.SetCreator("Tsuzuri", true)
	if opts.Title != "" {
		p.SetTitle(opts.Title, true)
	}
	d := &doc{pdf: p, opts: opts, th: th, glyphs: map[string]*sfnt.Font{}}
	for _, f := range []struct {
		family, style string
		ttf           []byte
	}{
		{"sans", "", goregular.TTF},
		{"sans", "B", gobold.TTF},
		{"sans", "I", goitalic.TTF},
		{"sans", "BI", gobolditalic.TTF},
		{"mono", "", gomono.TTF},
		{"mono", "B", gomonobold.TTF},
	} {
		p.AddUTF8FontFromBytes(f.family, f.style, f.ttf)
		if parsed, err := sfnt.Parse(f.ttf); err == nil {
			d.glyphs[f.family+f.style] = parsed
		}
	}
	p.AliasNbPages("")
	p.SetFooterFunc(func() {
		p.SetY(-margin + 6)
		d.font("sans", "", 8.5)
		d.color(inkMuted)
		p.CellFormat(0, 4, fmt.Sprintf("%d / {nb}", p.PageNo()), "", 0, "C", false, 0, "")
	})
	p.AddPage()
	return d
}

// --- small drawing helpers ---------------------------------------------------

func (d *doc) font(family, style string, size float64) {
	d.pdf.SetFont(family, style, size)
	d.cur = family + style
}

func (d *doc) color(c rgb) { d.pdf.SetTextColor(c.r, c.g, c.b) }

func (d *doc) fill(c rgb) { d.pdf.SetFillColor(c.r, c.g, c.b) }

// clean swaps runes the current font cannot draw (Nerd Font icons, some
// symbols) for a stand-in so they never print as empty boxes.
func (d *doc) clean(s string) string {
	f := d.glyphs[d.cur]
	if f == nil {
		return s
	}
	var buf sfnt.Buffer
	return strings.Map(func(r rune) rune {
		if r == '\t' {
			return ' '
		}
		if r < 0x20 {
			return -1
		}
		if i, err := f.GlyphIndex(&buf, r); err == nil && i != 0 {
			return r
		}
		if fb, ok := fallbackRunes[r]; ok {
			return fb
		}
		if r >= 0xe000 && r <= 0xf8ff || r >= 0xf0000 || r > 0xffff || unicode.Is(unicode.So, r) {
			return -1 // icons and emoji: leave them out
		}
		return '?'
	}, s)
}

// fallbackRunes stand in for symbols the Go fonts lack.
var fallbackRunes = map[rune]rune{
	'✓': 'x', '✔': 'x', '✗': 'x', '☐': '-', '☑': 'x', '…': '.', '⋯': '.',
	'▏': '|', '▕': '|', '╭': '┌', '╮': '┐', '╯': '┘', '╰': '└',
	'❯': '>', '›': '>', '‹': '<', '⟨': '<', '⟩': '>',
	'╱': '/', '╲': '\\', '◀': '◄', '▶': '►', '◆': '♦', '◇': '◊',
}

// ensure starts a new page when fewer than h millimetres are left.
func (d *doc) ensure(h float64) {
	_, ph := d.pdf.GetPageSize()
	if d.pdf.GetY()+h > ph-margin {
		d.pdf.AddPage()
	}
}

func (d *doc) gap(h float64) {
	if d.pdf.GetY() > margin+0.1 { // no gap at the top of a page
		d.pdf.Ln(h)
	}
}

func (d *doc) contentWidth() float64 {
	pw, _ := d.pdf.GetPageSize()
	l, _, r, _ := d.pdf.GetMargins()
	return pw - l - r
}

// --- block parsing -------------------------------------------------------------

var (
	headingRe = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*#*\s*$`)
	hrRe      = regexp.MustCompile(`^(\*\s*){3,}$|^(-\s*){3,}$|^(_\s*){3,}$`)
	listRe    = regexp.MustCompile(`^(\s*)([-*+]|\d+[.)])\s+(.*)$`)
	taskRe    = regexp.MustCompile(`^\[([ xX])\]\s*(.*)$`)
	tableSep  = regexp.MustCompile(`^\s*\|?\s*:?-{2,}:?\s*(\|\s*:?-{2,}:?\s*)*\|?\s*$`)
	calloutRe = regexp.MustCompile(`^\[!(\w+)\]\s*(.*)$`)
	tagOnly   = regexp.MustCompile(`^<[^>]+>$`)
)

func (d *doc) blocks(lines []string) {
	var para []string
	flush := func() {
		if len(para) > 0 {
			d.paragraph(strings.Join(para, " "))
			para = nil
		}
	}
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		t := strings.TrimSpace(line)
		switch {
		case t == "":
			flush()

		case strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~"):
			flush()
			fence, lang := t[:3], strings.ToLower(strings.TrimSpace(t[3:]))
			var body []string
			for i++; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), fence); i++ {
				body = append(body, lines[i])
			}
			d.fenced(lang, body)

		case strings.HasPrefix(t, "$$"):
			flush()
			rest := strings.TrimSpace(t[2:])
			var body []string
			if end := strings.Index(rest, "$$"); end >= 0 {
				body = []string{rest[:end]}
			} else {
				if rest != "" {
					body = append(body, rest)
				}
				for i++; i < len(lines); i++ {
					if end := strings.Index(lines[i], "$$"); end >= 0 {
						body = append(body, lines[i][:end])
						break
					}
					body = append(body, lines[i])
				}
			}
			d.math(body)

		case headingRe.MatchString(t):
			flush()
			m := headingRe.FindStringSubmatch(t)
			d.heading(len(m[1]), m[2])

		case hrRe.MatchString(t):
			flush()
			d.rule()

		case strings.Contains(t, "|") && i+1 < len(lines) && tableSep.MatchString(lines[i+1]):
			flush()
			rows := [][]string{splitRow(t)}
			for i += 2; i < len(lines) && strings.Contains(lines[i], "|") && strings.TrimSpace(lines[i]) != ""; i++ {
				rows = append(rows, splitRow(lines[i]))
			}
			i--
			d.table(rows)

		case strings.HasPrefix(t, ">"):
			flush()
			var quote []string
			for ; i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), ">"); i++ {
				q := strings.TrimPrefix(strings.TrimSpace(lines[i]), ">")
				quote = append(quote, strings.TrimPrefix(q, " "))
			}
			i--
			d.quote(quote)

		case listRe.MatchString(line):
			flush()
			var items []string
			for ; i < len(lines); i++ {
				l := lines[i]
				if listRe.MatchString(l) {
					items = append(items, l)
					continue
				}
				// A non-blank indented line continues the item above it.
				if strings.TrimSpace(l) != "" && (strings.HasPrefix(l, "  ") || strings.HasPrefix(l, "\t")) && len(items) > 0 {
					items[len(items)-1] += " " + strings.TrimSpace(l)
					continue
				}
				break
			}
			i--
			d.list(items)

		default:
			if _, src, ok := preview.StandaloneImage(line); ok {
				flush()
				d.image(src)
				continue
			}
			if strings.HasPrefix(strings.ToLower(t), "<summary") {
				// A toggle's title prints as a bold line.
				flush()
				d.paragraphStyled(strings.TrimSpace(preview.StripLayoutTags(t)), "B")
				continue
			}
			if tagOnly.MatchString(t) && strings.TrimSpace(preview.StripLayoutTags(t)) == "" {
				flush()
				continue
			}
			para = append(para, strings.TrimSpace(preview.StripLayoutTags(line)))
		}
	}
	flush()
}

func splitRow(l string) []string {
	l = strings.TrimSpace(l)
	l = strings.TrimPrefix(l, "|")
	l = strings.TrimSuffix(l, "|")
	cells := strings.Split(l, "|")
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	return cells
}

// --- blocks ----------------------------------------------------------------------

var headingSizes = [...]float64{0, 22, 17, 14, 12.5, 11.5, 11}

func (d *doc) heading(level int, text string) {
	size := headingSizes[level]
	d.gap(size * 0.35)
	d.ensure(size*0.9 + 12) // keep a heading with the text after it
	d.pdf.Bookmark(plain(text), level-1, -1)
	d.writeRuns(parseInline(text), "sans", "B", size, size*0.48, inkText)
	d.pdf.Ln(size * 0.48)
	if level <= 2 {
		y := d.pdf.GetY() + 1
		d.pdf.SetDrawColor(inkRule.r, inkRule.g, inkRule.b)
		d.pdf.SetLineWidth(0.3)
		l, _, r, _ := d.pdf.GetMargins()
		pw, _ := d.pdf.GetPageSize()
		d.pdf.Line(l, y, pw-r, y)
		d.pdf.SetY(y + 2)
	} else {
		d.pdf.Ln(1.5)
	}
}

func (d *doc) paragraph(text string) { d.paragraphStyled(text, "") }

func (d *doc) paragraphStyled(text, style string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	d.gap(1.5)
	d.writeRuns(parseInline(text), "sans", style, bodySize, bodyLine, inkText)
	d.pdf.Ln(bodyLine)
	d.pdf.Ln(1.5)
}

func (d *doc) rule() {
	d.gap(2)
	y := d.pdf.GetY() + 1
	l, _, r, _ := d.pdf.GetMargins()
	pw, _ := d.pdf.GetPageSize()
	d.pdf.SetDrawColor(inkRule.r, inkRule.g, inkRule.b)
	d.pdf.SetLineWidth(0.4)
	d.pdf.Line(l, y, pw-r, y)
	d.pdf.SetY(y + 4)
}

// calloutColors tints GitHub-style "> [!NOTE]" callouts.
var calloutColors = map[string]rgb{
	"NOTE": {9, 105, 218}, "TIP": {26, 127, 55}, "IMPORTANT": {130, 80, 223},
	"WARNING": {154, 103, 0}, "CAUTION": {207, 34, 46},
}

func (d *doc) quote(lines []string) {
	bar := inkRule
	label := ""
	if len(lines) > 0 {
		if m := calloutRe.FindStringSubmatch(strings.TrimSpace(lines[0])); m != nil {
			kind := strings.ToUpper(m[1])
			if c, ok := calloutColors[kind]; ok {
				bar = c
			}
			label = strings.ToUpper(kind[:1]) + strings.ToLower(kind[1:])
			lines = lines[1:]
			if m[2] != "" {
				lines = append([]string{m[2]}, lines...)
			}
		}
	}
	d.gap(1.5)
	l, t, r, _ := d.pdf.GetMargins()
	d.pdf.SetLeftMargin(l + 6)
	d.pdf.SetX(l + 6)
	page, y0 := d.pdf.PageNo(), d.pdf.GetY()
	if label != "" {
		d.writeRuns([]run{{text: label, bold: true}}, "sans", "", bodySize, bodyLine, bar)
		d.pdf.Ln(bodyLine)
	}
	var para []string
	flush := func() {
		if len(para) > 0 {
			d.writeRuns(parseInline(strings.Join(para, " ")), "sans", "", bodySize, bodyLine, inkMuted)
			d.pdf.Ln(bodyLine)
			para = nil
		}
	}
	for _, q := range lines {
		if strings.TrimSpace(q) == "" {
			flush()
			continue
		}
		para = append(para, strings.TrimSpace(q))
	}
	flush()
	y1 := d.pdf.GetY()
	d.pdf.SetMargins(l, t, r)
	if d.pdf.PageNo() == page { // bar only when the quote stayed on one page
		d.fill(bar)
		d.pdf.Rect(l+1, y0, 1.2, y1-y0, "F")
	}
	d.pdf.SetX(l)
	d.pdf.Ln(2)
}

func (d *doc) list(items []string) {
	d.gap(1)
	l, t, r, _ := d.pdf.GetMargins()
	nums := map[int]int{}
	for _, item := range items {
		m := listRe.FindStringSubmatch(item)
		indent := len(strings.ReplaceAll(m[1], "\t", "    ")) / 2
		marker, text := m[2], m[3]
		x := l + float64(indent)*listIndent
		d.ensure(bodyLine)
		y := d.pdf.GetY()
		d.font("sans", "", bodySize)
		d.color(inkText)
		textX := x + listIndent
		switch {
		case taskRe.MatchString(text):
			tm := taskRe.FindStringSubmatch(text)
			text = tm[2]
			done := tm[1] != " "
			box, by := 3.2, y+(bodyLine-3.2)/2
			d.pdf.SetDrawColor(inkMuted.r, inkMuted.g, inkMuted.b)
			d.pdf.SetLineWidth(0.3)
			d.pdf.Rect(x+0.5, by, box, box, "D")
			if done {
				d.pdf.SetLineWidth(0.45)
				d.pdf.Line(x+1.1, by+1.7, x+1.9, by+2.6)
				d.pdf.Line(x+1.9, by+2.6, x+3.3, by+0.6)
				text = "~~" + text + "~~"
			}
		case marker[0] >= '0' && marker[0] <= '9':
			n, _ := strconv.Atoi(strings.TrimRight(marker, ".)"))
			if nums[indent] == 0 {
				nums[indent] = n
			} else {
				nums[indent]++
			}
			d.pdf.SetXY(x, y)
			d.pdf.CellFormat(listIndent, bodyLine, strconv.Itoa(nums[indent])+".", "", 0, "L", false, 0, "")
		default:
			bullets := []string{"•", "◦", "▪"}
			d.pdf.SetXY(x+1, y)
			d.pdf.CellFormat(listIndent, bodyLine, d.clean(bullets[indent%3]), "", 0, "L", false, 0, "")
		}
		for k := range nums { // a shallower item restarts deeper numbering
			if k > indent {
				delete(nums, k)
			}
		}
		d.pdf.SetLeftMargin(textX)
		d.pdf.SetXY(textX, y)
		d.writeRuns(parseInline(text), "sans", "", bodySize, bodyLine, inkText)
		d.pdf.Ln(bodyLine)
		d.pdf.SetMargins(l, t, r)
		d.pdf.Ln(0.6)
	}
	d.pdf.SetX(l)
	d.pdf.Ln(1)
}

func (d *doc) table(rows [][]string) {
	cols := 0
	for _, r := range rows {
		cols = max(cols, len(r))
	}
	if cols == 0 {
		return
	}
	d.gap(2)
	d.font("sans", "", 9.5)
	width := d.contentWidth()
	// Share the width by each column's longest cell, within limits.
	want := make([]float64, cols)
	for _, r := range rows {
		for c, cell := range r {
			want[c] = max(want[c], d.pdf.GetStringWidth(d.clean(plain(cell)))+4)
		}
	}
	total := 0.0
	for c := range want {
		want[c] = min(max(want[c], 14), width*0.6)
		total += want[c]
	}
	for c := range want {
		want[c] *= width / total
	}
	const lineH = 4.8
	l, _, _, _ := d.pdf.GetMargins()
	d.pdf.SetDrawColor(inkRule.r, inkRule.g, inkRule.b)
	d.pdf.SetLineWidth(0.25)
	for ri, r := range rows {
		style := ""
		if ri == 0 {
			style = "B"
		}
		d.font("sans", style, 9.5)
		cells := make([][]string, cols)
		h := lineH
		for c := 0; c < cols; c++ {
			text := ""
			if c < len(r) {
				text = d.clean(plain(r[c]))
			}
			cells[c] = d.pdf.SplitText(text, want[c]-3)
			if len(cells[c]) == 0 {
				cells[c] = []string{""}
			}
			h = max(h, float64(len(cells[c]))*lineH)
		}
		h += 2
		d.ensure(h)
		y, x := d.pdf.GetY(), l
		for c := 0; c < cols; c++ {
			if ri == 0 {
				d.fill(paperCode)
				d.pdf.Rect(x, y, want[c], h, "FD")
			} else {
				d.pdf.Rect(x, y, want[c], h, "D")
			}
			d.color(inkText)
			for li, line := range cells[c] {
				d.pdf.SetXY(x+1.5, y+1+float64(li)*lineH)
				d.pdf.CellFormat(want[c]-3, lineH, line, "", 0, "L", false, 0, "")
			}
			x += want[c]
		}
		d.pdf.SetXY(l, y+h)
	}
	d.pdf.Ln(3)
}

func (d *doc) image(src string) {
	path, err := preview.ResolveImage(strings.Trim(src, "<>"), d.opts.BaseDir)
	if err != nil {
		d.paragraphStyled("[image: "+src+"]", "I")
		return
	}
	f, err := os.Open(path)
	if err != nil {
		d.paragraphStyled("[image not found: "+src+"]", "I")
		return
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		d.paragraphStyled("[image: "+src+"]", "I")
		return
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return
	}
	d.images++
	name := fmt.Sprintf("img%d", d.images)
	opt := fpdf.ImageOptions{ImageType: "PNG"}
	d.pdf.RegisterImageOptionsReader(name, opt, &buf)
	b := img.Bounds()
	w := min(d.contentWidth(), float64(b.Dx())*25.4/96)
	h := w * float64(b.Dy()) / float64(b.Dx())
	if h > 150 {
		h, w = 150, 150*float64(b.Dx())/float64(b.Dy())
	}
	d.gap(2)
	d.ensure(h)
	l, _, _, _ := d.pdf.GetMargins()
	d.pdf.ImageOptions(name, l, d.pdf.GetY(), w, h, false, opt, 0, "")
	d.pdf.SetY(d.pdf.GetY() + h + 3)
}

func (d *doc) fenced(lang string, body []string) {
	switch {
	case lang == "columns" || lang == "cols":
		// Columns print one after another.
		var part []string
		for _, l := range body {
			if strings.TrimSpace(l) == "+++" {
				d.blocks(part)
				part = nil
				continue
			}
			part = append(part, l)
		}
		d.blocks(part)
	case preview.IsMathLang(lang):
		d.math(body)
	default:
		if lines, ok := preview.PrintBlockLines(lang, body, d.th, blockCols); ok {
			d.textArt(lines)
			return
		}
		d.code(lang, body)
	}
}

func (d *doc) math(body []string) {
	rows := preview.MathRows(body)
	if len(rows) == 0 {
		return
	}
	d.gap(2)
	l, _, _, _ := d.pdf.GetMargins()
	d.font("sans", "I", 12)
	d.color(inkText)
	for _, row := range rows {
		for _, line := range strings.Split(row, "\n") {
			d.ensure(6.5)
			d.pdf.SetX(l + 8)
			d.pdf.CellFormat(d.contentWidth()-8, 6.5, d.clean(line), "", 1, "L", false, 0, "")
		}
	}
	d.pdf.Ln(2.5)
}

// code draws a fenced code block on a tinted panel, syntax-coloured with the
// print theme. Long lines wrap.
func (d *doc) code(lang string, body []string) {
	d.gap(2)
	d.font("mono", "", codeSize)
	l, _, _, _ := d.pdf.GetMargins()
	width := d.contentWidth()
	charW := d.pdf.GetStringWidth("m")
	perLine := max(int((width-6)/charW), 10)
	colors := highlight.Code(lang, strings.Join(body, "\n"), d.th)

	type piece struct {
		text string
		col  lipgloss.Color
	}
	var rows [][]piece
	for i, line := range body {
		runes := []rune(strings.ReplaceAll(line, "\t", "    "))
		cols := []lipgloss.Color(nil)
		if i < len(colors) && len(colors[i]) == len([]rune(line)) && !strings.Contains(line, "\t") {
			cols = colors[i]
		}
		for start := 0; start == 0 || start < len(runes); start += perLine {
			end := min(start+perLine, len(runes))
			var row []piece
			for j := start; j < end; j++ {
				c := lipgloss.Color("")
				if cols != nil {
					c = cols[j]
				}
				if n := len(row); n > 0 && row[n-1].col == c {
					row[n-1].text += string(runes[j])
				} else {
					row = append(row, piece{string(runes[j]), c})
				}
			}
			rows = append(rows, row)
			if end == len(runes) {
				break
			}
		}
	}
	pad := 2.5
	d.ensure(min(float64(len(rows))*codeLine+2*pad, 40))
	for i, row := range rows {
		top, bottom := 0.0, 0.0
		if i == 0 {
			top = pad
		}
		if i == len(rows)-1 {
			bottom = pad
		}
		d.ensure(codeLine + bottom)
		y := d.pdf.GetY()
		d.fill(paperCode)
		d.pdf.Rect(l, y, width, top+codeLine+bottom, "F")
		x := l + 3
		for _, p := range row {
			c := inkText
			if p.col != "" {
				c = hexColor(string(p.col), inkText)
			}
			d.color(c)
			text := d.clean(p.text)
			w := d.pdf.GetStringWidth(text)
			d.pdf.SetXY(x, y+top)
			d.pdf.CellFormat(w, codeLine, text, "", 0, "L", false, 0, "")
			x += w
		}
		d.pdf.SetXY(l, y+top+codeLine+bottom)
	}
	d.pdf.Ln(3)
}

// textArt draws a view the preview renders as styled terminal cells, keeping
// its colours, in a monospace font sized so blockCols cells fill the width.
func (d *doc) textArt(lines []string) {
	d.gap(2)
	width := d.contentWidth()
	size := width / blockCols / 0.6 * 72 / 25.4 // Go Mono advance is 0.6 em
	lineH := size * 25.4 / 72 * 1.18
	l, _, _, _ := d.pdf.GetMargins()
	cellW := width / blockCols
	_, ph := d.pdf.GetPageSize()
	d.ensure(min(float64(len(lines))*lineH, ph-2*margin)) // keep a view on one page
	for _, line := range lines {
		d.ensure(lineH)
		y := d.pdf.GetY()
		segs := splitBlocks(sgrSegments(line))
		// Backgrounds first, merging touching runs of one colour (chart
		// bars are solid blocks), so no glyph is painted over and no seams
		// show between cells.
		var pending *rgb
		px, pw := 0.0, 0.0
		paint := func() {
			if pending != nil && pw > 0 {
				d.fill(*pending)
				d.pdf.Rect(px, y, pw+0.05, lineH+0.05, "F")
			}
			pending, pw = nil, 0
		}
		x := l
		for _, sg := range segs {
			w := float64(ansi.StringWidth(sg.text)) * cellW
			fill := sg.bg
			if strings.Trim(sg.text, "█") == "" {
				fill = sg.fg
				if fill == nil {
					fill = &inkText
				}
			}
			switch {
			case fill == nil:
				paint()
			case pending != nil && *pending == *fill && px+pw >= x-0.01:
				pw += w
			default:
				paint()
				pending, px, pw = fill, x, w
			}
			x += w
		}
		paint()
		x = l
		for _, sg := range segs {
			w := float64(ansi.StringWidth(sg.text)) * cellW
			if strings.TrimSpace(strings.Trim(sg.text, "█")) == "" {
				x += w
				continue
			}
			style := ""
			if sg.bold {
				style = "B"
			}
			d.font("mono", style, size)
			fg := inkText
			if sg.fg != nil {
				fg = *sg.fg
			}
			d.color(fg)
			// One cell at a time keeps columns exact even where a glyph
			// was swapped or is wider than the cell.
			cx := x
			for _, r := range sg.text {
				rw := float64(ansi.StringWidth(string(r))) * cellW
				if t := d.clean(string(r)); t != "" && t != " " {
					d.pdf.SetXY(cx, y)
					d.pdf.CellFormat(rw, lineH, t, "", 0, "L", false, 0, "")
				}
				cx += rw
			}
			x += w
		}
		d.pdf.SetXY(l, y+lineH)
	}
	d.pdf.Ln(3)
}

// splitBlocks cuts segments so solid block runs ("█") stand alone and can
// be painted as rectangles, even when a bar ends in a partial block.
func splitBlocks(segs []segment) []segment {
	var out []segment
	for _, sg := range segs {
		start, solid := 0, false
		for i, r := range sg.text {
			if s := r == '█'; s != solid && i > start {
				part := sg
				part.text = sg.text[start:i]
				out = append(out, part)
				start = i
			}
			solid = r == '█'
		}
		part := sg
		part.text = sg.text[start:]
		out = append(out, part)
	}
	return out
}

// --- inline text -----------------------------------------------------------------

type run struct {
	text                       string
	bold, italic, code, strike bool
	link                       string
}

// plain is the visible text of inline Markdown.
func plain(s string) string {
	var b strings.Builder
	for _, r := range parseInline(s) {
		b.WriteString(r.text)
	}
	return b.String()
}

var inlineTag = regexp.MustCompile(`</?[A-Za-z][^>]*>`)

// parseInline splits inline Markdown into styled runs.
func parseInline(s string) []run {
	var out []run
	var walk func(s string, st run)
	walk = func(s string, st run) {
		var plainText strings.Builder
		emit := func() {
			if plainText.Len() > 0 {
				r := st
				r.text = plainText.String()
				out = append(out, r)
				plainText.Reset()
			}
		}
		for i := 0; i < len(s); {
			rest := s[i:]
			switch {
			case rest[0] == '\\' && len(rest) > 1 && strings.ContainsRune("\\`*_{}[]()#+-.!~<>|$", rune(rest[1])):
				plainText.WriteByte(rest[1])
				i += 2
				continue
			case rest[0] == '`':
				n := 1
				for n < len(rest) && rest[n] == '`' {
					n++
				}
				if end := strings.Index(rest[n:], rest[:n]); end >= 0 {
					emit()
					r := st
					r.code, r.text = true, strings.TrimSpace(rest[n:n+end])
					out = append(out, r)
					i += n + end + n
					continue
				}
			case strings.HasPrefix(rest, "**") || strings.HasPrefix(rest, "__"):
				if end := strings.Index(rest[2:], rest[:2]); end > 0 {
					emit()
					r := st
					r.bold = true
					walk(rest[2:2+end], r)
					i += 2 + end + 2
					continue
				}
			case strings.HasPrefix(rest, "~~"):
				if end := strings.Index(rest[2:], "~~"); end > 0 {
					emit()
					r := st
					r.strike = true
					walk(rest[2:2+end], r)
					i += 2 + end + 2
					continue
				}
			case rest[0] == '*' || (rest[0] == '_' && (i == 0 || s[i-1] == ' ')):
				if end := strings.IndexByte(rest[1:], rest[0]); end > 0 && rest[1] != ' ' {
					emit()
					r := st
					r.italic = true
					walk(rest[1:1+end], r)
					i += 1 + end + 1
					continue
				}
			case strings.HasPrefix(rest, "[[") || strings.HasPrefix(rest, "![["):
				if l, n, ok := core.WikiLinkAt(rest); ok {
					emit()
					r := st
					r.text = orDefault(l.Label(), l.Target)
					out = append(out, r)
					i += n
					continue
				}
			case strings.HasPrefix(rest, "!["):
				if text, _, n, ok := preview.LinkAt(rest[1:]); ok {
					emit()
					r := st
					r.italic, r.text = true, "["+orDefault(text, "image")+"]"
					out = append(out, r)
					i += 1 + n
					continue
				}
			case rest[0] == '[':
				if text, url, n, ok := preview.LinkAt(rest); ok {
					emit()
					r := st
					if strings.Contains(url, "://") || strings.HasPrefix(url, "mailto:") {
						r.link = url
					}
					walk(text, r)
					i += n
					continue
				}
			case rest[0] == '<':
				if m := inlineTag.FindString(rest); m != "" && strings.HasPrefix(rest, m) {
					if strings.HasPrefix(strings.ToLower(m), "<br") {
						plainText.WriteByte(' ')
					}
					i += len(m)
					continue
				}
			}
			plainText.WriteByte(s[i])
			i++
		}
		emit()
	}
	walk(s, run{})
	return out
}

func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

// writeRuns writes styled runs as flowing text that wraps at the margins.
func (d *doc) writeRuns(runs []run, family, baseStyle string, size, lineH float64, ink rgb) {
	for _, r := range runs {
		style := baseStyle
		if r.bold && !strings.Contains(style, "B") {
			style += "B"
		}
		if r.italic && !strings.Contains(style, "I") {
			style += "I"
		}
		fam, sz, c := family, size, ink
		switch {
		case r.code:
			fam, sz, c = "mono", size*0.9, inkCode
			style = strings.ReplaceAll(style, "I", "")
		case r.link != "":
			c = inkLink
		case r.strike:
			c = inkMuted
		}
		if fam == "mono" && style == "BI" {
			style = "B"
		}
		d.font(fam, normStyle(style), sz)
		d.color(c)
		text := d.clean(r.text)
		if r.link != "" {
			d.pdf.WriteLinkString(lineH, text, r.link)
		} else {
			d.pdf.Write(lineH, text)
		}
	}
}

// normStyle orders style letters the way fpdf registered them ("BI").
func normStyle(s string) string {
	b, i := strings.Contains(s, "B"), strings.Contains(s, "I")
	switch {
	case b && i:
		return "BI"
	case b:
		return "B"
	case i:
		return "I"
	}
	return ""
}

// --- terminal colour codes -------------------------------------------------------

type segment struct {
	text   string
	fg, bg *rgb
	bold   bool
}

// sgrSegments splits a line of terminal output into runs of text with their
// colours, reading the SGR escape codes lipgloss writes.
func sgrSegments(line string) []segment {
	var out []segment
	var cur segment
	var text strings.Builder
	flush := func() {
		if text.Len() > 0 {
			s := cur
			s.text = text.String()
			out = append(out, s)
			text.Reset()
		}
	}
	for i := 0; i < len(line); {
		if line[i] == 0x1b && i+1 < len(line) && line[i+1] == '[' {
			end := i + 2
			for end < len(line) && (line[end] < 0x40 || line[end] > 0x7e) {
				end++
			}
			if end >= len(line) {
				break
			}
			if line[end] == 'm' {
				flush()
				applySGR(&cur, line[i+2:end])
			}
			i = end + 1
			continue
		}
		if line[i] == 0x1b { // other escapes (OSC links …): skip to the terminator
			end := strings.IndexAny(line[i+1:], "\a\\")
			if end < 0 {
				break
			}
			i += end + 2
			continue
		}
		text.WriteByte(line[i])
		i++
	}
	flush()
	return out
}

func applySGR(s *segment, params string) {
	p := strings.Split(params, ";")
	for i := 0; i < len(p); i++ {
		switch p[i] {
		case "", "0":
			*s = segment{}
		case "1":
			s.bold = true
		case "22":
			s.bold = false
		case "39":
			s.fg = nil
		case "49":
			s.bg = nil
		case "38", "48":
			var c *rgb
			kind := p[i]
			switch {
			case i+4 < len(p) && p[i+1] == "2":
				r, _ := strconv.Atoi(p[i+2])
				g, _ := strconv.Atoi(p[i+3])
				b, _ := strconv.Atoi(p[i+4])
				c = &rgb{r, g, b}
				i += 4
			case i+2 < len(p) && p[i+1] == "5":
				n, _ := strconv.Atoi(p[i+2])
				c = xterm(n)
				i += 2
			}
			if kind == "48" {
				s.bg = c
			} else {
				s.fg = c
			}
		}
	}
}

// xterm converts a 256-colour palette index to RGB.
func xterm(n int) *rgb {
	base := []rgb{
		{0, 0, 0}, {205, 0, 0}, {0, 205, 0}, {205, 205, 0}, {0, 0, 238}, {205, 0, 205}, {0, 205, 205}, {229, 229, 229},
		{127, 127, 127}, {255, 0, 0}, {0, 255, 0}, {255, 255, 0}, {92, 92, 255}, {255, 0, 255}, {0, 255, 255}, {255, 255, 255},
	}
	switch {
	case n < 0 || n > 255:
		return nil
	case n < 16:
		c := base[n]
		return &c
	case n < 232:
		n -= 16
		level := func(v int) int {
			if v == 0 {
				return 0
			}
			return 55 + v*40
		}
		return &rgb{level(n / 36), level(n / 6 % 6), level(n % 6)}
	}
	v := 8 + (n-232)*10
	return &rgb{v, v, v}
}

// hexColor parses "#rrggbb", returning def when it can't.
func hexColor(s string, def rgb) rgb {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return def
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return def
	}
	return rgb{int(v >> 16), int(v >> 8 & 0xff), int(v & 0xff)}
}

// Target works out where an export of the note name in noteDir goes: arg as
// given (a file, or a folder to put name.pdf in), relative to noteDir, or
// noteDir/name.pdf when arg is empty. A missing ".pdf" is added.
func Target(arg, noteDir, name string) string {
	arg = strings.TrimSpace(arg)
	folder := strings.HasSuffix(arg, "/") || strings.HasSuffix(arg, string(filepath.Separator))
	if strings.HasPrefix(arg, "~/") || arg == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			arg = filepath.Join(home, arg[1:])
		}
	}
	if arg != "" && !filepath.IsAbs(arg) {
		arg = filepath.Join(noteDir, arg)
	}
	if arg == "" {
		arg = noteDir
	}
	if info, err := os.Stat(arg); err == nil && info.IsDir() || folder {
		arg = filepath.Join(arg, name)
	}
	if !strings.EqualFold(filepath.Ext(arg), ".pdf") {
		arg += ".pdf"
	}
	return arg
}
