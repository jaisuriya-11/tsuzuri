package app

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/jaisuriya-11/tsuzuri/internal/core"

	tea "github.com/charmbracelet/bubbletea"
)

// draftIDPrefix marks a buffer that has never been saved, so it has no file
// on disk (and no explorer entry) yet — VSCode's "Untitled-1".
const draftIDPrefix = "draft:"

func isDraftID(id string) bool { return strings.HasPrefix(id, draftIDPrefix) }

// buffer is one open tab. Unsaved edits live here (not on disk) until the
// user saves, so switching tabs never loses work.
type buffer struct {
	id    string // file ID, or draftIDPrefix+N
	title string // file name without ".md", or "Untitled-N"
	saved string // content as last written to disk ("" for drafts)
	text  string // latest text; for the active tab, the editor is the truth
	dir   string // drafts: default folder offered by Save As
}

func (b *buffer) draft() bool { return isDraftID(b.id) }

// graphID is the graph's tab. It counts as a draft (no file behind it)
// that is never dirty and never saved.
const graphID = draftIDPrefix + "graph"

func (b *buffer) isGraph() bool { return b.id == graphID }

func (b *buffer) fileName() string {
	if b.draft() {
		return b.title
	}
	return path.Base(b.id)
}

func (b *buffer) page() core.Page {
	return core.Page{ID: b.id, Title: b.title, Content: b.text}
}

// noteDir is the absolute folder a buffer's note lives (or will live) in.
func (m *Model) noteDir(b *buffer) string {
	rel := b.dir
	if !b.draft() {
		rel = path.Dir(b.id)
	}
	return filepath.Join(m.store.Root(), filepath.FromSlash(rel))
}

func (m *Model) bufferIndex(id string) int {
	for i, b := range m.buffers {
		if b.id == id {
			return i
		}
	}
	return -1
}

func (m *Model) activeBuffer() *buffer {
	if i := m.bufferIndex(m.active); i >= 0 {
		return m.buffers[i]
	}
	return nil
}

// liveText returns a buffer's current text, reading the editor for the active tab.
func (m *Model) liveText(b *buffer) string {
	if b.id == m.active && m.content.HasPage() {
		return m.content.Value()
	}
	return b.text
}

func (m *Model) isDirty(b *buffer) bool {
	text := m.liveText(b)
	if b.draft() {
		return text != ""
	}
	return text != b.saved
}

func (m *Model) dirtyBuffers() []*buffer {
	var out []*buffer
	for _, b := range m.buffers {
		if m.isDirty(b) {
			out = append(out, b)
		}
	}
	return out
}

// stashActive copies the editor's text into the active buffer before the
// editor is pointed at another one.
func (m *Model) stashActive() {
	if b := m.activeBuffer(); b != nil && m.content.HasPage() {
		b.text = m.content.Value()
	}
}

// refreshModified pushes the set of dirty saved files to the explorer.
func (m *Model) refreshModified() {
	mod := map[string]bool{}
	for _, b := range m.buffers {
		if !b.draft() && m.isDirty(b) {
			mod[b.id] = true
		}
	}
	m.sidebar.SetModified(mod)
}

// showBuffer points every pane at b without changing focus.
func (m *Model) showBuffer(b *buffer) {
	m.stashActive()
	m.active = b.id
	m.content.SetBuffer(b.page(), b.draft())
	m.preview.SetBaseDir(m.noteDir(b))
	m.preview.SetPage(b.page())
	m.refreshLinks()
	if b.isGraph() {
		m.content.SetBuffer(core.Page{}, false)
		m.previewNote(m.graph.Selected())
	}
	m.sidebar.SetActiveID(b.id)
	if !b.draft() {
		m.sidebar.SetSelectedID(b.id)
	}
	m.viewMode = viewModeWorkspace
}

// clearEditor shows the empty state when no tabs are left.
func (m *Model) clearEditor() {
	m.active = ""
	m.content.SetBuffer(core.Page{}, false)
	m.preview.SetPage(core.Page{})
	m.sidebar.SetActiveID("")
}

// openFile opens id in a tab (reusing an existing tab) and focuses the editor.
func (m *Model) openFile(id string) tea.Cmd {
	if i := m.bufferIndex(id); i >= 0 {
		m.showBuffer(m.buffers[i])
		return m.focusPane(focusEditor)
	}
	p, err := m.store.Get(id)
	if err != nil || p.IsFolder {
		m.setError("Cannot open " + id)
		return nil
	}
	b := &buffer{id: p.ID, title: p.Title, saved: p.Content, text: p.Content}
	m.insertBuffer(b)
	m.showBuffer(b)
	return m.focusPane(focusEditor)
}

// insertBuffer adds a tab just after the active one, like VSCode.
func (m *Model) insertBuffer(b *buffer) {
	i := m.bufferIndex(m.active)
	if i < 0 {
		m.buffers = append(m.buffers, b)
		return
	}
	m.buffers = append(m.buffers[:i+1], append([]*buffer{b}, m.buffers[i+1:]...)...)
}

// newDraft opens an empty "Untitled-N" tab. Nothing is written to disk
// until the user saves and picks a location.
func (m *Model) newDraft(parentID string) tea.Cmd {
	m.draftSeq++
	b := &buffer{
		id:    fmt.Sprintf("%s%d", draftIDPrefix, m.draftSeq),
		title: fmt.Sprintf("Untitled-%d", m.draftSeq),
		dir:   m.store.ChildDir(parentID),
	}
	m.insertBuffer(b)
	m.showBuffer(b)
	cmd := m.focusPane(focusEditor)
	return tea.Batch(cmd, m.content.EnterInsert())
}

// cycleBuffer activates the tab delta positions away (wrapping).
func (m *Model) cycleBuffer(delta int) tea.Cmd {
	if len(m.buffers) < 2 {
		return nil
	}
	i := m.bufferIndex(m.active)
	if i < 0 {
		i = 0
	}
	next := (i + delta + len(m.buffers)) % len(m.buffers)
	m.showBuffer(m.buffers[next])
	return nil
}

// removeBuffer drops a tab without any prompt, activating its neighbour.
func (m *Model) removeBuffer(id string) {
	i := m.bufferIndex(id)
	if i < 0 {
		return
	}
	wasActive := id == m.active
	m.buffers = append(m.buffers[:i], m.buffers[i+1:]...)
	if !wasActive {
		return
	}
	m.active = ""
	if len(m.buffers) == 0 {
		m.clearEditor()
		return
	}
	m.showBuffer(m.buffers[min(i, len(m.buffers)-1)])
}

// closeBuffer closes a tab, asking first when it has unsaved changes.
func (m *Model) closeBuffer(id string, force bool) tea.Cmd {
	i := m.bufferIndex(id)
	if i < 0 {
		return nil
	}
	b := m.buffers[i]
	if force || !m.isDirty(b) {
		m.removeBuffer(id)
		m.refreshModified()
		return nil
	}
	m.confirm = &confirmDialog{
		title:   "Unsaved changes",
		message: []string{fmt.Sprintf("Save changes to %s before closing?", b.fileName()), "Your changes will be lost if you don't save them."},
		buttons: []string{"Save", "Don't Save", "Cancel"},
		onChoose: func(m *Model, choice int) tea.Cmd {
			switch choice {
			case 0:
				return m.saveBuffer(b, func(m *Model) tea.Cmd {
					m.removeBuffer(b.id)
					m.refreshModified()
					return nil
				})
			case 1:
				m.removeBuffer(b.id)
				m.refreshModified()
			}
			return nil
		},
	}
	return nil
}

// saveBuffer writes b to disk. A draft has no location yet, so Save As
// opens first; after runs once the file is safely written.
func (m *Model) saveBuffer(b *buffer, after func(*Model) tea.Cmd) tea.Cmd {
	text := m.liveText(b)
	if b.isGraph() {
		return nil
	}
	if b.draft() {
		m.openSaveAs(b, after)
		return nil
	}
	saved, err := m.store.Update(core.Page{ID: b.id, Title: b.title, Content: text})
	if err != nil {
		m.setError("Save failed: " + err.Error())
		m.quitting = false
		return nil
	}
	b.saved, b.text = saved.Content, saved.Content
	m.refreshGraph()
	m.setStatus(fmt.Sprintf("\"%s\" %dL written", b.id, strings.Count(text, "\n")+1))
	m.refreshModified()
	if after != nil {
		return after(m)
	}
	return nil
}

// saveBufferAs writes b's text to dir/name and turns the tab into that file.
func (m *Model) saveBufferAs(b *buffer, dir, name string) error {
	text := m.liveText(b)
	p, err := m.store.SaveAs(dir, name, text)
	if err != nil {
		return err
	}
	wasActive := b.id == m.active
	b.id, b.title, b.saved, b.text, b.dir = p.ID, p.Title, text, text, ""
	if wasActive {
		m.active = b.id
	}
	m.reloadTree()
	if wasActive {
		m.content.SetBuffer(b.page(), false)
		m.preview.SetBaseDir(m.noteDir(b))
		m.preview.SetPage(b.page())
		m.sidebar.SetActiveID(b.id)
		m.sidebar.SetSelectedID(b.id)
	}
	m.setStatus(fmt.Sprintf("\"%s\" written", p.ID))
	m.refreshModified()
	return nil
}

// requestQuit exits, first offering to save any unsaved tabs.
func (m *Model) requestQuit(force bool) tea.Cmd {
	dirty := m.dirtyBuffers()
	if force || len(dirty) == 0 {
		return tea.Quit
	}
	names := make([]string, 0, len(dirty))
	for _, b := range dirty {
		names = append(names, b.fileName())
	}
	list := strings.Join(names, ", ")
	if len(names) > 3 {
		list = strings.Join(names[:3], ", ") + fmt.Sprintf(" and %d more", len(names)-3)
	}
	m.confirm = &confirmDialog{
		title:   "Quit Tsuzuri?",
		message: []string{fmt.Sprintf("%d unsaved: %s", len(dirty), list)},
		buttons: []string{"Save All", "Discard", "Cancel"},
		onChoose: func(m *Model, choice int) tea.Cmd {
			switch choice {
			case 0:
				m.quitting = true
				return m.continueSaveAll()
			case 1:
				return tea.Quit
			}
			return nil
		},
	}
	return nil
}

// continueSaveAll saves every dirty tab, stopping at each draft to ask
// where it should go, then quits.
func (m *Model) continueSaveAll() tea.Cmd {
	for _, b := range m.dirtyBuffers() {
		if b.draft() {
			m.showBuffer(b)
			return m.saveBuffer(b, func(m *Model) tea.Cmd { return m.continueSaveAll() })
		}
		m.saveBuffer(b, nil)
		if !m.quitting {
			return nil // a save failed; stay open
		}
	}
	return tea.Quit
}

// remapBuffers renames tabs after a file or folder moved on disk.
func (m *Model) remapBuffers(oldID, newID string) {
	oldPrefix := strings.TrimSuffix(oldID, ".md") + "/"
	newPrefix := strings.TrimSuffix(newID, ".md") + "/"
	remap := func(id string) string {
		switch {
		case id == oldID:
			return newID
		case strings.HasPrefix(id, oldPrefix):
			return newPrefix + strings.TrimPrefix(id, oldPrefix)
		}
		return id
	}
	m.active = remap(m.active)
	for _, b := range m.buffers {
		if nid := remap(b.id); nid != b.id {
			b.id = nid
			b.title = strings.TrimSuffix(path.Base(nid), ".md")
		}
	}
}

// dropBuffersUnder closes (without prompting) tabs for a deleted file or folder.
func (m *Model) dropBuffersUnder(id string) {
	prefix := strings.TrimSuffix(id, ".md") + "/"
	var gone []string
	for _, b := range m.buffers {
		if b.id == id || strings.HasPrefix(b.id, prefix) {
			gone = append(gone, b.id)
		}
	}
	for _, g := range gone {
		m.removeBuffer(g)
	}
}
