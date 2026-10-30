package preview

import (
	"strings"
	"time"
)

// The types below describe a view block's content as data, for editors that
// draw views themselves (the VS Code extension) rather than as terminal
// text. They come from the same parsers the preview draws from. Line
// numbers count from the first line inside the fence.

// View is one view block. Kind says which of the other fields is set.
type View struct {
	Kind string `json:"kind"` // board, calendar, timeline, chart or form
	// Title is a timeline's "title:" (charts and forms carry their own).
	Title    string        `json:"title,omitempty"`
	Board    []BoardColumn `json:"board,omitempty"`
	Calendar *Calendar     `json:"calendar,omitempty"`
	Timeline []Span        `json:"timeline,omitempty"`
	Chart    *Chart        `json:"chart,omitempty"`
	Form     *Form         `json:"form,omitempty"`
}

type BoardColumn struct {
	Name string `json:"name"`
	Line int    `json:"line"` // -1 when the board has cards but no heading
	// Tone is what the name says about its cards: "done", "progress",
	// "todo", "blocked", or "" for none of these.
	Tone  string      `json:"tone"`
	Cards []BoardCard `json:"cards"`
}

type BoardCard struct {
	Title       string   `json:"title"`
	Description []string `json:"description,omitempty"`
	Line        int      `json:"line"`
}

type Calendar struct {
	Month  string  `json:"month"` // 2006-01
	Events []Event `json:"events"`
}

type Event struct {
	Date  string `json:"date"` // 2006-01-02
	Title string `json:"title"`
	Line  int    `json:"line"`
}

type Span struct {
	Title string `json:"title"`
	Start string `json:"start"` // 2006-01-02
	End   string `json:"end"`
	Line  int    `json:"line"`
}

type Chart struct {
	Type   string  `json:"type"` // bar, hbar, line, pie or donut
	Title  string  `json:"title,omitempty"`
	Height int     `json:"height"`
	Data   []Datum `json:"data"`
}

type Datum struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
	Line  int     `json:"line"`
}

type Form struct {
	Title       string     `json:"title,omitempty"`
	Description string     `json:"description,omitempty"`
	Questions   []Question `json:"questions"`
}

type Question struct {
	Text     string   `json:"text"`
	Kind     string   `json:"kind"` // text, choice, multi …
	Required bool     `json:"required,omitempty"`
	Options  []string `json:"options,omitempty"`
	Answer   string   `json:"answer,omitempty"`
	Line     int      `json:"line"`
}

// ViewData reads a view block. ok is false for anything that isn't a
// board, calendar, timeline, chart or form.
func ViewData(lang string, body []string) (View, bool) {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "board", "kanban":
		v := View{Kind: "board", Board: []BoardColumn{}}
		for _, c := range parseBoard(body) {
			col := BoardColumn{Name: c.name, Line: c.line, Tone: statusTone(c.name), Cards: []BoardCard{}}
			for _, card := range c.cards {
				col.Cards = append(col.Cards, BoardCard{Title: card.title, Description: card.desc, Line: card.line})
			}
			v.Board = append(v.Board, col)
		}
		return v, true

	case "calendar":
		cal := &Calendar{Month: calendarMonth(body).Format("2006-01"), Events: []Event{}}
		for _, e := range parseEvents(body) {
			cal.Events = append(cal.Events, Event{Date: e.date.Format(dateLayout), Title: e.title, Line: e.line})
		}
		return View{Kind: "calendar", Calendar: cal}, true

	case "timeline", "gantt":
		meta, _ := keyValues(body, "title")
		v := View{Kind: "timeline", Title: meta["title"], Timeline: []Span{}}
		for _, e := range parseSpans(body) {
			v.Timeline = append(v.Timeline, Span{Title: e.title, Start: day(e.date), End: day(e.end), Line: e.line})
		}
		return v, true

	case "chart":
		meta, data := parseChart(body)
		c := &Chart{Type: chartKind(meta["type"]), Title: meta["title"], Height: chartHeight(meta["height"]), Data: []Datum{}}
		for _, d := range data {
			c.Data = append(c.Data, Datum{Label: d.label, Value: d.value, Line: d.line})
		}
		return View{Kind: "chart", Chart: c}, true

	case "form":
		meta, _ := keyValues(body, "title", "description")
		f := &Form{Title: meta["title"], Description: meta["description"], Questions: []Question{}}
		for _, q := range parseForm(body) {
			f.Questions = append(f.Questions, Question{
				Text: q.text, Kind: q.kind, Required: q.required, Options: q.options, Answer: q.answer, Line: q.line,
			})
		}
		return View{Kind: "form", Form: f}, true
	}
	return View{}, false
}

func day(t time.Time) string { return t.Format(dateLayout) }
