package app

import (
	"strings"

	"github.com/jaisuriya-11/tsuzuri/internal/note"
	"github.com/jaisuriya-11/tsuzuri/internal/preview"

	tea "github.com/charmbracelet/bubbletea"
)

// Tables

func (m *Model) tableEdited() {
	m.preview.SetContent(m.content.Value())
	m.refreshModified()
}

func (m *Model) tableHit(h preview.Hit) tea.Cmd {
	switch h.Kind {
	case "table:addrow":
		m.content.TableOpAt(h.Line, 0, "addrow")
		m.tableEdited()
	case "table:addcol":
		m.content.TableOpAt(h.Line, h.Index, "addcol")
		m.tableEdited()
	case "table:cell":
		lines := m.docLines()
		header := h.Line+1 < len(lines) && strings.Contains(lines[h.Line+1], "-") &&
			strings.Trim(strings.ReplaceAll(strings.ReplaceAll(lines[h.Line+1], "|", ""), ":", ""), " -") == ""
		edit := "Edit cell"
		if header {
			edit = "Rename column"
		}
		items := []string{edit, "Add row below", "Add column right", "Delete column"}
		toggled, hasBox := note.ToggleBox(h.Arg, 0)
		if hasBox && !header {
			tick := "Tick"
			if strings.Contains(toggled, "[ ]") {
				tick = "Untick"
			}
			items = append([]string{tick}, items...)
		}
		if !header {
			items = append(items, "Delete row")
		}
		label := h.Arg
		if label == "" {
			label = "(empty cell)"
		}
		m.menu(label, items, func(m *Model, choice int) tea.Cmd {
			switch items[choice] {
			case "Tick", "Untick":
				m.content.SetTableCell(h.Line, h.Index, toggled)
			case "Edit cell", "Rename column":
				return m.prompt(edit, h.Arg, "", func(m *Model, v string) tea.Cmd {
					m.content.SetTableCell(h.Line, h.Index, v)
					m.tableEdited()
					return nil
				})
			case "Add row below":
				m.content.TableOpAt(h.Line, h.Index, "addrow")
			case "Add column right":
				m.content.TableOpAt(h.Line, h.Index, "addcol")
			case "Delete column":
				m.content.TableOpAt(h.Line, h.Index, "delcol")
			case "Delete row":
				m.content.TableOpAt(h.Line, h.Index, "delrow")
			}
			m.tableEdited()
			return nil
		})
	}
	return nil
}
