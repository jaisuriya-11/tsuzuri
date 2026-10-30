package note

import "regexp"

// box is a to-do checkbox, "[ ]" or "[x]", anywhere on a line: at the start
// of a list item or inside a table cell.
var box = regexp.MustCompile(`\[[ xX]\]`)

// Boxes returns where each checkbox on line starts. "[x](url)" is a link,
// not a box.
func Boxes(line string) []int {
	var at []int
	for _, m := range box.FindAllStringIndex(line, -1) {
		if m[1] < len(line) && line[m[1]] == '(' {
			continue
		}
		at = append(at, m[0])
	}
	return at
}

// ToggleBox ticks or unticks the nth checkbox on line (from 0). ok is
// false when the line has no such box.
func ToggleBox(line string, nth int) (string, bool) {
	at := Boxes(line)
	if nth < 0 || nth >= len(at) {
		return line, false
	}
	i := at[nth] + 1
	mark := "x"
	if line[i] != ' ' {
		mark = " "
	}
	return line[:i] + mark + line[i+1:], true
}
