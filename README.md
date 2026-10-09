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




https://github.com/user-attachments/assets/edccf6de-940e-43ad-a4c2-cf73a77d7a37


</div>

## Installation & Update

**macOS and Linux**

```sh
curl -fsSL https://raw.githubusercontent.com/jaisuriya-11/tsuzuri/main/install.sh | sh
```

**Windows** (PowerShell)

```powershell
irm https://raw.githubusercontent.com/jaisuriya-11/tsuzuri/main/install.ps1 | iex
```

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
tsuzuri --themes-dir <path>    # custom themes folder or single .json file
tsuzuri export note.md         # write note.pdf next to the note (-o to pick the file)
```

## Features

- **Plain Markdown files.** No database or lock-in: use git, sync the folder, open notes in any editor.
- **A full workspace.** Start screen, file tree, tabs, statusline and 97 themes.
- **Modal keyboard editing** with mouse support, plus unsaved tabs and Save As.
- **`/` block menu** for headings, to-dos, tables, callouts, code, images and more.
- **Live preview** laid out like a page, with syntax highlighting, images and cover banners drawn in the terminal.
- **Drag blocks to reorder them** in the preview by their `⠿` handle, or add one below with `+`, like Notion.
- **Boards, calendars, timelines, charts and forms**, stored as text and editable from the preview.
- **Flowcharts** from Mermaid syntax (`graph TD` / `graph LR`), drawn with real shapes: boxes, decisions, circles, databases.
- **Equations** in LaTeX (`/equation`, a `math` block or `$$…$$`), shown as Unicode: `x^2` → `x²`, `\alpha` → `α`.
- **Export to PDF** with `:export`: a clean printable document, with boards, charts and flowcharts included.
- **2, 3 and 4 column layouts** from the `/` menu.
- **Resizable panes**: drag the dividers between the explorer, editor and preview.
- **Fast search** across file names and note contents.
- **Links between notes** with `[[Note]]`: jump to headings and blocks, embed one note in another, and see what links back.
- **Graph view** (`:graph`): a zoomable map of how your notes link together, with a live preview beside it.

## Links

Type `[[` to link to another note. Click a link in the preview, or press
`gd` on it in the editor, to follow it.

```md
[[Docker]]                          open a note
[[AWS Notes#S3 Storage]]            open it at a heading
[[AWS Notes#S3 Storage|Learn S3]]   show your own text
[[AWS Notes#^s3-basics]]            open it at a block marked ^s3-basics
![[AWS Notes#^s3-basics]]           show that block right here
![[diagram.png]]                    show an image from anywhere in the workspace
```

Mark a paragraph or list item as a block by ending it with ` ^some-id`.
A link to a note that doesn't exist yet is dimmed; following it starts that
note, and nothing is written until you save. Every note lists the notes that
link to it at the bottom of the preview. Web links (`[text](https://…)` and
`<https://…>`) open in your browser.

### Graph

`:graph` (or `Space g`) opens the graph in a tab, centred on the note you
were in: notes that link to it on the left, the notes it links to on the
right. The preview on the right shows whichever note is selected.

```
                              ╭─● Untitled-1
           Other ●─╮          │            ╭─● EC2
                   ├─● Note ──┼─● AWS ─────┤
 Index ●─── Study ●─╯          │            ╰─● S3
                              ╰─● Docker
```

The graph is used with the mouse:

- Click a note to preview it; click it again to open it.
- Scroll to zoom out and in: zoomed out, the map also shows the links of
  those notes, up to four steps away. The `−` and `+` buttons do the same.
- Drag to move around the map; `reset` puts it back.
- `centre` re-centres the map on the selected note; `back` returns to the
  previous one.

Close the graph like any tab.
It picks up links as you type them in other tabs.

## Flowcharts

Write a `flow` (or `mermaid`) code block and the preview draws it:

````md
```flow
graph LR
start([Start]) --> check{Is it working?}
check -->|yes| done([Ship it])
check -->|no| fix[Fix it]
fix --> check
```
````

Shapes: `[box]`, `(rounded)`, `([stadium])`, `{decision}`, `{{hexagon}}`, `((circle))`, `[(database)]`, `[[subroutine]]`.
Arrows: `-->`, `-.->` (dotted), `==>` (thick), with labels as `-->|yes|` or `-- yes -->`.

## Equations

Type `/equation`, or write a `math` block or `$$ … $$`. Each line is one equation:

````md
```math
x = \frac{-b \pm \sqrt{b^2 - 4ac}}{2a}
E = mc^2
```
````

The preview shows `x = (−b ± √(b² − 4ac))/(2a)` and `E = mc²`. Greek letters, operators, arrows, `^` / `_`, `\frac`, `\sqrt`, `\mathbb` and accents are supported. Unicode has no superscript for some letters, so those print as `^(…)`.

## Export to PDF

In the editor, `:export` writes the open note as a PDF next to it. `:export ~/Desktop/` or `:export report.pdf` picks another place. From the shell:

```sh
tsuzuri export notes/plan.md               # notes/plan.pdf
tsuzuri export notes/plan.md -o plan.pdf
```

The PDF is a printable A4 document with headings, lists, tables, syntax-highlighted code, equations and images. Boards, calendars, timelines, charts and flowcharts print in colour as they look in the preview.

## Columns

Type `/2 columns` (or 3, 4). Each `+++` line starts a new column, and each column is ordinary Markdown:

```md
~~~columns
## Todo
- [ ] Write the docs
+++
## Notes
Columns stack on narrow screens.
~~~
```

## Custom themes

Drop a JSON palette into the themes folder in your user config directory:

- **Linux:** `~/.config/tsuzuri/themes/<name>.json`
- **macOS:** `~/Library/Application Support/tsuzuri/themes/<name>.json`
- **Windows:** `%APPDATA%\tsuzuri\themes\<name>.json`

User themes appear automatically in the live theme picker (`Space` `t`) and in `--list-themes`. Bundled themes take precedence on a name collision.

To use themes from a different folder (or a single palette file), set `themes_dir` in your config:

- **config.json:** `{"theme": "onedark", "themes_dir": "~/my-themes"}` — a directory of `*.json` palettes, or one `.json` file
- **flag:** `tsuzuri --themes-dir ~/my-themes` (overrides the config value)

Example `~/.config/tsuzuri/themes/minimal.json`:

```json
{
  "light": false,
  "fg": "#d8dee9",
  "bg": "#2e3440",
  "darker_bg": "#242933",
  "line": "#3b4252",
  "blue": "#88c0d0"
}
```

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
