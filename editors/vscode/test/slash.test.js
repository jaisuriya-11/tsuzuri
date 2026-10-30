'use strict';

const test = require('node:test');
const assert = require('node:assert');
const { slashQuery, insideFence, isTableLine, snippetFor, blocksFor } = require('../src/blocks');

test('the menu opens after "/" at a line start or after a space', () => {
  assert.deepStrictEqual(slashQuery('/'), { start: 0, query: '' });
  assert.deepStrictEqual(slashQuery('  /boa'), { start: 2, query: 'boa' });
  assert.deepStrictEqual(slashQuery('text /h1'), { start: 5, query: 'h1' });
  assert.strictEqual(slashQuery('and/or'), null);
  assert.strictEqual(slashQuery('/a b'), null);
});

test('fences', () => {
  assert.strictEqual(insideFence(['```board', '- a']), true);
  assert.strictEqual(insideFence(['```', 'x', '```']), false);
  // Columns hold Markdown: the menu works there, but not in code inside them.
  assert.strictEqual(insideFence(['~~~columns', 'left']), false);
  assert.strictEqual(insideFence(['~~~columns', '```chart']), true);
  assert.strictEqual(insideFence(['~~~columns', '```chart', '```']), false);
  assert.strictEqual(insideFence(['~~~columns', '```chart', '```', '~~~']), false);
  assert.strictEqual(insideFence(['~~~', 'code']), true);
});

test('tables', () => {
  assert.ok(isTableLine('  | a | b |'));
  assert.ok(!isTableLine('a | b'));
});

test('inside a table only cell-safe entries are offered', () => {
  const blocks = [
    { section: 'Table', label: 'Add row below' },
    { section: 'Basic blocks', label: 'Heading 1' },
    { section: 'Basic blocks', label: 'To-do list', cell: true, before: '- [ ] ', cellBefore: '[ ] ' },
  ];
  assert.deepStrictEqual(blocksFor(blocks, true).map((b) => b.label), ['Add row below', 'To-do list']);
  assert.deepStrictEqual(blocksFor(blocks, false).map((b) => b.label), ['Heading 1', 'To-do list']);
  assert.strictEqual(snippetFor(blocks[2], true, true), '[ ] $0');
});

test('snippets keep the cursor between before and after', () => {
  const board = { before: '```board\n- ', after: '\n```', block: true };
  assert.strictEqual(snippetFor(board, false), '```board\n- $0\n```');
  assert.strictEqual(snippetFor(board, true), '\n```board\n- $0\n```');
  assert.strictEqual(snippetFor({ before: '[[', after: ']]' }, true), '[[$0]]');
  assert.strictEqual(snippetFor({ before: '$x}', after: '\\' }, false), '\\$x\\}$0\\\\');
});
