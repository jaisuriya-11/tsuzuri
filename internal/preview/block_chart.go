package preview

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/jaisuriya-11/tsuzuri/internal/theme"
	"github.com/jaisuriya-11/tsuzuri/internal/ui"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type datum struct {
	label string
	value float64
	line  int
}

// pixelGrid paints a w×h pixel canvas with half blocks (two pixels per row).
func pixelGrid(px [][]lipgloss.Color) []string {
	h := len(px)
	if h == 0 {
		return nil
	}
	w := len(px[0])
	var out []string
	for y := 0; y < h; y += 2 {
		var b strings.Builder
		for x := 0; x < w; x++ {
			top := px[y][x]
			var bot lipgloss.Color
			if y+1 < h {
				bot = px[y+1][x]
			}
			switch {
			case top == "" && bot == "":
				b.WriteString(" ")
			case top == bot:
				b.WriteString(lipgloss.NewStyle().Foreground(top).Render("█"))
			case top == "":
				b.WriteString(lipgloss.NewStyle().Foreground(bot).Render("▄"))
			case bot == "":
				b.WriteString(lipgloss.NewStyle().Foreground(top).Render("▀"))
			default:
				b.WriteString(lipgloss.NewStyle().Foreground(top).Background(bot).Render("▀"))
			}
		}
		out = append(out, b.String())
	}
	return out
}

func canvas(w, h int) [][]lipgloss.Color {
	px := make([][]lipgloss.Color, h)
	for i := range px {
		px[i] = make([]lipgloss.Color, w)
	}
	return px
}

func fmtNum(v float64) string {
	if v == math.Trunc(v) && math.Abs(v) < 1e12 {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'f', 1, 64)
}

// parseChart reads "Label: 12" lines and the type, title and height keys.
func parseChart(body []string) (meta map[string]string, data []datum) {
	meta, _ = keyValues(body, "type", "title", "height")
	for i, l := range body {
		k, v, ok := strings.Cut(strings.TrimPrefix(strings.TrimSpace(l), "- "), ":")
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "type", "title", "height":
			continue
		}
		if !ok {
			continue
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(v), "%")), 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			continue
		}
		data = append(data, datum{strings.TrimSpace(k), f, i})
	}
	return meta, data
}

// chartKind is the chart a type key asks for: bar, hbar, line, pie or donut.
func chartKind(t string) string {
	switch t = strings.ToLower(t); t {
	case "hbar", "barh", "horizontal":
		return "hbar"
	case "line", "pie", "donut":
		return t
	}
	return "bar"
}

// chartHeight is the height key when it's sensible, else 10 rows.
func chartHeight(h string) int {
	if n, err := strconv.Atoi(h); err == nil && n >= 3 && n <= 30 {
		return n
	}
	return 10
}

func renderChart(body []string, th theme.Theme, width int, hs *[]Hit) []string {
	meta, data := parseChart(body)
	muted := lipgloss.NewStyle().Foreground(th.GreyFg)
	var out []string
	if t := meta["title"]; t != "" {
		out = append(out, lipgloss.NewStyle().Foreground(th.Fg).Bold(true).Render(t), "")
	}
	if len(data) == 0 {
		addHit(hs, Hit{Row: len(out) + 1, X1: 13, Kind: "chart:add", Line: -1})
		return append(out, muted.Render("(empty chart: add \"Label: 12\" lines)"), muted.Render("+ Add value"))
	}
	height := chartHeight(meta["height"])
	var chart []string
	var local []Hit
	switch kind := chartKind(meta["type"]); kind {
	case "hbar":
		chart = hbarChart(data, th, width, &local)
	case "line":
		chart = lineChart(data, th, width, height, &local)
	case "pie", "donut":
		chart = pieChart(data, th, width, kind == "donut", &local)
	default:
		chart = barChart(data, th, width, height, &local)
	}
	for _, h := range local {
		h.Row += len(out)
		addHit(hs, h)
	}
	out = append(out, chart...)
	addHit(hs, Hit{Row: len(out), X1: 13, Kind: "chart:add", Line: -1})
	return append(out, muted.Render("+ Add value"))
}

func maxValue(data []datum) float64 {
	m := 0.0
	for _, d := range data {
		m = math.Max(m, d.value)
	}
	if m == 0 {
		m = 1
	}
	return m
}

func barChart(data []datum, th theme.Theme, width, height int, hs *[]Hit) []string {
	maxV := maxValue(data)
	// Wide enough for every axis label (max, half and 0), e.g. "7.5".
	axisW := max(len(fmtNum(maxV)), len(fmtNum(maxV/2))) + 1
	slot := min(max((width-axisW-1)/len(data), 2), 10)
	bw := max(slot-1, 1)
	plotW := slot * len(data)
	px := canvas(plotW, height*2)
	cols := palette(th)
	for i, d := range data {
		h := int(math.Round(math.Max(d.value, 0) / maxV * float64(height*2)))
		h = max(0, min(height*2, h))
		for y := height*2 - h; y < height*2; y++ {
			for x := i * slot; x < i*slot+bw; x++ {
				if y >= 0 && y < len(px) && x >= 0 && x < len(px[y]) {
					px[y][x] = cols[i%len(cols)]
				}
			}
		}
	}
	muted := lipgloss.NewStyle().Foreground(th.GreyFg)
	axis := lipgloss.NewStyle().Foreground(th.Line)
	rows := pixelGrid(px)
	var out []string
	for i, r := range rows {
		label := ""
		switch i {
		case 0:
			label = fmtNum(maxV)
		case len(rows) - 1:
			label = "0"
		case len(rows) / 2:
			label = fmtNum(maxV / 2)
		}
		out = append(out, muted.Render(fmt.Sprintf("%*s", axisW-1, label))+axis.Render("┤")+r)
	}
	out = append(out, strings.Repeat(" ", axisW-1)+axis.Render("└"+strings.Repeat("─", plotW)))
	var labels, values strings.Builder
	for i, d := range data {
		labels.WriteString(pad(ui.Truncate(d.label, bw), slot))
		values.WriteString(lipgloss.NewStyle().Foreground(cols[i%len(cols)]).Render(pad(ui.Truncate(fmtNum(d.value), bw), slot)))
	}
	out = append(out, strings.Repeat(" ", axisW)+muted.Render(labels.String()))
	out = append(out, strings.Repeat(" ", axisW)+values.String())
	for i, d := range data {
		addHit(hs, Hit{Row: 0, H: len(out), X0: axisW + i*slot, X1: axisW + i*slot + bw, Kind: "chart:value", Line: d.line, Arg: d.label})
	}
	return out
}

func hbarChart(data []datum, th theme.Theme, width int, hs *[]Hit) []string {
	maxV := maxValue(data)
	labelW := 0
	for _, d := range data {
		labelW = max(labelW, ansi.StringWidth(d.label))
	}
	labelW = min(labelW, width/3)
	barW := max(width-labelW-len(fmtNum(maxV))-3, 4)
	cols := palette(th)
	var out []string
	for i, d := range data {
		n := math.Max(d.value, 0) / maxV * float64(barW)
		full := max(0, min(int(n), barW))
		bar := strings.Repeat("█", full)
		if frac := n - float64(full); frac > 0.5 && full < barW {
			bar += "▌"
		}
		addHit(hs, Hit{Row: len(out), X1: width, Kind: "chart:value", Line: d.line, Arg: d.label})
		out = append(out, pad(ui.Truncate(d.label, labelW), labelW)+" "+
			lipgloss.NewStyle().Foreground(cols[i%len(cols)]).Render(bar)+" "+
			lipgloss.NewStyle().Foreground(th.GreyFg2).Render(fmtNum(d.value)))
	}
	return out
}

func lineChart(data []datum, th theme.Theme, width, height int, hs *[]Hit) []string {
	minV, maxV := data[0].value, data[0].value
	for _, d := range data {
		minV, maxV = math.Min(minV, d.value), math.Max(maxV, d.value)
	}
	if maxV == minV {
		maxV = minV + 1
	}
	axisW := max(len(fmtNum(maxV)), len(fmtNum(minV))) + 1
	plotW := max(width-axisW-1, len(data))
	ph := height * 2
	px := canvas(plotW, ph)
	pt := func(i int) (int, int) {
		x := 0
		if len(data) > 1 {
			x = i * (plotW - 1) / (len(data) - 1)
		}
		y := ph - 1 - int(math.Round((data[i].value-minV)/(maxV-minV)*float64(ph-1)))
		x = max(0, min(plotW-1, x))
		y = max(0, min(ph-1, y))
		return x, y
	}
	for i := 0; i < len(data); i++ {
		x0, y0 := pt(i)
		px[y0][x0] = th.Blue
		if i+1 < len(data) {
			x1, y1 := pt(i + 1)
			// Bresenham line between neighbouring points.
			dx, dy := abs(x1-x0), -abs(y1-y0)
			sx, sy := sign(x1-x0), sign(y1-y0)
			e := dx + dy
			for x, y := x0, y0; ; {
				px[y][x] = th.Blue
				if x == x1 && y == y1 {
					break
				}
				if e2 := 2 * e; e2 >= dy {
					e += dy
					x += sx
				} else {
					e += dx
					y += sy
				}
			}
		}
		px[y0][x0] = th.Yellow
	}
	muted := lipgloss.NewStyle().Foreground(th.GreyFg)
	axis := lipgloss.NewStyle().Foreground(th.Line)
	rows := pixelGrid(px)
	var out []string
	for i, r := range rows {
		label := ""
		if i == 0 {
			label = fmtNum(maxV)
		} else if i == len(rows)-1 {
			label = fmtNum(minV)
		}
		out = append(out, muted.Render(fmt.Sprintf("%*s", axisW-1, label))+axis.Render("┤")+r)
	}
	out = append(out, strings.Repeat(" ", axisW-1)+axis.Render("└"+strings.Repeat("─", plotW)))
	labels := []rune(strings.Repeat(" ", plotW))
	for i, d := range data {
		x, _ := pt(i)
		x = max(min(x, plotW-len([]rune(d.label))), 0)
		for j, r := range []rune(d.label) {
			if x+j < len(labels) {
				labels[x+j] = r
			}
		}
	}
	out = append(out, strings.Repeat(" ", axisW)+muted.Render(string(labels)))
	// Each point owns the columns closest to it.
	for i, d := range data {
		x, _ := pt(i)
		left, right := 0, plotW
		if i > 0 {
			px, _ := pt(i - 1)
			left = (px + x + 1) / 2
		}
		if i+1 < len(data) {
			nx, _ := pt(i + 1)
			right = (x + nx + 1) / 2
		}
		addHit(hs, Hit{Row: 0, H: len(out), X0: axisW + left, X1: axisW + right, Kind: "chart:value", Line: d.line, Arg: d.label})
	}
	return out
}

func pieChart(data []datum, th theme.Theme, width int, donut bool, hs *[]Hit) []string {
	total := 0.0
	for _, d := range data {
		total += math.Max(d.value, 0)
	}
	if total == 0 {
		total = 1
	}
	r := 8
	size := r * 2
	px := canvas(size, size)
	cols := palette(th)
	// Cumulative angles, starting at 12 o'clock and going clockwise.
	bounds := make([]float64, len(data))
	acc := 0.0
	for i, d := range data {
		acc += math.Max(d.value, 0) / total
		bounds[i] = acc
	}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			fx, fy := float64(x)-float64(r)+0.5, float64(y)-float64(r)+0.5
			dist := math.Hypot(fx, fy)
			if dist > float64(r) || (donut && dist < float64(r)*0.5) {
				continue
			}
			ang := math.Atan2(fx, -fy) / (2 * math.Pi)
			if ang < 0 {
				ang++
			}
			idx := sort.SearchFloat64s(bounds, ang)
			px[y][x] = cols[min(idx, len(data)-1)%len(cols)]
		}
	}
	pie := pixelGrid(px)
	var legend []string
	for i, d := range data {
		pct := math.Max(d.value, 0) / total * 100
		legend = append(legend, lipgloss.NewStyle().Foreground(cols[i%len(cols)]).Render("■ ")+
			lipgloss.NewStyle().Foreground(th.Fg).Render(ui.Truncate(d.label, max(width-size-16, 6)))+
			lipgloss.NewStyle().Foreground(th.GreyFg2).Render(fmt.Sprintf("  %s (%.0f%%)", fmtNum(d.value), pct)))
	}
	for i, d := range data {
		addHit(hs, Hit{Row: i, X0: size + 3, X1: width, Kind: "chart:value", Line: d.line, Arg: d.label})
	}
	out := make([]string, max(len(pie), len(legend)))
	for i := range out {
		left := strings.Repeat(" ", size)
		if i < len(pie) {
			left = pie[i]
		}
		right := ""
		if i < len(legend) {
			right = legend[i]
		}
		out[i] = left + "   " + right
	}
	return out
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func sign(x int) int {
	switch {
	case x > 0:
		return 1
	case x < 0:
		return -1
	}
	return 0
}
