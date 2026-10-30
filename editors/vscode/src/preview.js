'use strict';
// "Open with Tsuzuri": a preview tab beside the note that draws boards,
// calendars, timelines, charts, forms, flowcharts and equations the way the
// app does, and updates as you type.

const vscode = require('vscode');
const fs = require('fs');
const path = require('path');
const crypto = require('crypto');
const { createRenderer, fenceKey } = require('./markdown');
const { showError } = require('./bridge');

const VIEW_TYPE = 'tsuzuri.preview';
const RENDER_DELAY_MS = 150;

class PreviewManager {
  constructor(context, bridge) {
    this.extensionUri = context.extensionUri;
    this.bridge = bridge;
    /** One preview per note, keyed by the note's URI string. */
    this.previews = new Map();
  }

  register() {
    return vscode.Disposable.from(
      vscode.commands.registerCommand('tsuzuri.preview', (uri) => this.open(uri)),
      // Brings previews back after VS Code restarts.
      vscode.window.registerWebviewPanelSerializer(VIEW_TYPE, {
        deserializeWebviewPanel: async (panel, state) => {
          try {
            const doc = await vscode.workspace.openTextDocument(vscode.Uri.parse(state.uri));
            this.attach(panel, doc);
          } catch {
            panel.dispose();
          }
        },
      }),
      new vscode.Disposable(() => this.previews.forEach((p) => p.panel.dispose()))
    );
  }

  /** Opens (or focuses) the preview for a note, beside the editor. */
  async open(uri, preserveFocus = false) {
    const editor = vscode.window.activeTextEditor;
    const target = uri instanceof vscode.Uri ? uri : editor && editor.document.uri;
    if (!target || path.extname(target.path).toLowerCase() !== '.md') {
      vscode.window.showErrorMessage('Open a Markdown note to preview it.');
      return;
    }

    const existing = this.previews.get(target.toString());
    if (existing) {
      existing.panel.reveal(undefined, preserveFocus);
      return;
    }

    const doc = await vscode.workspace.openTextDocument(target);
    const panel = vscode.window.createWebviewPanel(
      VIEW_TYPE,
      title(doc),
      { viewColumn: vscode.ViewColumn.Beside, preserveFocus },
      { enableScripts: true }
    );
    this.attach(panel, doc);
  }

  attach(panel, doc) {
    const preview = new Preview(panel, doc, this);
    this.previews.set(doc.uri.toString(), preview);
    panel.onDidDispose(() => this.previews.delete(doc.uri.toString()));
  }

  /** Redraws every preview, e.g. after the theme or binary changed. */
  refreshAll() {
    this.previews.forEach((p) => p.refresh(true));
  }
}

class Preview {
  constructor(panel, doc, manager) {
    this.panel = panel;
    this.doc = doc;
    this.manager = manager;
    this.bridge = manager.bridge;
    this.width = 0; // terminal cells across, reported by the page
    this.drawn = new Map();
    this.generation = 0;
    this.drawing = false; // a render request is with the binary
    this.again = false; // the note changed while it was
    this.failed = false; // the last render request failed
    this.posted = false;
    this.months = {}; // calendar fence line → months moved from the note's month
    this.timer = undefined;

    const dir = vscode.Uri.file(path.dirname(doc.uri.fsPath));
    const roots = [vscode.Uri.joinPath(manager.extensionUri, 'media'), katexDir(manager.extensionUri), dir];
    for (const f of vscode.workspace.workspaceFolders || []) roots.push(f.uri);
    panel.webview.options = { enableScripts: true, localResourceRoots: roots };
    panel.title = title(doc);
    panel.iconPath = vscode.Uri.joinPath(manager.extensionUri, 'media', 'preview-icon.svg');

    this.renderer = createRenderer({ drawn: this.drawn, imageSrc: (src) => this.imageSrc(src) });
    panel.webview.html = this.shell();

    const key = doc.uri.toString();
    this.subscriptions = [
      vscode.workspace.onDidChangeTextDocument((e) => {
        if (e.document.uri.toString() !== key) return;
        this.doc = e.document;
        this.refresh();
      }),
      panel.webview.onDidReceiveMessage((msg) => this.onMessage(msg)),
    ];
    panel.onDidDispose(() => {
      clearTimeout(this.timer);
      this.subscriptions.forEach((s) => s.dispose());
    });
  }

  /** Re-renders after a short pause; `redraw` also forgets drawn views. */
  refresh(redraw = false) {
    if (redraw) this.forgetDrawn();
    clearTimeout(this.timer);
    this.timer = setTimeout(() => this.update(), RENDER_DELAY_MS);
  }

  forgetDrawn() {
    this.drawn.clear();
    this.generation++; // drawings already on their way are for the old width or theme
  }

  render() {
    const name = path.basename(this.doc.uri.fsPath, path.extname(this.doc.uri.fsPath));
    return this.renderer.render(this.doc.getText(), name, { months: this.months });
  }

  /** Renders the note, asking the binary to draw any views it hasn't yet. */
  async update() {
    if (!this.width) return; // the page hasn't reported its size yet
    if (this.drawing) {
      this.again = true; // one drawing at a time; catch up when it ends
      return;
    }
    const first = this.render();
    // While new views are being drawn, keep showing the last page rather
    // than flashing their source; the very first render shows at once.
    if (!first.missing.length || !this.posted) this.post(first.html, first.cover);
    if (!first.missing.length) return;

    this.drawing = true;
    const generation = this.generation;
    let drawn = false;
    try {
      const results = await this.bridge.render(first.missing, this.width);
      if (generation === this.generation) {
        first.missing.forEach((f, i) => this.drawn.set(fenceKey(f.lang, f.body), results[i] || { ok: false }));
      }
      drawn = true;
      this.failed = false;
    } catch (err) {
      // Say so once; the views show as code and the next edit tries again.
      if (!this.failed) showError("Tsuzuri couldn't draw this note", err);
      this.failed = true;
    } finally {
      this.drawing = false;
    }

    if (this.again) {
      this.again = false;
      if (drawn) return this.update();
    }
    const next = drawn ? this.render() : first;
    this.post(next.html, next.cover);
  }

  /** Sends the page the note as it is now, e.g. after an edit was refused. */
  repost() {
    const { html, cover } = this.render();
    this.post(html, cover);
  }

  post(html, cover) {
    this.posted = true;
    this.panel.webview.postMessage({
      type: 'update',
      html,
      cover,
      uri: this.doc.uri.toString(),
      version: this.doc.version,
    });
  }

  async onMessage(msg) {
    if (!msg || typeof msg !== 'object') return;
    switch (msg.type) {
      case 'width':
        if (!this.width) this.sendBlocks();
        if (Number.isInteger(msg.cols) && msg.cols > 0 && msg.cols !== this.width) {
          this.width = msg.cols;
          this.forgetDrawn();
          this.update();
        }
        break;
      case 'wikilink':
        if (typeof msg.target === 'string') await this.openWikilink(msg.target);
        break;
      case 'link':
        if (typeof msg.href === 'string') await this.openLink(msg.href);
        break;
      case 'source':
        this.sendSource(msg);
        break;
      case 'apply':
      case 'insert':
        if (typeof msg.text === 'string') await this.apply(msg);
        break;
      case 'move':
        await this.moveBlock(msg);
        break;
      case 'card':
        await this.moveCard(msg);
        break;
      case 'view':
        await this.editView(msg);
        break;
      case 'block':
        await this.blockAction(msg);
        break;
      case 'slash':
        if (typeof msg.action === 'string') await this.slashAction(msg);
        break;
      case 'month':
        if (Number.isInteger(msg.block) && Number.isInteger(msg.shift)) {
          this.months[msg.block] = msg.shift === 0 ? 0 : (this.months[msg.block] || 0) + msg.shift;
          this.repost();
        }
        break;
      case 'toggle':
        await this.toggleTask(msg);
        break;
    }
  }

  /* ----------------------------------------------------------- editing */

  /** Lines [start, end) of the note without trailing blank lines, or null if out of range. */
  blockRange(start, end) {
    const doc = this.doc;
    if (!Number.isInteger(start) || !Number.isInteger(end) || start < 0 || end < start) return null;
    if (start >= doc.lineCount) return { start: doc.lineCount, end: doc.lineCount };
    end = Math.min(end, doc.lineCount);
    while (end > start + 1 && !doc.lineAt(end - 1).text.trim()) end--;
    return { start, end };
  }

  /** Sends a block's Markdown so the page can edit it in place. */
  sendSource({ start, end, id, version }) {
    const range = this.blockRange(start, end);
    // Lines move when the note changes; the page is about to get the new render.
    if (!range || version !== this.doc.version) {
      this.panel.webview.postMessage({ type: 'source', id, stale: true });
      return;
    }
    const lines = [];
    for (let i = range.start; i < range.end; i++) lines.push(this.doc.lineAt(i).text);
    this.panel.webview.postMessage({ type: 'source', id, ...range, text: lines.join('\n'), version: this.doc.version });
  }

  /** Writes an edited block back into the note. */
  async apply(msg) {
    const ok = await this.applyEdit(msg);
    this.panel.webview.postMessage({ type: 'applied', id: msg.id, ok });
  }

  /** Returns false when the edit can't be applied, so the page keeps the draft. */
  async applyEdit({ type, start, end, text, version }) {
    const doc = this.doc;
    if (version !== doc.version) {
      vscode.window.showWarningMessage(
        'The note changed while you were editing this block. Your text is still in the box: copy it, or press Shift+Esc to discard it.'
      );
      return false;
    }
    const range = this.blockRange(start, end);
    if (!range) return false;
    const body = text.replace(/\r\n/g, '\n').replace(/\s+$/, '');

    const edit = new vscode.WorkspaceEdit();
    if (type === 'insert' && range.end < doc.lineCount) {
      // A new block after this one, kept apart from its neighbours by blank lines.
      if (!body) return true;
      const above = range.end > 0 && doc.lineAt(range.end - 1).text.trim();
      const below = doc.lineAt(range.end).text.trim();
      edit.insert(doc.uri, new vscode.Position(range.end, 0), `${above ? '\n' : ''}${body}\n${below ? '\n' : ''}`);
    } else if (range.start >= doc.lineCount || type === 'insert') {
      if (!body) return true;
      const last = doc.lineAt(doc.lineCount - 1);
      const gap = doc.getText().trim() ? (last.text.trim() ? '\n\n' : '\n') : '';
      edit.insert(doc.uri, last.range.end, gap + body + '\n');
    } else if (range.end >= doc.lineCount) {
      edit.replace(doc.uri, new vscode.Range(range.start, 0, doc.lineCount - 1, doc.lineAt(doc.lineCount - 1).text.length), body);
    } else if (body) {
      edit.replace(doc.uri, new vscode.Range(range.start, 0, range.end, 0), body + '\n');
    } else {
      // Removing a block also removes the blank line after it.
      const next = range.end < doc.lineCount && !doc.lineAt(range.end).text.trim() ? range.end + 1 : range.end;
      edit.delete(doc.uri, new vscode.Range(range.start, 0, next, 0));
    }
    return this.commit(edit);
  }

  /** Drags a block (⠿) to another place in the note. */
  async moveBlock({ from, end, to, version }) {
    const doc = this.doc;
    const range = this.blockRange(from, end);
    if (version !== doc.version || !range || range.start >= doc.lineCount || !Number.isInteger(to)) {
      this.repost();
      return;
    }
    const lines = this.lines();
    try {
      await this.replaceLines(await this.bridge.move(lines, range.start, range.end, Math.min(to, lines.length)), version);
    } catch (err) {
      showError("Couldn't move the block", err);
      this.repost();
    }
  }

  /** Drags a board card to another column or position. */
  async moveCard({ block, end, card, column, index, version }) {
    const doc = this.doc;
    if (version !== doc.version || ![block, end, card, column, index].every(Number.isInteger)) {
      this.repost();
      return;
    }
    try {
      await this.replaceLines(await this.bridge.card(this.lines(), { block, end }, card, column, index), version);
    } catch (err) {
      showError("Couldn't move the card", err);
      this.repost();
    }
  }

  /** Duplicate or delete a block, from its ⠿ menu. */
  async blockAction({ action, start, end, version }) {
    const range = this.blockRange(start, end);
    if (!range || range.start >= this.doc.lineCount || version !== this.doc.version) {
      this.repost();
      return;
    }
    if (action === 'delete') {
      await this.applyEdit({ type: 'apply', start: range.start, end: range.end, text: '', version });
    } else if (action === 'duplicate') {
      const text = this.lines().slice(range.start, range.end).join('\n');
      await this.applyEdit({ type: 'insert', start: range.start, end: range.end, text, version });
    }
  }

  /** Gives the page the "/" menu, so new lines can insert blocks. */
  async sendBlocks() {
    try {
      this.panel.webview.postMessage({ type: 'blocks', blocks: await this.bridge.blocks() });
    } catch {
      /* without the binary the page has no menu; the user has been told why */
    }
  }

  /**
   * "/" menu entries that need VS Code: a sub-page, a link to a note, or a
   * file. What they make is inserted where the new line was.
   */
  async slashAction({ action, start, end, version }) {
    const note = this.doc.uri;
    const insert = (text) => this.applyEdit({ type: 'insert', start, end, text, version: this.doc.version });
    try {
      if (action === 'page') {
        const title = await vscode.window.showInputBox({ prompt: 'Name of the new page', placeHolder: 'Untitled' });
        if (title === undefined) return;
        const created = await this.bridge.page(note, title);
        await insert(`[[${created.title}]]`);
      } else if (action === 'link') {
        const files = await vscode.workspace.findFiles('**/*.md', '**/{node_modules,.git}/**', 5000);
        const picked = await vscode.window.showQuickPick(
          files.filter((f) => f.toString() !== note.toString()).map((f) => ({
            label: path.basename(f.fsPath, '.md'),
            description: vscode.workspace.asRelativePath(f, false),
          })),
          { placeHolder: 'Link to which note?', matchOnDescription: true }
        );
        if (picked) await insert(`[[${picked.label}]]`);
      } else if (action.startsWith('media:')) {
        const kind = action.slice('media:'.length);
        const files = await vscode.window.showOpenDialog({
          canSelectMany: false,
          defaultUri: vscode.Uri.file(path.dirname(note.fsPath)),
          title: kind === 'cover' ? 'Choose a cover image' : `Choose ${kind === 'image' ? 'an image' : `a ${kind}`}`,
        });
        if (!files || !files.length) return;
        const result = await this.bridge.attach(note, files[0].fsPath, kind, this.doc.getText());
        if (kind === 'cover') await this.replaceLines(result.text.split(/\r?\n/), this.doc.version);
        else await insert(result.markdown);
      }
    } catch (err) {
      showError("Tsuzuri couldn't do that", err);
    }
    if (version !== this.doc.version) return;
    this.repost();
  }

  /** An edit made on a view's own controls, applied by the binary. */
  async editView({ block, end, edit, version }) {
    if (version !== this.doc.version || !Number.isInteger(block) || !Number.isInteger(end) || !edit || typeof edit.op !== 'string') {
      this.repost();
      return;
    }
    try {
      await this.replaceLines(await this.bridge.view(this.lines(), { block, end }, edit), version);
    } catch (err) {
      showError("Couldn't change the view", err);
      this.repost();
    }
  }

  lines() {
    const out = [];
    for (let i = 0; i < this.doc.lineCount; i++) out.push(this.doc.lineAt(i).text);
    return out;
  }

  /** Rewrites only the lines that changed, so undo and the editor's cursor behave. */
  async replaceLines(next, version) {
    const doc = this.doc;
    if (doc.version !== version) {
      this.repost();
      return;
    }
    const prev = this.lines();
    let top = 0;
    while (top < prev.length && top < next.length && prev[top] === next[top]) top++;
    let a = prev.length;
    let b = next.length;
    while (a > top && b > top && prev[a - 1] === next[b - 1]) {
      a--;
      b--;
    }
    if (top === a && top === b) {
      this.repost();
      return;
    }
    const edit = new vscode.WorkspaceEdit();
    const text = next.slice(top, b).join('\n');
    if (a < prev.length) {
      edit.replace(doc.uri, new vscode.Range(top, 0, a, 0), b > top ? `${text}\n` : '');
    } else {
      // The change runs to the end of the note.
      const last = doc.lineAt(doc.lineCount - 1).range.end;
      const from = top > 0 ? doc.lineAt(top - 1).range.end : new vscode.Position(0, 0);
      edit.replace(doc.uri, new vscode.Range(from, last), (top > 0 ? '\n' : '') + text);
    }
    await this.commit(edit);
  }

  /** Ticks or unticks a checkbox: a to-do, or one in a table cell. */
  async toggleTask({ line, box, version }) {
    const doc = this.doc;
    const ok = version === doc.version && Number.isInteger(line) && line >= 0 && line < doc.lineCount && Number.isInteger(box);
    let next = null;
    if (ok) {
      try {
        next = await this.bridge.toggle(doc.lineAt(line).text, box);
      } catch (err) {
        showError("Couldn't tick that box", err);
      }
    }
    if (next === null || version !== this.doc.version) {
      this.repost(); // put the checkbox back as the note has it
      return;
    }
    const edit = new vscode.WorkspaceEdit();
    edit.replace(doc.uri, doc.lineAt(line).range, next);
    await this.commit(edit);
  }

  /** Applies an edit; a note that isn't open in an editor is saved straight away. */
  async commit(edit) {
    const doc = this.doc;
    const wasDirty = doc.isDirty;
    if (!(await vscode.workspace.applyEdit(edit))) {
      vscode.window.showErrorMessage("Couldn't update the note.");
      return false;
    }
    const visible = vscode.window.visibleTextEditors.some((e) => e.document.uri.toString() === doc.uri.toString());
    if (!visible && !wasDirty) await doc.save();
    return true;
  }

  async openWikilink(target) {
    const [name, heading] = splitHeading(target);
    let file;
    try {
      file = name ? await this.bridge.resolve(this.doc.uri, name) : this.doc.uri.fsPath;
    } catch (err) {
      showError("Couldn't follow the link", err);
      return;
    }
    if (!file) {
      vscode.window.showInformationMessage(`There's no note named "${name}" yet.`);
      return;
    }
    await this.openNote(vscode.Uri.file(file), heading);
  }

  async openLink(href) {
    if (/^(https?|mailto):/i.test(href)) {
      vscode.env.openExternal(vscode.Uri.parse(href));
      return;
    }
    const [file, heading] = splitHeading(href);
    if (!file || /^[a-z][a-z0-9+.-]*:/i.test(file)) return;
    let target;
    try {
      target = vscode.Uri.file(path.resolve(path.dirname(this.doc.uri.fsPath), decodeURIComponent(file)));
    } catch {
      return; // a malformed %-escape
    }
    if (!fs.existsSync(target.fsPath) || fs.statSync(target.fsPath).isDirectory()) {
      vscode.window.showWarningMessage(`${path.basename(target.fsPath)} doesn't exist.`);
      return;
    }
    if (path.extname(target.fsPath).toLowerCase() === '.md') {
      await this.openNote(target, heading);
      return;
    }
    // Other files open inside VS Code, never with the system's default app,
    // and only from the note's own notes folder.
    const folder = vscode.workspace.getWorkspaceFolder(this.doc.uri);
    const root = folder ? folder.uri.fsPath : path.dirname(this.doc.uri.fsPath);
    const rel = path.relative(root, target.fsPath);
    if (rel === '..' || rel.startsWith(`..${path.sep}`) || path.isAbsolute(rel)) {
      vscode.window.showWarningMessage(`${path.basename(target.fsPath)} is outside this notes folder, so it wasn't opened.`);
      return;
    }
    await vscode.commands.executeCommand('vscode.open', target, { viewColumn: vscode.ViewColumn.One });
  }

  /** Opens a note in the editor at a heading, with its preview beside it. */
  async openNote(uri, heading) {
    const doc = await vscode.workspace.openTextDocument(uri);
    const editor = await vscode.window.showTextDocument(doc, { viewColumn: vscode.ViewColumn.One });
    if (heading) revealHeading(editor, heading);
    if (uri.toString() !== this.doc.uri.toString()) await this.manager.open(uri, true);
  }

  imageSrc(src) {
    if (/^[a-z][a-z0-9+.-]*:/i.test(src) || src.startsWith('//')) return src;
    let file;
    try {
      file = path.resolve(path.dirname(this.doc.uri.fsPath), decodeURI(src.replace(/^<|>$/g, '')));
    } catch {
      return src;
    }
    return this.panel.webview.asWebviewUri(vscode.Uri.file(file)).toString();
  }

  shell() {
    const { webview } = this.panel;
    const media = (name) => webview.asWebviewUri(vscode.Uri.joinPath(this.manager.extensionUri, 'media', name));
    const nonce = crypto.randomBytes(16).toString('base64');
    const csp = [
      "default-src 'none'",
      `img-src ${webview.cspSource} https: data:`,
      `style-src ${webview.cspSource} 'unsafe-inline'`,
      `font-src ${webview.cspSource}`,
      `script-src 'nonce-${nonce}'`,
    ].join('; ');
    return `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<meta http-equiv="Content-Security-Policy" content="${csp}">
<meta name="viewport" content="width=device-width, initial-scale=1">
<link rel="stylesheet" href="${webview.asWebviewUri(vscode.Uri.joinPath(katexDir(this.manager.extensionUri), 'katex.min.css'))}">
<link rel="stylesheet" href="${media('preview.css')}">
</head>
<body>
<main class="tsz-page" id="page"></main>
<script nonce="${nonce}" src="${media('preview.js')}"></script>
<script nonce="${nonce}" src="${media('views.js')}"></script>
</body>
</html>`;
  }
}

/** KaTeX's stylesheet and fonts, for typeset equations. */
function katexDir(extensionUri) {
  return vscode.Uri.joinPath(extensionUri, 'node_modules', 'katex', 'dist');
}

function title(doc) {
  return `Preview ${path.basename(doc.uri.fsPath)}`;
}

/** "Note#Heading" to ["Note", "Heading"]. */
function splitHeading(target) {
  const i = target.indexOf('#');
  return i < 0 ? [target, ''] : [target.slice(0, i), target.slice(i + 1)];
}

/** Headings match loosely, as in the app: "S3 Storage" finds "s3-storage". */
function headingKey(s) {
  return s.toLowerCase().replace(/[^\p{L}\p{N}]+/gu, '');
}

function revealHeading(editor, heading) {
  const want = headingKey(decodeURIComponentSafe(heading));
  const doc = editor.document;
  for (let i = 0; i < doc.lineCount; i++) {
    const m = /^#{1,6}\s+(.*)$/.exec(doc.lineAt(i).text);
    if (m && headingKey(m[1]) === want) {
      const pos = new vscode.Position(i, 0);
      editor.selection = new vscode.Selection(pos, pos);
      editor.revealRange(new vscode.Range(pos, pos), vscode.TextEditorRevealType.AtTop);
      return;
    }
  }
}

function decodeURIComponentSafe(s) {
  try {
    return decodeURIComponent(s);
  } catch {
    return s;
  }
}

module.exports = { PreviewManager };
