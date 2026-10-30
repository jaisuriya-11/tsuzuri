'use strict';
// Rendering without VS Code: `npm test`. Set TSUZURI_BIN to a built binary
// to also draw the demo note's views through "tsuzuri editor render".

const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const { createRenderer, fenceKey, frontMatter } = require('../src/markdown');
const binary = require('../src/binary');

test('prose, callouts, to-dos and links', () => {
  const { render } = createRenderer({ drawn: new Map() });
  const { html } = render([
    '> [!TIP]',
    '> Use the / menu.',
    '',
    '- [ ] open',
    '- [x] done',
    '',
    'See [[Project plan#Goals|the plan]] and ![[Meeting notes]].',
  ].join('\n'), 'Note');

  assert.match(html, /<h1 class="tsz-page-title">Note<\/h1>/);
  assert.match(html, /tsz-callout tsz-callout-tip/);
  assert.match(html, /<p class="tsz-callout-title">Tip<\/p>/);
  assert.doesNotMatch(html, /\[!TIP\]/);
  assert.match(html, /<li class="tsz-task"><input type="checkbox" data-box-line="3" data-box="0"> /);
  assert.match(html, /<li class="tsz-task is-done"><input type="checkbox" data-box-line="4" data-box="0" checked> /);
  assert.match(html, /data-target="Project plan#Goals"[^>]*>the plan<\/a>/);
  assert.match(html, /class="tsz-wikilink is-embed"/);
});

test('a note that opens with a heading keeps it as the title', () => {
  const { render } = createRenderer({ drawn: new Map() });
  assert.doesNotMatch(render('# Plan\n\ntext', 'file name').html, /tsz-page-title/);
});

test('nested quotes inside a callout close correctly', () => {
  const { render } = createRenderer({ drawn: new Map() });
  const { html } = render('> [!NOTE]\n> outer\n> > inner\n\nafter');
  assert.strictEqual((html.match(/<blockquote/g) || []).length, 2);
  assert.match(html, /<\/blockquote>\n<\/div><\/blockquote>/);
});

test('front matter: cover shown, block hidden', () => {
  const { meta, body, lines } = frontMatter('---\ncover: "assets/a.png"\ntitle: x\n---\n# Hi\n');
  assert.deepStrictEqual(meta, { cover: 'assets/a.png', title: 'x' });
  assert.strictEqual(body, '# Hi\n');
  assert.strictEqual(lines, 4);

  const { render } = createRenderer({ drawn: new Map(), imageSrc: (s) => `res:${s}` });
  const out = render('---\ncover: a.png\n---\ntext');
  assert.ok(out.cover);
  assert.match(out.html, /<img class="tsz-cover" src="res:a.png"/);
  assert.doesNotMatch(out.html, /<hr>/);

  const withIcon = render('---\ncover: a.png\nicon: 🫠\n---\ntext', 'Note').html;
  assert.match(withIcon, /<img class="tsz-cover"[^>]*>\n<div class="tsz-page-icon">🫠<\/div>\n<h1 class="tsz-page-title">Note<\/h1>/);
  assert.match(render('---\nicon: "<b>"\n---\n').html, /<div class="tsz-page-icon">&lt;b&gt;<\/div>/);
});

test('fences ask once, then use what the binary drew', () => {
  const drawn = new Map();
  const { render } = createRenderer({ drawn });
  const note = '```board\n## A\n- x\n```\n\n~~~columns\n```chart\nA: 1\n```\n+++\nright\n~~~\n\n```go\nx\n```\n';

  const first = render(note);
  assert.deepStrictEqual(first.missing.map((f) => f.lang), ['board', 'chart', 'go']);
  assert.match(first.html, /language-board/);

  drawn.set(fenceKey('board', '## A\n- x\n'), { ok: true, html: '<span style="color:#fff">BOARD</span>' });
  drawn.set(fenceKey('chart', 'A: 1\n'), { ok: true, html: 'CHART' });
  drawn.set(fenceKey('go', 'x\n'), { ok: false });

  const second = render(note);
  assert.strictEqual(second.missing.length, 0);
  assert.match(second.html, /<pre class="tsz-art" data-lang="board"[^>]*><span style="color:#fff">BOARD<\/span><\/pre>/);
  assert.match(second.html, /<div class="tsz-columns" style="--tsz-n:2"[^>]*><div class="tsz-column"><pre class="tsz-art" data-lang="chart"/);
  assert.match(second.html, /language-go/);
});

test('blocks know their note lines, after the front matter', () => {
  const { render } = createRenderer({ drawn: new Map() });
  const { html } = render('---\ncover: a.png\n---\n# Plan\n\ntext\n\n```go\nx\n```\n\n~~~columns\n- [ ] a\n+++\nb\n~~~\n');
  assert.match(html, /<h1 data-src="3:4">Plan/);
  assert.match(html, /<p data-src="5:6">text/);
  assert.match(html, /<pre data-src="7:10"><code class="language-go">/);
  assert.match(html, /<div class="tsz-columns" style="--tsz-n:2" data-src="11:16">/);
  // Blocks inside columns can't be edited one by one.
  assert.strictEqual((html.match(/data-src/g) || []).length, 4);
  assert.match(html, /<input type="checkbox" disabled>/);
});

test('$$ equations go through the same path as ```math', () => {
  const drawn = new Map();
  const { render } = createRenderer({ drawn });
  const note = 'before\n\n$$\n\\frac{a}{b}\n$$\n\n$$x^2$$\n\nafter';
  const first = render(note);
  assert.deepStrictEqual(first.missing, [
    { lang: 'math', body: '\\frac{a}{b}\n' },
    { lang: 'math', body: 'x^2\n' },
  ]);
  drawn.set(fenceKey('math', '\\frac{a}{b}\n'), { ok: true, html: 'a/b', math: ['\\frac{a}{b}'] });
  drawn.set(fenceKey('math', 'x^2\n'), { ok: true, html: 'x²', math: ['x^2', '\\badcommand{'] });

  const { html } = render(note);
  assert.match(html, /<div class="tsz-math" data-src="2:5">/);
  assert.match(html, /<div class="tsz-math" data-src="6:7">/);
  assert.match(html, /class="mfrac"/);
  assert.match(html, /katex-error/); // a broken equation shows its LaTeX instead of failing
  assert.match(html, /<p data-src="8:9">after<\/p>/);
});

test('an unclosed $$ stays text', () => {
  const { render } = createRenderer({ drawn: new Map() });
  const out = render('$$ x\n\nmore');
  assert.strictEqual(out.missing.length, 0);
  assert.match(out.html, /<p data-src="0:1">\$\$ x<\/p>/);
});

test('views are drawn from the data the binary reads', () => {
  const drawn = new Map();
  const { render } = createRenderer({ drawn });
  const note = 'x\n\n```board\n## Todo\n- <b>a</b>\n## Done\n```\n';
  drawn.set(fenceKey('board', '## Todo\n- <b>a</b>\n## Done\n'), {
    ok: true,
    view: {
      kind: 'board',
      board: [
        { name: 'Todo', line: 0, tone: 'todo', cards: [{ title: '<b>a</b>', line: 1 }] },
        { name: 'Done', line: 2, tone: 'done', cards: [] },
      ],
    },
  });
  const { html } = render(note);
  assert.match(html, /<div class="tsz-view tsz-board" data-src="2:7"><div class="tsz-board-cols" data-board="2:6">/);
  // Card lines are note lines: the fence is on line 2, so its body starts on 3.
  assert.match(html, /data-card-line="4"/);
  // Card titles show as Markdown and are edited as typed.
  assert.match(html, /<span class="tsz-editable" tabindex="0" data-edit="board.rename" data-raw="&lt;b&gt;a&lt;\/b&gt;" data-line="4"><b>a<\/b><\/span>/);
  // Edits name note lines: the column heading is on line 3.
  assert.match(html, /data-act="board.deletecolumn" data-line="3">/);
  assert.match(html, /data-edit="board.add" data-raw="" data-new="1" data-index="0"/);
  assert.match(html, /data-edit="board.column"[^>]*data-placeholder="Column name"/);

  // Inside columns the same view can't be placed in the note, so it's read-only.
  const nested = render('~~~columns\n```board\n## Todo\n- <b>a</b>\n## Done\n```\n+++\nx\n~~~').html;
  assert.doesNotMatch(nested, /data-edit|data-act|draggable/);
});

test('checkboxes in table cells count boxes the way the binary does', () => {
  const { render } = createRenderer({ drawn: new Map() });
  const { html } = render('| a | b |\n| --- | --- |\n| [ ] one | [x](link) and [x] two |\n| x [ ] | [x] three |');
  assert.match(html, /<td><input type="checkbox" class="tsz-cell-box" data-box-line="2" data-box="0">one<\/td>/);
  // The link isn't a box; "[x] two" isn't at the start of its cell, so it stays text.
  assert.match(html, /<td><a href="link">x<\/a> and \[x\] two<\/td>/);
  // On the next row the first cell's mid-text box still counts, so this is box 1.
  assert.match(html, /<td><input type="checkbox" class="tsz-cell-box" data-box-line="3" data-box="1" checked>three<\/td>/);
});

test('html in link text is escaped', () => {
  const { render } = createRenderer({ drawn: new Map() });
  assert.match(render('[[<b>x</b>]]').html, /&lt;b&gt;x&lt;\/b&gt;/);
});

const bin = process.env.TSUZURI_BIN;
test('every view draws through the tsuzuri binary', { skip: !bin && 'set TSUZURI_BIN' }, async () => {
  const drawn = new Map();
  const { render } = createRenderer({ drawn });
  const note = fs.readFileSync(path.join(__dirname, 'fixtures', 'views.md'), 'utf8');

  const { missing } = render(note);
  const { protocol } = await binary.request(bin, 'version');
  assert.strictEqual(protocol, 4);
  const { fences } = await binary.request(bin, 'render', { width: 80, fences: missing });
  missing.forEach((f, i) => drawn.set(fenceKey(f.lang, f.body), fences[i]));

  const { html } = render(note);
  for (const kind of ['board', 'calendar', 'timeline', 'chart', 'form']) {
    assert.match(html, new RegExp(`class="tsz-view tsz-${kind}"`), `${kind} not drawn`);
  }
  assert.match(html, /class="tsz-art" data-lang="flow"/);
  assert.match(html, /<div class="tsz-board-cols" data-board="\d+:\d+">/);
  assert.match(html, /class="tsz-card" draggable="true" data-card-line="\d+"/);
  // Equations are typeset: a stacked fraction, not "(n(n + 1))/2".
  assert.match(html, /<div class="tsz-math"[^>]*>.*class="mfrac"/s);
  assert.doesNotMatch(html, /katex-error/);
  assert.doesNotMatch(html, /\x1b/);
  fs.writeFileSync(path.join(__dirname, 'demo.html'), `<!doctype html><meta charset="utf-8"><link rel="stylesheet" href="../media/preview.css"><main class="tsz-page">${html}</main>`);
});
