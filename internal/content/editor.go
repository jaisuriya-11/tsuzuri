package content

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/jaisuriya-11/tsuzuri/internal/core"
	"github.com/jaisuriya-11/tsuzuri/internal/textarea"
	"github.com/jaisuriya-11/tsuzuri/internal/theme"
)

// VimMode represents the operational mode of the editor.
type VimMode int

const (
	ModeNormal VimMode = iota
	ModeInsert
	ModeCommand
	ModeVisual
	ModeVisualLine
)

func configureTextareaStyles(ta *textarea.Model, th theme.Theme) {
	ta.FocusedStyle.Base = lipgloss.NewStyle()
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle().Background(th.Bg2).Foreground(th.Fg)
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(th.GreyFg)
	ta.FocusedStyle.Text = lipgloss.NewStyle().Foreground(th.Fg)
	ta.FocusedStyle.LineNumber = lipgloss.NewStyle().Foreground(th.Grey)
	ta.FocusedStyle.CursorLineNumber = lipgloss.NewStyle().Foreground(th.Fg).Bold(true)
	ta.FocusedStyle.EndOfBuffer = lipgloss.NewStyle().Foreground(th.Line)

	ta.BlurredStyle.Base = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle().Foreground(th.Fg)
	ta.BlurredStyle.Placeholder = lipgloss.NewStyle().Foreground(th.Grey)
	ta.BlurredStyle.Text = lipgloss.NewStyle().Foreground(th.Fg)
	ta.BlurredStyle.LineNumber = lipgloss.NewStyle().Foreground(th.Grey)
	ta.BlurredStyle.CursorLineNumber = lipgloss.NewStyle().Foreground(th.GreyFg2)
	ta.BlurredStyle.EndOfBuffer = lipgloss.NewStyle().Foreground(th.Line)
	ta.SelectionStyle = lipgloss.NewStyle().Background(th.OneBg3).Foreground(th.Fg)
}

// parseVimCommand turns an ex command line into a domain message. It returns
// nil for an empty line and a StatusMsg for unknown commands.
func parseVimCommand(cmdStr, content string) any {
	cmdStr = strings.TrimSpace(cmdStr)
	name, arg, _ := strings.Cut(cmdStr, " ")
	arg = strings.TrimSpace(arg)

	switch name {
	case "":
		return nil
	case "w", "write", "sav", "saveas":
		return core.VimSaveMsg{Content: content, Path: arg}
	case "q", "quit", "qa", "qall":
		return core.VimQuitMsg{}
	case "q!", "quit!", "qa!", "qall!":
		return core.VimQuitMsg{Force: true}
	case "wq", "x", "xit", "wqa", "xa":
		return core.VimQuitMsg{Save: true}
	case "bd", "bdelete", "bw", "close":
		return core.VimCloseBufferMsg{}
	case "bd!", "bdelete!", "bw!":
		return core.VimCloseBufferMsg{Force: true}
	case "export", "pdf":
		return core.ExportMsg{Path: arg}
	case "graph":
		return core.GraphMsg{}
	case "colorscheme", "colo", "theme":
		return core.ThemeMsg{Name: arg}
	case "enew", "new", "e", "edit":
		if arg == "" || name == "enew" || name == "new" {
			return core.VimNewBufferMsg{}
		}
	}
	return core.StatusMsg{Text: "E492: Not an editor command: " + cmdStr, Error: true}
}

func createCommandInput(th theme.Theme) textinput.Model {
	ti := textinput.New()
	ti.Prompt = ":"
	ti.CharLimit = 256
	ti.PromptStyle = lipgloss.NewStyle().Foreground(th.Fg).Bold(true)
	ti.TextStyle = lipgloss.NewStyle().Foreground(th.Fg)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(th.Fg)
	return ti
}
