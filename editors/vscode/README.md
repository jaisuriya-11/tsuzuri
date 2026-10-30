# Tsuzuri for VS Code

Read and edit [Tsuzuri](https://github.com/jaisuriya-11/tsuzuri) notes in VS Code, with the same boards, calendars, timelines, charts, forms, flowcharts and equations the app draws.

## Features

**Open with Tsuzuri.** Right-click a `.md` file and choose **Open with Tsuzuri**, click the preview icon at the top right of a Markdown editor, or press `Cmd+Alt+V` (`Ctrl+Alt+V` on Windows and Linux). The note opens in a clean, document-style page beside the editor and updates as you type.

**Edit in the preview.** Click a paragraph or heading to edit it where it is; press `Enter` or click away to save (`Esc` too, `Shift+Esc` discards). Other blocks open as Markdown in place. Tick to-dos with a click. There are no modes: you type as you would anywhere in VS Code.

**Edit views through their own controls.** Rename cards, columns, events, timeline items, chart values and questions by clicking them. Add with **+ New**, remove with **×**. Pick timeline dates, chart types, question types and form answers (text, choices, stars, dates) directly. Drag calendar events to another day, and step through months. Every change is made by Tsuzuri, the same as clicking the view in the app. To edit a view as text, hover it and click **Markdown**.

**Drag things around.** Hover a block and drag its `⠿` handle to move it, or click `+` to add a block below it. Drag board cards between columns, or up and down within one.

**Views.** `board`, `calendar`, `timeline`, `chart` (bar, hbar, line, pie, donut) and `form` blocks are drawn as cards and charts in your VS Code theme's colours. Tsuzuri reads the blocks, so the preview always agrees with the app about what a block holds. `flow` charts are drawn by Tsuzuri itself, and equations (```` ```math ```` and `$$ … $$`) are typeset with KaTeX. Callouts (`> [!NOTE]`), toggles, columns, covers and `[[links]]` are shown too. Clicking a `[[link]]` opens the note it names, found the same way the app finds it.

**Insert blocks with `/`.** Type `/` at the start of a line, or after a space, to get Tsuzuri's block menu: headings, to-dos, toggles, callouts, tables, equations, columns, sub-pages, boards, calendars, timelines, charts, forms, flowcharts, images, video, audio, files and covers. Inside a table the menu also adds and removes rows and columns. After `[[`, note names are suggested.

**Export to PDF.** Right-click a note and choose **Tsuzuri: Export Note to PDF**. The PDF is written next to the note.

**Pick a theme.** **Tsuzuri: Choose Theme** sets the Tsuzuri theme used for views in the preview.

## Requirements

Tsuzuri **0.4.0 or later** must be installed. The extension uses it for everything Tsuzuri-specific, so the preview and the `/` menu always match your version of the app. If it can't be found, or is too old, the extension offers to install it or to set `tsuzuri.path`.

## Settings

| Setting | Default | What it does |
| --- | --- | --- |
| `tsuzuri.path` | empty | Full path to the binary. Empty searches `PATH`, `~/.local/bin`, `/usr/local/bin`, `/opt/homebrew/bin`, `~/go/bin` and `%LOCALAPPDATA%\Programs\tsuzuri`. |
| `tsuzuri.theme` | empty | Tsuzuri theme for views in the preview. Empty uses Tsuzuri's own config. |
| `tsuzuri.slashMenu` | `true` | The `/` block menu and `[[` note suggestions in Markdown files. |

## How it works

The extension holds no Tsuzuri logic of its own. It runs `tsuzuri editor <command>` with a JSON request on stdin:

| Command | Used for |
| --- | --- |
| `version` | Checking the binary speaks the same protocol |
| `blocks` | The `/` menu entries |
| `render` | What each view block holds, flowcharts as drawn, and equations |
| `table` | Table edits from the `/` menu |
| `move` | Dragging a block with `⠿` |
| `card` | Dragging a board card |
| `view` | Edits made on a view's controls |
| `attach` | Copying images and files next to the note, and covers |
| `page` | Creating sub-pages |
| `resolve` | Finding the note a `[[link]]` names |

The Go side lives in `internal/bridge`; the line edits behind dragging are in `internal/note`, shared with the app.

## Development

From the repository root:

```sh
make vscode-test    # builds tsuzuri, then runs the extension's tests against it
make vscode         # also packages tsuzuri-<version>.vsix
```

To try changes, open `editors/vscode` in VS Code and press `F5`. Point `tsuzuri.path` at `bin/tsuzuri` in the Extension Development Host so it uses the binary you just built.

## Publishing

1. Publisher `jaisuriya-11` on https://marketplace.visualstudio.com/manage; log in once with `npx @vscode/vsce login jaisuriya-11`.
2. `npx @vscode/vsce publish` uploads the extension.
3. Also publish to [Open VSX](https://open-vsx.org) with `npx ovsx publish`, so VSCodium, Cursor and other forks can install it.

The extension needs a Tsuzuri release that includes `tsuzuri editor` (0.4.0); publish the extension after that release.
