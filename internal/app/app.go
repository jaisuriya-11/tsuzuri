// Package app acts as the root orchestrator component (<App/>).
package app

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/jaisuriya-11/tsuzuri/internal/content"
	"github.com/jaisuriya-11/tsuzuri/internal/core"
	"github.com/jaisuriya-11/tsuzuri/internal/dashboard"
	"github.com/jaisuriya-11/tsuzuri/internal/graph"
	"github.com/jaisuriya-11/tsuzuri/internal/preview"
	"github.com/jaisuriya-11/tsuzuri/internal/sidebar"
	"github.com/jaisuriya-11/tsuzuri/internal/theme"

	tea "github.com/charmbracelet/bubbletea"
)

type viewMode int

const (
	viewModeDashboard viewMode = iota
	viewModeWorkspace
)

type focus int

const (
	focusSidebar focus = iota
	focusEditor
	focusPreview
)

// Model represents the root App container state.
type Model struct {
	store    *core.Store
	theme    theme.Theme
	keys     KeyMap
	focus    focus
	viewMode viewMode

	width  int
	height int

	sidebarOpen   bool
	previewOpen   bool
	showKeymap    bool
	leaderPending bool

	// Open tabs, in order. Unsaved text lives in the buffers, never on disk.
	buffers  []*buffer
	active   string
	draftSeq int

	confirm *confirmDialog
	saveAs  *saveAsDialog
	finder  *finder
	themes  *themePicker
	browser *fileBrowser

	promptBox *promptDialog
	menuBox   *menuDialog
	datePick  *datePicker

	configPath string // where the theme choice is saved ("" = don't save)
	quitting   bool   // "Save All" before quitting is in progress

	status    string
	statusErr bool

	writeClipboard func(string) error
	openURL        func(string) error // opens links outside Tsuzuri; nil = system opener
	notes          []core.Page        // every note, for resolving [[links]]

	toast    string
	toastSeq int
	dragging bool // a mouse drag started in the editor

	split    Split   // dragged divider positions
	resizing divider // the divider being dragged, if any

	dashboard dashboard.Model
	sidebar   sidebar.Model
	content   content.Model
	preview   preview.Model
	graph     graph.Model

	graphNotes []core.Page // the workspace's notes and links, from disk
	graphEdges []core.Edge
}

// Option customises the app at construction.
type Option func(*Model)

// WithTheme starts with the named base46 theme (unknown names are ignored).
func WithTheme(name string) Option {
	return func(m *Model) {
		if th, ok := theme.Get(name); ok {
			m.applyTheme(th, false)
		}
	}
}

// WithClipboard replaces the system clipboard writer (used by tests).
func WithClipboard(write func(string) error) Option {
	return func(m *Model) { m.writeClipboard = write }
}

// WithOpener replaces the system opener for web links and files (used by
// tests).
func WithOpener(open func(string) error) Option {
	return func(m *Model) { m.openURL = open }
}

// WithConfigPath saves theme changes to the given config file.
func WithConfigPath(path string) Option {
	return func(m *Model) { m.configPath = path }
}

// New constructs the root App container for a workspace on disk. Nothing is
// seeded and nothing is opened until the user asks.
func New(store *core.Store, opts ...Option) Model {
	th := theme.DefaultTheme()

	m := Model{
		store:       store,
		theme:       th,
		keys:        DefaultKeyMap(),
		focus:       focusSidebar,
		viewMode:    viewModeDashboard,
		sidebarOpen: true,
		previewOpen: true,
		dashboard:   dashboard.New(th),
		sidebar:     sidebar.New(th),
		content:     content.New(th),
		preview:     preview.New(th),
		graph:       graph.New(th),
	}
	m.sidebar.SetWorkspaceName(filepath.Base(store.Root()))
	m.dashboard.SetWorkspace(tildePath(store.Root()))
	m.reloadTree()
	for _, opt := range opts {
		opt(&m)
	}
	return m
}

func tildePath(p string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" && strings.HasPrefix(p, home) {
		return "~" + strings.TrimPrefix(p, home)
	}
	return p
}

// Init initializes child component commands.
func (m Model) Init() tea.Cmd {
	return m.content.Init()
}

func (m *Model) layout() Layout {
	return CalculateLayout(m.width, m.height, m.sidebarOpen, m.previewOpen, m.split)
}

// updateLayout recalculates dimensions across child panes.
func (m *Model) updateLayout() {
	l := m.layout()
	m.dashboard.SetSize(m.width, max(m.height-footerHeight, 1))
	m.sidebar.SetSize(l.SidebarW, l.BodyH)
	m.content.SetSize(l.EditorW, l.BodyH)
	m.preview.SetSize(l.PreviewW, l.BodyH)
	m.graph.SetSize(l.EditorW, l.BodyH)
	if m.focus == focusPreview && l.PreviewW == 0 {
		m.focusPane(focusEditor)
	}
}

func (m *Model) reloadTree() {
	pages := m.store.List()
	m.sidebar.SetPages(pages)
	m.dashboard.SetRecentPages(pages)
	m.notes = nil
	for _, p := range pages {
		if !p.IsFolder {
			m.notes = append(m.notes, p)
		}
	}
	m.refreshLinks()
	m.refreshGraph()
}

func (m *Model) setStatus(s string) { m.status, m.statusErr = s, false }
func (m *Model) setError(s string)  { m.status, m.statusErr = s, true }

// focusPane moves keyboard focus, opening the target pane if it was hidden.
func (m *Model) focusPane(f focus) tea.Cmd {
	m.viewMode = viewModeWorkspace
	if f == focusSidebar && !m.sidebarOpen {
		m.sidebarOpen = true
		m.updateLayout()
	}
	if f == focusPreview && m.layout().PreviewW == 0 {
		f = focusEditor
	}
	m.focus = f
	m.sidebar.SetFocused(f == focusSidebar)
	m.preview.SetFocused(f == focusPreview)
	m.graph.SetFocused(f == focusEditor)
	if f == focusEditor {
		return m.content.Focus()
	}
	m.content.SetFocused(false)
	return nil
}

// panes lists the visible panes left to right.
func (m *Model) panes() []focus {
	l := m.layout()
	var out []focus
	if l.SidebarW > 0 {
		out = append(out, focusSidebar)
	}
	out = append(out, focusEditor)
	if l.PreviewW > 0 {
		out = append(out, focusPreview)
	}
	return out
}

func (m *Model) movePane(delta int, wrap bool) tea.Cmd {
	ps := m.panes()
	i := 0
	for j, p := range ps {
		if p == m.focus {
			i = j
		}
	}
	i += delta
	if wrap {
		i = (i + len(ps)) % len(ps)
	} else {
		i = max(0, min(i, len(ps)-1))
	}
	return m.focusPane(ps[i])
}

// toggleSidebar is Ctrl+B: show and focus the explorer, or hide it when it
// already has focus. Works from every editor mode.
func (m *Model) toggleSidebar() tea.Cmd {
	if m.sidebarOpen && m.focus == focusSidebar && m.viewMode == viewModeWorkspace {
		m.sidebarOpen = false
		m.updateLayout()
		return m.focusPane(focusEditor)
	}
	if m.content.Mode() == content.ModeInsert {
		m.content.ExitInsert()
	}
	return m.focusPane(focusSidebar)
}

func (m *Model) togglePreview() tea.Cmd {
	m.previewOpen = !m.previewOpen
	m.updateLayout()
	if m.previewOpen && m.layout().PreviewW == 0 {
		m.setStatus("Window too narrow for the preview")
	}
	if m.focus == focusPreview && !m.previewOpen {
		return m.focusPane(focusEditor)
	}
	return nil
}

func (m *Model) startFind() tea.Cmd { return m.openFinder() }

func (m *Model) goHome() {
	m.stashActive()
	m.reloadTree()
	m.viewMode = viewModeDashboard
	m.content.SetFocused(false)
	m.sidebar.SetFocused(false)
}
