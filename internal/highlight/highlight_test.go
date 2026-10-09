package highlight_test

import (
	"strings"
	"testing"

	"github.com/jaisuriya-11/tsuzuri/internal/highlight"
	"github.com/jaisuriya-11/tsuzuri/internal/theme"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestCodeColoursKnownLanguages(t *testing.T) {
	th := theme.DefaultTheme()
	cases := map[string]string{
		"c":          "#include <stdio.h>\nint main(void) { return 0; }",
		"go":         "package main\nfunc main() { fmt.Println(\"hi\") }",
		"python":     "def f(x):\n    return 'a' + str(x)  # c",
		"javascript": "const x = () => { return 42 }",
		"rust":       "fn main() { let s = String::new(); }",
		"bash":       "for i in 1 2; do echo \"$i\"; done",
		"sql":        "SELECT * FROM t WHERE id = 1;",
		"yaml":       "key: value\nlist:\n  - 1",
	}
	for lang, code := range cases {
		cols := highlight.Code(lang, code, th)
		colored := 0
		for _, line := range cols {
			for _, c := range line {
				if c != "" {
					colored++
				}
			}
		}
		if colored == 0 {
			t.Errorf("%s: expected some highlighted runes", lang)
		}
	}
}

func TestFrontMatterHighlight(t *testing.T) {
	th := theme.DefaultTheme()
	doc := "---\ntitle: Hi\ntags: [a]\n---\n# Hi\nbody"
	cols := highlight.Markdown(doc, th)
	if len(cols) != 6 {
		t.Fatalf("expected 6 lines, got %d", len(cols))
	}
	if cols[0][0] != th.GreyFg {
		t.Errorf("front-matter delimiter should be grey")
	}
	if cols[1][0] != th.Red {
		t.Errorf("front-matter key should be red, got %q", cols[1][0])
	}
}

func TestInlineElementsHighlight(t *testing.T) {
	th := theme.DefaultTheme()
	doc := "see [link](http://x) and `code` and **emph** and #tag"
	cols := highlight.Markdown(doc, th)
	if len(cols) != 1 {
		t.Fatalf("expected 1 line")
	}
	nonEmpty := 0
	for _, c := range cols[0] {
		if c != "" {
			nonEmpty++
		}
	}
	if nonEmpty == 0 {
		t.Error("expected inline colours")
	}
}

func TestRenderPaintsRunes(t *testing.T) {
	r := lipgloss.NewRenderer(nil)
	r.SetColorProfile(termenv.TrueColor)
	lipgloss.SetDefaultRenderer(r)
	th := theme.DefaultTheme()
	runes := []rune("ab")
	colors := []lipgloss.Color{th.Red, th.Blue}
	out := highlight.Render(runes, colors, lipgloss.NewStyle())
	if !strings.Contains(out, "\x1b[") {
		t.Errorf("expected styled output, got %q", out)
	}
}

func TestCodeUnknownLanguageFallsBack(t *testing.T) {
	th := theme.DefaultTheme()
	cols := highlight.Code("not-a-language", "x = 1", th)
	if len(cols) != 1 {
		t.Fatalf("expected 1 line")
	}
}

func TestCodeEmptyInput(t *testing.T) {
	th := theme.DefaultTheme()
	cols := highlight.Code("go", "", th)
	if len(cols) != 0 && len(cols[0]) != 0 {
		t.Errorf("expected empty highlight, got %v", cols)
	}
}

func TestMarkdownQuoteAndRule(t *testing.T) {
	th := theme.DefaultTheme()
	doc := "> quoted\n\n---\n\n- [x] done"
	cols := highlight.Markdown(doc, th)
	if cols[0][0] != th.Purple {
		t.Errorf("quote marker should be purple")
	}
	if cols[2][0] != th.GreyFg {
		t.Errorf("rule should be grey")
	}
	if cols[4][3] != th.Green {
		t.Errorf("checked box should be green, got %q", cols[4][3])
	}
}

func TestMarkdownHighlightsFencedCode(t *testing.T) {
	th := theme.DefaultTheme()
	doc := "# Title\n\n```c\nint x = 1;\n```\n- item `code`"
	cols := highlight.Markdown(doc, th)
	if len(cols) != 6 {
		t.Fatalf("expected 6 lines, got %d", len(cols))
	}
	if cols[0][0] != th.Blue {
		t.Errorf("heading should be blue")
	}
	if cols[3][0] != th.Yellow { // "int" is a type keyword
		t.Errorf("expected C type keyword coloured, got %q", cols[3][0])
	}
	if cols[5][0] != th.Red {
		t.Errorf("list marker should be coloured")
	}
}

func TestWikiLinkAndBlockIDHighlight(t *testing.T) {
	th := theme.DefaultTheme()
	line := "go [[AWS#S3|s3]] now ^ref"
	cols := highlight.Markdown(line, th)[0]
	at := func(s string) int { return len([]rune(line[:strings.Index(line, s)])) }
	if cols[at("[[")] != th.GreyFg || cols[at("AWS")] != th.Blue || cols[at("]]")] != th.GreyFg {
		t.Errorf("wiki link colours = %v", cols)
	}
	if cols[at("^ref")] != th.GreyFg || cols[at("now")] != "" {
		t.Errorf("block id colours = %v", cols)
	}
}
