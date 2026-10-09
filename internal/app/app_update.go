package app

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/jaisuriya-11/tsuzuri/internal/core"
	"github.com/jaisuriya-11/tsuzuri/internal/graph"
	"github.com/jaisuriya-11/tsuzuri/internal/preview"

	tea "github.com/charmbracelet/bubbletea"
)

// Update routes events down and handles events emitted up from child components.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.updateLayout()
		return m, nil
	case core.StatusMsg:
		m.status, m.statusErr = msg.Text, msg.Error
		return m, nil
	case tea.KeyMsg:
		m.status = ""
	}

	// Modal layers own all input while open.
	if m.saveAs != nil {
		return m, m.saveAs.update(&m, msg)
	}
	if m.confirm != nil {
		switch msg.(type) {
		case tea.KeyMsg, tea.MouseMsg:
			cmd, _ := m.confirm.update(&m, msg)
			return m, cmd
		}
		return m, nil
	}
	if m.finder != nil {
		return m, m.finder.update(&m, msg)
	}
	if m.themes != nil {
		return m, m.themes.update(&m, msg)
	}
	if m.browser != nil {
		return m, m.browser.update(&m, msg)
	}
	if m.datePick != nil {
		switch msg.(type) {
		case tea.KeyMsg, tea.MouseMsg:
			return m, m.datePick.update(&m, msg)
		}
		return m, nil
	}
	if m.promptBox != nil {
		return m, m.promptBox.update(&m, msg)
	}
	if m.menuBox != nil {
		switch msg.(type) {
		case tea.KeyMsg, tea.MouseMsg:
			return m, m.menuBox.update(&m, msg)
		}
		return m, nil
	}
	if m.showKeymap {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc", "q", "?", "enter", " ", "space":
				m.showKeymap = false
			case "ctrl+c":
				return m, m.requestQuit(false)
			}
		case tea.MouseMsg:
			if msg.Action == tea.MouseActionPress {
				m.showKeymap = false
			}
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case core.PageSelectedMsg:
		if msg.KeepFocus {
			keep := m.focus
			m.openFile(msg.ID)
			return m, m.focusPane(keep)
		}
		return m, m.openFile(msg.ID)
	case core.FindRequestMsg:
		return m, m.openFinder()
	case core.GraphMsg:
		return m, m.openGraph()
	case graph.OpenMsg:
		return m, m.openFile(msg.ID)
	case graph.SelectMsg:
		m.previewNote(msg.ID)
		return m, nil
	case preview.HitMsg:
		return m, m.handleViewHit(msg.Hit)
	case core.FollowLinkMsg:
		return m, m.followEditorLink(msg)
	case preview.MoveBlockMsg:
		if m.activeBuffer() != nil {
			m.setDocLines(moveBlock(m.docLines(), msg.Line, msg.End, msg.To))
		}
		return m, nil
	case preview.AddBlockMsg:
		return m, m.addBlockAfter(msg.After)
	case core.CopyMsg:
		return m, m.copyText(msg.Text)
	case toastExpiredMsg:
		if msg.id == m.toastSeq {
			m.toast = ""
		}
		return m, nil
	case core.SlashActionMsg:
		switch msg.Action {
		case "page":
			parent := m.sidebar.ContextParentID()
			if b := m.activeBuffer(); b != nil && !b.draft() {
				parent = b.id
			}
			return m, m.newDraft(parent)
		case "link":
			return m, m.openLinkPicker()
		case "wikilink":
			cmd := m.openLinkPicker()
			m.finder.wiki = true
			return m, cmd
		}
		if kind, ok := strings.CutPrefix(msg.Action, "media:"); ok {
			return m, m.pickMedia(kind)
		}
		return m, nil
	case mediaPickedMsg:
		switch {
		case msg.unavailable:
			return m, m.openFileBrowser(msg.kind)
		case msg.err != nil:
			m.setError("File dialog failed: " + msg.err.Error())
		case msg.cancelled || msg.path == "":
			m.setStatus("")
		default:
			m.attachMedia(msg.kind, msg.path)
		}
		return m, nil
	case core.ThemeMsg:
		if msg.Name == "" {
			return m, m.openThemePicker()
		}
		m.setThemeByName(msg.Name)
		return m, nil
	case core.PageCreatedMsg:
		return m, m.newDraft(msg.Page.ParentID)
	case core.PageUpdatedMsg:
		m.renamePage(msg.Page.ID, msg.Page.Title)
		return m, nil
	case core.PageDeletedMsg:
		m.confirmDelete(msg.ID)
		return m, nil
	case core.VimSaveMsg:
		return m, m.handleSave(msg)
	case core.VimQuitMsg:
		return m, m.handleQuit(msg)
	case core.VimCloseBufferMsg:
		return m, m.closeBuffer(m.active, msg.Force)
	case core.ExportMsg:
		return m, m.exportPDF(msg.Path)
	case core.VimNewBufferMsg:
		return m, m.newDraft(m.sidebar.ContextParentID())
	case tea.MouseMsg:
		return m, m.handleMouse(msg)
	case tea.KeyMsg:
		return m, m.handleKey(msg)
	}

	// Anything else (cursor blinks etc.) goes to the components that animate.
	var c1, c2 tea.Cmd
	m.sidebar, c1 = m.sidebar.Update(msg)
	m.content, c2 = m.content.Update(msg)
	return m, tea.Batch(c1, c2)
}

func (m *Model) handleSave(msg core.VimSaveMsg) tea.Cmd {
	b := m.activeBuffer()
	if b == nil {
		m.setError("E32: No file name")
		return nil
	}
	if msg.Path != "" {
		dir, name := path.Split(strings.TrimPrefix(filepath.ToSlash(msg.Path), "/"))
		if err := m.saveBufferAs(b, dir, name); err != nil {
			m.setError(err.Error())
		}
		return nil
	}
	return m.saveBuffer(b, nil)
}

func (m *Model) handleQuit(msg core.VimQuitMsg) tea.Cmd {
	if msg.Save {
		if b := m.activeBuffer(); b != nil {
			return m.saveBuffer(b, func(m *Model) tea.Cmd { return m.requestQuit(false) })
		}
	}
	return m.requestQuit(msg.Force)
}

func (m *Model) renamePage(id, title string) {
	m.stashActive()
	updated, err := m.store.Rename(id, title)
	if err != nil {
		m.setError("Rename failed: " + err.Error())
		return
	}
	activeBefore := m.active
	m.remapBuffers(id, updated.ID)
	m.reloadTree()
	m.sidebar.SetSelectedID(updated.ID)
	if b := m.activeBuffer(); b != nil && m.active != activeBefore {
		m.content.SetBuffer(b.page(), false)
		m.preview.SetBaseDir(m.noteDir(b))
		m.preview.SetPage(b.page())
		m.sidebar.SetActiveID(b.id)
	}
	m.setStatus("Renamed to " + updated.ID)
}

func (m *Model) confirmDelete(id string) {
	p, err := m.store.Get(id)
	if err != nil {
		m.setError("Not found: " + id)
		return
	}
	msg := []string{"The file is removed from disk. This cannot be undone."}
	if p.IsFolder {
		msg = []string{"The folder and everything inside it are removed from disk.", "This cannot be undone."}
	} else if sidecar := strings.TrimSuffix(id, ".md"); sidecar != id {
		if info, err := os.Stat(filepath.Join(m.store.Root(), filepath.FromSlash(sidecar))); err == nil && info.IsDir() {
			msg = []string{"The note and all of its sub-notes are removed from disk.", "This cannot be undone."}
		}
	}
	m.confirm = &confirmDialog{
		title:   "Delete " + id + "?",
		message: msg,
		buttons: []string{"Delete", "Cancel"},
		danger:  true,
		onChoose: func(m *Model, choice int) tea.Cmd {
			if choice != 0 {
				return nil
			}
			if err := m.store.Delete(id); err != nil {
				m.setError("Delete failed: " + err.Error())
				return nil
			}
			m.dropBuffersUnder(id)
			m.reloadTree()
			m.refreshModified()
			m.setStatus("Deleted " + id)
			if len(m.buffers) == 0 && len(m.store.List()) == 0 {
				m.goHome()
			}
			return nil
		},
	}
}
