package preview

import (
	"strings"

	"github.com/jaisuriya-11/tsuzuri/internal/core"

	"github.com/charmbracelet/lipgloss"
)

// inline renders inline Markdown (code, bold, italic, strike, links, images
// and a little inline HTML) by scanning the text once, so styling is never
// applied to text that already contains escape sequences.
func (c *compiler) inline(s string, base lipgloss.Style) string {
	var out strings.Builder
	var plain strings.Builder
	flush := func() {
		if plain.Len() > 0 {
			out.WriteString(base.Render(plain.String()))
			plain.Reset()
		}
	}
	emit := func(rendered string) {
		flush()
		out.WriteString(rendered)
	}

	for i := 0; i < len(s); {
		rest := s[i:]
		switch {
		case rest[0] == '\\' && len(rest) > 1 && strings.ContainsRune("\\`*_{}[]()#+-.!~<>|", rune(rest[1])):
			plain.WriteByte(rest[1])
			i += 2
			continue

		case rest[0] == '`':
			n := 1
			for n < len(rest) && rest[n] == '`' {
				n++
			}
			fence := rest[:n]
			if end := strings.Index(rest[n:], fence); end >= 0 {
				emit(c.st.code.Render(" " + strings.TrimSpace(rest[n:n+end]) + " "))
				i += n + end + n
				continue
			}

		case strings.HasPrefix(rest, "**") || strings.HasPrefix(rest, "__"):
			if end := strings.Index(rest[2:], rest[:2]); end > 0 {
				emit(c.inline(rest[2:2+end], base.Bold(true)))
				i += 2 + end + 2
				continue
			}

		case strings.HasPrefix(rest, "~~"):
			if end := strings.Index(rest[2:], "~~"); end > 0 {
				emit(c.inline(rest[2:2+end], base.Strikethrough(true).Foreground(c.st.th.GreyFg2)))
				i += 2 + end + 2
				continue
			}

		case rest[0] == '*' || (rest[0] == '_' && (i == 0 || s[i-1] == ' ')):
			if end := strings.IndexByte(rest[1:], rest[0]); end > 0 && rest[1] != ' ' {
				emit(c.inline(rest[1:1+end], base.Italic(true)))
				i += 1 + end + 1
				continue
			}

		case strings.HasPrefix(rest, "[[") || strings.HasPrefix(rest, "![["):
			if l, n, ok := core.WikiLinkAt(rest); ok {
				emit(c.wikiLink(l))
				i += n
				continue
			}

		case strings.HasPrefix(rest, "!["):
			if text, _, n, ok := linkAt(rest[1:]); ok {
				emit(lipgloss.NewStyle().Foreground(c.st.th.Purple).Render("󰋩 " + orDefault(text, "image")))
				i += 1 + n
				continue
			}

		case rest[0] == '[':
			if text, url, n, ok := linkAt(rest); ok {
				emit(markLink("url", strings.Trim(url, "<>"), c.inline(text, c.st.link)))
				i += n
				continue
			}

		case rest[0] == '<':
			end := strings.IndexByte(rest, '>')
			if end < 0 {
				break
			}
			tag := rest[:end+1]
			lower := strings.ToLower(tag)
			switch {
			case strings.HasPrefix(lower, "<http"):
				emit(markLink("url", tag[1:len(tag)-1], c.st.link.Render(tag[1:len(tag)-1])))
			case strings.HasPrefix(lower, "<img"):
				emit(lipgloss.NewStyle().Foreground(c.st.th.Purple).Render("󰋩 " + orDefault(attr(altAttr, tag), "image")))
			case strings.HasPrefix(lower, "<a "):
				closeIdx := strings.Index(strings.ToLower(rest), "</a>")
				if closeIdx > end {
					emit(c.inline(rest[end+1:closeIdx], c.st.link))
					i += closeIdx + len("</a>")
					continue
				}
			case strings.HasPrefix(lower, "<br"):
				plain.WriteByte(' ')
			case strings.HasPrefix(lower, "<kbd>"):
				if ce := strings.Index(strings.ToLower(rest), "</kbd>"); ce > end {
					emit(c.st.code.Render(" " + rest[end+1:ce] + " "))
					i += ce + len("</kbd>")
					continue
				}
			case looksLikeTag(lower):
				// Unknown or layout tag: drop it, keep its text.
			default:
				plain.WriteByte('<')
				i++
				continue
			}
			i += end + 1
			continue
		}

		plain.WriteByte(rest[0])
		i++
	}
	flush()
	return out.String()
}

func looksLikeTag(lower string) bool {
	if len(lower) < 3 {
		return false
	}
	ch := lower[1]
	if ch == '/' && len(lower) > 3 {
		ch = lower[2]
	}
	return ch >= 'a' && ch <= 'z'
}

// linkAt parses "[text](url)" at the start of s, returning the byte length.
func linkAt(s string) (text, url string, n int, ok bool) {
	if !strings.HasPrefix(s, "[") {
		return "", "", 0, false
	}
	depth := 0
	closeText := -1
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				closeText = i
			}
		}
		if closeText >= 0 {
			break
		}
	}
	if closeText < 0 || closeText+1 >= len(s) || s[closeText+1] != '(' {
		return "", "", 0, false
	}
	end := strings.IndexByte(s[closeText+1:], ')')
	if end < 0 {
		return "", "", 0, false
	}
	return s[1:closeText], s[closeText+2 : closeText+1+end], closeText + 1 + end + 1, true
}

func orDefault(s, d string) string {
	if strings.TrimSpace(s) == "" {
		return d
	}
	return s
}
