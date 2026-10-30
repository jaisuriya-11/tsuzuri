// Runs inside the Tsuzuri preview. Shows each render, keeps the scroll
// position, reports how many monospace cells fit across, follows links, and
// lets any block be edited in place: click it to see its Markdown, then
// click away or press Escape to write it back to the note.
(function () {
  const vscode = acquireVsCodeApi();
  const page = document.getElementById('page');
  const APPEND = Number.MAX_SAFE_INTEGER; // "after the last line"
  const SETTLE_MS = 1500; // how long a written edit waits for the new render

  let state = vscode.getState() || {};
  let latest = null; // the newest render from the extension
  let editor = null; // the block being edited, if any
  let pendingBlock = null; // a block whose Markdown was asked for
  let requestId = 0;
  let hovered = null; // the top-level block the handles belong to
  let dragBlock = null; // {start, end, el} while a block is dragged
  let dragCard = null; // {line, board, el} while a card is dragged

  // One pair of handles follows the block under the mouse: ⠿ drags it,
  // + adds a block below it.
  const handles = document.createElement('div');
  handles.className = 'tsz-handles';
  handles.innerHTML = '<button class="tsz-handle" data-plus title="Add a block below">'
    + '<svg viewBox="0 0 16 16" aria-hidden="true"><path d="M8 3v10M3 8h10"/></svg></button>'
    + '<button class="tsz-handle" data-grip draggable="true" title="Drag to move">'
    + '<svg viewBox="0 0 16 16" aria-hidden="true"><circle cx="5.5" cy="3.5" r="1.2"/><circle cx="10.5" cy="3.5" r="1.2"/>'
    + '<circle cx="5.5" cy="8" r="1.2"/><circle cx="10.5" cy="8" r="1.2"/>'
    + '<circle cx="5.5" cy="12.5" r="1.2"/><circle cx="10.5" cy="12.5" r="1.2"/></svg></button>';
  document.body.appendChild(handles);
  const dropLine = document.createElement('div');
  dropLine.className = 'tsz-drop-line';
  document.body.appendChild(dropLine);

  function save(patch) {
    state = Object.assign({}, state, patch);
    vscode.setState(state);
  }

  /* ------------------------------------------------------------ width */

  /** Monospace cells that fit inside a view's card, for drawing views. */
  function columns() {
    const probe = document.createElement('pre');
    probe.className = 'tsz-art tsz-probe';
    const run = document.createElement('span');
    run.textContent = '0'.repeat(100);
    probe.appendChild(run);
    page.appendChild(probe);
    const style = getComputedStyle(probe);
    const inner = probe.clientWidth - parseFloat(style.paddingLeft) - parseFloat(style.paddingRight);
    const cell = run.getBoundingClientRect().width / 100 || 8;
    probe.remove();
    return Math.max(24, Math.floor(inner / cell));
  }

  let lastCols = 0;
  function reportWidth() {
    const cols = columns();
    if (cols === lastCols) return;
    lastCols = cols;
    vscode.postMessage({ type: 'width', cols });
  }

  let resizeTimer;
  window.addEventListener('resize', () => {
    clearTimeout(resizeTimer);
    resizeTimer = setTimeout(reportWidth, 150);
  });

  /* ------------------------------------------------------------ render */

  let restored = false;
  function show(update) {
    hovered = null;
    handles.classList.remove('is-visible');
    page.innerHTML = update.html + '<div class="tsz-add" data-add>Add a block</div>';
    page.classList.toggle('has-cover', Boolean(update.cover));
    if (state.uri !== update.uri) save({ uri: update.uri, scrollY: 0 });
    if (!restored) {
      restored = true;
      window.scrollTo(0, state.scrollY || 0);
    }
  }

  let scrollFrame;
  window.addEventListener('scroll', () => {
    cancelAnimationFrame(scrollFrame);
    scrollFrame = requestAnimationFrame(() => save({ scrollY: window.scrollY }));
  });

  /* ----------------------------------------------------------- editing */

  function fitHeight(area) {
    area.style.height = 'auto';
    area.style.height = `${area.scrollHeight + 2}px`;
  }

  /** Swaps a block (or the add row) for a text box holding its Markdown. */
  function openEditor(target, start, end, text, version, mode = 'replace') {
    const box = document.createElement('div');
    box.className = 'tsz-editing';
    const area = document.createElement('textarea');
    area.value = text;
    area.spellcheck = true;
    area.placeholder = 'Type Markdown';
    if (/^(```|~~~|\||<)/.test(text.trimStart())) area.classList.add('is-code');
    box.appendChild(area);
    const hint = document.createElement('div');
    hint.className = 'tsz-editing-hint';
    hint.textContent = 'Esc or click outside to save · Shift+Esc to discard';
    box.appendChild(hint);
    target.replaceWith(box);

    editor = { box, area, start, end, original: text, version, mode, settling: false };
    fitHeight(area);
    area.focus();
    area.setSelectionRange(area.value.length, area.value.length);

    area.addEventListener('input', () => fitHeight(area));
    area.addEventListener('keydown', (e) => {
      if (e.key === 'Escape') {
        e.preventDefault();
        closeEditor(!e.shiftKey);
      } else if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        closeEditor(true);
      } else if (e.key === 'Tab') {
        // Indent rather than leave the box, as lists need it.
        e.preventDefault();
        area.setRangeText('  ', area.selectionStart, area.selectionEnd, 'end');
        fitHeight(area);
      }
    });
    area.addEventListener('blur', () => closeEditor(true));
  }

  /**
   * Paragraphs and headings are edited where they are, in their own style:
   * the text is shown as typed (with its **marks**) but without the "# ".
   */
  function canEditInline(block, text) {
    return /^(P|H[1-6])$/.test(block.tagName) && !/!\[|<|^\s*([-*+]|\d+[.)])\s/.test(text);
  }

  function openInline(block, start, end, text, version, mode = 'replace') {
    const m = /^(\s*#{1,6}\s+)/.exec(text);
    const prefix = m ? m[1] : '';
    const body = text.slice(prefix.length);
    block.textContent = body;
    block.classList.add('is-inline-editing');
    block.contentEditable = 'plaintext-only';
    // The same shape as the text box, so saving works the same way.
    const area = {
      get value() { return block.innerText.replace(/\n+$/, ''); },
      set readOnly(v) { block.contentEditable = v ? 'false' : 'plaintext-only'; },
      focus() { block.focus(); },
    };
    editor = { box: block, area, start, end, original: body, prefix, version, mode, settling: false };
    block.focus();
    const range = document.createRange();
    range.selectNodeContents(block);
    range.collapse(false);
    const sel = window.getSelection();
    sel.removeAllRanges();
    sel.addRange(range);

    block.addEventListener('keydown', (e) => {
      if (menu.open && menuKey(e)) return;
      if (e.key === 'Escape') {
        e.preventDefault();
        closeEditor(!e.shiftKey);
      } else if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        closeEditor(true);
      }
    });
    block.addEventListener('input', updateMenu);
    block.addEventListener('blur', () => {
      // A click on the block menu takes focus away for a moment; it decides.
      // Once a pick has swapped this line for something else, it's done.
      if (!menu.choosing && editor && editor.box === block) closeEditor(true);
    });
  }

  /** An empty line to type into, after `after` (or at the end of the note). */
  function openNewBlock(after, start, end) {
    if (!latest || editor) return;
    const p = document.createElement('p');
    p.className = 'tsz-new-block';
    p.dataset.placeholder = "Type '/' for blocks";
    if (after) after.after(p);
    else page.querySelector('[data-add]').before(p);
    placeHandles(null);
    openInline(p, start, end, '', latest.version, 'insert');
  }

  /* -------------------------------------------------------- block menu */

  // Typing "/" on a new line opens Tsuzuri's own "/" menu, as in the app.
  const LINE_STARTS = { '# ': 'h1', '## ': 'h2', '### ': 'h3', '#### ': 'h4', '- ': 'bullet', '1. ': 'number', '- [ ] ': 'todo', '> ': 'quote' };
  const VIEWS = /^```(board|calendar|timeline|chart|form)\n/;
  const menu = { el: document.createElement('div'), open: false, items: [], sel: 0, choosing: false };
  menu.el.className = 'tsz-menu';
  menu.el.setAttribute('role', 'listbox');
  document.body.appendChild(menu.el);
  let blocks = []; // from the binary, via the extension

  function hideMenu() {
    menu.open = false;
    menu.el.classList.remove('is-open');
  }

  function updateMenu() {
    if (!editor || editor.mode !== 'insert' || editor.prefix) return hideMenu();
    const m = /^\/([^\s/]*)$/.exec(editor.area.value);
    if (!m || !blocks.length) return hideMenu();
    const q = m[1].toLowerCase();
    menu.items = blocks.filter((b) => b.section !== 'Table' && `${b.label} ${b.aliases || ''}`.toLowerCase().includes(q));
    menu.sel = 0;
    drawMenu();
  }

  function drawMenu() {
    if (!menu.items.length) {
      menu.el.innerHTML = '<div class="tsz-menu-empty">No blocks match</div>';
    } else {
      let section = '';
      menu.el.innerHTML = menu.items.map((b, i) => {
        const head = b.section !== section ? `<div class="tsz-menu-section">${escapeHtml((section = b.section))}</div>` : '';
        const hint = b.hint ? `<span class="tsz-menu-hint">${escapeHtml(b.hint)}</span>` : '';
        return `${head}<div class="tsz-menu-item${i === menu.sel ? ' is-on' : ''}" role="option" data-i="${i}">${escapeHtml(b.label)}${hint}</div>`;
      }).join('');
    }
    const r = editor.box.getBoundingClientRect();
    menu.el.style.left = `${r.left + window.scrollX}px`;
    menu.el.style.top = `${r.bottom + window.scrollY + 6}px`;
    menu.el.classList.add('is-open');
    menu.open = true;
    const on = menu.el.querySelector('.is-on');
    if (on) on.scrollIntoView({ block: 'nearest' });
  }

  /** Arrow keys, Enter and Escape while the menu is open. */
  function menuKey(e) {
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      if (menu.items.length) {
        menu.sel = (menu.sel + (e.key === 'ArrowDown' ? 1 : -1) + menu.items.length) % menu.items.length;
        drawMenu();
      }
      return true;
    }
    if (e.key === 'Enter' || e.key === 'Tab') {
      e.preventDefault();
      if (menu.items.length) choose(menu.items[menu.sel]);
      return true;
    }
    if (e.key === 'Escape') {
      e.preventDefault();
      hideMenu();
      return true;
    }
    return false;
  }

  menu.el.addEventListener('mousedown', (e) => {
    const item = e.target.closest('[data-i]');
    if (!item) return;
    e.preventDefault(); // keep the line focused
    menu.choosing = true;
    choose(menu.items[Number(item.dataset.i)]);
    menu.choosing = false;
  });

  function escapeHtml(s) {
    return String(s).replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
  }

  /** What picking a block does: start a styled line, insert a view, or open a box. */
  function choose(block) {
    hideMenu();
    const current = editor;
    if (!current) return;
    const before = block.before || '';
    const after = block.after || '';

    if (block.action) {
      // Pages, links and files need VS Code's dialogs; the extension inserts the result.
      editor = null;
      vscode.postMessage({ type: 'slash', action: block.action, start: current.start, end: current.end, version: current.version });
      if (latest) show(latest);
      return;
    }
    if (LINE_STARTS[before] && !after) {
      // Keep typing on the same line, now styled as what it will be.
      current.prefix = before;
      current.box.textContent = '';
      current.box.dataset.kind = LINE_STARTS[before];
      current.box.dataset.placeholder = block.label;
      current.box.focus();
      return;
    }
    if (VIEWS.test(before) || before === '---\n') {
      // Views are filled in through their own controls once they're on the page.
      current.forced = before + after;
      closeEditor(true);
      return;
    }
    // Anything else (code, tables, equations…) is typed as Markdown in a box.
    editor = null;
    openEditor(current.box, current.start, current.end, before + after, current.version, 'insert');
    editor.area.setSelectionRange(before.length, before.length);
    editor.original = null; // even unchanged, it's a new block
  }

  /** Writes the edit (when `keep` and changed) and goes back to the page. */

  /** Writes the edit (when `keep` and changed) and goes back to the page. */
  function closeEditor(keep) {
    if (!editor || editor.settling) return;
    const current = editor;
    hideMenu();
    const text = current.forced !== undefined ? current.forced : current.area.value;
    if (!keep || (current.forced === undefined && text === current.original) || (current.mode === 'insert' && !text.trim())) {
      editor = null;
      if (latest) show(latest);
      return;
    }
    current.id = ++requestId;
    vscode.postMessage({
      type: current.mode === 'insert' ? 'insert' : 'apply',
      id: current.id,
      start: current.start,
      end: current.end,
      text: current.forced !== undefined ? text : (current.prefix || '') + text,
      version: current.version,
    });
    // Keep the text on screen until the extension answers.
    current.settling = true;
    current.area.readOnly = true;
    current.box.classList.add('is-saving');
  }

  /** The extension's answer to an edit: saved, or refused with the draft kept. */
  function applied(msg) {
    if (!editor || editor.id !== msg.id) return;
    const current = editor;
    if (!msg.ok) {
      current.settling = false;
      current.area.readOnly = false;
      current.box.classList.remove('is-saving');
      current.box.classList.add('is-refused');
      current.area.focus();
      return;
    }
    // Show the new render when it comes, or the latest one if the edit changed nothing visible.
    setTimeout(() => {
      if (editor !== current) return;
      editor = null;
      if (latest) show(latest);
    }, SETTLE_MS);
  }

  /** Asks for a block's Markdown to edit it; `asMarkdown` skips editing in place. */
  function startEditing(block, asMarkdown = false) {
    const [start, end] = block.dataset.src.split(':').map(Number);
    const id = ++requestId;
    pendingBlock = { id, block, asMarkdown };
    vscode.postMessage({ type: 'source', id, start, end, version: latest && latest.version });
  }

  /* ---------------------------------------------------------- messages */

  window.addEventListener('message', (event) => {
    const msg = event.data;
    if (!msg) return;
    if (msg.type === 'update') {
      const fresh = latest && msg.version !== latest.version;
      latest = msg;
      if (!editor) {
        show(msg);
      } else if (editor.settling && fresh) {
        editor = null;
        show(msg);
      }
    } else if (msg.type === 'source') {
      if (!pendingBlock || pendingBlock.id !== msg.id || editor) return;
      const { block, asMarkdown } = pendingBlock;
      pendingBlock = null;
      // A stale answer means the note moved on; the new render is on its way.
      if (msg.stale || !latest || msg.version !== latest.version || !block.isConnected) return;
      if (!asMarkdown && canEditInline(block, msg.text)) openInline(block, msg.start, msg.end, msg.text, msg.version);
      else openEditor(block, msg.start, msg.end, msg.text, msg.version);
    } else if (msg.type === 'applied') {
      applied(msg);
    } else if (msg.type === 'blocks' && Array.isArray(msg.blocks)) {
      blocks = msg.blocks;
    }
  });

  /* ------------------------------------------------------------ clicks */

  page.addEventListener('change', (e) => {
    const box = e.target.closest('input[data-box-line]');
    if (!box || !latest) return;
    vscode.postMessage({ type: 'toggle', line: Number(box.dataset.boxLine), box: Number(box.dataset.box), version: latest.version });
  });

  document.addEventListener('click', (e) => {
    const target = e.target;
    if (editor && editor.box.contains(target)) return;

    const link = target.closest('a');
    if (link) {
      if (link.dataset.target !== undefined) {
        e.preventDefault();
        vscode.postMessage({ type: 'wikilink', target: link.dataset.target });
        return;
      }
      const href = link.getAttribute('href');
      if (!href || href.startsWith('#')) return;
      e.preventDefault();
      vscode.postMessage({ type: 'link', href });
      return;
    }
    if (target.closest('input, textarea, select, summary, button, [contenteditable="plaintext-only"]')) return;
    if (target.closest('.tsz-view')) return; // views are edited through their own controls
    // Selecting text to copy shouldn't open the editor.
    if (String(window.getSelection() || '').length > 0) return;
    if (editor) return; // the blur that closes it handles this click

    if (target.closest('[data-add]')) {
      openNewBlock(null, APPEND, APPEND);
      return;
    }
    const block = target.closest('[data-src]');
    if (block && page.contains(block)) startEditing(block);
  });

  /* ------------------------------------------------- dragging blocks */

  function range(el) {
    const [start, end] = el.dataset.src.split(':').map(Number);
    return { start, end };
  }

  const HANDLE = 24; // px, the size of each handle button
  const GUTTER = 52; // px left of the text where the handles sit

  /**
   * The top-level block at a height on the page. Only the height counts, so
   * the margin beside a block, where the handles are, still belongs to it.
   */
  function blockAtY(y) {
    for (const el of page.children) {
      if (!el.dataset || !el.dataset.src) continue;
      const r = el.getBoundingClientRect();
      if (y >= r.top - 4 && y <= r.bottom + 4) return el;
    }
    return null;
  }

  /** Puts the handles beside the block's first line of text. */
  function placeHandles(block) {
    hovered = block;
    if (!block || editor) {
      handles.classList.remove('is-visible');
      return;
    }
    const r = block.getBoundingClientRect();
    const style = getComputedStyle(block);
    const pageStyle = getComputedStyle(page);
    const textLeft = page.getBoundingClientRect().left + parseFloat(pageStyle.paddingLeft);
    const lineHeight = parseFloat(style.lineHeight) || parseFloat(style.fontSize) * 1.6 || HANDLE;
    const firstLine = Math.min(lineHeight, r.height);
    const top = r.top + parseFloat(style.paddingTop || 0) + Math.max(0, (firstLine - HANDLE) / 2);
    handles.style.left = `${textLeft + window.scrollX - GUTTER}px`;
    handles.style.top = `${top + window.scrollY}px`;
    handles.classList.add('is-visible');
  }

  document.addEventListener('mousemove', (e) => {
    if (dragBlock || dragCard || editor) return;
    if (handles.contains(e.target)) return; // on the handles: keep them
    const r = page.getBoundingClientRect();
    const inside = e.clientX >= r.left && e.clientX <= r.right;
    const block = inside ? blockAtY(e.clientY) : null;
    // In the small gap between two blocks, keep the last one rather than flicker.
    if (!block && inside && hovered && hovered.isConnected) return;
    if (block !== hovered) placeHandles(block);
  });
  document.addEventListener('mouseleave', () => !dragBlock && placeHandles(null));
  window.addEventListener('scroll', () => hovered && placeHandles(hovered));

  handles.querySelector('[data-plus]').addEventListener('click', (e) => {
    e.stopPropagation();
    if (!hovered || !latest || editor) return;
    const { start, end } = range(hovered);
    openNewBlock(hovered, start, end);
  });

  /* ------------------------------------------------- block actions */

  // Clicking ⠿ (rather than dragging it) opens the block's own menu.
  const actions = document.createElement('div');
  actions.className = 'tsz-menu tsz-block-menu';
  actions.innerHTML = [
    ['markdown', 'Edit as Markdown'],
    ['duplicate', 'Duplicate'],
    ['delete', 'Delete'],
  ].map(([a, label]) => `<div class="tsz-menu-item${a === 'delete' ? ' is-danger' : ''}" data-block-action="${a}">${label}</div>`).join('');
  document.body.appendChild(actions);
  let actionsFor = null;

  function hideActions() {
    actions.classList.remove('is-open');
    actionsFor = null;
  }

  handles.querySelector('[data-grip]').addEventListener('click', (e) => {
    e.stopPropagation();
    if (!hovered || editor) return;
    actionsFor = hovered;
    const r = e.currentTarget.getBoundingClientRect();
    actions.style.left = `${r.left + window.scrollX}px`;
    actions.style.top = `${r.bottom + window.scrollY + 4}px`;
    actions.classList.add('is-open');
  });

  actions.addEventListener('mousedown', (e) => {
    const item = e.target.closest('[data-block-action]');
    if (!item || !actionsFor || !latest) return;
    e.preventDefault();
    const block = actionsFor;
    const { start, end } = range(block);
    hideActions();
    const action = item.dataset.blockAction;
    if (action === 'markdown') startEditing(block, true);
    else vscode.postMessage({ type: 'block', action, start, end, version: latest.version });
  });

  document.addEventListener('mousedown', (e) => {
    if (actionsFor && !actions.contains(e.target) && !handles.contains(e.target)) hideActions();
  });
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && actionsFor) hideActions();
  });

  const grip = handles.querySelector('[data-grip]');
  grip.addEventListener('dragstart', (e) => {
    if (!hovered || !latest) return e.preventDefault();
    dragBlock = Object.assign(range(hovered), { el: hovered });
    hovered.classList.add('is-dragging-block');
    e.dataTransfer.effectAllowed = 'move';
    e.dataTransfer.setData('text/plain', 'tsuzuri-block');
    e.dataTransfer.setDragImage(hovered, 0, 0);
  });
  grip.addEventListener('dragend', endBlockDrag);

  /** The gap nearest the pointer: before a block, or after the last one. */
  function blockGap(y) {
    const blocks = [...page.children].filter((el) => el.dataset && el.dataset.src);
    for (const el of blocks) {
      const r = el.getBoundingClientRect();
      if (y < r.top + r.height / 2) return { to: range(el).start, top: r.top, el };
    }
    const last = blocks[blocks.length - 1];
    return last ? { to: APPEND, top: last.getBoundingClientRect().bottom, el: null } : null;
  }

  function endBlockDrag() {
    if (dragBlock) dragBlock.el.classList.remove('is-dragging-block');
    dragBlock = null;
    dropLine.style.display = 'none';
  }

  /* -------------------------------------------------- dragging cards */

  page.addEventListener('dragstart', (e) => {
    const card = e.target.closest && e.target.closest('.tsz-card[data-card-line]');
    if (!card || !latest) return;
    dragCard = { line: Number(card.dataset.cardLine), board: card.closest('[data-board]'), el: card };
    card.classList.add('is-dragging');
    placeHandles(null);
    e.dataTransfer.effectAllowed = 'move';
    e.dataTransfer.setData('text/plain', 'tsuzuri-card');
  });

  const cardGap = document.createElement('div');
  cardGap.className = 'tsz-card-gap';

  /** Where a dragged card would land in a column: the cards it goes before. */
  function cardSlot(col, y) {
    const cards = [...col.querySelectorAll('.tsz-card')].filter((c) => c !== dragCard.el);
    let index = cards.findIndex((c) => {
      const r = c.getBoundingClientRect();
      return y < r.top + r.height / 2;
    });
    if (index < 0) index = cards.length;
    return { index, before: cards[index] || null };
  }

  function clearCardDrop() {
    cardGap.remove();
    page.querySelectorAll('.tsz-col.is-drop-target').forEach((c) => c.classList.remove('is-drop-target'));
  }

  function endCardDrag() {
    if (dragCard) dragCard.el.classList.remove('is-dragging');
    dragCard = null;
    clearCardDrop();
  }
  page.addEventListener('dragend', endCardDrag);

  /* ---------------------------------------------------- shared drop */

  document.addEventListener('dragover', (e) => {
    if (dragCard) {
      const col = e.target.closest && e.target.closest('.tsz-col');
      if (!col || col.closest('[data-board]') !== dragCard.board) {
        clearCardDrop();
        return;
      }
      e.preventDefault();
      const { before } = cardSlot(col, e.clientY);
      clearCardDrop();
      col.classList.add('is-drop-target');
      const list = col.querySelector('.tsz-cards');
      list.insertBefore(cardGap, before);
    } else if (dragBlock) {
      e.preventDefault();
      const gap = blockGap(e.clientY);
      if (!gap) return;
      const r = page.getBoundingClientRect();
      const style = getComputedStyle(page);
      dropLine.style.display = 'block';
      dropLine.style.left = `${r.left + window.scrollX + parseFloat(style.paddingLeft)}px`;
      dropLine.style.width = `${r.width - parseFloat(style.paddingLeft) - parseFloat(style.paddingRight)}px`;
      dropLine.style.top = `${gap.top + window.scrollY - 2}px`;
    }
  });

  document.addEventListener('drop', (e) => {
    if (dragCard) {
      const col = e.target.closest && e.target.closest('.tsz-col');
      if (col && col.closest('[data-board]') === dragCard.board) {
        e.preventDefault();
        const [block, end] = dragCard.board.dataset.board.split(':').map(Number);
        vscode.postMessage({
          type: 'card',
          block,
          end,
          card: dragCard.line,
          column: Number(col.dataset.col),
          index: cardSlot(col, e.clientY).index,
          version: latest.version,
        });
      }
      endCardDrag();
    } else if (dragBlock) {
      e.preventDefault();
      const gap = blockGap(e.clientY);
      if (gap && latest) {
        vscode.postMessage({ type: 'move', from: dragBlock.start, end: dragBlock.end, to: gap.to, version: latest.version });
      }
      endBlockDrag();
    }
  });

  // Shared with media/views.js, which can't acquire the API a second time.
  window.tsuzuri = {
    post: (msg) => vscode.postMessage(msg),
    version: () => (latest ? latest.version : undefined),
  };

  reportWidth();
})();
