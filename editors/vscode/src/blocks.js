'use strict';
// Text rules for the "/" menu that need no VS Code API, kept apart so they
// can be tested with plain Node.

/** The "/query" being typed before the cursor, or null when the menu shouldn't open. */
function slashQuery(before) {
  const m = /(^|\s)\/([\w-]*)$/.exec(before);
  if (!m) return null;
  return { start: before.length - m[2].length - 1, query: m[2] };
}

/**
 * True when the line after `lines` sits inside a code fence. A ~~~columns
 * block holds Markdown, not code, so the menu works inside it (but not in a
 * fence within it).
 */
function insideFence(lines) {
  const open = []; // the fences around the line, innermost last
  for (const line of lines) {
    const m = /^\s*(`{3,}|~{3,})\s*([\w-]*)/.exec(line);
    if (!m) continue;
    const top = open[open.length - 1];
    if (top && top.marker === m[1][0] && !m[2] && !(top.columns && m[1][0] === '`')) {
      open.pop();
    } else if (!top || top.columns) {
      open.push({ marker: m[1][0], columns: m[2].toLowerCase() === 'columns' });
    }
  }
  return open.some((f) => !f.columns);
}

function isTableLine(line) {
  return line.trim().startsWith('|');
}

function escapeSnippet(s) {
  return s.replace(/[$}\\]/g, '\\$&');
}

/**
 * The snippet for a block: its text around the cursor ($0). A block typed
 * after other text starts on its own line, as in the app.
 */
function snippetFor(block, midLine, inCell = false) {
  if (inCell) return escapeSnippet(block.cellBefore || '') + '$0' + escapeSnippet(block.cellAfter || '');
  const lead = midLine && block.block ? '\n' : '';
  return lead + escapeSnippet(block.before || '') + '$0' + escapeSnippet(block.after || '');
}

/** The menu entries for a spot: inside a table cell only what fits there, as in the app. */
function blocksFor(blocks, inTable) {
  return blocks.filter((b) => (inTable ? b.section === 'Table' || b.cell : b.section !== 'Table'));
}

module.exports = { slashQuery, insideFence, isTableLine, escapeSnippet, snippetFor, blocksFor };
