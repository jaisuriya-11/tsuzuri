package export

import (
	"fmt"
	"html"
	"strings"
)

// HTMLArt turns styled terminal lines (a board, chart, flowchart … from
// preview.PrintBlockLines) into HTML spans, one line per row, keeping their
// colours and bold text. Text without a colour inherits the page's.
func HTMLArt(lines []string) string {
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		for _, s := range sgrSegments(line) {
			text := html.EscapeString(s.text)
			var style []string
			if s.fg != nil {
				style = append(style, "color:"+s.fg.css())
			}
			if s.bg != nil {
				style = append(style, "background:"+s.bg.css())
			}
			if s.bold {
				style = append(style, "font-weight:600")
			}
			if len(style) == 0 {
				b.WriteString(text)
				continue
			}
			fmt.Fprintf(&b, `<span style="%s">%s</span>`, strings.Join(style, ";"), text)
		}
	}
	return b.String()
}

func (c rgb) css() string { return fmt.Sprintf("#%02x%02x%02x", c.r, c.g, c.b) }
