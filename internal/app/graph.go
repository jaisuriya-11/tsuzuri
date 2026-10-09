package app

import (
	"path"
	"path/filepath"

	"github.com/jaisuriya-11/tsuzuri/internal/core"
	"github.com/jaisuriya-11/tsuzuri/internal/preview"

	tea "github.com/charmbracelet/bubbletea"
)

// The graph lives in its own tab. While it is showing, the preview on the
// right shows the note selected in it.

func (m *Model) graphActive() bool { return m.active == graphID }

// openGraph shows the graph tab (":graph", Space g), centred on the note
// that was open.
func (m *Model) openGraph() tea.Cmd {
	centre := m.currentNoteID()
	m.leaderPending = false
	m.graphNotes, m.graphEdges = m.store.LinkGraph()
	m.liveGraph()
	m.graph.SetCentre(centre)
	if i := m.bufferIndex(graphID); i >= 0 {
		m.showBuffer(m.buffers[i])
	} else {
		b := &buffer{id: graphID, title: "Graph"}
		m.insertBuffer(b)
		m.showBuffer(b)
	}
	return m.focusPane(focusEditor)
}

// refreshGraph re-reads every note's links from disk while the graph tab
// is open, then adds the unsaved links of the note being edited.
func (m *Model) refreshGraph() {
	if m.bufferIndex(graphID) < 0 {
		return
	}
	m.graphNotes, m.graphEdges = m.store.LinkGraph()
	m.liveGraph()
	if m.graphActive() {
		m.previewNote(m.graph.Selected())
	}
}

// liveGraph updates the edited note's links from the editor, so links show
// up in the graph as they are typed.
func (m *Model) liveGraph() {
	edges := m.graphEdges
	if id := m.currentNoteID(); id != "" && m.content.HasPage() {
		edges = make([]core.Edge, 0, len(m.graphEdges))
		for _, e := range m.graphEdges {
			if e.From != id {
				edges = append(edges, e)
			}
		}
		for _, to := range core.OutLinks(m.graphNotes, core.Page{ID: id, Content: m.content.Value()}) {
			if to != id {
				edges = append(edges, core.Edge{From: id, To: to})
			}
		}
	}
	m.graph.SetLinks(m.graphNotes, edges)
}

// previewNote shows a note in the preview without opening it.
func (m *Model) previewNote(id string) {
	p, err := m.store.Get(id)
	if err != nil || id == "" {
		m.preview.SetPage(core.Page{})
		m.preview.SetBacklinks(nil)
		return
	}
	live := map[string]string{}
	for _, b := range m.buffers {
		if !b.draft() {
			live[b.id] = b.text
		}
	}
	if text, ok := live[id]; ok {
		p.Content = text
	}
	m.preview.SetBaseDir(filepath.Join(m.store.Root(), filepath.FromSlash(path.Dir(id))))
	m.preview.SetNotes(preview.StoreNotes(m.store, m.notes, id, live))
	m.preview.SetPage(p)
	m.preview.SetBacklinks(m.store.Backlinks(id))
}

// editorPaneUpdate sends input to whatever the editor pane shows.
func (m *Model) editorPaneUpdate(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	if m.graphActive() {
		m.graph, cmd = m.graph.Update(msg)
		return cmd
	}
	m.content, cmd = m.content.Update(msg)
	return cmd
}
