package app_test

import (
	"strings"
	"testing"

	"github.com/jaisuriya-11/tsuzuri/internal/app"
	"github.com/jaisuriya-11/tsuzuri/internal/core"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// linkedNotes is a small workspace whose notes link to each other.
func linkedNotes(t *testing.T) *core.Store {
	t.Helper()
	store := newTestStore(t)
	for _, n := range []struct{ dir, name, text string }{
		{"", "Start", "Learn about [[Docker]] or [[AWS Notes#EC2 Compute|compute]].\n\nNew idea: [[Fresh Idea]]\n\nWeb: [docs](https://example.com/docs)"},
		{"", "Docker", "# Docker\n\ncontainers"},
		{"tech", "AWS Notes", "# AWS Notes\n\n" + strings.Repeat("filler\n\n", 30) + "## EC2 Compute\n\nvirtual machines"},
	} {
		if _, err := store.SaveAs(n.dir, n.name, n.text); err != nil {
			t.Fatal(err)
		}
	}
	return store
}

// openStart opens Start.md (the most recent file is listed first, so go
// through the finder).
func openStart(t *testing.T, store *core.Store, opts ...app.Option) *harness {
	t.Helper()
	h := newHarness(t, store, opts...)
	h.send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'\\'}})
	h.keys("start")
	h.key(tea.KeyEnter)
	if v := h.view(); !strings.Contains(v, "Learn about [[Docker]]") {
		t.Fatalf("Start.md not open:\n%s", v)
	}
	return h
}

func TestClickWikiLinkOpensNote(t *testing.T) {
	h := openStart(t, linkedNotes(t))
	h.clickPreview("Docker")
	if v := h.view(); !strings.Contains(v, "1 # Docker") || strings.Contains(v, "Learn about [[Docker]]") {
		t.Fatalf("expected Docker.md in place of Start.md:\n%s", v)
	}
	if v := h.view(); !strings.Contains(v, "Linked from 1 note") || !strings.Contains(v, "Learn about Docker or compute.") {
		t.Fatalf("expected a backlink to Start:\n%s", v)
	}
}

func TestHeadingLinkJumpsToSection(t *testing.T) {
	h := openStart(t, linkedNotes(t))
	h.clickPreview("compute")
	v := h.view()
	if !strings.Contains(v, "## EC2 Compute") || !strings.Contains(v, "virtual machines") {
		t.Fatalf("expected AWS Notes scrolled to EC2 Compute:\n%s", v)
	}
	if !strings.Contains(v, "Ln 63") {
		t.Fatalf("expected the cursor on the heading line (63):\n%s", v)
	}
}

func TestUnresolvedLinkStartsDraftWithItsName(t *testing.T) {
	store := linkedNotes(t)
	h := openStart(t, store)
	h.clickPreview("Fresh Idea")
	h.keys("hello")
	h.key(tea.KeyEsc)
	h.send(tea.KeyMsg{Type: tea.KeyCtrlS})
	if v := h.view(); !strings.Contains(v, "Fresh Idea") {
		t.Fatalf("Save As should offer the link's name:\n%s", v)
	}
	h.key(tea.KeyEnter)
	p, err := store.Get("Fresh Idea.md")
	if err != nil || p.Content != "hello" {
		t.Fatalf("Fresh Idea.md = %q, %v", p.Content, err)
	}
}

func TestExternalLinkUsesOpener(t *testing.T) {
	var opened []string
	h := openStart(t, linkedNotes(t), app.WithOpener(func(s string) error { opened = append(opened, s); return nil }))
	h.clickPreview("docs")
	if len(opened) != 1 || opened[0] != "https://example.com/docs" {
		t.Fatalf("opened = %v", opened)
	}
}

func TestGdFollowsLinkUnderCursor(t *testing.T) {
	h := openStart(t, linkedNotes(t))
	h.key(tea.KeyEsc)
	h.keys("gg0" + strings.Repeat("l", 16)) // onto "[[Docker]]"
	h.keys("gd")
	if v := h.view(); !strings.Contains(v, "containers") {
		t.Fatalf("gd should open Docker.md:\n%s", v)
	}
}

func TestTypingDoubleBracketPicksNote(t *testing.T) {
	store := linkedNotes(t)
	h := openStart(t, store)
	h.keys("Go")
	h.keys("See [[")
	if v := h.view(); !strings.Contains(v, "Link to Note") {
		t.Fatalf("expected the note picker:\n%s", v)
	}
	h.keys("aws")
	h.key(tea.KeyEnter)
	h.keys(" and [[")
	h.keys("Brand New")
	h.send(tea.KeyMsg{Type: tea.KeyBackspace}) // "Brand Ne": no match yet
	h.keys("w")
	h.key(tea.KeyEnter)
	h.key(tea.KeyEsc)
	h.send(tea.KeyMsg{Type: tea.KeyCtrlS})
	p, _ := store.Get("Start.md")
	if !strings.HasSuffix(p.Content, "See [[AWS Notes]] and [[Brand New]]") {
		t.Fatalf("unexpected text: %q", p.Content)
	}
}

// clickPreview clicks the first occurrence of text in the preview pane (the
// part of each row right of the last divider).
func (h *harness) clickPreview(text string) {
	h.t.Helper()
	for y, l := range strings.Split(h.view(), "\n") {
		cut := strings.LastIndex(l, "│")
		if cut < 0 {
			continue
		}
		if i := strings.Index(l[cut:], text); i >= 0 {
			h.click(ansi.StringWidth(l[:cut+i]), y)
			return
		}
	}
	h.t.Fatalf("%q not in preview:\n%s", text, h.view())
}

func TestBacklinkOpensLinkingNoteAtTheLink(t *testing.T) {
	h := openStart(t, linkedNotes(t))
	h.clickPreview("Docker")
	h.clickPreview("󰈙 Start")
	if v := h.view(); !strings.Contains(v, "Learn about [[Docker]]") || !strings.Contains(v, "Ln 1,") {
		t.Fatalf("expected Start.md at line 1:\n%s", v)
	}
}
