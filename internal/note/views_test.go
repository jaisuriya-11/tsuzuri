package note

import (
	"strings"
	"testing"
)

func apply(t *testing.T, doc string, e ViewEdit) string {
	t.Helper()
	ls := lines(doc)
	end := len(ls) - 1 // every test view ends with its closing fence
	out, err := ApplyView(ls, 0, end, e)
	if err != nil {
		t.Fatalf("%s: %v", e.Op, err)
	}
	return strings.Join(out, "\n")
}

func TestBoardEdits(t *testing.T) {
	doc := "```board\n## Todo\n- a\n  note\n## Done\n- b\n```"
	cases := []struct {
		e    ViewEdit
		want string
	}{
		{ViewEdit{Op: "board.add", Index: 0, Text: "c"}, "```board\n## Todo\n- a\n  note\n- c\n## Done\n- b\n```"},
		{ViewEdit{Op: "board.add", Index: 1, Text: "c"}, "```board\n## Todo\n- a\n  note\n## Done\n- b\n- c\n```"},
		{ViewEdit{Op: "board.rename", Line: 2, Text: "A"}, "```board\n## Todo\n- A\n  note\n## Done\n- b\n```"},
		{ViewEdit{Op: "board.delete", Line: 2}, "```board\n## Todo\n## Done\n- b\n```"},
		{ViewEdit{Op: "board.describe", Line: 2, Lines: []string{"first", "", "second\n# not a heading"}}, "```board\n## Todo\n- a\n  first\n  second\n## Done\n- b\n```"},
		{ViewEdit{Op: "board.describe", Line: 5, Lines: []string{"why"}}, "```board\n## Todo\n- a\n  note\n## Done\n- b\n  why\n```"},
		{ViewEdit{Op: "board.describe", Line: 2}, "```board\n## Todo\n- a\n## Done\n- b\n```"},
		{ViewEdit{Op: "board.column", Text: "Later"}, "```board\n## Todo\n- a\n  note\n## Done\n- b\n## Later\n```"},
		{ViewEdit{Op: "board.renamecolumn", Line: 4, Text: "Shipped"}, "```board\n## Todo\n- a\n  note\n## Shipped\n- b\n```"},
		{ViewEdit{Op: "board.deletecolumn", Line: 1}, "```board\n## Done\n- b\n```"},
	}
	for _, c := range cases {
		if got := apply(t, doc, c.e); got != c.want {
			t.Errorf("%s:\n%s\nwant\n%s", c.e.Op, got, c.want)
		}
	}
}

func TestCalendarTimelineChartEdits(t *testing.T) {
	cal := "```calendar\nmonth: 2026-10\n2026-10-03: Release\n```"
	if got := apply(t, cal, ViewEdit{Op: "calendar.add", Date: "2026-10-09", Text: "Call"}); !strings.Contains(got, "2026-10-03: Release\n2026-10-09: Call\n```") {
		t.Errorf("calendar.add: %s", got)
	}
	if got := apply(t, cal, ViewEdit{Op: "calendar.set", Line: 2, Date: "2026-10-04", Text: "Launch"}); !strings.Contains(got, "\n2026-10-04: Launch\n") {
		t.Errorf("calendar.set: %s", got)
	}

	tl := "```timeline\nBuild: 2026-10-05 -> 2026-10-18\n```"
	if got := apply(t, tl, ViewEdit{Op: "timeline.set", Line: 1, Text: "Make", Date: "2026-10-06", End: "2026-10-01"}); got != "```timeline\nMake: 2026-10-06 -> 2026-10-06\n```" {
		t.Errorf("timeline.set (end before start): %s", got)
	}

	ch := "```chart\ntitle: T\nA: 1\n```"
	if got := apply(t, ch, ViewEdit{Op: "chart.type", Value: "donut"}); got != "```chart\ntype: donut\ntitle: T\nA: 1\n```" {
		t.Errorf("chart.type: %s", got)
	}
	if got := apply(t, ch, ViewEdit{Op: "chart.set", Line: 2, Text: "B", Value: "2.5"}); got != "```chart\ntitle: T\nB: 2.5\n```" {
		t.Errorf("chart.set: %s", got)
	}
	if got := apply(t, ch, ViewEdit{Op: "title", Text: "Sales"}); got != "```chart\ntitle: Sales\nA: 1\n```" {
		t.Errorf("title: %s", got)
	}
	if got := apply(t, ch, ViewEdit{Op: "title"}); got != "```chart\nA: 1\n```" {
		t.Errorf("title removed: %s", got)
	}
	if got := apply(t, tl, ViewEdit{Op: "title", Text: "Plan"}); got != "```timeline\ntitle: Plan\nBuild: 2026-10-05 -> 2026-10-18\n```" {
		t.Errorf("timeline title: %s", got)
	}
	if got := apply(t, ch, ViewEdit{Op: "delete", Line: 2}); got != "```chart\ntitle: T\n```" {
		t.Errorf("delete: %s", got)
	}
}

func TestFormEdits(t *testing.T) {
	doc := "```form\ntitle: F\n?* Name\n  = Jai\n? Pick (choice): A | B\n? Many (multi): X | Y | Z\n  = Y\n? Stars (rating)\n```"
	cases := []struct {
		e    ViewEdit
		want string // a line the result must contain
	}{
		{ViewEdit{Op: "form.title", Text: "Survey"}, "title: Survey\n"},
		{ViewEdit{Op: "form.description", Text: "Hi"}, "```form\ndescription: Hi\ntitle: F"},
		{ViewEdit{Op: "form.answer", Line: 2, Text: "Sam"}, "?* Name\n  = Sam\n"},
		{ViewEdit{Op: "form.pick", Line: 4, Index: 1}, "? Pick (choice): A | B\n  = B\n"},
		{ViewEdit{Op: "form.toggle", Line: 5, Index: 0}, "? Many (multi): X | Y | Z\n  = X, Y\n"},
		{ViewEdit{Op: "form.toggle", Line: 5, Index: 1}, "? Many (multi): X | Y | Z\n? Stars"},
		{ViewEdit{Op: "form.rate", Line: 7, Index: 4}, "? Stars (rating)\n  = 4\n"},
		{ViewEdit{Op: "form.add", Text: "Why", Value: "choice"}, "? Why (choice): Option 1 | Option 2 | Option 3\n```"},
		{ViewEdit{Op: "form.rename", Line: 4, Text: "Choose"}, "? Choose (choice): A | B\n"},
		{ViewEdit{Op: "form.type", Line: 5, Value: "long"}, "? Many (long)\n? Stars"},
		{ViewEdit{Op: "form.required", Line: 2}, "? Name\n  = Jai"},
		{ViewEdit{Op: "form.option", Line: 4, Text: "C"}, "? Pick (choice): A | B | C\n"},
		{ViewEdit{Op: "delete", Line: 2}, "title: F\n? Pick"},
	}
	for _, c := range cases {
		if got := apply(t, doc, c.e); !strings.Contains(got, c.want) {
			t.Errorf("%s: missing %q in\n%s", c.e.Op, c.want, got)
		}
	}
}

func TestViewEditsStayOnOneLine(t *testing.T) {
	got := apply(t, "```board\n## A\n```", ViewEdit{Op: "board.add", Index: 0, Text: "two\nlines\r\n"})
	if got != "```board\n## A\n- two lines\n```" {
		t.Fatalf("got %q", got)
	}
}

func TestViewEditsRejectBadInput(t *testing.T) {
	doc := lines("intro\n```chart\nA: 1\n```\nafter")
	bad := []ViewEdit{
		{Op: "chart.set", Line: 0, Text: "A", Value: "1"}, // outside the view
		{Op: "chart.set", Line: 4, Text: "A", Value: "1"},
		{Op: "chart.add", Text: "B", Value: "lots"},
		{Op: "chart.add", Text: " ", Value: "1"},
		{Op: "chart.type", Value: "radar"},
		{Op: "calendar.add", Text: "x", Date: "tomorrow"},
		{Op: "explode"},
		{Op: "form.add", Text: "Q", Value: "x) evil"},
	}
	for _, e := range bad {
		if _, err := ApplyView(doc, 1, 3, e); err == nil {
			t.Errorf("%+v was accepted", e)
		}
	}
	if _, err := ApplyView(doc, 1, 9, ViewEdit{Op: "delete", Line: 2}); err == nil {
		t.Error("a view running past the note was accepted")
	}
}
