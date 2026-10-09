// Command tsuzuri is the entry point for the Tsuzuri terminal notebook application.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/jaisuriya-11/tsuzuri/internal/app"
	"github.com/jaisuriya-11/tsuzuri/internal/config"
	"github.com/jaisuriya-11/tsuzuri/internal/core"
	"github.com/jaisuriya-11/tsuzuri/internal/export"
	"github.com/jaisuriya-11/tsuzuri/internal/preview"
	"github.com/jaisuriya-11/tsuzuri/internal/theme"

	tea "github.com/charmbracelet/bubbletea"
)

// version is stamped at build time via -ldflags "-X main.version=...".
// GoReleaser sets this automatically for tagged release builds.
var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "export" {
		os.Exit(runExport(os.Args[2:]))
	}

	var (
		dir        string
		themeName  string
		themesDir  string
		showVer    bool
		listThemes bool
	)
	flag.StringVar(&dir, "dir", "", "workspace directory to open (defaults to $TSUZURI_WORKSPACE, then the current directory)")
	flag.BoolVar(&showVer, "version", false, "print the Tsuzuri version and exit")
	flag.StringVar(&themeName, "theme", "", "colour theme for this session (see --list-themes)")
	flag.StringVar(&themesDir, "themes-dir", "", "directory or .json file with custom themes (overrides config themes_dir)")
	flag.BoolVar(&listThemes, "list-themes", false, "list the available colour themes and exit")
	flag.Parse()

	cfgPath := config.DefaultPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ignoring unreadable config %s: %v\n", cfgPath, err)
	}
	// Custom themes location must be registered before any theme lookup.
	if themesDir == "" {
		themesDir = cfg.ThemesDir
	}
	if themesDir != "" {
		theme.SetUserThemesPath(themesDir)
		if _, err := os.Stat(theme.UserThemesDir()); err != nil {
			fmt.Fprintf(os.Stderr, "Custom themes path %s not found; using bundled themes only\n", theme.UserThemesDir())
		}
	}

	if showVer {
		fmt.Printf("tsuzuri %s\n", version)
		return
	}
	if listThemes {
		for _, n := range theme.Names() {
			fmt.Println(n)
		}
		return
	}

	if themeName == "" {
		themeName = cfg.Theme
	}
	if themeName != "" {
		if _, ok := theme.Get(themeName); !ok {
			fmt.Fprintf(os.Stderr, "Unknown theme %q (run tsuzuri --list-themes)\n", themeName)
			os.Exit(2)
		}
	}

	workspace := resolveWorkspace(dir)

	store, err := core.NewStore(workspace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not open workspace %q: %v\n", workspace, err)
		os.Exit(1)
	}

	logPath := logFilePath()
	f, err := tea.LogToFile(logPath, "debug")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not open log file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	log.Printf("Starting Tsuzuri (workspace: %s)\n", store.Root())

	appModel := app.New(store, app.WithTheme(themeName), app.WithConfigPath(cfgPath))

	p := tea.NewProgram(appModel, tea.WithAltScreen(), tea.WithMouseAllMotion())
	if _, err := p.Run(); err != nil {
		log.Printf("Fatal error running program: %v\n", err)
		fmt.Fprintf(os.Stderr, "Error running Tsuzuri application: %v\n", err)
		os.Exit(1)
	}

	log.Println("Tsuzuri exited cleanly.")
}

// resolveWorkspace picks the notes directory to open: an explicit --dir flag
// wins, then $TSUZURI_WORKSPACE, falling back to the current directory.
func resolveWorkspace(flagDir string) string {
	if flagDir != "" {
		return flagDir
	}
	if envDir := os.Getenv("TSUZURI_WORKSPACE"); envDir != "" {
		return envDir
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

// logFilePath keeps the debug log out of the user's notes workspace,
// writing it to the OS user cache directory instead.
func logFilePath() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "tsuzuri.log"
	}
	dir := filepath.Join(cacheDir, "tsuzuri")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "tsuzuri.log"
	}
	return filepath.Join(dir, "tsuzuri.log")
}

// runExport handles "tsuzuri export note.md [-o out.pdf]".
func runExport(args []string) int {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	out := fs.String("o", "", "output file or folder (defaults to the note's name with .pdf, next to it)")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: tsuzuri export <note.md> [-o out.pdf]")
		fs.PrintDefaults()
	}
	// Allow the flag after the note as well as before it.
	var notes []string
	for len(args) > 0 {
		if err := fs.Parse(args); err != nil {
			return 2
		}
		if fs.NArg() == 0 {
			break
		}
		notes = append(notes, fs.Arg(0))
		args = fs.Args()[1:]
	}
	if len(notes) != 1 {
		fs.Usage()
		return 2
	}
	note := notes[0]
	data, err := os.ReadFile(note)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not read %s: %v\n", note, err)
		return 1
	}
	dir := filepath.Dir(note)
	name := strings.TrimSuffix(filepath.Base(note), filepath.Ext(note))
	target := *out
	if target != "" && !filepath.IsAbs(target) {
		// -o is relative to where the command runs, not to the note.
		if cwd, err := os.Getwd(); err == nil {
			folder := strings.HasSuffix(target, "/") || strings.HasSuffix(target, string(filepath.Separator))
			target = filepath.Join(cwd, target)
			if folder {
				target += string(filepath.Separator)
			}
		}
	}
	dest := export.Target(target, dir, name)
	text := string(data)
	// ![[embeds]] resolve against the notes in the note's folder.
	if store, err := core.NewStore(dir); err == nil {
		notes := preview.StoreNotes(store, nil, filepath.Base(note), nil)
		text = export.Printable(text, func(t string) (preview.Note, bool) {
			if t == "" {
				return preview.Note{Title: name, Content: string(data), Dir: dir}, true
			}
			return notes(t)
		})
	}
	if err := export.WriteFile(text, export.Options{Title: name, BaseDir: dir}, dest); err != nil {
		fmt.Fprintf(os.Stderr, "Export failed: %v\n", err)
		return 1
	}
	fmt.Println(dest)
	return 0
}
