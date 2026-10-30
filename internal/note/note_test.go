package note

import (
	"strings"
	"testing"
)

const board = "intro\n```board\n## Todo\n- Write\n  draft it\n- Test\n## Done\n- Ship\n```\nafter"

func lines(s string) []string { return strings.Split(s, "\n") }

func TestBoardColumns(t *testing.T) {
	cols := BoardColumns(lines(board), 1, 8)
	if len(cols) != 2 {
		t.Fatalf("got %d columns", len(cols))
	}
	if cols[0].Name != "Todo" || cols[0].Line != 2 || len(cols[0].Cards) != 2 || cols[0].Cards[1] != 5 || cols[0].EndLine != 6 {
		t.Fatalf("todo = %+v", cols[0])
	}
	if cols[1].Name != "Done" || cols[1].EndLine != 8 {
		t.Fatalf("done = %+v", cols[1])
	}
}

func TestMoveCard(t *testing.T) {
	cases := []struct {
		name             string
		card, col, index int
		want             string
	}{
		{"to the end of another column", 3, 1, 99, "intro\n```board\n## Todo\n- Test\n## Done\n- Ship\n- Write\n  draft it\n```\nafter"},
		{"to the top of another column", 3, 1, 0, "intro\n```board\n## Todo\n- Test\n## Done\n- Write\n  draft it\n- Ship\n```\nafter"},
		{"within its column", 5, 0, 0, "intro\n```board\n## Todo\n- Test\n- Write\n  draft it\n## Done\n- Ship\n```\nafter"},
		{"back from done", 7, 0, 1, "intro\n```board\n## Todo\n- Write\n  draft it\n- Ship\n- Test\n## Done\n```\nafter"},
	}
	for _, c := range cases {
		got, ok := MoveCard(lines(board), 1, 8, c.card, c.col, c.index)
		if !ok || strings.Join(got, "\n") != c.want {
			t.Errorf("%s: ok=%v\n%s\nwant\n%s", c.name, ok, strings.Join(got, "\n"), c.want)
		}
	}
}

func TestMoveCardRejectsBadTargets(t *testing.T) {
	for _, c := range []struct{ card, col int }{{2, 1}, {3, 5}, {3, -1}, {0, 0}} {
		if _, ok := MoveCard(lines(board), 1, 8, c.card, c.col, 0); ok {
			t.Errorf("card %d to column %d was accepted", c.card, c.col)
		}
	}
}

func TestMoveBlockKeepsBlocksApart(t *testing.T) {
	doc := lines("# A\n\none\n\ntwo")
	got := strings.Join(MoveBlock(doc, 4, 5, 2), "\n")
	if want := "# A\n\ntwo\n\none"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
