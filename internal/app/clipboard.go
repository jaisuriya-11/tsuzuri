package app

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jaisuriya-11/tsuzuri/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

const toastDuration = 1800 * time.Millisecond

// toastExpiredMsg hides toast number id (newer toasts keep their own timer).
type toastExpiredMsg struct{ id int }

// copyText puts text on the system clipboard. Termux uses its native
// clipboard command when available; other terminals use OSC 52.
func (m *Model) copyText(text string) tea.Cmd {
	if err := writeClipboard(text); err != nil {
		termenv.NewOutput(os.Stdout).Copy(text)
	}
	n := utf8.RuneCountInString(text)
	lines := strings.Count(strings.TrimSuffix(text, "\n"), "\n") + 1
	msg := fmt.Sprintf("Copied %d characters", n)
	if lines > 1 {
		msg = fmt.Sprintf("Copied %d lines", lines)
	}
	return m.showToast(msg)
}

func writeClipboard(text string) error {
	// Do not use os/exec.LookPath here. On Android/Termux, the Go runtime's
	// LookPath path can invoke faccessat2, which Android may reject with SIGSYS.
	if strings.HasPrefix(os.Getenv("PREFIX"), "/data/data/com.termux/files/usr") {
		cmd := exec.Command("/data/data/com.termux/files/usr/bin/termux-clipboard-set")
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	// OSC 52 is handled by the terminal and does not require a clipboard
	// utility to be installed.
	termenv.NewOutput(os.Stdout).Copy(text)
	return nil
}

func (m *Model) showToast(text string) tea.Cmd {
	m.toastSeq++
	m.toast = text
	id := m.toastSeq
	return tea.Tick(toastDuration, func(time.Time) tea.Msg { return toastExpiredMsg{id: id} })
}

// renderToast draws the toast in the bottom-left corner, above the statusline.
func (m *Model) renderToast(screen string) string {
	if m.toast == "" {
		return screen
	}
	th := m.theme
	box := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(th.Green).
		BorderBackground(th.DarkerBg).
		Background(th.DarkerBg).
		Render(lipgloss.NewStyle().Background(th.DarkerBg).Foreground(th.Green).Render(" \U000f018f ") +
			lipgloss.NewStyle().Background(th.DarkerBg).Foreground(th.Fg).Render(m.toast+" "))
	h := lipgloss.Height(box)
	return ui.Overlay(screen, box, 1, max(m.height-footerHeight-h, 0))
}
