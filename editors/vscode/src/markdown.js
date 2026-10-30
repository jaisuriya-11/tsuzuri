'use strict';
// Turns a note into HTML for the preview. Prose goes through markdown-it;
// views and equations are drawn by the tsuzuri binary and arrive through
// the `drawn` cache, so this file holds no drawing logic of its own.

const MarkdownIt = require('markdown-it');
const katex = require('katex');
const { renderView } = require('./views');

const CALLOUT = /^\[!(\w+)\][ \t]*/;
const TASK = /^\[([ xX])\][ \t]+/;

function esc(s) {
  return String(s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

/** Cache key for a fenced block. */
function fenceKey(lang, body) {
  return `${lang}\n${body}`;
}

/** Splits a leading "---" block off a note; returns its key/values and the rest. */
function frontMatter(text) {
  const m = /^---\r?\n([\s\S]*?)\r?\n---[ \t]*(?:\r?\n|$)/.exec(text);
  if (!m) return { meta: {}, body: text, lines: 0 };
  const meta = {};
  for (const line of m[1].split(/\r?\n/)) {
    const kv = /^([\w-]+):\s*(.*)$/.exec(line);
    if (kv) meta[kv[1].toLowerCase()] = kv[2].trim().replace(/^["']|["']$/g, '');
  }
  return { meta, body: text.slice(m[0].length), lines: m[0].split('\n').length - 1 };
}

/* ------------------------------------------------------------ wikilinks */

function wikilinkRule(state, silent) {
  const src = state.src;
  let start = state.pos;
  let embed = false;
  if (src.charCodeAt(start) === 0x21 /* ! */ && src.startsWith('[[', start + 1)) {
    embed = true;
    start += 1;
  } else if (!src.startsWith('[[', start)) {
    return false;
  }
  const end = src.indexOf(']]', start + 2);
  if (end < 0) return false;
  const inner = src.slice(start + 2, end);
  if (!inner.trim() || /[[\]\n]/.test(inner)) return false;

  if (!silent) {
    const bar = inner.indexOf('|');
    const target = (bar < 0 ? inner : inner.slice(0, bar)).trim();
    const label = (bar < 0 ? target : inner.slice(bar + 1)).trim();
    const token = state.push('tsuzuri_wikilink', '', 0);
    token.meta = { target, label, embed };
  }
  state.pos = end + 2;
  return true;
}

function renderWikilink(tokens, idx) {
  const { target, label, embed } = tokens[idx].meta;
  const cls = embed ? 'tsz-wikilink is-embed' : 'tsz-wikilink';
  return `<a class="${cls}" href="#" data-target="${esc(target)}" title="${esc(target)}">${esc(label)}</a>`;
}

/* -------------------------------------------------- callouts and to-dos */

/** Marks "> [!NOTE]" quotes as callouts and "- [ ]" items as to-dos. */
function blockMarkers(state) {
  const tokens = state.tokens;
  for (let i = 0; i + 2 < tokens.length; i++) {
    const open = tokens[i];
    const inline = tokens[i + 2];
    if (tokens[i + 1].type !== 'paragraph_open' || inline.type !== 'inline') continue;

    if (open.type === 'blockquote_open') {
      const m = CALLOUT.exec(inline.content);
      if (!m) continue;
      open.meta = { callout: m[1].toLowerCase() };
      stripPrefix(inline, m[0]);
    } else if (open.type === 'list_item_open') {
      const m = TASK.exec(inline.content);
      if (!m) continue;
      open.meta = { task: m[1] !== ' ' };
      stripPrefix(inline, m[0]);
    }
  }
}

/** Removes prefix from the start of an inline token and its first text child. */
function stripPrefix(inline, prefix) {
  inline.content = inline.content.slice(prefix.length);
  const first = inline.children && inline.children[0];
  if (first && first.type === 'text' && first.content.startsWith(prefix)) {
    first.content = first.content.slice(prefix.length);
  } else if (first && first.type === 'text') {
    // The marker ends a line on its own ("> [!NOTE]" then a soft break).
    first.content = first.content.replace(prefix.trimEnd(), '');
  }
}

const CALLOUT_ICONS = { note: 'ℹ️', tip: '💡', important: '❗', warning: '⚠️', caution: '🛑' };

function calloutTitle(kind) {
  return kind.charAt(0).toUpperCase() + kind.slice(1);
}

/* ----------------------------------------------------------------- math */

/**
 * "$$ … $$" equations, on one line or spread over several, become a math
 * fence so they're split and drawn the same way as a ```math block.
 */
function mathBlockRule(state, startLine, endLine, silent) {
  const first = state.src.slice(state.bMarks[startLine] + state.tShift[startLine], state.eMarks[startLine]);
  if (state.sCount[startLine] - state.blkIndent >= 4 || !first.startsWith('$$')) return false;

  const lines = [];
  let rest = first.slice(2);
  let line = startLine;
  for (;;) {
    const end = rest.indexOf('$$');
    if (end >= 0) {
      if (rest.slice(end + 2).trim()) return false; // text after the closing $$
      lines.push(rest.slice(0, end));
      break;
    }
    lines.push(rest);
    line++;
    if (line >= endLine) return false; // never closed
    rest = state.src.slice(state.bMarks[line] + state.tShift[line], state.eMarks[line]);
  }
  if (silent) return true;

  const token = state.push('fence', 'code', 0);
  token.info = 'math';
  token.markup = '$$';
  token.content = lines.map((l) => l.trim()).filter(Boolean).join('\n') + '\n';
  token.map = [startLine, line + 1];
  state.line = line + 1;
  return true;
}

/** Typesets each equation; one KaTeX can't read shows its LaTeX in red. */
function typeset(equations) {
  return equations
    .map((tex) => katex.renderToString(tex, { displayMode: true, throwOnError: false, output: 'html', strict: 'ignore' }))
    .join('\n');
}

/* ------------------------------------------------------- source lines */

/**
 * Tags each top-level block with data-src="start:end", the note lines it
 * came from, so the preview can edit it in place. Blocks rendered inside
 * columns are left alone: their lines are relative to the column.
 */
function sourceLines(state) {
  if (state.env.nested) return;
  const offset = state.env.lineOffset || 0;
  let quoted = 0;
  for (const t of state.tokens) {
    if (t.type === 'blockquote_open') quoted++;
    if (t.type === 'blockquote_close') quoted--;
    if (!t.map) continue;
    if (t.level === 0 && t.nesting !== -1) t.attrSet('data-src', `${t.map[0] + offset}:${t.map[1] + offset}`);
    // A to-do inside a quote starts with "> ", which a tick can't safely rewrite.
    if (t.type === 'list_item_open' && !quoted) t.meta = Object.assign({}, t.meta, { line: t.map[0] + offset });
  }
}

/** Where "[ ]" and "[x]" boxes are in text; "[x](url)" is a link. */
function boxesIn(text) {
  const at = [];
  const re = /\[[ xX]\]/g;
  let m;
  while ((m = re.exec(text))) {
    if (text[m.index + 3] !== '(') at.push(m.index);
  }
  return at;
}

/**
 * A table cell that starts with "[ ]" or "[x]" shows a checkbox, as in the
 * app. Each box knows its row's note line and which box on that line it
 * is, counted the way the binary counts when it ticks one.
 */
function cellBoxes(state) {
  const offset = state.env.lineOffset || 0;
  let row = null;
  let seen = 0;
  for (const t of state.tokens) {
    if (t.type === 'tr_open') {
      row = t.map ? t.map[0] + offset : null;
      seen = 0;
    }
    if (t.type !== 'inline' || row === null || !t.children) continue;
    const before = seen;
    seen += boxesIn(t.content).length;
    const m = /^\[([ xX])\](\s|$)/.exec(t.content);
    const first = t.children[0];
    if (!m || !first || first.type !== 'text' || !first.content.startsWith(m[0].trimEnd())) continue;
    first.content = first.content.slice(m[0].trimEnd().length).replace(/^\s/, '');
    const box = new state.Token('html_inline', '', 0);
    const ticked = m[1] !== ' ';
    const where = state.env.nested ? ' disabled' : ` data-box-line="${row}" data-box="${before}"`;
    box.content = `<input type="checkbox" class="tsz-cell-box"${where}${ticked ? ' checked' : ''}>`;
    t.children.unshift(box);
  }
}

/** Adds data-src to the first tag of rendered HTML. */
function withSource(src, html) {
  return html.replace(/^<(\w+)/, `<$1 data-src="${src}"`);
}

/* ------------------------------------------------------------- renderer */

/**
 * Creates a renderer. `drawn` maps fenceKey(lang, body) to {ok, html} from
 * the binary; `imageSrc` turns a relative image path into a loadable URL.
 * render(text) returns the HTML and the fences not yet in `drawn`.
 */
function createRenderer({ drawn, imageSrc = (s) => s }) {
  const md = new MarkdownIt({ html: true, linkify: true });
  let missing = [];

  md.core.ruler.after('inline', 'tsuzuri_markers', blockMarkers);
  md.core.ruler.push('tsuzuri_source', sourceLines);
  md.core.ruler.push('tsuzuri_cell_boxes', cellBoxes);
  md.inline.ruler.before('link', 'tsuzuri_wikilink', wikilinkRule);
  md.block.ruler.before('fence', 'tsuzuri_math', mathBlockRule, { alt: ['paragraph', 'reference', 'blockquote', 'list'] });
  md.renderer.rules.tsuzuri_wikilink = renderWikilink;

  // Quotes nest, so remember which open quotes were callouts.
  const quotes = [];
  md.renderer.rules.blockquote_open = (tokens, idx, options, env, self) => {
    const kind = tokens[idx].meta && tokens[idx].meta.callout;
    quotes.push(Boolean(kind));
    if (!kind) return self.renderToken(tokens, idx, options);
    const icon = CALLOUT_ICONS[kind] || CALLOUT_ICONS.note;
    const src = tokens[idx].attrGet('data-src');
    const attr = src ? ` data-src="${src}"` : '';
    return `<blockquote class="tsz-callout tsz-callout-${esc(kind)}"${attr}><span class="tsz-callout-icon" aria-hidden="true">${icon}</span>`
      + `<div class="tsz-callout-body"><p class="tsz-callout-title">${esc(calloutTitle(kind))}</p>\n`;
  };
  md.renderer.rules.blockquote_close = (tokens, idx, options, env, self) =>
    quotes.pop() ? '</div></blockquote>\n' : self.renderToken(tokens, idx, options);

  md.renderer.rules.list_item_open = (tokens, idx, options, env, self) => {
    const meta = tokens[idx].meta;
    if (!meta || meta.task === undefined) return self.renderToken(tokens, idx, options);
    const checked = meta.task ? ' checked' : '';
    // Only top-level notes know their line; to-dos inside columns stay read-only.
    const toggle = meta.line === undefined || env.nested ? ' disabled' : ` data-box-line="${meta.line}" data-box="0"`;
    return `<li class="tsz-task${meta.task ? ' is-done' : ''}"><input type="checkbox"${toggle}${checked}> `;
  };

  const image = md.renderer.rules.image;
  md.renderer.rules.image = (tokens, idx, options, env, self) => {
    const token = tokens[idx];
    const src = token.attrGet('src');
    if (src) token.attrSet('src', imageSrc(src));
    return image(tokens, idx, options, env, self);
  };

  const fence = md.renderer.rules.fence;
  md.renderer.rules.fence = (tokens, idx, options, env, self) => {
    const token = tokens[idx];
    const lang = (token.info || '').trim().split(/\s+/)[0].toLowerCase();
    const src = token.attrGet('data-src');
    const attr = src ? ` data-src="${src}"` : '';

    if (lang === 'columns') {
      const parts = [[]];
      for (const l of token.content.replace(/\n$/, '').split('\n')) {
        if (l.trim() === '+++') parts.push([]);
        else parts[parts.length - 1].push(l);
      }
      // Rendering a column parses it separately, so its own fences land in `missing` too.
      const cols = parts.map((p) => `<div class="tsz-column">${md.render(p.join('\n'), { nested: true })}</div>`);
      return `<div class="tsz-columns" style="--tsz-n:${parts.length}"${attr}>${cols.join('')}</div>\n`;
    }

    if (lang) {
      const key = fenceKey(lang, token.content);
      const hit = drawn.get(key);
      if (hit && hit.ok && hit.view) {
        const inline = (text) => md.renderInline(String(text), env);
        const [start, end] = src ? src.split(':').map(Number) : [];
        // The closing fence is the block's last line; boards need both fences to move cards.
        const at = src ? { base: start + 1, block: start, end: end - 1 } : null;
        return `<div class="tsz-view tsz-${esc(hit.view.kind)}"${attr}>${renderView(hit.view, inline, at, env.viewState)}</div>\n`;
      }
      if (hit && hit.ok && hit.math) {
        return `<div class="tsz-math"${attr}>${typeset(hit.math)}</div>\n`;
      }
      if (hit && hit.ok) {
        return `<pre class="tsz-art" data-lang="${esc(lang)}"${attr}>${hit.html}</pre>\n`;
      }
      if (!hit && !missing.some((f) => fenceKey(f.lang, f.body) === key)) {
        missing.push({ lang, body: token.content });
      }
    }
    if (!src) return fence(tokens, idx, options, env, self);
    // The default renderer would put data-src on <code>; the preview wants it on <pre>.
    token.attrs = token.attrs.filter(([name]) => name !== 'data-src');
    return withSource(src, fence(tokens, idx, options, env, self));
  };

  return {
    /**
     * Renders a note. `title` (the file name) heads the page unless the
     * note opens with its own "# heading"; `viewState` is display state for
     * views, such as `months` (calendar fence line to months shown ahead).
     */
    render(text, title = '', viewState = {}) {
      missing = [];
      quotes.length = 0;
      const { meta, body, lines } = frontMatter(text);
      const cover = meta.cover
        ? `<img class="tsz-cover" src="${esc(imageSrc(meta.cover))}" alt="">\n`
        : '';
      const icon = meta.icon ? `<div class="tsz-page-icon">${esc(meta.icon)}</div>\n` : '';
      const heading = title && !/^\s*#\s/.test(body) ? `<h1 class="tsz-page-title">${esc(title)}</h1>\n` : '';
      const html = cover + icon + heading + md.render(body, { lineOffset: lines, viewState });
      return { html, missing, cover: Boolean(meta.cover) };
    },
  };
}

module.exports = { createRenderer, fenceKey, frontMatter };
