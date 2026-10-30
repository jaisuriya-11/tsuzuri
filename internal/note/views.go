package note

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Edits made by clicking on a view (board, calendar, timeline, chart,
// form). Each takes the note's lines and returns them changed. A view
// block sits between its opening fence line block and closing fence line
// end; lines named by an edit are note lines inside it.

const dateLayout = "2006-01-02"

// questionKind is a question type as written in "(kind)": a plain word.
var questionKind = regexp.MustCompile(`^[a-z]{0,20}$`)

// ViewEdit is one change to a view, as an editor sends it. Which fields
// count depends on Op.
type ViewEdit struct {
	Op     string `json:"op"`
	Line   int    `json:"line"`   // the card, event, item, value or question
	Index  int    `json:"index"`  // a column, option or star
	Text   string `json:"text"`   // a title, name, label or answer
	Value  string `json:"value"`  // a number, a question type or a chart type
	Date   string `json:"date"`   // 2006-01-02
	End    string `json:"end"`    // 2006-01-02, a timeline item's last day
	Values string `json:"values"` // a choice question's options, "A | B"
	// Lines is a card's description, one entry per line.
	Lines []string `json:"lines,omitempty"`
}

// ApplyView applies e to the view block [block, end] of lines.
func ApplyView(lines []string, block, end int, e ViewEdit) ([]string, error) {
	if block < 0 || end <= block || end >= len(lines)+1 {
		return nil, errors.New("view is outside the note")
	}
	inside := func(l int) error {
		if l <= block || l >= end || l >= len(lines) {
			return fmt.Errorf("line %d is not in this view", l)
		}
		return nil
	}
	needText := func() error {
		if strings.TrimSpace(e.Text) == "" {
			return errors.New("text is empty")
		}
		return nil
	}
	// Everything an edit writes stays on its own line.
	oneLine := strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ")
	text := strings.TrimSpace(oneLine.Replace(e.Text))
	e.Values = oneLine.Replace(e.Values)
	e.Value = strings.TrimSpace(oneLine.Replace(e.Value))

	switch e.Op {
	// Board
	case "board.add":
		if err := needText(); err != nil {
			return nil, err
		}
		cols := BoardColumns(lines, block, end)
		at := end
		if e.Index >= 0 && e.Index < len(cols) {
			at = cols[e.Index].EndLine
		}
		return Insert(lines, at, "- "+text), nil
	case "board.rename":
		if err := firstOf(inside(e.Line), needText()); err != nil {
			return nil, err
		}
		return RenameCard(lines, e.Line, text), nil
	case "board.delete":
		if err := inside(e.Line); err != nil {
			return nil, err
		}
		return Remove(lines, e.Line, CardExtent(lines, e.Line, end)), nil
	case "board.describe":
		if err := inside(e.Line); err != nil {
			return nil, err
		}
		return DescribeCard(lines, e.Line, end, e.Lines), nil
	case "board.column":
		if err := needText(); err != nil {
			return nil, err
		}
		return Insert(lines, end, "## "+text), nil
	case "board.renamecolumn":
		if err := firstOf(inside(e.Line), needText()); err != nil {
			return nil, err
		}
		return RenameColumn(lines, e.Line, text), nil
	case "board.deletecolumn":
		if err := inside(e.Line); err != nil {
			return nil, err
		}
		return DeleteColumn(lines, block, end, e.Line), nil

	// Calendar
	case "calendar.add":
		if err := firstOf(needText(), validDate(e.Date)); err != nil {
			return nil, err
		}
		return Insert(lines, end, e.Date+": "+text), nil
	case "calendar.set":
		if err := firstOf(inside(e.Line), needText(), validDate(e.Date)); err != nil {
			return nil, err
		}
		return replace(lines, e.Line, e.Date+": "+text), nil

	// Timeline
	case "timeline.add", "timeline.set":
		start, last, err := span(e.Date, e.End)
		if err == nil {
			err = needText()
		}
		if err == nil && e.Op == "timeline.set" {
			err = inside(e.Line)
		}
		if err != nil {
			return nil, err
		}
		if e.Op == "timeline.add" {
			return Insert(lines, end, SpanLine(text, start, last)), nil
		}
		return replace(lines, e.Line, SpanLine(text, start, last)), nil

	// Chart
	case "chart.add", "chart.set":
		if err := needText(); err != nil {
			return nil, err
		}
		v := strings.TrimSpace(e.Value)
		if _, err := strconv.ParseFloat(strings.TrimSuffix(v, "%"), 64); err != nil {
			return nil, fmt.Errorf("not a number: %q", v)
		}
		if e.Op == "chart.add" {
			return Insert(lines, end, text+": "+v), nil
		}
		if err := inside(e.Line); err != nil {
			return nil, err
		}
		return replace(lines, e.Line, text+": "+v), nil
	case "chart.type":
		switch e.Value {
		case "bar", "hbar", "line", "pie", "donut":
		default:
			return nil, fmt.Errorf("unknown chart type %q", e.Value)
		}
		return SetKey(lines, block, end, "type", e.Value), nil

	// Form
	case "form.title", "form.description":
		return SetKey(lines, block, end, strings.TrimPrefix(e.Op, "form."), text), nil
	case "form.answer":
		if err := inside(e.Line); err != nil {
			return nil, err
		}
		return SetAnswer(lines, e.Line, text), nil
	case "form.pick", "form.toggle", "form.rate":
		if err := inside(e.Line); err != nil {
			return nil, err
		}
		return PickAnswer(lines, e.Line, strings.TrimPrefix(e.Op, "form."), e.Index), nil
	case "form.add":
		if err := needText(); err != nil {
			return nil, err
		}
		if !questionKind.MatchString(e.Value) {
			return nil, fmt.Errorf("unknown question type %q", e.Value)
		}
		return Insert(lines, end, QuestionLine(text, e.Value, e.Values, false)), nil
	case "form.rename":
		if err := firstOf(inside(e.Line), needText()); err != nil {
			return nil, err
		}
		q := ParseQuestion(lines[e.Line])
		return replace(lines, e.Line, QuestionLine(text, q.Kind, strings.Join(q.Options, " | "), q.Required)), nil
	case "form.type":
		if err := inside(e.Line); err != nil {
			return nil, err
		}
		if !questionKind.MatchString(e.Value) {
			return nil, fmt.Errorf("unknown question type %q", e.Value)
		}
		return SetQuestionType(lines, e.Line, e.Value, e.Values), nil
	case "form.required":
		if err := inside(e.Line); err != nil {
			return nil, err
		}
		q := ParseQuestion(lines[e.Line])
		return replace(lines, e.Line, QuestionLine(q.Text, q.Kind, strings.Join(q.Options, " | "), !q.Required)), nil
	case "form.option":
		if err := firstOf(inside(e.Line), needText()); err != nil {
			return nil, err
		}
		return AddOption(lines, e.Line, text), nil

	// Any view with a title (chart, timeline, form); empty text removes it.
	case "title":
		if text == "" {
			return RemoveKey(lines, block, end, "title"), nil
		}
		return SetKey(lines, block, end, "title", text), nil

	// Any view: a line that is one item (event, item, value, question).
	case "delete":
		if err := inside(e.Line); err != nil {
			return nil, err
		}
		n := 1
		if AnswerLine(lines, e.Line) >= 0 {
			n = 2 // a question takes its answer with it
		}
		return Remove(lines, e.Line, n), nil
	}
	return nil, fmt.Errorf("unknown view edit %q", e.Op)
}

func firstOf(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func replace(lines []string, at int, line string) []string {
	out := append([]string{}, lines...)
	out[at] = line
	return out
}

func indentOf(line string) string {
	return line[:len(line)-len(strings.TrimLeft(line, " \t"))]
}

func validDate(s string) error {
	if _, err := time.Parse(dateLayout, s); err != nil {
		return fmt.Errorf("not a date: %q", s)
	}
	return nil
}

/* ----------------------------------------------------------------- board */

// RenameCard gives the card on line at a new title, keeping its indent.
func RenameCard(lines []string, at int, title string) []string {
	return replace(lines, at, indentOf(lines[at])+"- "+title)
}

// RenameColumn renames the "## Column" heading on line at.
func RenameColumn(lines []string, at int, name string) []string {
	t := strings.TrimSpace(lines[at])
	hashes := t[:len(t)-len(strings.TrimLeft(t, "#"))]
	if hashes == "" {
		hashes = "##"
	}
	return replace(lines, at, hashes+" "+name)
}

// DeleteColumn removes the column headed on line at, with its cards.
func DeleteColumn(lines []string, block, end, at int) []string {
	cols := BoardColumns(lines, block, end)
	for i, c := range cols {
		if c.Line != at {
			continue
		}
		stop := end
		if i+1 < len(cols) {
			stop = cols[i+1].Line
		}
		return Remove(lines, at, stop-at)
	}
	return lines
}

// DescribeCard replaces the description under the card on line at with
// desc, one line each (indented, so they stay the card's); blank lines are
// dropped, as a blank line would end the card.
func DescribeCard(lines []string, at, end int, desc []string) []string {
	n := CardExtent(lines, at, end)
	var add []string
	indent := indentOf(lines[at]) + "  "
	for _, d := range desc {
		for _, l := range strings.Split(strings.ReplaceAll(d, "\r", ""), "\n") {
			if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
				add = append(add, indent+l)
			}
		}
	}
	return Insert(Remove(lines, at+1, n-1), at+1, add...)
}

/* -------------------------------------------------------------- timeline */

func span(a, b string) (start, end time.Time, err error) {
	if start, err = time.Parse(dateLayout, a); err != nil {
		return start, end, fmt.Errorf("not a date: %q", a)
	}
	if end, err = time.Parse(dateLayout, b); err != nil || end.Before(start) {
		end = start
	}
	return start, end, nil
}

// ParseSpan reads a timeline line, "Name: 2026-10-01 -> 2026-10-05".
func ParseSpan(line string) (name string, start, end time.Time, ok bool) {
	n, s, found := strings.Cut(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "- ")), ":")
	if !found {
		return "", start, end, false
	}
	a, b, _ := strings.Cut(s, "->")
	var err error
	if start, err = time.Parse(dateLayout, strings.TrimSpace(a)); err != nil {
		return "", start, end, false
	}
	if end, err = time.Parse(dateLayout, strings.TrimSpace(b)); err != nil || end.Before(start) {
		end = start
	}
	return strings.TrimSpace(n), start, end, true
}

// SpanLine writes a timeline line.
func SpanLine(name string, start, end time.Time) string {
	return name + ": " + start.Format(dateLayout) + " -> " + end.Format(dateLayout)
}

/* ----------------------------------------------------------------- forms */

// SetKey sets a "key: value" line inside the view, adding it after the
// opening fence when the view has none.
func SetKey(lines []string, block, end int, key, value string) []string {
	for i := block + 1; i < end && i < len(lines); i++ {
		if k, _, ok := strings.Cut(lines[i], ":"); ok && strings.EqualFold(strings.TrimSpace(k), key) {
			return replace(lines, i, key+": "+value)
		}
	}
	return Insert(lines, block+1, key+": "+value)
}

// RemoveKey removes a "key: value" line from the view.
func RemoveKey(lines []string, block, end int, key string) []string {
	for i := block + 1; i < end && i < len(lines); i++ {
		if k, _, ok := strings.Cut(lines[i], ":"); ok && strings.EqualFold(strings.TrimSpace(k), key) {
			return Remove(lines, i, 1)
		}
	}
	return lines
}

// Question is a form question line: "?* Text (kind): A | B".
type Question struct {
	Text     string
	Kind     string // "" for a short answer
	Required bool
	Options  []string
}

// ParseQuestion reads a question line.
func ParseQuestion(line string) Question {
	t := strings.TrimSpace(line)
	q := Question{Required: strings.HasPrefix(t, "?*")}
	t = strings.TrimSpace(strings.TrimLeft(t, "?*"))
	text, opts, _ := strings.Cut(t, ":")
	if j := strings.LastIndex(text, "("); j > 0 && strings.HasSuffix(strings.TrimSpace(text), ")") {
		q.Kind = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(text[j+1:]), ")"))
		text = text[:j]
	}
	q.Text = strings.TrimSpace(text)
	for _, o := range strings.Split(opts, "|") {
		if o = strings.TrimSpace(o); o != "" {
			q.Options = append(q.Options, o)
		}
	}
	return q
}

// SplitOptions reads options as typed, "A | B" or "A, B".
func SplitOptions(s string) []string {
	var out []string
	for _, o := range strings.FieldsFunc(s, func(r rune) bool { return r == '|' || r == ',' }) {
		if o = strings.TrimSpace(o); o != "" {
			out = append(out, o)
		}
	}
	return out
}

// HasOptions reports whether a question kind offers options to pick from.
func HasOptions(kind string) bool {
	switch kind {
	case "choice", "select", "radio", "multi", "checkbox", "checkboxes":
		return true
	}
	return false
}

// QuestionLine writes a question line. Kinds with options get options
// (three placeholders when none are given); other kinds drop them.
func QuestionLine(text, kind, options string, required bool) string {
	line := "? "
	if required {
		line = "?* "
	}
	line += text
	if kind != "" && kind != "text" {
		line += " (" + kind + ")"
	}
	if HasOptions(kind) {
		opts := SplitOptions(options)
		if len(opts) == 0 {
			opts = []string{"Option 1", "Option 2", "Option 3"}
		}
		line += ": " + strings.Join(opts, " | ")
	}
	return line
}

// SetQuestionType changes a question's kind. Its answer goes, as it may
// not fit the new kind.
func SetQuestionType(lines []string, q int, kind, options string) []string {
	cur := ParseQuestion(lines[q])
	if options == "" {
		options = strings.Join(cur.Options, " | ")
	}
	out := replace(lines, q, QuestionLine(cur.Text, kind, options, cur.Required))
	if at := AnswerLine(out, q); at >= 0 {
		out = Remove(out, at, 1)
	}
	return out
}

// AddOption adds an option to the question on line q.
func AddOption(lines []string, q int, option string) []string {
	if strings.Contains(lines[q], ":") {
		return replace(lines, q, strings.TrimRight(lines[q], " ")+" | "+option)
	}
	return replace(lines, q, lines[q]+": "+option)
}

// AnswerLine returns the "= answer" line under question line q, or -1.
func AnswerLine(lines []string, q int) int {
	if q+1 < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[q+1]), "=") {
		return q + 1
	}
	return -1
}

// Answer returns the answer to question line q.
func Answer(lines []string, q int) string {
	if at := AnswerLine(lines, q); at >= 0 {
		return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[at]), "="))
	}
	return ""
}

// SetAnswer writes (or with "" removes) the answer to question line q.
func SetAnswer(lines []string, q int, value string) []string {
	at := AnswerLine(lines, q)
	switch {
	case value == "" && at >= 0:
		return Remove(lines, at, 1)
	case value == "":
		return lines
	case at >= 0:
		return replace(lines, at, "  = "+value)
	}
	return Insert(lines, q+1, "  = "+value)
}

// PickAnswer answers by clicking: "pick" chooses option index (again
// clears it), "toggle" ticks or unticks it among several, "rate" sets index
// stars (again clears them).
func PickAnswer(lines []string, q int, how string, index int) []string {
	current := Answer(lines, q)
	opts := ParseQuestion(lines[q]).Options
	switch how {
	case "pick":
		if index < 0 || index >= len(opts) {
			return lines
		}
		v := opts[index]
		if current == v {
			v = ""
		}
		return SetAnswer(lines, q, v)
	case "toggle":
		if index < 0 || index >= len(opts) {
			return lines
		}
		picked := map[string]bool{}
		for _, a := range strings.Split(current, ",") {
			if a = strings.TrimSpace(a); a != "" {
				picked[a] = true
			}
		}
		picked[opts[index]] = !picked[opts[index]]
		var keep []string
		for _, o := range opts {
			if picked[o] {
				keep = append(keep, o)
			}
		}
		return SetAnswer(lines, q, strings.Join(keep, ", "))
	case "rate":
		v := strconv.Itoa(index)
		if current == v || index <= 0 {
			v = ""
		}
		return SetAnswer(lines, q, v)
	}
	return lines
}
