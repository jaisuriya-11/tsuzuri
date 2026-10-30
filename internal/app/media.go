package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jaisuriya-11/tsuzuri/internal/content"
)

// mediaKind describes what the "/" menu asked to attach.
type mediaKind struct {
	name  string   // image, video, audio, file
	title string   // dialog title
	exts  []string // allowed extensions (empty = any file)
	uti   string   // macOS uniform type for the native dialog
}

var mediaKinds = map[string]mediaKind{
	"image": {"image", "Choose an image", []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".bmp", ".heic", ".avif"}, "public.image"},
	"video": {"video", "Choose a video", []string{".mp4", ".mov", ".webm", ".mkv", ".avi", ".m4v"}, "public.movie"},
	"audio": {"audio", "Choose an audio file", []string{".mp3", ".wav", ".m4a", ".ogg", ".flac", ".aac"}, "public.audio"},
	"file":  {"file", "Choose a file", nil, ""},
	"cover": {"cover", "Choose a cover image", []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp"}, "public.image"},
}

func (k mediaKind) accepts(name string) bool {
	if len(k.exts) == 0 {
		return true
	}
	ext := strings.ToLower(filepath.Ext(name))
	for _, e := range k.exts {
		if e == ext {
			return true
		}
	}
	return false
}

// mediaPickedMsg reports the result of the native file dialog.
type mediaPickedMsg struct {
	kind        string
	path        string
	cancelled   bool
	unavailable bool // no GUI dialog could be shown: use the built-in browser
	err         error
}

var errNoDialog = errors.New("no native file dialog")

// pickMedia opens the operating system's file dialog in the background; if
// none is available (SSH, no desktop, TSUZURI_NATIVE_PICKER=0) the built-in
// browser takes over.
func (m *Model) pickMedia(kind string) tea.Cmd {
	k, ok := mediaKinds[kind]
	if !ok {
		return nil
	}
	if os.Getenv("TSUZURI_NATIVE_PICKER") == "0" {
		return m.openFileBrowser(kind)
	}
	m.setStatus(k.title + "… (a file dialog opened)")
	return func() tea.Msg {
		p, cancelled, err := nativeFileDialog(k)
		if errors.Is(err, errNoDialog) {
			return mediaPickedMsg{kind: kind, unavailable: true}
		}
		return mediaPickedMsg{kind: kind, path: p, cancelled: cancelled, err: err}
	}
}

func nativeFileDialog(k mediaKind) (string, bool, error) {
	switch runtime.GOOS {
	case "darwin":
		if os.Getenv("SSH_CONNECTION") != "" {
			return "", false, errNoDialog
		}
		script := fmt.Sprintf(`POSIX path of (choose file with prompt %q`, k.title)
		if k.uti != "" {
			script += fmt.Sprintf(` of type {%q}`, k.uti)
		}
		script += ")"
		out, err := exec.Command("osascript", "-e", script).Output()
		if err != nil {
			var ee *exec.ExitError
			if errors.As(err, &ee) && strings.Contains(string(ee.Stderr), "-128") {
				return "", true, nil // user pressed Cancel
			}
			return "", false, errNoDialog
		}
		return strings.TrimSpace(string(out)), false, nil

	case "linux", "freebsd", "openbsd":
		if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
			return "", false, errNoDialog
		}
		var cmd *exec.Cmd
		if bin, err := exec.LookPath("zenity"); err == nil {
			args := []string{"--file-selection", "--title=" + k.title}
			if len(k.exts) > 0 {
				args = append(args, "--file-filter="+k.name+" | *"+strings.Join(k.exts, " *"))
			}
			cmd = exec.Command(bin, args...)
		} else if bin, err := exec.LookPath("kdialog"); err == nil {
			filter := ""
			if len(k.exts) > 0 {
				filter = "*" + strings.Join(k.exts, " *")
			}
			cmd = exec.Command(bin, "--getopenfilename", ".", filter, "--title", k.title)
		} else {
			return "", false, errNoDialog
		}
		out, err := cmd.Output()
		if err != nil {
			var ee *exec.ExitError
			if errors.As(err, &ee) && ee.ExitCode() == 1 {
				return "", true, nil
			}
			return "", false, errNoDialog
		}
		return strings.TrimSpace(string(out)), false, nil

	case "windows":
		filter := "All files|*.*"
		if len(k.exts) > 0 {
			filter = k.name + "|*" + strings.Join(k.exts, ";*")
		}
		ps := fmt.Sprintf(`Add-Type -AssemblyName System.Windows.Forms; $d = New-Object System.Windows.Forms.OpenFileDialog; $d.Title = '%s'; $d.Filter = '%s'; if ($d.ShowDialog() -eq 'OK') { $d.FileName }`, k.title, filter)
		out, err := exec.Command("powershell", "-NoProfile", "-STA", "-Command", ps).Output()
		if err != nil {
			return "", false, errNoDialog
		}
		p := strings.TrimSpace(string(out))
		return p, p == "", nil
	}
	return "", false, errNoDialog
}

// attachMedia copies the picked file next to the note and links it.
func (m *Model) attachMedia(kind, src string) {
	b := m.activeBuffer()
	if b == nil {
		return
	}
	noteDir := b.dir
	if !b.draft() {
		noteDir = path.Dir(b.id)
		if noteDir == "." {
			noteDir = ""
		}
	}
	link, err := m.store.AttachFile(noteDir, src)
	if err != nil {
		m.setError("Could not attach file: " + err.Error())
		return
	}
	if kind == "cover" {
		m.content.SetFrontMatter("cover", link)
		m.preview.SetContent(m.content.Value())
		m.refreshModified()
		m.setStatus("Cover set to " + link)
		return
	}
	md := content.MediaMarkdown(kind, src, link)
	m.content.InsertText(md)
	m.preview.SetContent(m.content.Value())
	m.refreshModified()
	if b.draft() {
		m.setStatus("Attached " + link + " — saved with the note's folder once you save")
	} else {
		m.setStatus("Attached " + link)
	}
}
