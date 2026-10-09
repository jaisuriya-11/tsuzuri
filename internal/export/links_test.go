package export

import (
	"strings"
	"testing"

	"github.com/jaisuriya-11/tsuzuri/internal/preview"
)

func TestPrintableExpandsEmbeds(t *testing.T) {
	notes := map[string]string{
		"AWS":  "# AWS\n\nS3 stores objects. ^s3\n\n## EC2\n\nvms",
		"Loop": "![[Loop]]",
	}
	lookup := func(target string) (preview.Note, bool) {
		if target == "pic.png" {
			return preview.Note{File: "/tmp/my pic.png"}, true
		}
		text, ok := notes[target]
		return preview.Note{Title: target, Content: text}, ok
	}
	in := "intro ^top\n\n![[AWS#^s3]]\n\n![[AWS#EC2]]\n\n![[pic.png]]\n\n![[Missing]]\n\n![[Loop]]\n\n```\n![[AWS]]\n```"
	got := Printable(in, lookup)
	for _, want := range []string{"intro\n", "S3 stores objects.\n", "## EC2\n\nvms", "![](</tmp/my pic.png>)", "> Missing", "```\n![[AWS]]\n```"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "^s3") || strings.Contains(got, "^top") {
		t.Errorf("block ids should be removed:\n%s", got)
	}
}

func TestWikiLinksPrintAsTheirLabel(t *testing.T) {
	got := plain("See [[AWS Notes#S3|Learn S3]] and [[Docker]].")
	if got != "See Learn S3 and Docker." {
		t.Fatalf("plain = %q", got)
	}
}
