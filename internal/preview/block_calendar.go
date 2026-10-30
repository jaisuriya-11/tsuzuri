package preview

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jaisuriya-11/tsuzuri/internal/theme"
	"github.com/jaisuriya-11/tsuzuri/internal/ui"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type event struct {
	date  time.Time
	end   time.Time
	title string
	line  int
}

// parseEvents reads "2026-10-03: title" lines.
func parseEvents(lines []string) []event {
	var out []event
	for i, l := range lines {
		d, title, ok := strings.Cut(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(l), "- ")), ":")
		if !ok {
			continue
		}
		t, err := time.Parse(dateLayout, strings.TrimSpace(d))
		if err != nil {
			continue
		}
		out = append(out, event{date: t, title: strings.TrimSpace(title), line: i})
	}
	return out
}

// calendarMonth is the month a calendar shows: its "month:" key, or this
// month.
func calendarMonth(body []string) time.Time {
	meta, _ := keyValues(body, "month")
	if m, err := time.Parse("2006-01", meta["month"]); err == nil {
		return m
	}
	today := now()
	return time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func renderCalendar(body []string, th theme.Theme, width int, cal CalendarView, hs *[]Hit) []string {
	today := now()
	month := calendarMonth(body)
	if cal.Today {
		month = time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
	month = month.AddDate(0, cal.Shift, 0)
	events := parseEvents(body)
	byDay := map[string][]event{}
	for _, e := range events {
		k := e.date.Format(dateLayout)
		byDay[k] = append(byDay[k], e)
	}

	cw := max(min((width-8)/7, 16), 3)
	cellH := 3
	if cw < 6 {
		cellH = 1
	}
	line := lipgloss.NewStyle().Foreground(th.Line)
	muted := lipgloss.NewStyle().Foreground(th.GreyFg)
	text := lipgloss.NewStyle().Foreground(th.Fg)

	title := text.Bold(true).Render(month.Format("January 2006")) + "  " + muted.Render(fmt.Sprintf("%d events", len(events)))
	nav := lipgloss.NewStyle().Foreground(th.GreyFg2).Render(calendarNav)
	gridW := 7*cw + 8
	gap := max(gridW-ansi.StringWidth(title)-ansi.StringWidth(nav), 2)
	out := []string{title + strings.Repeat(" ", gap) + nav, ""}
	navX := ansi.StringWidth(title) + gap
	addHit(hs, Hit{Row: 0, X0: navX, X1: navX + 2, Kind: "cal:prev", Line: -1})
	addHit(hs, Hit{Row: 0, X0: navX + 3, X1: navX + 8, Kind: "cal:today", Line: -1})
	addHit(hs, Hit{Row: 0, X0: navX + 9, X1: navX + 12, Kind: "cal:next", Line: -1})

	var head strings.Builder
	head.WriteString(" ")
	for _, d := range []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"} {
		head.WriteString(muted.Render(center(d[:min(len(d), cw)], cw)))
		head.WriteString(" ")
	}
	out = append(out, head.String())

	border := func(l, m, r string) string {
		segs := make([]string, 7)
		for i := range segs {
			segs[i] = strings.Repeat("─", cw)
		}
		return line.Render(l + strings.Join(segs, m) + r)
	}
	out = append(out, border("┌", "┬", "┐"))

	start := month.AddDate(0, 0, -int(month.Weekday()))
	todayKey := today.Format(dateLayout)
	for week := 0; week < 6; week++ {
		weekStart := start.AddDate(0, 0, week*7)
		if week > 0 && weekStart.Month() != month.Month() {
			break
		}
		rows := make([]strings.Builder, cellH)
		for r := range rows {
			rows[r].WriteString(line.Render("│"))
		}
		for d := 0; d < 7; d++ {
			day := weekStart.AddDate(0, 0, d)
			key := day.Format(dateLayout)
			shade := lipgloss.NewStyle()
			if d == 0 || d == 6 {
				shade = shade.Background(th.Bg2)
			}
			num := strconv.Itoa(day.Day())
			if day.Day() == 1 && cw >= 6 {
				num = day.Format("Jan 2")
			}
			numStyle := shade.Foreground(th.Fg)
			if day.Month() != month.Month() {
				numStyle = shade.Foreground(th.GreyFg)
			}
			numStr := numStyle.Render(num + " ")
			if key == todayKey {
				numStr = lipgloss.NewStyle().Background(th.Red).Foreground(th.Bg).Bold(true).Render(" "+num+" ") + shade.Render(" ")
			}
			rows[0].WriteString(shade.Render(strings.Repeat(" ", max(cw-ansi.StringWidth(numStr), 0))))
			rows[0].WriteString(numStr)
			evs := byDay[key]
			x0 := 1 + d*(cw+1)
			rowBase := len(out)
			if week > 0 {
				rowBase++ // the separator line is appended first
			}
			addHit(hs, Hit{Row: rowBase, H: cellH, X0: x0, X1: x0 + cw, Kind: "cal:day", Line: -1, Arg: key})
			for r := 1; r < cellH; r++ {
				cell := ""
				if i := r - 1; i < len(evs) {
					label := evs[i].title
					if r == cellH-1 && len(evs) > cellH-1 {
						label = fmt.Sprintf("+%d more", len(evs)-(cellH-2))
					} else {
						addHit(hs, Hit{Row: rowBase + r, X0: x0, X1: x0 + cw, Kind: "cal:event", Line: evs[i].line, Arg: evs[i].title})
					}
					cell = lipgloss.NewStyle().Background(th.OneBg2).Foreground(th.Fg).Render(ui.Truncate(" "+label, cw-1))
				}
				rows[r].WriteString(cell + shade.Render(strings.Repeat(" ", max(cw-ansi.StringWidth(cell), 0))))
			}
			for r := range rows {
				rows[r].WriteString(line.Render("│"))
			}
		}
		if week > 0 {
			out = append(out, border("├", "┼", "┤"))
		}
		for r := range rows {
			out = append(out, rows[r].String())
		}
	}
	out = append(out, border("└", "┴", "┘"))
	return out
}
