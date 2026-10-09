package app

import (
	"strings"

	"github.com/jaisuriya-11/tsuzuri/internal/theme"
	"github.com/jaisuriya-11/tsuzuri/internal/ui"

	"github.com/charmbracelet/lipgloss"
)

type keymapSection struct {
	title string
	keys  [][2]string
}

var keymapSections = [][]keymapSection{
	{
		{"󰕭 GENERAL", [][2]string{
			{"Ctrl+N", "New note (unsaved tab)"},
			{"Ctrl+S", "Save (new notes: pick folder)"},
			{"\\ / Ctrl+P", "Find note (opens in this tab)"},
			{"Ctrl+B", "Toggle explorer"},
			{"Tab", "Next pane"},
			{"SPC Tab", "Next tab (SPC S-Tab: previous)"},
			{"[ / ]", "Previous / next tab"},
			{"SPC x", "Close tab"},
			{"SPC p", "Toggle preview"},
			{"SPC t", "Themes"},
			{"SPC g", "Graph of this note's links"},
			{"SPC d", "Home screen"},
			{"Ctrl+C", "Quit (asks to save)"},
		}},
		{"󰙅 EXPLORER", [][2]string{
			{"j / k", "Move"},
			{"Enter / l", "Open note / expand"},
			{"h", "Collapse / go to parent"},
			{"n", "New note in this folder"},
			{"a", "New sub-note"},
			{"r", "Rename"},
			{"d", "Delete (asks first)"},
			{"\\", "Find note"},
		}},
	},
	{
		{" EDITOR · NORMAL", [][2]string{
			{"i a A I", "Insert mode"},
			{"Tab/S-Tab", "Indent / outdent (insert)"},
			{"o / O", "New line below / above"},
			{"h j k l", "Move"},
			{"w / b", "Next / previous word"},
			{"0 / $", "Line start / end"},
			{"gg / G", "Top / bottom"},
			{"Ctrl+D/U", "Half page down / up"},
			{"x / dd", "Delete char / line"},
			{"yy / p", "Copy line / paste"},
			{"v / V", "Select, then y copy, d cut"},
			{"/ (insert)", "Block menu: headings, lists…"},
			{"[[ (insert)", "Link to a note"},
			{"gd / gx", "Follow the link under the cursor"},
		}},
		{" COMMANDS", [][2]string{
			{":w", "Save"},
			{":w name", "Save as name.md"},
			{":wq / :x", "Save and quit"},
			{":q / :q!", "Quit / discard"},
			{":bd", "Close tab"},
			{":enew", "New note"},
			{":colo name", "Switch theme"},
			{":export [file]", "Export note as PDF"},
			{":addrow/:addcol", "Table row / column"},
		}},
		{"󰍽 MOUSE", [][2]string{
			{"Click", "Tabs, tree, cursor"},
			{"Wheel", "Scroll any pane"},
			{"< / > / T", "Calendar month (preview)"},
			{"zM / zR", "Fold / unfold all (preview)"},
		}},
	},
}

// RenderKeymapModal renders the cheatsheet panel (without positioning).
func RenderKeymapModal(th theme.Theme, termWidth, termHeight int) string {
	colW := 40
	twoCols := termWidth >= colW*2+8
	bg := lipgloss.NewStyle().Background(th.DarkerBg)
	keyStyle := bg.Foreground(th.Blue).Bold(true)
	descStyle := bg.Foreground(th.Fg)
	headStyle := bg.Foreground(th.Purple).Bold(true)

	column := func(sections []keymapSection) []string {
		var rows []string
		for i, s := range sections {
			if i > 0 {
				rows = append(rows, "")
			}
			rows = append(rows, headStyle.Render(" "+s.title))
			for _, k := range s.keys {
				key := keyStyle.Render(" " + padRight(k[0], 11))
				rows = append(rows, ui.FitLine(key+descStyle.Render(ui.Truncate(k[1], colW-13)), colW, bg))
			}
		}
		return rows
	}

	left := column(keymapSections[0])
	right := column(keymapSections[1])
	var body []string
	if twoCols {
		n := max(len(left), len(right))
		for i := 0; i < n; i++ {
			l, r := "", ""
			if i < len(left) {
				l = left[i]
			}
			if i < len(right) {
				r = right[i]
			}
			body = append(body, ui.FitLine(l, colW, bg)+bg.Render("  ")+ui.FitLine(r, colW, bg))
		}
	} else {
		body = append(left, append([]string{""}, right...)...)
	}

	width := colW
	if twoCols {
		width = colW*2 + 2
	}
	center := func(s string) string {
		pad := max((width-lipgloss.Width(s))/2, 0)
		return bg.Render(strings.Repeat(" ", pad)) + s
	}
	title := bg.Foreground(th.Blue).Render(" ") + bg.Foreground(th.Fg).Bold(true).Render("TSUZURI KEYMAPS")
	rows := []string{center(title), ""}
	rows = append(rows, body...)
	rows = append(rows, "", center(bg.Foreground(th.GreyFg).Render("Esc / q / ? to close")))

	if maxRows := termHeight - 2; len(rows) > maxRows && maxRows > 3 {
		rows = append(rows[:maxRows-1], bg.Foreground(th.GreyFg).Render(" … enlarge the terminal to see all"))
	}
	return panel(th, rows, width)
}

func padRight(s string, width int) string {
	if w := lipgloss.Width(s); w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}
