package app

import (
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/jaisuriya-11/tsuzuri/internal/core"
	"github.com/jaisuriya-11/tsuzuri/internal/ui"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// finder is the global, Telescope-style note picker. It searches every
// Markdown note under the workspace root, recursively: file names (fuzzy)
// first, then the text inside the notes (like live grep).
type finder struct {
	input   textinput.Model
	root    string
	notes   []core.Page
	matches []finderMatch
	sel     int
	offset  int
	lines   map[string][]string // note text, split into lines
	link    bool                // pick a note to link to instead of opening it
	wiki    bool                // finish a typed "[[" instead of a Markdown link
}

// maxTextHits caps text matches so huge workspaces stay responsive.
const maxTextHits = 500

type finderMatch struct {
	page  core.Page
	score int
	pos   []int // matched byte offsets into page.ID (name) or text (line hit)
	line  int   // 1-based line of a text hit; 0 for a file-name match
	text  string
}

func (m *Model) openFinder() tea.Cmd {
	th := m.theme
	in := textinput.New()
	in.Prompt = ""
	in.Placeholder = "Search file names and text in every note…"
	in.CharLimit = 120
	field := lipgloss.NewStyle().Background(th.Bg2)
	in.TextStyle = field.Foreground(th.Fg)
	in.PlaceholderStyle = field.Foreground(th.GreyFg)
	in.Cursor.Style = lipgloss.NewStyle().Foreground(th.Blue)
	in.Cursor.TextStyle = field.Foreground(th.Fg)

	var notes []core.Page
	for _, p := range m.store.List() {
		if !p.IsFolder {
			notes = append(notes, p)
		}
	}
	f := &finder{input: in, root: filepath.Base(m.store.Root()), notes: notes, lines: map[string][]string{}}
	for _, p := range notes {
		if full, err := m.store.Get(p.ID); err == nil {
			f.lines[p.ID] = strings.Split(strings.ReplaceAll(full.Content, "\t", "    "), "\n")
		}
	}
	f.refilter()
	m.finder = f
	m.leaderPending = false
	return f.input.Focus()
}

// fuzzyScore matches query as a subsequence of target (case-insensitive).
// Consecutive characters and matches at word or path boundaries score higher,
// and matches inside the file name beat matches in the folder path.
func fuzzyScore(query, target string) (int, []int, bool) {
	if query == "" {
		return 0, nil, true
	}
	q := []rune(strings.ToLower(query))
	t := strings.ToLower(target)
	nameStart := strings.LastIndex(t, "/") + 1

	var pos []int
	score, qi, prev := 0, 0, -2
	for i, r := range t {
		if qi >= len(q) {
			break
		}
		if r != q[qi] {
			continue
		}
		pos = append(pos, i)
		s := 1
		if i == prev+1 {
			s += 5
		}
		if i == 0 || i == nameStart || isBoundary(t[i-1]) {
			s += 4
		}
		if i >= nameStart {
			s += 2
		}
		score += s
		prev = i
		qi++
	}
	if qi < len(q) {
		return 0, nil, false
	}
	return score - len(t)/10, pos, true
}

func isBoundary(b byte) bool {
	r := rune(b)
	return b < 0x80 && !unicode.IsLetter(r) && !unicode.IsDigit(r)
}

func (f *finder) refilter() {
	q := strings.TrimSpace(f.input.Value())
	f.matches = f.matches[:0]
	for _, p := range f.notes {
		if score, pos, ok := fuzzyScore(strings.ReplaceAll(q, " ", ""), p.ID); ok {
			f.matches = append(f.matches, finderMatch{page: p, score: score, pos: pos})
		}
	}
	sort.SliceStable(f.matches, func(i, j int) bool {
		if f.matches[i].score != f.matches[j].score {
			return f.matches[i].score > f.matches[j].score
		}
		return f.matches[i].page.UpdatedAt.After(f.matches[j].page.UpdatedAt)
	})

	// Text inside notes (case-insensitive), in path order, after name hits.
	if needle := strings.ToLower(q); len([]rune(needle)) >= 2 {
		byPath := append([]core.Page(nil), f.notes...)
		sort.Slice(byPath, func(i, j int) bool { return byPath[i].ID < byPath[j].ID })
		hits := 0
	scan:
		for _, p := range byPath {
			for n, l := range f.lines[p.ID] {
				lower := strings.ToLower(l)
				idx := strings.Index(lower, needle)
				if idx < 0 || len(lower) != len(l) {
					continue
				}
				trimmed := strings.TrimLeft(l, " ")
				shift := len(l) - len(trimmed)
				var pos []int
				for k := idx; k < idx+len(needle); k++ {
					pos = append(pos, k-shift)
				}
				f.matches = append(f.matches, finderMatch{page: p, pos: pos, line: n + 1, text: trimmed})
				if hits++; hits >= maxTextHits {
					break scan
				}
			}
		}
	}
	f.sel, f.offset = 0, 0
}

func (f *finder) move(delta, visible int) {
	if len(f.matches) == 0 {
		return
	}
	f.sel = (f.sel + delta + len(f.matches)) % len(f.matches)
	if f.sel < f.offset {
		f.offset = f.sel
	}
	if visible > 0 && f.sel >= f.offset+visible {
		f.offset = f.sel - visible + 1
	}
}

func (f *finder) selected() (core.Page, bool) {
	if f.sel < 0 || f.sel >= len(f.matches) {
		return core.Page{}, false
	}
	return f.matches[f.sel].page, true
}

// finderGeom is the finder's on-screen box.
type finderGeom struct {
	x, y, w, h  int
	listW       int // results column width (inner)
	previewW    int // 0 when too narrow for a preview
	listTop     int // first result row, relative to the inner area
	listRows    int
	innerHeight int
}

func finderLayout(W, H int) finderGeom {
	w := min(max(W*4/5, 50), 130)
	w = min(w, W-2)
	h := min(max(H*7/10, 12), 34)
	h = min(h, H-2)
	g := finderGeom{w: w, h: h}
	g.x, g.y = ui.Center(W, H, w, h)
	inner := w - 2
	g.innerHeight = h - 2
	if inner >= 90 {
		g.listW = inner * 9 / 20
		g.previewW = inner - g.listW - 1
	} else {
		g.listW = inner
	}
	g.listTop = 3
	g.listRows = max(g.innerHeight-g.listTop-1, 1)
	return g
}

func (m *Model) finderOpen(f *finder, newTab bool) tea.Cmd {
	p, ok := f.selected()
	if f.wiki && !ok {
		// No such note yet: link to it anyway; following it starts it.
		if q := strings.TrimSpace(f.input.Value()); q != "" {
			m.finder = nil
			m.insertWikiText(q)
			return m.focusPane(focusEditor)
		}
	}
	if !ok {
		return nil
	}
	if f.link {
		m.finder = nil
		if f.wiki {
			m.insertWikiLink(p)
		} else {
			m.insertLinkTo(p)
		}
		return m.focusPane(focusEditor)
	}
	line := f.matches[f.sel].line
	m.finder = nil
	var cmd tea.Cmd
	if newTab {
		cmd = m.openFile(p.ID)
	} else {
		cmd = m.openInCurrentBuffer(p.ID)
	}
	if line > 0 && m.active == p.ID {
		m.content.GotoLine(line)
	}
	return cmd
}
