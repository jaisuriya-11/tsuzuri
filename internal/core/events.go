package core

// PageSelectedMsg asks to open a page. KeepFocus leaves keyboard focus where
// it is (a mouse click in the explorer) instead of moving it to the editor.
type PageSelectedMsg struct {
	ID        string
	KeepFocus bool
}

// FindRequestMsg opens the global note finder.
type FindRequestMsg struct{}

// PageCreatedMsg is emitted when a new page is created.
type PageCreatedMsg struct {
	Page Page
}

// PageUpdatedMsg is emitted when a page title or content is updated.
type PageUpdatedMsg struct {
	Page Page
}

// PageDeletedMsg asks for a page (or folder) to be deleted. The app confirms
// with the user before anything is removed from disk.
type PageDeletedMsg struct {
	ID string
}

// VimSaveMsg is emitted by :w. Path is set by ":w <name>", which saves the
// buffer under a new name (Save As).
type VimSaveMsg struct {
	Content string
	Path    string
}

// VimQuitMsg is emitted by :q, :q!, :wq and :x. Force skips the unsaved
// changes prompt.
type VimQuitMsg struct {
	Save  bool
	Force bool
}

// VimCloseBufferMsg is emitted by :bd / :bd!.
type VimCloseBufferMsg struct {
	Force bool
}

// VimNewBufferMsg is emitted by :enew / :new.
type VimNewBufferMsg struct{}

// StatusMsg shows a one-line message in the command line row.
type StatusMsg struct {
	Text  string
	Error bool
}

// SlashActionMsg is emitted by "/" menu entries the app must handle:
// "page" (new sub-note) and "link" (pick a note to link to), and by typing
// "[[" ("wikilink": pick the note to finish the [[link]]).
type SlashActionMsg struct {
	Action string
}

// CopyMsg asks the app to put Text on the system clipboard and confirm it
// with a toast.
type CopyMsg struct {
	Text string
}

// ExportMsg is emitted by ":export [file]": write the open note as a PDF.
// An empty Path saves it next to the note under the note's name.
type ExportMsg struct {
	Path string
}

// GraphMsg opens the graph of notes and links (":graph").
type GraphMsg struct{}

// ThemeMsg switches the colour theme (":colorscheme name"). An empty Name
// opens the theme picker.
type ThemeMsg struct {
	Name string
}

// FollowLinkMsg is emitted by "gd" / "gx" in the editor: open the link on
// Line (the cursor's line) at rune column Col.
type FollowLinkMsg struct {
	Line string
	Col  int
}
