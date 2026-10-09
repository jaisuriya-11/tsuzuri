package core

import (
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
)

// WikiLink is a "[[Target#Heading|Alias]]" reference between notes, or an
// embed when written "![[…]]". Block references use "#^id".
type WikiLink struct {
	Target  string // note name or path; "" means the note it is written in
	Heading string // heading text after "#"
	Block   string // block id after "#^"
	Alias   string // text shown instead of the target
	Embed   bool
}

var (
	wikiLinkRe = regexp.MustCompile(`(!?)\[\[([^\[\]\n]+)\]\]`)
	// BlockIDRe matches a block id ("^id") at the end of a line.
	BlockIDRe = regexp.MustCompile(`(^|\s)\^([A-Za-z0-9-]+)\s*$`)
	headingRe = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*#*\s*$`)
)

// ParseWikiLink parses the text between "[[" and "]]".
func ParseWikiLink(inner string) WikiLink {
	var l WikiLink
	inner, l.Alias, _ = strings.Cut(inner, "|")
	l.Alias = strings.TrimSpace(l.Alias)
	target, frag, _ := strings.Cut(inner, "#")
	l.Target = strings.TrimSpace(target)
	frag = strings.TrimSpace(frag)
	if id, ok := strings.CutPrefix(frag, "^"); ok {
		l.Block = id
	} else {
		l.Heading = frag
	}
	return l
}

// WikiLinkAt parses "[[…]]" or "![[…]]" at the start of s, returning the
// link and its byte length.
func WikiLinkAt(s string) (WikiLink, int, bool) {
	loc := wikiLinkRe.FindStringSubmatchIndex(s)
	if loc == nil || loc[0] != 0 {
		return WikiLink{}, 0, false
	}
	l := ParseWikiLink(s[loc[4]:loc[5]])
	l.Embed = loc[3] > loc[2]
	if l.Target == "" && l.Heading == "" && l.Block == "" {
		return WikiLink{}, 0, false
	}
	return l, loc[1], true
}

// Ref is the link target without the alias, as written ("Note#Heading").
func (l WikiLink) Ref() string {
	switch {
	case l.Block != "":
		return l.Target + "#^" + l.Block
	case l.Heading != "":
		return l.Target + "#" + l.Heading
	}
	return l.Target
}

// Label is the text a link shows: its alias, or the target and section.
func (l WikiLink) Label() string {
	if l.Alias != "" {
		return l.Alias
	}
	name := path.Base(strings.TrimSuffix(l.Target, mdExt))
	if l.Target == "" {
		name = ""
	}
	sec := l.Heading
	if l.Block != "" {
		sec = "^" + l.Block
	}
	switch {
	case sec == "":
		return name
	case name == "":
		return sec
	}
	return name + " › " + sec
}

// WikiLinks returns every wiki link in text, skipping fenced code blocks
// and inline code.
func WikiLinks(text string) []WikiLink {
	var out []WikiLink
	fence := ""
	for _, line := range strings.Split(text, "\n") {
		t := strings.TrimSpace(line)
		if fence != "" {
			if strings.HasPrefix(t, fence) {
				fence = ""
			}
			continue
		}
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			fence = t[:3]
			continue
		}
		for _, m := range wikiLinkRe.FindAllStringSubmatchIndex(stripInlineCode(line), -1) {
			l := ParseWikiLink(line[m[4]:m[5]])
			l.Embed = m[3] > m[2]
			out = append(out, l)
		}
	}
	return out
}

// stripInlineCode blanks `code` spans, keeping byte offsets unchanged.
func stripInlineCode(line string) string {
	b := []byte(line)
	for i := 0; i < len(b); i++ {
		if b[i] != '`' {
			continue
		}
		end := strings.IndexByte(line[i+1:], '`')
		if end < 0 {
			break
		}
		for j := i; j <= i+1+end; j++ {
			b[j] = ' '
		}
		i += end + 1
	}
	return string(b)
}

// ResolveNote finds the note a link target names among notes. A target is a
// path from the workspace root ("Projects/AWS") or just a note name ("AWS"),
// with or without ".md", matched case-insensitively. A path relative to
// fromID's folder wins, then one from the root; when several notes share a
// name, the one in fromID's folder wins, then the shortest path.
func ResolveNote(notes []Page, target, fromID string) (Page, bool) {
	t := strings.ToLower(strings.Trim(strings.ReplaceAll(strings.TrimSpace(target), `\`, "/"), "/"))
	t = strings.TrimSuffix(t, mdExt)
	if t == "" {
		return Page{}, false
	}
	fromDir := strings.ToLower(path.Dir(fromID))
	var best Page
	bestScore := -1
	for _, p := range notes {
		if p.IsFolder {
			continue
		}
		id := strings.ToLower(strings.TrimSuffix(p.ID, mdExt))
		score := -1
		switch {
		case path.Join(fromDir, t) == id:
			score = 3 // relative to the linking note's folder
		case id == t:
			score = 2
		case strings.HasSuffix(id, "/"+t):
			score = 1
		}
		if score < 0 {
			continue
		}
		if score > bestScore || score == bestScore && closer(p.ID, best.ID, fromDir) {
			best, bestScore = p, score
		}
	}
	return best, bestScore >= 0
}

// closer reports whether a sits nearer to dir than b (same folder first,
// then the shorter path).
func closer(a, b, dir string) bool {
	ad, bd := strings.ToLower(path.Dir(a)) == dir, strings.ToLower(path.Dir(b)) == dir
	if ad != bd {
		return ad
	}
	return len(a) < len(b)
}

// LinkLine returns the 0-based line a link's heading or block points at in
// content, or -1 when the link names neither or nothing matches.
func LinkLine(content string, l WikiLink) int {
	lines := strings.Split(content, "\n")
	switch {
	case l.Block != "":
		for i, line := range lines {
			if m := BlockIDRe.FindStringSubmatch(line); m != nil && m[2] == l.Block {
				if strings.TrimSpace(line) == "^"+l.Block && i > 0 {
					return i - 1 // an id on its own line marks the block above
				}
				return i
			}
		}
	case l.Heading != "":
		want := headingKey(l.Heading)
		for i, line := range lines {
			if m := headingRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil && headingKey(m[2]) == want {
				return i
			}
		}
	}
	return -1
}

// headingKey normalises heading text so "S3 Storage", "s3-storage" and
// "S3  storage!" all match.
func headingKey(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r == ' ' || r == '-' || r == '_':
			b.WriteByte(' ')
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r > 0x7f:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// LinkSection returns the part of content a link points at: a heading and
// the lines under it up to the next heading of the same or higher level, a
// single block, or the whole note. ok is false when the section is missing.
func LinkSection(content string, l WikiLink) (string, bool) {
	if l.Heading == "" && l.Block == "" {
		return content, true
	}
	start := LinkLine(content, l)
	if start < 0 {
		return "", false
	}
	lines := strings.Split(content, "\n")
	end := start + 1
	if l.Block != "" {
		end = blockEnd(lines, start)
		start = blockStart(lines, start)
	} else {
		level := len(headingRe.FindStringSubmatch(strings.TrimSpace(lines[start]))[1])
		fence := ""
		for ; end < len(lines); end++ {
			t := strings.TrimSpace(lines[end])
			if fence != "" {
				if strings.HasPrefix(t, fence) {
					fence = ""
				}
				continue
			}
			if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
				fence = t[:3]
				continue
			}
			if m := headingRe.FindStringSubmatch(t); m != nil && len(m[1]) <= level {
				break
			}
		}
	}
	return strings.TrimRight(strings.Join(lines[start:end], "\n"), "\n"), true
}

// blockStart and blockEnd widen a block reference on line i to its whole
// paragraph; list items and headings are blocks of their own.
func blockStart(lines []string, i int) int {
	if isListOrHeading(lines[i]) {
		return i
	}
	for i > 0 && strings.TrimSpace(lines[i-1]) != "" && !isListOrHeading(lines[i-1]) {
		i--
	}
	return i
}

func blockEnd(lines []string, i int) int {
	if isListOrHeading(lines[i]) {
		return i + 1
	}
	for i+1 < len(lines) && strings.TrimSpace(lines[i+1]) != "" && !isListOrHeading(lines[i+1]) {
		i++
	}
	return i + 1
}

var listItemRe = regexp.MustCompile(`^\s*([-*+]|\d+[.)])\s`)

func isListOrHeading(line string) bool {
	t := strings.TrimSpace(line)
	return listItemRe.MatchString(line) || headingRe.MatchString(t) || strings.HasPrefix(t, "^")
}

// Backlink is a note that links to another one.
type Backlink struct {
	ID    string
	Title string
	Line  int    // 1-based line of the first link
	Text  string // that line, trimmed
}

// Backlinks lists the notes linking to id, with [[wiki links]] or Markdown
// links to its file, sorted by title. Links from id to itself don't count.
func (s *Store) Backlinks(id string) []Backlink {
	notes := s.notes()
	var out []Backlink
	for _, p := range notes {
		if p.ID == id {
			continue
		}
		full, err := s.Get(p.ID)
		if err != nil {
			continue
		}
		for _, l := range noteLinks(notes, full) {
			if l.to == id {
				out = append(out, Backlink{ID: p.ID, Title: p.Title, Line: l.line, Text: l.text})
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Title) < strings.ToLower(out[j].Title) })
	return out
}

// Edge is a link from one note to another, as drawn in the note graph.
type Edge struct{ From, To string }

// LinkGraph returns every note and the links between them: [[wiki links]]
// and Markdown links to note files, each pair once per direction, with
// self-links left out.
func (s *Store) LinkGraph() (notes []Page, edges []Edge) {
	notes = s.notes()
	seen := map[Edge]bool{}
	for _, p := range notes {
		full, err := s.Get(p.ID)
		if err != nil {
			continue
		}
		for _, l := range noteLinks(notes, full) {
			e := Edge{From: p.ID, To: l.to}
			if l.to != p.ID && !seen[e] {
				seen[e] = true
				edges = append(edges, e)
			}
		}
	}
	return notes, edges
}

// notes lists the workspace's notes, without folders.
func (s *Store) notes() []Page {
	var out []Page
	for _, p := range s.List() {
		if !p.IsFolder {
			out = append(out, p)
		}
	}
	return out
}

var mdLinkRe = regexp.MustCompile(`\]\(<?([^)>]+?\.md)>?(#[^)]*)?\)`)

// OutLinks lists the notes that from links to, in order, without repeats.
func OutLinks(notes []Page, from Page) []string {
	var out []string
	seen := map[string]bool{}
	for _, l := range noteLinks(notes, from) {
		if !seen[l.to] {
			seen[l.to] = true
			out = append(out, l.to)
		}
	}
	return out
}

// noteLink is one resolved link inside a note.
type noteLink struct {
	to   string // linked note ID
	line int    // 1-based line
	text string // that line, trimmed
}

// noteLinks finds every link in from that resolves to a note, skipping code.
func noteLinks(notes []Page, from Page) []noteLink {
	var out []noteLink
	fence := ""
	for i, line := range strings.Split(from.Content, "\n") {
		t := strings.TrimSpace(line)
		if fence != "" {
			if strings.HasPrefix(t, fence) {
				fence = ""
			}
			continue
		}
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			fence = t[:3]
			continue
		}
		code := stripInlineCode(line)
		for _, m := range wikiLinkRe.FindAllStringSubmatch(code, -1) {
			l := ParseWikiLink(m[2])
			if p, ok := ResolveNote(notes, l.Target, from.ID); ok {
				out = append(out, noteLink{p.ID, i + 1, t})
			}
		}
		for _, m := range mdLinkRe.FindAllStringSubmatch(code, -1) {
			if strings.Contains(m[1], "://") {
				continue
			}
			target := m[1]
			if u, err := url.PathUnescape(target); err == nil {
				target = u
			}
			id := path.Clean(path.Join(path.Dir(from.ID), target))
			for _, p := range notes {
				if p.ID == id {
					out = append(out, noteLink{id, i + 1, t})
					break
				}
			}
		}
	}
	return out
}
