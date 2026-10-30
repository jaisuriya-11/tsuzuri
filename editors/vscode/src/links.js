'use strict';
// Note names after "[[". A bare name is enough for Tsuzuri to find a note
// anywhere in the notes folder, so the path is only written when two notes
// share a name.

const vscode = require('vscode');
const path = require('path');

const EXCLUDE = '**/{node_modules,.git}/**';
const MAX_NOTES = 5000;

function provider(enabled) {
  return {
    async provideCompletionItems(doc, pos) {
      if (!enabled()) return undefined;
      const before = doc.lineAt(pos.line).text.slice(0, pos.character);
      const m = /\[\[([^[\]|#]*)$/.exec(before);
      if (!m) return undefined;

      const files = await vscode.workspace.findFiles('**/*.md', EXCLUDE, MAX_NOTES);
      const counts = new Map();
      for (const f of files) {
        const name = path.basename(f.fsPath, '.md').toLowerCase();
        counts.set(name, (counts.get(name) || 0) + 1);
      }

      const range = new vscode.Range(pos.line, pos.character - m[1].length, pos.line, pos.character);
      return files
        .filter((f) => f.toString() !== doc.uri.toString())
        .map((f) => {
          const name = path.basename(f.fsPath, '.md');
          const rel = vscode.workspace.asRelativePath(f, false).replace(/\.md$/i, '');
          const item = new vscode.CompletionItem(name, vscode.CompletionItemKind.Reference);
          item.insertText = counts.get(name.toLowerCase()) > 1 ? rel : name;
          item.filterText = `${name} ${rel}`;
          item.detail = rel;
          item.range = range;
          return item;
        });
    },
  };
}

function register(enabled) {
  return vscode.languages.registerCompletionItemProvider({ language: 'markdown' }, provider(enabled), '[');
}

module.exports = { register };
