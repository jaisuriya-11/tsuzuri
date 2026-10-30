package content

import "testing"

func TestWithFrontMatter(t *testing.T) {
	cases := []struct {
		in, want string
		added    int
	}{
		{"# Note", "---\ncover: a.png\n---\n# Note", 3},
		{"---\ntitle: x\n---\nbody", "---\ntitle: x\ncover: a.png\n---\nbody", 1},
		{"---\nCover: old.png\n---\nbody", "---\ncover: a.png\n---\nbody", 0},
		{"---\r\ntitle: x\r\n---\r\nbody", "---\r\ntitle: x\r\ncover: a.png\r\n---\r\nbody", 1},
		{"# Note\r\n", "---\r\ncover: a.png\r\n---\r\n# Note\r\n", 3},
	}
	for _, c := range cases {
		got, added := WithFrontMatter(c.in, "cover", "a.png")
		if got != c.want || added != c.added {
			t.Errorf("WithFrontMatter(%q) = %q, %d; want %q, %d", c.in, got, added, c.want, c.added)
		}
	}
}

func TestMediaMarkdown(t *testing.T) {
	cases := map[string]string{
		"image": "![my pic](<assets/my pic.png>)",
		"video": "[▶ my pic](<assets/my pic.png>)",
		"file":  "[📎 my pic.png](<assets/my pic.png>)",
	}
	for kind, want := range cases {
		if got := MediaMarkdown(kind, "/tmp/my pic.png", "assets/my pic.png"); got != want {
			t.Errorf("%s: got %q, want %q", kind, got, want)
		}
	}
}
