package preview

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jaisuriya-11/tsuzuri/internal/theme"

	"github.com/charmbracelet/lipgloss"
)

// parseSpans reads "Task: 2026-10-01 -> 2026-10-05" lines; a missing or
// earlier end date makes a one-day span. A "title:" line names the timeline.
func parseSpans(body []string) []event {
	var items []event
	for i, l := range body {
		name, span, ok := strings.Cut(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(l), "- ")), ":")
		if !ok {
			continue
		}
		a, b, _ := strings.Cut(span, "->")
		start, err1 := time.Parse(dateLayout, strings.TrimSpace(a))
		end, err2 := time.Parse(dateLayout, strings.TrimSpace(b))
		if err1 != nil {
			continue
		}
		if err2 != nil || end.Before(start) {
			end = start
		}
		items = append(items, event{date: start, end: end, title: strings.TrimSpace(name), line: i})
	}
	return items
}

func renderTimeline(body []string, th theme.Theme, width int, hs *[]Hit) []string {
	items := parseSpans(body)
	muted := lipgloss.NewStyle().Foreground(th.GreyFg)
	var head []string
	if meta, _ := keyValues(body, "title"); meta["title"] != "" {
		head = []string{lipgloss.NewStyle().Foreground(th.Fg).Bold(true).Render(meta["title"]), ""}
	}
	if len(items) == 0 {
		addHit(hs, Hit{Row: len(head) + 1, X1: 5, Kind: "tl:add", Line: -1})
		return append(head, muted.Render("(empty timeline: add \"Task: 2026-10-01 -> 2026-10-05\" lines)"), muted.Render("+ New"))
	}

	first, last := items[0].date, items[0].end
	for _, it := range items {
		if it.date.Before(first) {
			first = it.date
		}
		if it.end.After(last) {
			last = it.end
		}
	}
	today := now()
	t0 := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	// Pad the range a little and include today when it's close by.
	first = first.AddDate(0, 0, -1)
	last = last.AddDate(0, 0, 2)
	span := int(last.Sub(first).Hours()/24) + 1
	if t0.After(first.AddDate(0, 0, -span/4)) && t0.Before(last.AddDate(0, 0, span/4)) {
		if t0.Before(first) {
			first = t0.AddDate(0, 0, -1)
		}
		if t0.After(last) {
			last = t0.AddDate(0, 0, 1)
		}
	}
	days := int(last.Sub(first).Hours()/24) + 1
	if days < 7 {
		last = first.AddDate(0, 0, 6)
		days = 7
	}

	// Every day maps onto the full width, so long ranges always fit.
	col := func(t time.Time) int {
		d := t.Sub(first).Hours() / 24
		return min(int(d*float64(width)/float64(days)), width-1)
	}
	perDay := float64(width) / float64(days)

	// Header: month names on one row, finer ticks (days, weeks or months) on
	// the next; labels that would collide are skipped.
	monthRow := []rune(strings.Repeat(" ", width))
	tickRow := []rune(strings.Repeat(" ", width))
	put := func(row []rune, at int, label string, gap int) bool {
		r := []rune(label)
		if at+len(r) > len(row) {
			at = len(row) - len(r) // slide labels at the right edge inwards
		}
		if at < 0 {
			return false
		}
		for i := max(at-gap, 0); i < min(at+len(r)+gap, len(row)); i++ {
			if row[i] != ' ' {
				return false
			}
		}
		copy(row[at:], r)
		return true
	}
	yearShown := false
	for d := first; !d.After(last); d = d.AddDate(0, 0, 1) {
		// Label each month where it starts (or the first, if it starts
		// early enough in its month to be worth naming).
		if d.Day() == 1 || (d.Equal(first) && d.Day() <= 20) {
			label := d.Format("Jan")
			if !yearShown || d.Month() == time.January {
				label = d.Format("Jan 2006")
			}
			if put(monthRow, col(d), label, 1) {
				yearShown = true
			}
		}
	}
	todayCol := -1
	todayLabel := []rune(strconv.Itoa(t0.Day()))
	if !t0.Before(first) && !t0.After(last) {
		todayCol = min(col(t0), width-len(todayLabel))
		copy(tickRow[todayCol:], todayLabel) // reserve it so ticks avoid it
	}
	for d := first; !d.After(last); d = d.AddDate(0, 0, 1) {
		var label string
		switch {
		case perDay >= 3:
			label = strconv.Itoa(d.Day())
		case perDay*7 >= 4 && d.Weekday() == time.Monday:
			label = strconv.Itoa(d.Day())
		default:
			continue
		}
		put(tickRow, col(d), label, 1)
	}

	var tickLine strings.Builder
	if todayCol >= 0 {
		end := todayCol + len(todayLabel)
		tickLine.WriteString(muted.Render(string(tickRow[:todayCol])))
		tickLine.WriteString(lipgloss.NewStyle().Background(th.Red).Foreground(th.Bg).Bold(true).Render(string(todayLabel)))
		tickLine.WriteString(muted.Render(string(tickRow[end:])))
	} else {
		tickLine.WriteString(muted.Render(string(tickRow)))
	}
	out := append(head,
		lipgloss.NewStyle().Foreground(th.Fg).Bold(true).Render(string(monthRow)),
		tickLine.String(),
		lipgloss.NewStyle().Foreground(th.Line).Render(strings.Repeat("─", width)),
	)

	for i, it := range items {
		a := col(it.date)
		b := max(col(it.end.AddDate(0, 0, 1))-1, a)
		color := palette(th)[i%len(palette(th))]
		label := []rune(" " + it.title)
		if dur := int(it.end.Sub(it.date).Hours()/24) + 1; dur > 1 {
			label = append(label, []rune(fmt.Sprintf(" · %dd", dur))...)
		}
		// Names that don't fit inside the bar go after it, or before it when
		// the bar sits against the right edge.
		inside := b - a
		outside := []rune{}
		outStart := -1
		if inside < len(label) {
			outside = []rune(strings.TrimSpace(string(label)))
			if b+2+len(outside) <= width {
				outStart = b + 2
			} else {
				outStart = max(a-1-len(outside), 0)
			}
		}
		var row strings.Builder
		for x := 0; x < width; x++ {
			if outStart >= 0 && x >= outStart && x < outStart+len(outside) && (x < a || x > b) {
				row.WriteString(lipgloss.NewStyle().Foreground(th.GreyFg2).Render(string(outside[x-outStart])))
				continue
			}
			switch {
			case x == a:
				row.WriteString(lipgloss.NewStyle().Background(th.OneBg2).Foreground(color).Render("▌"))
			case x > a && x <= b:
				ch := " "
				if k := x - a - 1; inside >= len(label) && k < len(label) {
					ch = string(label[k])
				}
				row.WriteString(lipgloss.NewStyle().Background(th.OneBg2).Foreground(th.Fg).Bold(true).Render(ch))
			case x == todayCol:
				row.WriteString(lipgloss.NewStyle().Foreground(th.Red).Render("│"))
			default:
				row.WriteString(" ")
			}
		}
		addHit(hs, Hit{Row: len(out), X1: width, Kind: "tl:item", Line: it.line, Arg: it.title})
		out = append(out, row.String())
	}
	addHit(hs, Hit{Row: len(out), X1: 5, Kind: "tl:add", Line: -1})
	out = append(out, muted.Render("+ New"))
	return out
}
