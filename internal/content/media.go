package content

import (
	"path/filepath"
	"strings"
)

// WithFrontMatter sets key: value in text's leading "---" block, creating
// the block if needed. added is how many lines now come before the text
// that was there, so a cursor can stay on the same line.
func WithFrontMatter(text, key, value string) (out string, added int) {
	eol := "\n"
	if strings.Contains(text, "\r\n") {
		eol = "\r\n"
	}
	lines := strings.Split(text, "\n")
	line := func(i int) string { return strings.TrimSuffix(lines[i], "\r") }
	entry := key + ": " + value
	if len(lines) > 1 && line(0) == "---" {
		end := -1
		for i := 1; i < len(lines); i++ {
			if line(i) == "---" {
				end = i
				break
			}
		}
		if end > 0 {
			cr := strings.TrimSuffix(eol, "\n")
			for i := 1; i < end; i++ {
				if strings.HasPrefix(strings.ToLower(line(i)), key+":") {
					lines[i] = entry + cr
					return strings.Join(lines, "\n"), 0
				}
			}
			lines = append(lines[:end], append([]string{entry + cr}, lines[end:]...)...)
			return strings.Join(lines, "\n"), 1
		}
	}
	return "---" + eol + entry + eol + "---" + eol + text, 3
}

// MediaMarkdown is the Markdown that shows an attached file: an image, or a
// link marked by kind ("video", "audio", anything else is a file). src is
// the original file, link where the note reaches it.
func MediaMarkdown(kind, src, link string) string {
	name := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
	target := link
	if strings.ContainsAny(target, " ()") {
		target = "<" + target + ">"
	}
	switch kind {
	case "image":
		return "![" + name + "](" + target + ")"
	case "video":
		return "[▶ " + name + "](" + target + ")"
	case "audio":
		return "[♪ " + name + "](" + target + ")"
	}
	return "[📎 " + filepath.Base(src) + "](" + target + ")"
}
