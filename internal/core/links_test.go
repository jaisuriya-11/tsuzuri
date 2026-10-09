package core

import (
	"os"
	"reflect"
	"testing"
)

func TestParseWikiLink(t *testing.T) {
	cases := map[string]WikiLink{
		"[[Docker]]":                              {Target: "Docker"},
		"[[AWS Notes#S3 Storage]]":                {Target: "AWS Notes", Heading: "S3 Storage"},
		"[[AWS Notes#S3 Storage|Learn about S3]]": {Target: "AWS Notes", Heading: "S3 Storage", Alias: "Learn about S3"},
		"[[AWS Notes#^s3-basics]]":                {Target: "AWS Notes", Block: "s3-basics"},
		"![[AWS Notes]]":                          {Target: "AWS Notes", Embed: true},
		"[[#Setup]]":                              {Heading: "Setup"},
	}
	for in, want := range cases {
		got, n, ok := WikiLinkAt(in)
		if !ok || n != len(in) || got != want {
			t.Errorf("WikiLinkAt(%q) = %+v, %d, %v; want %+v", in, got, n, ok, want)
		}
	}
	if _, _, ok := WikiLinkAt("[[]]"); ok {
		t.Error("empty link should not parse")
	}
	if got := (WikiLink{Target: "dir/AWS", Heading: "S3"}).Label(); got != "AWS › S3" {
		t.Errorf("Label = %q", got)
	}
}

func TestWikiLinksSkipsCode(t *testing.T) {
	text := "see [[A]] and `[[B]]`\n```\n[[C]]\n```\n![[D#^x]]"
	var got []string
	for _, l := range WikiLinks(text) {
		got = append(got, l.Ref())
	}
	if want := []string{"A", "D#^x"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("WikiLinks = %v, want %v", got, want)
	}
}

func TestResolveNote(t *testing.T) {
	notes := []Page{
		{ID: "AWS.md"}, {ID: "work/AWS.md"}, {ID: "work/Docker.md"},
		{ID: "a/b/Deep Note.md"}, {ID: "folder", IsFolder: true},
	}
	cases := []struct{ target, from, want string }{
		{"aws", "", "AWS.md"},
		{"AWS", "work/x.md", "work/AWS.md"}, // same folder wins
		{"work/AWS.md", "", "work/AWS.md"},
		{"docker", "", "work/Docker.md"},
		{"b/deep note", "", "a/b/Deep Note.md"},
		{"Docker", "work/sub/x.md", "work/Docker.md"},
		{"nope", "", ""},
		{"folder", "", ""},
	}
	for _, c := range cases {
		p, ok := ResolveNote(notes, c.target, c.from)
		if p.ID != c.want || ok != (c.want != "") {
			t.Errorf("ResolveNote(%q from %q) = %q, %v; want %q", c.target, c.from, p.ID, ok, c.want)
		}
	}
}

func TestLinkSection(t *testing.T) {
	note := "# AWS Notes\n\nintro\n\n## S3 Storage\n\nS3 stores objects\nin buckets. ^s3-basics\n\n- item one ^li\n\n```\n# not a heading\n```\n\n## EC2\n\nvms\n\nlone para\n^lone"
	cases := []struct {
		link WikiLink
		want string
	}{
		{WikiLink{Heading: "s3-storage"}, "## S3 Storage\n\nS3 stores objects\nin buckets. ^s3-basics\n\n- item one ^li\n\n```\n# not a heading\n```"},
		{WikiLink{Block: "s3-basics"}, "S3 stores objects\nin buckets. ^s3-basics"},
		{WikiLink{Block: "li"}, "- item one ^li"},
		{WikiLink{Block: "lone"}, "lone para"},
	}
	for _, c := range cases {
		got, ok := LinkSection(note, c.link)
		if !ok || got != c.want {
			t.Errorf("LinkSection(%+v) = %q, %v\nwant %q", c.link, got, ok, c.want)
		}
	}
	if _, ok := LinkSection(note, WikiLink{Heading: "Missing"}); ok {
		t.Error("missing heading should not be found")
	}
	if got := LinkLine(note, WikiLink{Heading: "EC2"}); got != 15 {
		t.Errorf("LinkLine(EC2) = %d, want 15", got)
	}
}

func TestBacklinksAndGraph(t *testing.T) {
	s, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []struct{ dir, name, text string }{
		{"", "Docker", "containers"},
		{"", "Study", "Learn about [[docker|containers]] and [[Docker#Volumes]]."},
		{"notes", "Ops", "See [the docs](<../Docker.md>)"},
		{"", "Code", "`[[Docker]]` is not a link"},
		{"", "Self", "[[Self]] [[Missing]]"},
	} {
		if _, err := s.SaveAs(n.dir, n.name, n.text); err != nil {
			t.Fatal(err)
		}
	}
	back := s.Backlinks("Docker.md")
	var ids []string
	for _, b := range back {
		ids = append(ids, b.ID)
	}
	if want := []string{"notes/Ops.md", "Study.md"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("Backlinks = %v, want %v", ids, want)
	}
	if back[1].Line != 1 || back[1].Text == "" {
		t.Errorf("backlink line = %+v", back[1])
	}
	_, edges := s.LinkGraph()
	want := []Edge{{From: "notes/Ops.md", To: "Docker.md"}, {From: "Study.md", To: "Docker.md"}}
	if !reflect.DeepEqual(edges, want) {
		t.Fatalf("LinkGraph edges = %v, want %v", edges, want)
	}
}

func TestFindFile(t *testing.T) {
	s, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveAs("assets", "readme", ""); err != nil {
		t.Fatal(err)
	}
	abs := s.idToAbs("assets/diagram.png")
	if err := writeFile(abs); err != nil {
		t.Fatal(err)
	}
	if got, ok := s.FindFile("diagram.png", "notes/x.md"); !ok || got != abs {
		t.Fatalf("FindFile = %q, %v", got, ok)
	}
	if _, ok := s.FindFile("../etc/passwd", ""); ok {
		t.Fatal("FindFile must stay inside the workspace")
	}
}

func writeFile(abs string) error { return os.WriteFile(abs, []byte("x"), 0o644) }
