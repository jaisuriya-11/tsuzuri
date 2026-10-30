'use strict';
// The "/" block menu. Type "/" at the start of a line, or after a space, in
// a Markdown note to insert the blocks Tsuzuri's own menu offers. The list
// and every edit it makes come from the tsuzuri binary.

const { BridgeUnavailable, showError } = require('./bridge');
const { slashQuery, insideFence, isTableLine, escapeSnippet, snippetFor, blocksFor } = require('./blocks');

const MEDIA_FILTERS = {
  image: { Images: ['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg', 'bmp', 'heic', 'avif'] },
  video: { Videos: ['mp4', 'mov', 'webm', 'mkv', 'avi', 'm4v'] },
  audio: { Audio: ['mp3', 'wav', 'm4a', 'ogg', 'flac', 'aac'] },
  cover: { Images: ['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp'] },
};

const MEDIA_TITLES = {
  image: 'Choose an image',
  video: 'Choose a video',
  audio: 'Choose an audio file',
  file: 'Choose a file',
  cover: 'Choose a cover image',
};

/* --------------------------------------------------------------- actions */

class SlashActions {
  constructor(vscode, bridge) {
    this.vscode = vscode;
    this.bridge = bridge;
  }

  async run(action) {
    const editor = this.vscode.window.activeTextEditor;
    if (!editor) return;
    try {
      if (action === 'link') await this.link(editor);
      else if (action === 'page') await this.page(editor);
      else if (action.startsWith('media:')) await this.media(editor, action.slice('media:'.length));
      else if (action.startsWith('table:')) await this.table(editor, action.slice('table:'.length));
    } catch (err) {
      showError("Tsuzuri couldn't do that", err);
    }
  }

  /** "[[" with the note picker open. */
  async link(editor) {
    await editor.insertSnippet(new this.vscode.SnippetString('[[$0]]'));
    await this.vscode.commands.executeCommand('editor.action.triggerSuggest');
  }

  /** A sub-page in the folder named after this note, linked here and opened. */
  async page(editor) {
    const note = editor.document.uri;
    if (note.scheme !== 'file') {
      this.vscode.window.showErrorMessage('Save the note before adding a page to it.');
      return;
    }
    const title = await this.vscode.window.showInputBox({ prompt: 'Name of the new page', placeHolder: 'Untitled' });
    if (title === undefined) return;
    const created = await this.bridge.page(note, title);
    await editor.insertSnippet(new this.vscode.SnippetString(escapeSnippet(`[[${created.title}]]`)));
    const doc = await this.vscode.workspace.openTextDocument(created.path);
    await this.vscode.window.showTextDocument(doc);
  }

  async media(editor, kind) {
    const note = editor.document.uri;
    if (note.scheme !== 'file') {
      this.vscode.window.showErrorMessage('Save the note before attaching files to it.');
      return;
    }
    const picked = await this.vscode.window.showOpenDialog({
      title: MEDIA_TITLES[kind] || MEDIA_TITLES.file,
      canSelectMany: false,
      filters: MEDIA_FILTERS[kind],
      defaultUri: this.vscode.Uri.joinPath(note, '..'),
    });
    if (!picked || !picked.length) return;

    const doc = editor.document;
    const version = doc.version;
    const result = await this.bridge.attach(note, picked[0].fsPath, kind, doc.getText());
    if (kind !== 'cover') {
      await editor.insertSnippet(new this.vscode.SnippetString(escapeSnippet(result.markdown)));
      return;
    }
    if (doc.version !== version) {
      this.vscode.window.showWarningMessage('The note changed while attaching; pick the cover again.');
      return;
    }
    const all = new this.vscode.Range(doc.positionAt(0), doc.positionAt(doc.getText().length));
    await editor.edit((e) => e.replace(all, result.text));
    this.vscode.window.setStatusBarMessage(`Cover set to ${result.link}`, 4000);
  }

  async table(editor, op) {
    const doc = editor.document;
    const version = doc.version;
    const lines = doc.getText().split(/\r?\n/);
    const pos = editor.selection.active;
    const edit = await this.bridge.table(lines, pos.line, pos.character, op);
    if (!edit || doc.version !== version) return;

    const { Position, Range, Selection } = this.vscode;
    const range = new Range(edit.start, 0, edit.end - 1, lines[edit.end - 1].length);
    await editor.edit((e) => e.replace(range, edit.lines.join('\n')));
    const cursor = new Position(edit.row, edit.col);
    editor.selection = new Selection(cursor, cursor);
  }
}

/* -------------------------------------------------------------- provider */

function provider(vscode, bridge, enabled) {
  const kinds = {
    Table: vscode.CompletionItemKind.Struct,
    'Basic blocks': vscode.CompletionItemKind.Text,
    Views: vscode.CompletionItemKind.Module,
    Media: vscode.CompletionItemKind.File,
  };

  return {
    async provideCompletionItems(doc, pos) {
      if (!enabled()) return undefined;
      const line = doc.lineAt(pos.line).text;
      const before = line.slice(0, pos.character);
      const slash = slashQuery(before);
      if (!slash) return undefined;

      const above = [];
      for (let i = 0; i < pos.line; i++) above.push(doc.lineAt(i).text);
      if (insideFence(above)) return undefined;

      let blocks;
      try {
        blocks = await bridge.blocks();
      } catch (err) {
        if (!(err instanceof BridgeUnavailable)) showError("Couldn't load Tsuzuri's / menu", err);
        return undefined;
      }

      const midLine = before.slice(0, slash.start).trim() !== '';
      const inTable = isTableLine(line);
      const range = new vscode.Range(pos.line, slash.start, pos.line, pos.character);

      return blocksFor(blocks, inTable)
        .map((b, i) => {
          const item = new vscode.CompletionItem(b.label, kinds[b.section] || vscode.CompletionItemKind.Text);
          item.range = range;
          item.filterText = `/${b.label} ${b.aliases || ''}`;
          item.sortText = String(i).padStart(3, '0');
          item.detail = b.hint ? `${b.section} · ${b.hint}` : b.section;
          if (b.action) {
            item.insertText = '';
            item.command = { command: 'tsuzuri.slashAction', title: b.label, arguments: [b.action] };
          } else {
            item.insertText = new vscode.SnippetString(snippetFor(b, midLine, inTable));
            const text = (b.before || '') + (b.after || '');
            item.documentation = new vscode.MarkdownString().appendCodeblock(text, 'markdown');
          }
          return item;
        });
    },
  };
}

function register(bridge, enabled) {
  const vscode = require('vscode');
  const actions = new SlashActions(vscode, bridge);
  return vscode.Disposable.from(
    vscode.languages.registerCompletionItemProvider({ language: 'markdown' }, provider(vscode, bridge, enabled), '/'),
    vscode.commands.registerCommand('tsuzuri.slashAction', (action) => actions.run(action))
  );
}

module.exports = { register };
