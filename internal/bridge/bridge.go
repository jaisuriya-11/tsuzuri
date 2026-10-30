// Package bridge lets editor integrations (the VS Code extension) reuse
// Tsuzuri's own logic instead of copying it. Each request is one command,
// "tsuzuri editor <command>", reading a JSON request on stdin and writing a
// JSON response on stdout, so views, the "/" menu, table edits and links
// behave exactly as they do in the app.
package bridge

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/jaisuriya-11/tsuzuri/internal/content"
	"github.com/jaisuriya-11/tsuzuri/internal/core"
	"github.com/jaisuriya-11/tsuzuri/internal/export"
	"github.com/jaisuriya-11/tsuzuri/internal/note"
	"github.com/jaisuriya-11/tsuzuri/internal/preview"
	"github.com/jaisuriya-11/tsuzuri/internal/theme"
)

// Protocol is bumped when a request or response changes shape, so an
// extension can tell an older binary to update.
const Protocol = 4

// Width limits for drawn views, in terminal cells.
const (
	defaultWidth = 96
	minWidth     = 24
	maxWidth     = 400
)

// maxRequest caps how much JSON a request may send.
const maxRequest = 32 << 20

// Options carries the user's settings into requests.
type Options struct {
	// Theme draws views when a request names none (the theme from
	// Tsuzuri's config, or "" for the default).
	Theme string
}

func (o Options) handlers() map[string]func(json.RawMessage) (any, error) {
	return map[string]func(json.RawMessage) (any, error){
		"version": func(json.RawMessage) (any, error) { return versionResponse{Protocol: Protocol}, nil },
		"blocks":  func(json.RawMessage) (any, error) { return blocksResponse{Blocks: content.Blocks()}, nil },
		"render": decode(func(req renderRequest) (renderResponse, error) {
			if req.Theme == "" {
				req.Theme = o.Theme
			}
			return render(req)
		}),
		"table":   decode(table),
		"move":    decode(move),
		"card":    decode(card),
		"view":    decode(view),
		"toggle":  decode(toggle),
		"attach":  decode(attach),
		"page":    decode(page),
		"resolve": decode(resolve),
	}
}

// Run handles "tsuzuri editor <command>" and returns the exit code. Errors
// are written to stderr as plain text.
func Run(args []string, opts Options, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "Usage: tsuzuri editor <command>  (reads JSON on stdin)")
		return 2
	}
	handle, ok := opts.handlers()[args[0]]
	if !ok {
		fmt.Fprintf(stderr, "Unknown editor command %q\n", args[0])
		return 2
	}

	var raw json.RawMessage
	if args[0] != "version" && args[0] != "blocks" {
		data, err := io.ReadAll(io.LimitReader(stdin, maxRequest))
		if err != nil {
			fmt.Fprintf(stderr, "Could not read request: %v\n", err)
			return 1
		}
		raw = data
	}

	resp, err := handle(raw)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := json.NewEncoder(stdout).Encode(resp); err != nil {
		fmt.Fprintf(stderr, "Could not write response: %v\n", err)
		return 1
	}
	return 0
}

// decode wraps a typed handler so it receives its parsed request.
func decode[Req any, Resp any](fn func(Req) (Resp, error)) func(json.RawMessage) (any, error) {
	return func(raw json.RawMessage) (any, error) {
		var req Req
		if err := json.Unmarshal(raw, &req); err != nil {
			return nil, fmt.Errorf("invalid request: %w", err)
		}
		return fn(req)
	}
}

type versionResponse struct {
	Protocol int `json:"protocol"`
}

type blocksResponse struct {
	Blocks []content.Block `json:"blocks"`
}

/* ---------------------------------------------------------------- render */

// Fence is one fenced block of a note.
type Fence struct {
	Lang string `json:"lang"`
	Body string `json:"body"`
}

type renderRequest struct {
	Theme  string  `json:"theme"`
	Width  int     `json:"width"`
	Fences []Fence `json:"fences"`
}

// Drawn is a fence as the app sees it. OK is false for ordinary code,
// which the editor shows as it would any code block. Views (board,
// calendar, timeline, chart, form) come as data for the editor to draw;
// flowcharts as the app draws them, in HTML; math blocks as their
// equations in LaTeX.
type Drawn struct {
	OK   bool          `json:"ok"`
	View *preview.View `json:"view,omitempty"`
	HTML string        `json:"html,omitempty"`
	Math []string      `json:"math,omitempty"`
}

type renderResponse struct {
	Fences []Drawn `json:"fences"`
}

// render draws views (board, calendar, timeline, chart, form, flow) and
// equations with the same code as the app's preview and PDF export.
func render(req renderRequest) (renderResponse, error) {
	th, err := pickTheme(req.Theme)
	if err != nil {
		return renderResponse{}, err
	}
	width := req.Width
	if width <= 0 {
		width = defaultWidth
	}
	width = min(max(width, minWidth), maxWidth)

	// Views are drawn with colour even though stdout is a pipe.
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prev)

	out := make([]Drawn, len(req.Fences))
	for i, f := range req.Fences {
		out[i] = draw(f, th, width)
	}
	return renderResponse{Fences: out}, nil
}

// draw draws one fence. A block the drawing code can't cope with shows as
// code instead of failing the whole note.
func draw(f Fence, th theme.Theme, width int) (d Drawn) {
	defer func() {
		if recover() != nil {
			d = Drawn{}
		}
	}()
	body := strings.Split(strings.TrimSuffix(f.Body, "\n"), "\n")
	if v, ok := preview.ViewData(f.Lang, body); ok {
		return Drawn{OK: true, View: &v}
	}
	if preview.IsMathLang(strings.ToLower(f.Lang)) {
		return Drawn{OK: true, Math: preview.MathEquations(body)}
	}
	lines, ok := preview.PrintBlockLines(f.Lang, body, th, width)
	if !ok {
		return Drawn{}
	}
	return Drawn{OK: true, HTML: export.HTMLArt(lines)}
}

func pickTheme(name string) (theme.Theme, error) {
	if name == "" {
		return theme.DefaultTheme(), nil
	}
	th, ok := theme.Get(name)
	if !ok {
		return theme.Theme{}, fmt.Errorf("unknown theme %q (run tsuzuri --list-themes)", name)
	}
	return th, nil
}

/* ----------------------------------------------------------------- table */

type tableRequest struct {
	Lines []string `json:"lines"`
	Row   int      `json:"row"`
	Col   int      `json:"col"`
	Op    string   `json:"op"`
}

type tableResponse struct {
	InTable bool               `json:"inTable"`
	Edit    *content.TableEdit `json:"edit,omitempty"`
}

var tableOps = map[string]bool{"addrow": true, "addcol": true, "delrow": true, "delcol": true, "format": true}

func table(req tableRequest) (tableResponse, error) {
	if !tableOps[req.Op] {
		return tableResponse{}, fmt.Errorf("unknown table operation %q", req.Op)
	}
	edit, ok := content.EditTable(req.Lines, req.Row, req.Col, req.Op)
	if !ok {
		return tableResponse{}, nil
	}
	if edit.Lines == nil {
		return tableResponse{InTable: true}, nil
	}
	return tableResponse{InTable: true, Edit: &edit}, nil
}

/* ------------------------------------------------------------------ drag */

type linesResponse struct {
	Lines []string `json:"lines"`
}

type moveRequest struct {
	Lines []string `json:"lines"`
	From  int      `json:"from"`
	End   int      `json:"end"`
	To    int      `json:"to"`
}

// move drags lines [From, End) of a note to start where line To was.
func move(req moveRequest) (linesResponse, error) {
	if req.From < 0 || req.End <= req.From || req.End > len(req.Lines) || req.To < 0 || req.To > len(req.Lines) {
		return linesResponse{}, errors.New("block or target is outside the note")
	}
	return linesResponse{Lines: note.MoveBlock(req.Lines, req.From, req.End, req.To)}, nil
}

type cardRequest struct {
	Lines []string `json:"lines"`
	// Block and End are the board's opening and closing fence lines.
	Block  int `json:"block"`
	End    int `json:"end"`
	Card   int `json:"card"`
	Column int `json:"column"`
	Index  int `json:"index"`
}

// card drags a board card to a column, before the card at Index there.
func card(req cardRequest) (linesResponse, error) {
	if req.Block < 0 || req.End <= req.Block || req.End > len(req.Lines) {
		return linesResponse{}, errors.New("board is outside the note")
	}
	lines, ok := note.MoveCard(req.Lines, req.Block, req.End, req.Card, req.Column, req.Index)
	if !ok {
		return linesResponse{}, errors.New("that card or column isn't on the board")
	}
	return linesResponse{Lines: lines}, nil
}

type viewRequest struct {
	Lines []string `json:"lines"`
	// Block and End are the view's opening and closing fence lines.
	Block int           `json:"block"`
	End   int           `json:"end"`
	Edit  note.ViewEdit `json:"edit"`
}

// view applies an edit made by clicking on a view, the same edit the app
// makes for the same click.
func view(req viewRequest) (linesResponse, error) {
	lines, err := note.ApplyView(req.Lines, req.Block, req.End, req.Edit)
	if err != nil {
		return linesResponse{}, err
	}
	return linesResponse{Lines: lines}, nil
}

type toggleRequest struct {
	Line string `json:"line"`
	Nth  int    `json:"nth"`
}

type toggleResponse struct {
	Line string `json:"line"`
	OK   bool   `json:"ok"`
}

// toggle ticks or unticks the nth checkbox on a line: a to-do, or one in a
// table cell.
func toggle(req toggleRequest) (toggleResponse, error) {
	line, ok := note.ToggleBox(req.Line, req.Nth)
	return toggleResponse{Line: line, OK: ok}, nil
}

/* ------------------------------------------------------------- workspace */

// workspace opens the notes folder and returns a note's ID within it.
func workspace(root, note string) (*core.Store, string, error) {
	if root == "" || note == "" {
		return nil, "", errors.New("root and note are required")
	}
	store, err := core.NewStore(root)
	if err != nil {
		return nil, "", err
	}
	abs, err := filepath.Abs(note)
	if err != nil {
		return nil, "", err
	}
	rel, err := filepath.Rel(store.Root(), abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, "", fmt.Errorf("%s is outside the notes folder %s", note, store.Root())
	}
	return store, filepath.ToSlash(rel), nil
}

// noteDir is the folder ID holding a note ID ("" for the root).
func noteDir(id string) string {
	dir := filepath.ToSlash(filepath.Dir(id))
	if dir == "." {
		return ""
	}
	return dir
}

type attachRequest struct {
	Root string `json:"root"`
	Note string `json:"note"`
	Src  string `json:"src"`
	// Kind is "image", "video", "audio", "file" or "cover".
	Kind string `json:"kind"`
	// Text is the note's current text, needed for a cover.
	Text string `json:"text,omitempty"`
}

type attachResponse struct {
	Link string `json:"link"`
	// Markdown shows the file in the note (not for a cover).
	Markdown string `json:"markdown,omitempty"`
	// Text is the note with the cover set in its front matter.
	Text string `json:"text,omitempty"`
}

// attach copies a file next to the note (into assets/) unless it already
// lives in the notes folder, and says what to write: the Markdown that
// shows it, or for a cover the note with its front matter updated.
func attach(req attachRequest) (attachResponse, error) {
	store, id, err := workspace(req.Root, req.Note)
	if err != nil {
		return attachResponse{}, err
	}
	link, err := store.AttachFile(noteDir(id), req.Src)
	if err != nil {
		return attachResponse{}, err
	}
	if req.Kind == "cover" {
		text, _ := content.WithFrontMatter(req.Text, "cover", link)
		return attachResponse{Link: link, Text: text}, nil
	}
	return attachResponse{Link: link, Markdown: content.MediaMarkdown(req.Kind, req.Src, link)}, nil
}

type pageRequest struct {
	Root  string `json:"root"`
	Note  string `json:"note"`
	Title string `json:"title"`
}

type pageResponse struct {
	Path  string `json:"path"`
	Title string `json:"title"`
}

// page creates a sub-page of the note, in the folder named after it.
func page(req pageRequest) (pageResponse, error) {
	store, id, err := workspace(req.Root, req.Note)
	if err != nil {
		return pageResponse{}, err
	}
	p, err := store.Create(req.Title, id)
	if err != nil {
		return pageResponse{}, err
	}
	return pageResponse{Path: filepath.Join(store.Root(), filepath.FromSlash(p.ID)), Title: p.Title}, nil
}

type resolveRequest struct {
	Root   string `json:"root"`
	Note   string `json:"note"`
	Target string `json:"target"`
}

type resolveResponse struct {
	Path string `json:"path,omitempty"`
}

// resolve finds the note a [[link]] names, as the app does.
func resolve(req resolveRequest) (resolveResponse, error) {
	store, id, err := workspace(req.Root, req.Note)
	if err != nil {
		return resolveResponse{}, err
	}
	p, ok := core.ResolveNote(store.List(), req.Target, id)
	if !ok {
		return resolveResponse{}, nil
	}
	abs := filepath.Join(store.Root(), filepath.FromSlash(p.ID))
	if _, err := os.Stat(abs); err != nil {
		return resolveResponse{}, nil
	}
	return resolveResponse{Path: abs}, nil
}
