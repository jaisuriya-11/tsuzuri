package export

import (
	"path/filepath"
	"strings"

	"github.com/jaisuriya-11/tsuzuri/internal/core"
	"github.com/jaisuriya-11/tsuzuri/internal/preview"
)

// Printable prepares a note for paper: each "![[embed]]" line is replaced
// by the note, section or image it shows, and "^id" block anchors are
// removed.
func Printable(text string, lookup preview.NoteLookup) string { return printable(text, lookup, 0) }

func printable(text string, lookup preview.NoteLookup, depth int) string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	fence := ""
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if fence != "" || strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			switch {
			case fence == "":
				fence = t[:3]
			case strings.HasPrefix(t, fence):
				fence = ""
			}
			out = append(out, line)
			continue
		}
		l, n, ok := core.WikiLinkAt(t)
		if !ok || !l.Embed || n != len(t) {
			out = append(out, stripBlockID(line))
			continue
		}
		note, found := lookup(l.Target)
		section, inNote := core.LinkSection(note.Content, l)
		switch {
		case found && note.File != "":
			out = append(out, "![](<"+filepath.ToSlash(note.File)+">)")
		case !found || !inNote || depth >= 3:
			out = append(out, "> "+strings.TrimSpace(linkName(l)))
		default:
			_, body := preview.SplitFrontMatter(section)
			out = append(out, "", printable(body, lookup, depth+1), "")
		}
	}
	return strings.Join(out, "\n")
}

func linkName(l core.WikiLink) string {
	if s := l.Label(); s != "" {
		return s
	}
	return l.Target
}

func stripBlockID(line string) string {
	if loc := core.BlockIDRe.FindStringIndex(line); loc != nil {
		return strings.TrimRight(line[:loc[0]], " ")
	}
	return line
}
