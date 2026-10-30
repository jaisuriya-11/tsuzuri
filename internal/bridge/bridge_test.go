package bridge

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jaisuriya-11/tsuzuri/internal/note"
)

func call(t *testing.T, cmd string, req any, resp any) {
	t.Helper()
	var in bytes.Buffer
	if req != nil {
		if err := json.NewEncoder(&in).Encode(req); err != nil {
			t.Fatal(err)
		}
	}
	var out, errOut bytes.Buffer
	if code := Run([]string{cmd}, Options{}, &in, &out, &errOut); code != 0 {
		t.Fatalf("%s exited %d: %s", cmd, code, errOut.String())
	}
	if err := json.Unmarshal(out.Bytes(), resp); err != nil {
		t.Fatalf("%s: bad response %q: %v", cmd, out.String(), err)
	}
}

func TestVersion(t *testing.T) {
	var resp versionResponse
	call(t, "version", nil, &resp)
	if resp.Protocol != Protocol {
		t.Fatalf("protocol = %d", resp.Protocol)
	}
}

func TestBlocksMatchTheSlashMenu(t *testing.T) {
	var resp blocksResponse
	call(t, "blocks", nil, &resp)
	labels := map[string]bool{}
	for _, b := range resp.Blocks {
		labels[b.Label] = true
	}
	for _, want := range []string{"Heading 1", "Board", "Flowchart", "Image", "Add row below"} {
		if !labels[want] {
			t.Errorf("missing %q", want)
		}
	}
}

func TestRenderDrawsViewsAndSkipsCode(t *testing.T) {
	req := renderRequest{Fences: []Fence{
		{Lang: "board", Body: "## Todo\n- Write\n## Done\n- Ship\n"},
		{Lang: "flow", Body: "a[Start] --> b[End]\n"},
		{Lang: "math", Body: "x^2\n"},
		{Lang: "go", Body: "package main\n"},
	}}
	var resp renderResponse
	call(t, "render", req, &resp)
	if len(resp.Fences) != 4 {
		t.Fatalf("got %d fences", len(resp.Fences))
	}
	if v := resp.Fences[0].View; !resp.Fences[0].OK || v == nil || v.Kind != "board" || len(v.Board) != 2 ||
		v.Board[0].Name != "Todo" || v.Board[1].Tone != "done" || v.Board[0].Cards[0].Title != "Write" || v.Board[0].Cards[0].Line != 1 {
		t.Errorf("board = %+v", resp.Fences[0])
	}
	if f := resp.Fences[1]; !f.OK || !strings.Contains(f.HTML, "Start") || !strings.Contains(f.HTML, "<span style=\"color:#") {
		t.Errorf("flow = %+v", f)
	}
	if strings.Contains(resp.Fences[1].HTML, "\x1b") {
		t.Error("escape codes left in HTML")
	}
	if got := resp.Fences[2].Math; !resp.Fences[2].OK || len(got) != 1 || got[0] != "x^2" {
		t.Errorf("math = %+v", resp.Fences[2])
	}
	if resp.Fences[3].OK {
		t.Error("ordinary code was drawn as a view")
	}
}

func TestRenderRejectsUnknownTheme(t *testing.T) {
	var out, errOut bytes.Buffer
	in := strings.NewReader(`{"theme":"no-such-theme","fences":[]}`)
	if code := Run([]string{"render"}, Options{}, in, &out, &errOut); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(errOut.String(), "no-such-theme") {
		t.Fatalf("stderr = %q", errOut.String())
	}
}

func TestTable(t *testing.T) {
	lines := []string{"intro", "| a | b |", "| --- | --- |", "| 1 | 2 |"}
	var resp tableResponse
	// Cursor after the closing pipe of the last row.
	call(t, "table", tableRequest{Lines: lines, Row: 3, Col: len(lines[3]), Op: "addcol"}, &resp)
	if !resp.InTable || resp.Edit == nil {
		t.Fatalf("resp = %+v", resp)
	}
	if resp.Edit.Start != 1 || resp.Edit.End != 4 || !strings.Contains(resp.Edit.Lines[0], "Column") {
		t.Fatalf("edit = %+v", resp.Edit)
	}

	resp = tableResponse{}
	call(t, "table", tableRequest{Lines: lines, Row: 1, Col: 2, Op: "delrow"}, &resp)
	if !resp.InTable || resp.Edit != nil {
		t.Fatalf("deleting the header should be refused: %+v", resp)
	}

	resp = tableResponse{}
	call(t, "table", tableRequest{Lines: lines, Row: 0, Col: 0, Op: "format"}, &resp)
	if resp.InTable {
		t.Fatal("line outside a table reported as inside")
	}
}

func TestMoveAndCard(t *testing.T) {
	doc := []string{"one", "", "two"}
	var resp linesResponse
	call(t, "move", moveRequest{Lines: doc, From: 2, End: 3, To: 0}, &resp)
	if strings.Join(resp.Lines, "|") != "two||one" {
		t.Fatalf("move = %q", resp.Lines)
	}

	board := []string{"```board", "## Todo", "- a", "## Done", "```"}
	call(t, "card", cardRequest{Lines: board, Block: 0, End: 4, Card: 2, Column: 1, Index: 0}, &resp)
	if strings.Join(resp.Lines, "|") != "```board|## Todo|## Done|- a|```" {
		t.Fatalf("card = %q", resp.Lines)
	}

	var out, errOut bytes.Buffer
	in := strings.NewReader(`{"lines":["x"],"from":0,"end":5,"to":0}`)
	if code := Run([]string{"move"}, Options{}, in, &out, &errOut); code != 1 {
		t.Fatalf("out-of-range move exited %d", code)
	}
}

func TestView(t *testing.T) {
	doc := []string{"```form", "? Pick (choice): A | B", "```"}
	var resp linesResponse
	call(t, "view", viewRequest{Lines: doc, Block: 0, End: 2, Edit: note.ViewEdit{Op: "form.pick", Line: 1, Index: 1}}, &resp)
	if strings.Join(resp.Lines, "|") != "```form|? Pick (choice): A | B|  = B|```" {
		t.Fatalf("view = %q", resp.Lines)
	}
	var out, errOut bytes.Buffer
	in := strings.NewReader(`{"lines":["x"],"block":0,"end":1,"edit":{"op":"nope"}}`)
	if code := Run([]string{"view"}, Options{}, in, &out, &errOut); code != 1 {
		t.Fatalf("unknown edit exited %d", code)
	}
}

func TestToggle(t *testing.T) {
	var resp toggleResponse
	call(t, "toggle", toggleRequest{Line: "| [ ] a | [ ] b |", Nth: 1}, &resp)
	if !resp.OK || resp.Line != "| [ ] a | [x] b |" {
		t.Fatalf("toggle = %+v", resp)
	}
}

func TestTableRejectsUnknownOp(t *testing.T) {
	var out, errOut bytes.Buffer
	in := strings.NewReader(`{"lines":["| a |"],"op":"explode"}`)
	if code := Run([]string{"table"}, Options{}, in, &out, &errOut); code != 1 {
		t.Fatalf("exit %d", code)
	}
}

func TestPageAttachAndResolve(t *testing.T) {
	root := t.TempDir()
	note := filepath.Join(root, "Projects", "Plan.md")
	if err := os.MkdirAll(filepath.Dir(note), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(note, []byte("# Plan\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var pg pageResponse
	call(t, "page", pageRequest{Root: root, Note: note, Title: "Ideas"}, &pg)
	if want := filepath.Join(root, "Projects", "Plan", "Ideas.md"); pg.Path != want || pg.Title != "Ideas" {
		t.Fatalf("page = %+v, want %s", pg, want)
	}

	var res resolveResponse
	call(t, "resolve", resolveRequest{Root: root, Note: note, Target: "ideas"}, &res)
	if res.Path != pg.Path {
		t.Fatalf("resolve = %q, want %q", res.Path, pg.Path)
	}
	res = resolveResponse{}
	call(t, "resolve", resolveRequest{Root: root, Note: note, Target: "Missing"}, &res)
	if res.Path != "" {
		t.Fatalf("missing note resolved to %q", res.Path)
	}

	outside := filepath.Join(t.TempDir(), "photo.png")
	if err := os.WriteFile(outside, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	var at attachResponse
	call(t, "attach", attachRequest{Root: root, Note: note, Src: outside, Kind: "image"}, &at)
	if at.Link != "assets/photo.png" || at.Markdown != "![photo](assets/photo.png)" {
		t.Fatalf("attach = %+v", at)
	}
	at = attachResponse{}
	call(t, "attach", attachRequest{Root: root, Note: note, Src: outside, Kind: "cover", Text: "# Plan\n"}, &at)
	if at.Text != "---\ncover: assets/photo-2.png\n---\n# Plan\n" || at.Markdown != "" {
		t.Fatalf("cover = %+v", at)
	}
	if _, err := os.Stat(filepath.Join(root, "Projects", "assets", "photo.png")); err != nil {
		t.Fatalf("file not copied: %v", err)
	}
}

func TestWorkspaceRejectsNotesOutsideRoot(t *testing.T) {
	_, _, err := workspace(t.TempDir(), filepath.Join(t.TempDir(), "x.md"))
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestUsage(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run(nil, Options{}, strings.NewReader(""), &out, &errOut); code != 2 {
		t.Fatalf("exit %d", code)
	}
	if code := Run([]string{"nope"}, Options{}, strings.NewReader(""), &out, &errOut); code != 2 {
		t.Fatalf("exit %d", code)
	}
}
