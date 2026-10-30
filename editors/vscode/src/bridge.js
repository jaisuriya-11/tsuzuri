'use strict';
// The extension's link to the tsuzuri binary. Views, the "/" menu, table
// edits, attachments and links all come from "tsuzuri editor <command>", so
// they behave exactly as they do in the app.

const vscode = require('vscode');
const path = require('path');
const binary = require('./binary');

/** Must match bridge.Protocol in the Go code. */
const PROTOCOL = 4;

/** A Tsuzuri theme made for light backgrounds, used when VS Code is light. */
const LIGHT_THEME = 'vscode_light';

class Bridge {
  constructor() {
    /** Resolves to the binary path, or undefined when it's missing or too old. */
    this.ready = undefined;
    this.blocksCache = undefined;
    this.warned = false;
  }

  /** Forgets the binary, so a changed tsuzuri.path or a fresh install is picked up. */
  reset() {
    this.ready = undefined;
    this.blocksCache = undefined;
    this.warned = false;
  }

  /** The binary path once it's found and speaks our protocol; undefined otherwise. */
  binary() {
    if (!this.ready) this.ready = this.locate();
    return this.ready;
  }

  async locate() {
    const configured = binary.expandPath(settings().get('path', ''), workspaceRoot());
    const bin = binary.find(configured);
    if (!bin) {
      this.warn(
        configured
          ? `Tsuzuri wasn't found at ${configured}. Check the tsuzuri.path setting.`
          : "Tsuzuri isn't installed, or it isn't in a folder VS Code can see."
      );
      return undefined;
    }
    try {
      const { protocol } = await binary.request(bin, 'version');
      if (protocol === PROTOCOL) return bin;
    } catch {
      /* an older release has no "editor" command */
    }
    this.warn('This extension needs Tsuzuri 0.4.0 or later. Update Tsuzuri to use the preview and the / menu.');
    return undefined;
  }

  /** Shows a missing or outdated binary once, until the next reset. */
  async warn(message) {
    if (this.warned) return;
    this.warned = true;
    const choice = await vscode.window.showWarningMessage(message, 'Install Tsuzuri', 'Set path');
    if (choice === 'Install Tsuzuri') vscode.commands.executeCommand('tsuzuri.install');
    if (choice === 'Set path') vscode.commands.executeCommand('workbench.action.openSettings', 'tsuzuri.path');
  }

  async call(command, payload) {
    const bin = await this.binary();
    if (!bin) throw new BridgeUnavailable();
    return binary.request(bin, command, payload);
  }

  /** The "/" menu entries, fetched once per binary. */
  async blocks() {
    if (!this.blocksCache) {
      this.blocksCache = this.call('blocks').then((r) => r.blocks, (err) => {
        this.blocksCache = undefined;
        throw err;
      });
    }
    return this.blocksCache;
  }

  /** Draws fences ({lang, body}) at width cells; returns {ok, html} per fence. */
  async render(fences, width) {
    const { fences: drawn } = await this.call('render', { theme: drawingTheme(), width, fences });
    return drawn;
  }

  /** Applies a table operation; returns the edit, or null when nothing changes. */
  async table(lines, row, col, op) {
    const { edit } = await this.call('table', { lines, row, col, op });
    return edit || null;
  }

  /** Moves note lines [from, end) to start at line to; returns the note's new lines. */
  async move(lines, from, end, to) {
    return (await this.call('move', { lines, from, end, to })).lines;
  }

  /** Moves a board card; returns the note's new lines. */
  async card(lines, board, card, column, index) {
    return (await this.call('card', { lines, block: board.block, end: board.end, card, column, index })).lines;
  }

  /** Applies an edit made on a view's controls; returns the note's new lines. */
  async view(lines, board, edit) {
    return (await this.call('view', { lines, block: board.block, end: board.end, edit })).lines;
  }

  /** Ticks or unticks the nth checkbox on a line; returns the line, or null. */
  async toggle(line, nth) {
    const r = await this.call('toggle', { line, nth });
    return r.ok ? r.line : null;
  }

  attach(note, src, kind, text) {
    return this.call('attach', { root: notesRoot(note), note: note.fsPath, src, kind, text });
  }

  page(note, title) {
    return this.call('page', { root: notesRoot(note), note: note.fsPath, title });
  }

  /** The file a [[link]] names, or undefined. */
  async resolve(note, target) {
    const { path: file } = await this.call('resolve', { root: notesRoot(note), note: note.fsPath, target });
    return file || undefined;
  }
}

/** Thrown when the binary is missing or too old; the user has already been told. */
class BridgeUnavailable extends Error {
  constructor() {
    super('Tsuzuri is not available');
    this.name = 'BridgeUnavailable';
  }
}

/**
 * The Tsuzuri theme flowcharts are drawn in: the tsuzuri.theme setting, or
 * with none set, Tsuzuri's own theme on a dark VS Code and its light VS Code
 * theme on a light one, so the drawing stays readable on the page.
 */
function drawingTheme() {
  const chosen = settings().get('theme', '').trim();
  if (chosen) return chosen;
  const kind = vscode.window.activeColorTheme.kind;
  const light = kind === vscode.ColorThemeKind.Light || kind === vscode.ColorThemeKind.HighContrastLight;
  return light ? LIGHT_THEME : '';
}

function settings() {
  return vscode.workspace.getConfiguration('tsuzuri');
}

function workspaceRoot() {
  const folders = vscode.workspace.workspaceFolders;
  return folders && folders.length ? folders[0].uri.fsPath : undefined;
}

/** The notes folder a note belongs to: its workspace folder, or its own folder. */
function notesRoot(note) {
  const folder = vscode.workspace.getWorkspaceFolder(note);
  return folder ? folder.uri.fsPath : path.dirname(note.fsPath);
}

/** Reports an error from a bridge call unless the user was already told why. */
function showError(prefix, err) {
  if (err instanceof BridgeUnavailable) return;
  vscode.window.showErrorMessage(`${prefix}: ${err.message}`);
}

module.exports = { Bridge, BridgeUnavailable, settings, showError };
