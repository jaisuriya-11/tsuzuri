package note

import "testing"

func TestToggleBox(t *testing.T) {
	cases := []struct {
		line string
		nth  int
		want string
		ok   bool
	}{
		{"- [ ] write", 0, "- [x] write", true},
		{"  * [X] done", 0, "  * [ ] done", true},
		{"| [ ] a | [x] b |", 1, "| [ ] a | [ ] b |", true},
		{"| [x](link) | [ ] c |", 0, "| [x](link) | [x] c |", true},
		{"no boxes", 0, "no boxes", false},
		{"- [ ] a", 1, "- [ ] a", false},
	}
	for _, c := range cases {
		got, ok := ToggleBox(c.line, c.nth)
		if got != c.want || ok != c.ok {
			t.Errorf("ToggleBox(%q, %d) = %q, %v; want %q, %v", c.line, c.nth, got, ok, c.want, c.ok)
		}
	}
}
