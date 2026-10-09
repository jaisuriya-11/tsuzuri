package preview_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/jaisuriya-11/tsuzuri/internal/core"
	"github.com/jaisuriya-11/tsuzuri/internal/preview"
	"github.com/jaisuriya-11/tsuzuri/internal/theme"
)

func testNotes(notes map[string]string) preview.NoteLookup {
	return func(target string) (preview.Note, bool) {
		text, ok := notes[target]
		return preview.Note{ID: target + ".md", Title: target, Content: text}, ok
	}
}

func linkHits(hits []preview.Hit) []preview.Hit {
	var out []preview.Hit
	for _, h := range hits {
		if strings.HasPrefix(h.Kind, "link:") {
			out = append(out, h)
		}
	}
	return out
}

func TestWikiLinksRenderAsClickableLabels(t *testing.T) {
	md := "Intro [[Docker]], [[AWS Notes#S3 Storage|Learn about S3]], [[Ghost]] and [site](https://x.dev) <https://y.dev> end ^para"
	cal := preview.CalendarView{Notes: testNotes(map[string]string{"Docker": "", "AWS Notes": ""})}
	out, hits := preview.CompileHits(md, theme.DefaultTheme(), 200, "", cal)
	plain := ansi.Strip(out)
	if strings.Contains(out, "\x1b]8;") {
		t.Fatal("link markers must be stripped from the output")
	}
	for _, s := range []string{"Docker", "Learn about S3", "Ghost", "site", "https://y.dev"} {
		if !strings.Contains(plain, s) {
			t.Errorf("missing %q in %q", s, plain)
		}
	}
	if strings.Contains(plain, "[[") || strings.Contains(plain, "^para") {
		t.Errorf("raw link syntax or block id shown: %q", plain)
	}

	want := []struct{ kind, arg, label string }{
		{preview.HitWikiLink, "Docker", "Docker"},
		{preview.HitWikiLink, "AWS Notes#S3 Storage", "Learn about S3"},
		{preview.HitWikiLink, "Ghost", "Ghost"},
		{preview.HitURLLink, "https://x.dev", "site"},
		{preview.HitURLLink, "https://y.dev", "https://y.dev"},
	}
	got := linkHits(hits)
	if len(got) != len(want) {
		t.Fatalf("got %d link hits, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		h := got[i]
		label := string([]rune(plain)[h.X0:h.X1])
		if h.Kind != w.kind || h.Arg != w.arg || label != w.label {
			t.Errorf("hit %d = %s %q over %q; want %s %q over %q", i, h.Kind, h.Arg, label, w.kind, w.arg, w.label)
		}
	}
}

func TestWrappedLinkIsClickableOnEveryRow(t *testing.T) {
	md := "aaaa bbbb cccc [[A very long note name that wraps]] tail"
	out, hits := preview.CompileHits(md, theme.DefaultTheme(), 30, "", preview.CalendarView{})
	rows := map[int]bool{}
	for _, h := range linkHits(hits) {
		rows[h.Row] = true
	}
	if n := strings.Count(out, "\n") + 1; n < 2 || len(rows) < 2 {
		t.Fatalf("expected a wrapped link with a hit per row, rows=%v:\n%s", rows, ansi.Strip(out))
	}
}

func TestEmbedsShowLinkedContent(t *testing.T) {
	notes := testNotes(map[string]string{
		"AWS Notes": "# AWS Notes\n\nS3 stores objects in buckets. ^s3-basics\n\n## EC2\n\nVirtual machines.",
		"Loop":      "![[Loop]]",
	})
	md := "before\n\n![[AWS Notes#^s3-basics]]\n\n![[AWS Notes#EC2]]\n\n![[Missing]]\n\n![[Loop]]"
	out, hits := preview.CompileHits(md, theme.DefaultTheme(), 80, "", preview.CalendarView{Notes: notes})
	plain := ansi.Strip(out)
	for _, s := range []string{"S3 stores objects in buckets.", "Virtual machines.", "no such note", "…"} {
		if !strings.Contains(plain, s) {
			t.Errorf("missing %q in:\n%s", s, plain)
		}
	}
	if strings.Contains(plain, "buckets. ^") || strings.Contains(plain, "▏ # AWS Notes") || strings.Contains(plain, "▾") {
		t.Errorf("embed shows more than its block:\n%s", plain)
	}
	if len(linkHits(hits)) < 3 {
		t.Errorf("embed headers should be clickable: %+v", linkHits(hits))
	}
}

func TestBacklinksSectionIsClickable(t *testing.T) {
	m := preview.New(theme.DefaultTheme())
	m.SetSize(80, 30)
	m.SetPage(core.Page{ID: "Docker.md", Title: "Docker", Content: "body"})
	m.SetBacklinks([]core.Backlink{{ID: "Study.md", Title: "Study", Line: 3, Text: "Learn [[Docker]]"}})
	v := ansi.Strip(m.View())
	if !strings.Contains(v, "Linked from 1 note") || !strings.Contains(v, "Study") {
		t.Fatalf("backlinks missing:\n%s", v)
	}
	for y, l := range strings.Split(v, "\n") {
		if i := strings.Index(l, "Study"); i >= 0 {
			h, ok := m.HitAt(ansi.StringWidth(l[:i]), y)
			if !ok || h.Kind != preview.HitNoteLink || h.Arg != "Study.md" || h.Index != 3 {
				t.Fatalf("backlink hit = %+v, %v", h, ok)
			}
			return
		}
	}
}
