package preview

import (
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/jaisuriya-11/tsuzuri/internal/core"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Note is a linked note, or a linked non-note file such as an image.
type Note struct {
	ID      string // note ID ("" for a non-note file)
	Title   string
	Content string
	Dir     string // absolute folder of the note, for its relative images
	File    string // absolute path of a non-note file
}

// NoteLookup finds the note a [[link]] target names ("" is the note being
// previewed).
type NoteLookup func(target string) (Note, bool)

// StoreNotes resolves links against store's notes (all of them when notes
// is nil) as seen from the note fromID. live holds the unsaved text of open
// notes, which wins over what is on disk.
func StoreNotes(store *core.Store, notes []core.Page, fromID string, live map[string]string) NoteLookup {
	if notes == nil {
		notes = store.List()
	}
	return func(target string) (Note, bool) {
		if isImageName(target) {
			abs, ok := store.FindFile(target, fromID)
			return Note{File: abs}, ok
		}
		p, ok := core.ResolveNote(notes, target, fromID)
		if !ok {
			return Note{}, false
		}
		text, open := live[p.ID]
		if !open {
			full, err := store.Get(p.ID)
			if err != nil {
				return Note{}, false
			}
			text = full.Content
		}
		dir := filepath.Join(store.Root(), filepath.FromSlash(path.Dir(p.ID)))
		return Note{ID: p.ID, Title: p.Title, Content: text, Dir: dir}, true
	}
}

// Link hit kinds, reported through HitMsg. Arg holds the target.
const (
	HitWikiLink = "link:wiki" // Arg: "Note#Heading" or "Note#^block"
	HitURLLink  = "link:url"  // Arg: the URL or relative path
	HitNoteLink = "link:note" // Arg: note ID, Index: 1-based line (backlinks)
)

// maxEmbedDepth stops notes that embed each other from recursing forever.
const maxEmbedDepth = 3

// Links are marked while compiling with OSC 8 hyperlink sequences: they
// are zero-width, so wrapping and width maths ignore them, and once the
// page is laid out collectLinks turns them into click targets and removes
// them.
var oscLinkRe = regexp.MustCompile("\x1b\\]8;[^;\x07\x1b]*;([^\x07\x1b]*)(?:\x07|\x1b\\\\)")

func markLink(kind, arg, rendered string) string {
	return ansi.SetHyperlink(kind+"|"+arg) + rendered + ansi.ResetHyperlink()
}

// collectLinks records a hit for every marked link (one per row a wrapped
// link spans) and strips the markers from the output.
func (c *compiler) collectLinks() {
	open, start := "", -1
	for r, line := range c.out {
		if line == "" {
			open = ""
			continue
		}
		if open != "" {
			plain := ansi.Strip(line)
			start = ansi.StringWidth(plain) - ansi.StringWidth(strings.TrimLeft(plain, " "))
		}
		locs := oscLinkRe.FindAllStringSubmatchIndex(line, -1)
		if locs == nil && open == "" {
			continue
		}
		var b strings.Builder
		last := 0
		for _, l := range locs {
			b.WriteString(line[last:l[0]])
			col := ansi.StringWidth(b.String())
			if open != "" && col > start {
				c.addLinkHit(r, start, col, open)
			}
			open, start = line[l[2]:l[3]], col
			last = l[1]
		}
		b.WriteString(line[last:])
		c.out[r] = b.String()
		if open != "" {
			if end := ansi.StringWidth(strings.TrimRight(ansi.Strip(c.out[r]), " ")); end > start {
				c.addLinkHit(r, start, end, open)
			}
		}
	}
}

func (c *compiler) addLinkHit(row, x0, x1 int, uri string) {
	kind, arg, ok := strings.Cut(uri, "|")
	if !ok {
		return
	}
	c.hits = append(c.hits, Hit{Row: row, H: 1, X0: x0, X1: x1, Kind: "link:" + kind, Arg: arg, Line: -1})
}

// lookup resolves a link target through the preview's NoteLookup.
func (c *compiler) lookup(target string) (Note, bool) {
	if c.cal.Notes == nil {
		return Note{}, false
	}
	return c.cal.Notes(target)
}

// wikiLink renders an inline [[link]]: link-coloured when the note exists,
// dimmed when it doesn't yet (clicking it starts that note).
func (c *compiler) wikiLink(l core.WikiLink) string {
	label := l.Label()
	if isImageName(l.Target) && l.Embed {
		return lipgloss.NewStyle().Foreground(c.st.th.Purple).Render("󰋩 " + orDefault(l.Alias, path.Base(l.Target)))
	}
	if label == "" {
		label = l.Target
	}
	st := c.st.link
	if l.Target != "" && c.cal.Notes != nil {
		if _, ok := c.lookup(l.Target); !ok {
			st = c.st.muted.Underline(true)
		}
	}
	return markLink("wiki", l.Ref(), st.Render(label))
}

// standaloneEmbed reports whether a whole line is one "![[…]]" embed.
func standaloneEmbed(trimmed string) (core.WikiLink, bool) {
	if !strings.HasPrefix(trimmed, "![[") {
		return core.WikiLink{}, false
	}
	l, n, ok := core.WikiLinkAt(trimmed)
	return l, ok && l.Embed && n == len(trimmed)
}

// embed draws a linked note (or one section or block of it) inside the
// current note, under a clickable header. Embedded images are drawn as
// pictures.
func (c *compiler) embed(l core.WikiLink) {
	if isImageName(l.Target) {
		if n, ok := c.lookup(l.Target); ok && n.File != "" {
			c.image(l.Alias, n.File)
			return
		}
		c.emit(lipgloss.NewStyle().Foreground(c.st.th.Purple).Render("󰋩 "+path.Base(l.Target)) + c.st.muted.Render(" (not found)"))
		return
	}

	note, ok := c.lookup(l.Target)
	label := l.Label()
	if label == "" {
		label = note.Title
	}
	if !ok {
		c.emit(markLink("wiki", l.Ref(), c.st.muted.Underline(true).Render("󰈙 "+label)) + c.st.muted.Render("  (no such note)"))
		return
	}
	bar := c.st.quoteBar.Render("▏ ")
	c.emit(markLink("wiki", l.Ref(), c.st.link.Render("󰈙 "+label)))
	section, found := core.LinkSection(note.Content, l)
	switch {
	case !found:
		c.emit(bar + c.st.muted.Render("section not found"))
		return
	case c.depth >= maxEmbedDepth:
		c.emit(bar + c.st.muted.Render("…"))
		return
	}

	sub := &compiler{st: c.st, width: max(c.width-2, 8), baseDir: note.Dir, cal: CalendarView{Notes: c.cal.Notes}, depth: c.depth + 1}
	if sub.baseDir == "" {
		sub.baseDir = c.baseDir
	}
	sub.compile(section)
	for len(sub.out) > 0 && sub.out[0] == "" {
		sub.out = sub.out[1:]
	}
	for len(sub.out) > 0 && sub.out[len(sub.out)-1] == "" {
		sub.out = sub.out[:len(sub.out)-1]
	}
	for _, line := range sub.out {
		c.emit(bar + line)
	}
}

// stripBlockID removes a trailing "^id" block anchor from a line.
func stripBlockID(line string) string {
	if !strings.Contains(line, "^") {
		return line
	}
	loc := core.BlockIDRe.FindStringIndex(line)
	if loc == nil {
		return line
	}
	return strings.TrimRight(line[:loc[0]], " ")
}

var imageExts = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".bmp": true, ".svg": true}

func isImageName(name string) bool { return imageExts[strings.ToLower(filepath.Ext(name))] }

// IsImageName reports whether a link target names an image file.
func IsImageName(name string) bool { return isImageName(name) }

// SetNotes sets how [[links]] are resolved and embeds are found.
func (m *Model) SetNotes(notes NoteLookup) {
	m.notes = notes
	m.recompile()
}

// SetBacklinks lists the notes linking to this one under the page.
func (m *Model) SetBacklinks(links []core.Backlink) {
	m.backlinks = links
	m.recompile()
}

// appendBacklinks adds the "Linked from" section below the note, each
// entry clickable.
func (m *Model) appendBacklinks(compiled string, hits []Hit, width int) (string, []Hit) {
	st := newStyles(m.theme)
	lines := strings.Split(compiled, "\n")
	if compiled == "" {
		lines = nil
	}
	title := " Linked from " + plural(len(m.backlinks), "note") + " "
	lines = append(lines, "", st.rule.Render("──")+st.muted.Bold(true).Render(title)+st.rule.Render(strings.Repeat("─", max(width-2-ansi.StringWidth(title), 0))))
	for _, b := range m.backlinks {
		row := len(lines)
		name := st.link.Render("󰈙 " + b.Title)
		lines = append(lines, name)
		hits = append(hits, Hit{Row: row, H: 2, X1: width, Kind: HitNoteLink, Arg: b.ID, Index: b.Line, Line: -1})
		snippet, _, _ := strings.Cut(ansi.Strip(Compile(b.Text, m.theme, 1000)), "\n")
		lines = append(lines, ansi.Truncate(st.muted.Render("  "+strings.TrimSpace(snippet)), width, "…"))
	}
	return strings.Join(lines, "\n"), hits
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
}

// ScrollToLine scrolls so the block holding document line (0-based) is at
// the top of the preview.
func (m *Model) ScrollToLine(line int) {
	row := -1
	for _, b := range m.blocks {
		if b.Line <= line && line < b.End {
			row = b.Row
			if b.Line == line {
				break
			}
		}
	}
	if row >= 0 {
		m.viewport.SetYOffset(row)
	}
}
