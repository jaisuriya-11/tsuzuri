package content

import (
	"strings"
	"time"

	"github.com/jaisuriya-11/tsuzuri/internal/core"

	tea "github.com/charmbracelet/bubbletea"
)

// slashItem is one entry of the "/" block menu.
type slashItem struct {
	section string
	icon    string
	label   string
	hint    string
	aliases string
	before  string // inserted before the cursor
	after   string // inserted after the cursor
	action  string // "page" / "link" are handled by the app
	block   bool   // starts its own line
	// cell marks entries that also work inside a table cell; cellBefore and
	// cellAfter replace before and after there (an action needs neither).
	cell                  bool
	cellBefore, cellAfter string
}

var slashItems = []slashItem{
	{section: "Table", icon: "\U000f0a15", label: "Add row below", aliases: "row insert", action: "table:addrow"},
	{section: "Table", icon: "\U000f0a12", label: "Add column right", aliases: "column col insert", action: "table:addcol"},
	{section: "Table", icon: "\U000f0a17", label: "Delete row", aliases: "row remove", action: "table:delrow"},
	{section: "Table", icon: "\U000f0a14", label: "Delete column", aliases: "column col remove", action: "table:delcol"},
	{section: "Table", icon: "\U000f04eb", label: "Format table", aliases: "align tidy", action: "table:format"},
	{section: "Basic blocks", icon: "\U000f026b", label: "Heading 1", hint: "#", aliases: "h1 title", before: "# ", block: true},
	{section: "Basic blocks", icon: "\U000f026c", label: "Heading 2", hint: "##", aliases: "h2 subtitle", before: "## ", block: true},
	{section: "Basic blocks", icon: "\U000f026d", label: "Heading 3", hint: "###", aliases: "h3", before: "### ", block: true},
	{section: "Basic blocks", icon: "\U000f026e", label: "Heading 4", hint: "####", aliases: "h4", before: "#### ", block: true},
	{section: "Basic blocks", icon: "\U000f0279", label: "Bulleted list", hint: "-", aliases: "bullet ul unordered", before: "- ", block: true},
	{section: "Basic blocks", icon: "\U000f027b", label: "Numbered list", hint: "1.", aliases: "ol ordered number", before: "1. ", block: true},
	{section: "Basic blocks", icon: "\U000f0135", label: "To-do list", hint: "[]", aliases: "todo task checkbox check", before: "- [ ] ", block: true, cell: true, cellBefore: "[ ] "},
	{section: "Basic blocks", icon: "\U000f0142", label: "Toggle list", hint: ">", aliases: "details collapse", before: "<details>\n<summary>", after: "</summary>\n\n</details>", block: true},
	{section: "Basic blocks", icon: "\U000f0219", label: "Page", aliases: "subpage sub-note note new", action: "page"},
	{section: "Basic blocks", icon: "\U000f02fd", label: "Callout", aliases: "note tip info admonition", before: "> [!NOTE]\n> ", block: true},
	{section: "Basic blocks", icon: "\U000f027e", label: "Quote", hint: "\"", aliases: "blockquote", before: "> ", block: true},
	{section: "Basic blocks", icon: "\U000f04eb", label: "Table", aliases: "grid", before: "| ", after: " | Column 2 |\n| --- | --- |\n|  |  |", block: true},
	{section: "Basic blocks", icon: "\U000f04a0", label: "Equation", hint: "$$", aliases: "math latex tex formula katex", before: "```math\n", after: "\n```", block: true},
	{section: "Basic blocks", icon: "\U000f0374", label: "Divider", hint: "---", aliases: "hr rule line separator", before: "---\n", block: true},
	{section: "Basic blocks", icon: "\U000f0bcc", label: "2 columns", hint: "+++", aliases: "columns cols layout side split", before: "~~~columns\n", after: "\n+++\nColumn 2\n~~~", block: true},
	{section: "Basic blocks", icon: "\U000f056d", label: "3 columns", hint: "+++", aliases: "columns cols layout side split", before: "~~~columns\n", after: "\n+++\nColumn 2\n+++\nColumn 3\n~~~", block: true},
	{section: "Basic blocks", icon: "\U000f0728", label: "4 columns", hint: "+++", aliases: "columns cols layout side split", before: "~~~columns\n", after: "\n+++\nColumn 2\n+++\nColumn 3\n+++\nColumn 4\n~~~", block: true},
	{section: "Basic blocks", icon: "\U000f0337", label: "Link to page", aliases: "mention reference note", action: "link", cell: true},
	{section: "Views", icon: "\U000f0564", label: "Board", hint: "kanban", aliases: "kanban status tasks", before: "```board\n## Not started\n- ", after: "\n## In progress\n- Card 2\n  Lines under a card are its description\n## Done\n- Card 3\n```", block: true},
	{section: "Views", icon: "\U000f00ed", label: "Calendar", aliases: "month events dates", before: "```calendar\nmonth: " + time.Now().Format("2006-01") + "\n" + time.Now().Format("2006-01-02") + ": ", after: "\n```", block: true},
	{section: "Views", icon: "\U000f0b77", label: "Timeline", hint: "gantt", aliases: "gantt schedule roadmap", before: "```timeline\n", after: "Card 1: " + time.Now().AddDate(0, 0, -3).Format("2006-01-02") + " -> " + time.Now().AddDate(0, 0, 2).Format("2006-01-02") + "\nCard 2: " + time.Now().Format("2006-01-02") + " -> " + time.Now().AddDate(0, 0, 4).Format("2006-01-02") + "\n```", block: true},
	{section: "Views", icon: "\U000f0128", label: "Chart", hint: "bar line pie", aliases: "graph plot bar line pie donut hbar", before: "```chart\ntype: bar\ntitle: ", after: "\nJan: 12\nFeb: 20\nMar: 7\nApr: 15\n```", block: true},
	{section: "Views", icon: "\U000f0b4b", label: "Form", aliases: "survey questions questionnaire", before: "```form\ntitle: ", after: "\ndescription: Optional description\n? Question 1\n? Question 2 (choice): Option 1 | Option 2 | Option 3\n```", block: true},
	{section: "Views", icon: "\U000f04aa", label: "Flowchart", hint: "-->", aliases: "flow diagram graph mermaid excalidraw chart process", before: "```flow\nstart([Start]) --> ", after: "check{Is it working?}\ncheck -->|yes| done([Ship it])\ncheck -->|no| fix[Fix it]\nfix --> check\n```", block: true},
	{section: "Media", icon: "\U000f0e7c", label: "Cover", hint: "banner", aliases: "banner header hero", action: "media:cover"},
	{section: "Media", icon: "\U000f02e9", label: "Image", aliases: "picture photo img upload", action: "media:image", cell: true},
	{section: "Media", icon: "\U000f0567", label: "Video", aliases: "movie upload", action: "media:video", cell: true},
	{section: "Media", icon: "\U000f075a", label: "Audio", aliases: "sound music upload", action: "media:audio", cell: true},
	{section: "Media", icon: "\U000f0214", label: "File", aliases: "attachment pdf upload document", action: "media:file", cell: true},
	{section: "Media", icon: "\U000f0169", label: "Code", hint: "```", aliases: "snippet block pre", before: "```\n", after: "\n```", block: true, cell: true, cellBefore: "`", cellAfter: "`"},
}

// Block is a "/" menu entry as other editors (the VS Code extension) offer
// it: Before and After surround the cursor, or Action names what the editor
// does instead ("page", "link", "media:image", "table:addrow" …). Table
// entries only apply inside a table, and there only entries marked Cell.
type Block struct {
	Section string `json:"section"`
	Label   string `json:"label"`
	Hint    string `json:"hint,omitempty"`
	Aliases string `json:"aliases,omitempty"`
	Before  string `json:"before,omitempty"`
	After   string `json:"after,omitempty"`
	Action  string `json:"action,omitempty"`
	Block   bool   `json:"block,omitempty"`
	// Cell marks entries that work inside a table cell, where CellBefore
	// and CellAfter are inserted instead of Before and After.
	Cell       bool   `json:"cell,omitempty"`
	CellBefore string `json:"cellBefore,omitempty"`
	CellAfter  string `json:"cellAfter,omitempty"`
}

// Blocks returns the "/" menu in display order.
func Blocks() []Block {
	out := make([]Block, len(slashItems))
	for i, it := range slashItems {
		out[i] = Block{
			Section:    it.section,
			Label:      it.label,
			Hint:       it.hint,
			Aliases:    it.aliases,
			Before:     it.before,
			After:      it.after,
			Action:     it.action,
			Block:      it.block,
			Cell:       it.cell,
			CellBefore: it.cellBefore,
			CellAfter:  it.cellAfter,
		}
	}
	return out
}

const (
	slashWidth   = 36
	slashMaxRows = 12
)

type slashMenu struct {
	row, col int // position of the "/" that opened the menu
	sel      int
	offset   int
}

// SlashOpen reports whether the "/" block menu is showing.
func (m Model) SlashOpen() bool { return m.slash != nil }

func (m *Model) slashQuery() (string, bool) {
	row, col := m.textarea.RowCol()
	if m.slash == nil || row != m.slash.row || col <= m.slash.col {
		return "", false
	}
	before := []rune(m.textarea.LineBeforeCursor())
	if m.slash.col >= len(before) || before[m.slash.col] != '/' {
		return "", false
	}
	q := string(before[m.slash.col+1:])
	if strings.Contains(q, "  ") || strings.HasPrefix(q, " ") {
		return "", false
	}
	return q, true
}

func (m *Model) slashMatches() []slashItem {
	q, _ := m.slashQuery()
	q = strings.ToLower(strings.TrimSpace(q))
	inTable := m.InTable()
	var out []slashItem
	for _, it := range slashItems {
		// Inside a table only what fits in a cell is offered.
		if (it.section == "Table") != inTable && !(inTable && it.cell) {
			continue
		}
		if q == "" || strings.Contains(strings.ToLower(it.label+" "+it.aliases), q) {
			out = append(out, it)
		}
	}
	return out
}

// maybeOpenSlash opens the menu right after a "/" typed at the start of a
// line or after whitespace.
func (m *Model) maybeOpenSlash(k tea.KeyMsg) {
	if k.Type != tea.KeyRunes || string(k.Runes) != "/" {
		return
	}
	before := []rune(m.textarea.LineBeforeCursor())
	n := len(before)
	if n == 0 || before[n-1] != '/' {
		return
	}
	if n >= 2 && before[n-2] != ' ' {
		return
	}
	row, _ := m.textarea.RowCol()
	m.slash = &slashMenu{row: row, col: n - 1}
}

// updateSlash handles a key while the menu is open. handled=false lets the
// key reach the editor (typing filters the menu).
func (m *Model) updateSlash(k tea.KeyMsg) (tea.Cmd, bool) {
	items := m.slashMatches()
	switch k.String() {
	case "esc":
		m.slash = nil
		return nil, true
	case "up", "ctrl+p", "ctrl+k":
		if len(items) > 0 {
			m.slash.sel = (m.slash.sel - 1 + len(items)) % len(items)
			m.scrollSlash()
		}
		return nil, true
	case "down", "ctrl+n", "ctrl+j":
		if len(items) > 0 {
			m.slash.sel = (m.slash.sel + 1) % len(items)
			m.scrollSlash()
		}
		return nil, true
	case "enter", "tab":
		if len(items) == 0 {
			m.slash = nil
			return nil, k.String() == "tab"
		}
		return m.applySlash(items[min(m.slash.sel, len(items)-1)]), true
	}
	return nil, false
}

func (m *Model) scrollSlash() {
	s := m.slash
	if s.sel < s.offset {
		s.offset = s.sel
	}
	if s.sel >= s.offset+slashMaxRows {
		s.offset = s.sel - slashMaxRows + 1
	}
}

// afterSlashKey re-validates the menu once the editor consumed a key.
func (m *Model) afterSlashKey() {
	if m.slash == nil {
		return
	}
	if _, ok := m.slashQuery(); !ok {
		m.slash = nil
		return
	}
	if q, _ := m.slashQuery(); strings.HasSuffix(q, " ") && len(m.slashMatches()) == 0 {
		m.slash = nil
		return
	}
	m.slash.sel, m.slash.offset = 0, 0
}

// applySlash replaces "/query" with the chosen block's Markdown.
func (m *Model) applySlash(it slashItem) tea.Cmd {
	q, _ := m.slashQuery()
	ta := &m.textarea
	ta.DeleteBefore(1 + len([]rune(q)))
	m.slash = nil

	if op, ok := strings.CutPrefix(it.action, "table:"); ok {
		m.tableOp(op)
		return nil
	}
	if it.action != "" {
		action := it.action
		return func() tea.Msg { return core.SlashActionMsg{Action: action} }
	}
	before, after := it.before, it.after
	if it.cell && m.InTable() {
		before, after = it.cellBefore, it.cellAfter
	} else if it.block && strings.TrimSpace(ta.LineBeforeCursor()) != "" {
		ta.InsertString("\n")
	}
	ta.InsertString(before)
	row, col := ta.RowCol()
	if after != "" {
		ta.InsertString(after)
		ta.SetRowCol(row, col)
	}
	ta.EnsureVisible()
	return nil
}

// SetFrontMatter sets key: value in the note's leading "---" block,
// creating the block if needed, and keeps the cursor on the same text.
func (m *Model) SetFrontMatter(key, value string) {
	row, col := m.textarea.RowCol()
	text, added := WithFrontMatter(m.textarea.Value(), key, value)
	m.textarea.SetValue(text)
	m.textarea.SetRowCol(row+added, col)
}

// InsertText types text at the cursor (used for "Link to page").
func (m *Model) InsertText(s string) {
	m.textarea.InsertString(s)
	m.textarea.EnsureVisible()
}
