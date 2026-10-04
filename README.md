<div align="center">

# Tsuzuri

### ~ 綴り • Terminal Markdown Notebook ~

A block-based notebook for your terminal.
Your notes stay plain Markdown files.

<p>
  <a href="https://github.com/jaisuriya-11/tsuzuri/actions/workflows/ci.yml"><img src="https://github.com/jaisuriya-11/tsuzuri/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/jaisuriya-11/tsuzuri/releases/latest"><img src="https://img.shields.io/github/v/release/jaisuriya-11/tsuzuri" alt="Release"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="MIT License"></a>
</p>

https://github.com/user-attachments/assets/86768fcb-3852-4e6b-8b07-6341bd9b3739

</div>

## Installation

**macOS and Linux**

```sh
curl -fsSL https://raw.githubusercontent.com/jaisuriya-11/tsuzuri/main/install.sh | sh
```

**Windows** (PowerShell)

```powershell
irm https://raw.githubusercontent.com/jaisuriya-11/tsuzuri/main/install.ps1 | iex
```

**Nix flake**
To run flake once:

```sh
nix run jaisuriya-11/tsuzuri

```

To install flake:
Add flake to your flake imports

```nix
inputs = {
    tsuzuri.url = "github:jaisuriya-11/tsuzuri"
};
```

and outputs's inputs

```nix
outputs = {tsuzuri, ...} ...
```

then use in your configuration as `inputs.tsuzuri.packages.${pkgs.stdenv.hostPlatform.system}.default`.

Then add to your packages as

**Manual download:** grab `tsuzuri-macos.tar.gz`, `tsuzuri-linux.tar.gz` or
`tsuzuri-windows.zip` from the
[latest release](https://github.com/jaisuriya-11/tsuzuri/releases/latest) and put
`tsuzuri` on your `PATH`.

**Go:** `go install github.com/jaisuriya-11/tsuzuri/cmd/tsuzuri@latest`

Tsuzuri is a single binary with no dependencies. It runs on macOS (Intel and
Apple Silicon), Windows 10/11 and any x86-64 Linux distribution. On other
CPUs (e.g. Raspberry Pi), install with Go.
A [Nerd Font](https://www.nerdfonts.com/) is recommended for icons.

## Quick start

```sh
cd ~/notes        # any folder; your .md files show up in the explorer
tsuzuri
```

Press `?` inside the app to see every shortcut.

```sh
tsuzuri --dir ~/notes          # open a specific folder
tsuzuri --theme <name>         # pick a theme for this session
tsuzuri --list-themes
```

## Features

- **Plain Markdown files.** No database or lock-in: use git, sync the folder, open notes in any editor.
- **A full workspace.** Start screen, file tree, tabs, statusline and 96 themes.
- **Modal keyboard editing** with mouse support, plus unsaved tabs and Save As.
- **`/` block menu** for headings, to-dos, tables, callouts, code, images and more.
- **Live preview** with syntax highlighting, images and cover banners drawn in the terminal.
- **Boards, calendars, timelines, charts and forms**, stored as text and editable from the preview.
- **Fast search** across file names and note contents.

## Uninstall

Your notes are never touched. Remove the binary the way you installed it:

```sh
rm -f /usr/local/bin/tsuzuri ~/.local/bin/tsuzuri   # macOS / Linux
```

On Windows, delete `%LOCALAPPDATA%\Programs\tsuzuri` and remove it from your
`PATH`. Settings are in `~/.config/tsuzuri` (macOS: `~/Library/Application Support/tsuzuri`,
Windows: `%APPDATA%\tsuzuri`).

## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md).

## Credits

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and
[Lip Gloss](https://github.com/charmbracelet/lipgloss) (the editor is adapted
from [Bubbles](https://github.com/charmbracelet/bubbles)). Theme palettes are
MIT-licensed community palettes, syntax highlighting by
[chroma](https://github.com/alecthomas/chroma).

## License

[MIT](LICENSE)
