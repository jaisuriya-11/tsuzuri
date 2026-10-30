'use strict';

const vscode = require('vscode');
const path = require('path');
const binary = require('./src/binary');
const { Bridge, settings, showError } = require('./src/bridge');
const { PreviewManager } = require('./src/preview');
const slash = require('./src/slash');
const links = require('./src/links');

async function chooseTheme(bridge) {
  const bin = await bridge.binary();
  if (!bin) return;

  let names;
  try {
    names = (await binary.run(bin, ['--list-themes'])).split(/\r?\n/).map((s) => s.trim()).filter(Boolean);
  } catch (err) {
    showError("Couldn't list Tsuzuri themes", err);
    return;
  }

  const current = settings().get('theme', '').trim();
  const items = [
    { label: 'Use my Tsuzuri config', description: current ? '' : 'current', value: '' },
    { label: '', kind: vscode.QuickPickItemKind.Separator },
    ...names.map((n) => ({ label: n, description: n === current ? 'current' : '', value: n })),
  ];
  const picked = await vscode.window.showQuickPick(items, { placeHolder: `Pick one of ${names.length} themes` });
  if (!picked) return;
  await settings().update('theme', picked.value || undefined, vscode.ConfigurationTarget.Global);
}

async function exportPdf(bridge, uri) {
  const editor = vscode.window.activeTextEditor;
  const target = uri instanceof vscode.Uri ? uri : editor && editor.document.uri;
  if (!target || target.scheme !== 'file' || path.extname(target.fsPath).toLowerCase() !== '.md') {
    vscode.window.showErrorMessage('Open a saved Markdown note to export it.');
    return;
  }

  const doc = vscode.workspace.textDocuments.find((d) => d.uri.toString() === target.toString());
  if (doc && doc.isDirty && !(await doc.save())) return;

  const bin = await bridge.binary();
  if (!bin) return;

  const dir = path.dirname(target.fsPath);
  const out = path.join(dir, `${path.basename(target.fsPath, path.extname(target.fsPath))}.pdf`);
  try {
    await vscode.window.withProgress(
      { location: vscode.ProgressLocation.Notification, title: `Exporting ${path.basename(target.fsPath)}` },
      () => binary.run(bin, ['export', target.fsPath, '-o', out], dir)
    );
  } catch (err) {
    showError('Export failed', err);
    return;
  }

  const choice = await vscode.window.showInformationMessage(`Exported ${path.basename(out)}`, 'Open PDF', 'Show in folder');
  if (choice === 'Open PDF') vscode.env.openExternal(vscode.Uri.file(out));
  if (choice === 'Show in folder') vscode.commands.executeCommand('revealFileInOS', vscode.Uri.file(out));
}

function install(bridge, onInstalled) {
  const { shellPath, text } = binary.installCommand();
  const terminal = vscode.window.createTerminal({ name: 'Install Tsuzuri', shellPath });
  terminal.show();
  terminal.sendText(text, true);
  // Look for the binary again once the install has had a chance to finish.
  const sub = vscode.window.onDidCloseTerminal((t) => {
    if (t !== terminal) return;
    sub.dispose();
    bridge.reset();
    onInstalled();
  });
  vscode.window.showInformationMessage('When the install finishes, reopen the preview.');
}

function activate(context) {
  const bridge = new Bridge();
  const previews = new PreviewManager(context, bridge);
  const slashEnabled = () => settings().get('slashMenu', true);

  context.subscriptions.push(
    previews.register(),
    slash.register(bridge, slashEnabled),
    links.register(slashEnabled),
    vscode.commands.registerCommand('tsuzuri.chooseTheme', () => chooseTheme(bridge)),
    vscode.commands.registerCommand('tsuzuri.exportPdf', (uri) => exportPdf(bridge, uri)),
    vscode.commands.registerCommand('tsuzuri.install', () => install(bridge, () => previews.refreshAll())),
    // Flowcharts are drawn in colours that suit the theme, so redraw on a switch.
    vscode.window.onDidChangeActiveColorTheme(() => previews.refreshAll()),
    vscode.workspace.onDidChangeConfiguration((e) => {
      if (e.affectsConfiguration('tsuzuri.path')) bridge.reset();
      if (e.affectsConfiguration('tsuzuri.path') || e.affectsConfiguration('tsuzuri.theme')) previews.refreshAll();
    })
  );
}

function deactivate() {}

module.exports = { activate, deactivate };
