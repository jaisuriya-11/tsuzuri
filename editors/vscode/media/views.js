// Runs inside the Tsuzuri preview, after preview.js: makes views (boards,
// calendars, timelines, charts, forms) editable through their own controls.
// Each control says in data attributes which edit it makes (see
// src/views.js); the edit goes to the extension, which has the tsuzuri
// binary apply it to the note, exactly as the app does for the same click.
(function () {
  const page = document.getElementById('page');
  const app = window.tsuzuri;

  /** The view a control sits in, as its fence lines. */
  function viewOf(el) {
    const block = el.closest('.tsz-view[data-src]');
    if (!block) return null;
    const [start, end] = block.dataset.src.split(':').map(Number);
    return { block: start, end: end - 1 };
  }

  function num(v) {
    return v === undefined || v === '' ? undefined : Number(v);
  }

  /** Sends an edit built from a control's data attributes plus `extra`. */
  function send(el, op, extra = {}) {
    const view = viewOf(el);
    if (!view || app.version() === undefined) return;
    const d = el.dataset;
    const edit = {
      op,
      line: num(d.line),
      index: num(d.index),
      value: d.value,
      text: d.text,
      date: d.dateValue,
      end: d.endValue,
      ...extra,
    };
    app.post({ type: 'view', ...view, edit, version: app.version() });
  }

  /* ------------------------------------------------- text edited in place */

  let editing = null; // {el, html}

  function startTextEdit(el) {
    if (editing && editing.el === el) return;
    if (editing) finishTextEdit(true);
    // A card can't be dragged while its text is being edited.
    const card = el.closest('[draggable="true"]');
    if (card) card.draggable = false;
    editing = { el, html: el.innerHTML, card };
    el.classList.add('is-editing');
    el.contentEditable = 'plaintext-only';
    el.textContent = el.dataset.raw || '';
    el.focus();
    const range = document.createRange();
    range.selectNodeContents(el);
    const sel = window.getSelection();
    sel.removeAllRanges();
    sel.addRange(range);
  }

  function finishTextEdit(keep) {
    if (!editing) return;
    const { el, html, card } = editing;
    editing = null;
    if (card) card.draggable = true;
    const multiline = Boolean(el.dataset.multiline);
    const text = multiline
      ? el.innerText.split('\n').map((l) => l.trim()).filter(Boolean).join('\n')
      : el.textContent.replace(/\s*\n\s*/g, ' ').trim();
    el.contentEditable = 'false';
    el.classList.remove('is-editing');
    const changed = text !== (el.dataset.raw || '').trim();
    el.innerHTML = html; // the new render replaces it once the note changes
    if (!keep || !changed || (!text && el.dataset.new)) return;

    const field = el.dataset.field;
    if (multiline) {
      send(el, el.dataset.edit, { lines: text ? text.split('\n') : [] });
    } else if (field === 'value') {
      send(el, el.dataset.edit, { value: text });
    } else if (field === 'pair') {
      const i = text.lastIndexOf(':');
      if (i < 0) return;
      send(el, el.dataset.edit, { text: text.slice(0, i).trim(), value: text.slice(i + 1).trim() });
    } else {
      send(el, el.dataset.edit, { text });
    }
  }

  page.addEventListener('keydown', (e) => {
    if (!editing || e.target !== editing.el) return;
    if (e.key === 'Enter' && !(e.shiftKey && editing.el.dataset.multiline)) {
      e.preventDefault();
      finishTextEdit(true);
    } else if (e.key === 'Escape') {
      e.preventDefault();
      finishTextEdit(false);
    }
  });
  page.addEventListener('focusout', (e) => {
    if (editing && e.target === editing.el) finishTextEdit(true);
  });

  /* ---------------------------------------------------------------- clicks */

  page.addEventListener('click', (e) => {
    const t = e.target;
    if (!(t instanceof Element) || !t.closest('.tsz-view')) return;

    const editable = t.closest('[data-edit]');
    if (editable && !editable.isContentEditable) {
      e.preventDefault();
      e.stopPropagation();
      startTextEdit(editable);
      return;
    }

    const act = t.closest('[data-act]');
    if (act) {
      e.preventDefault();
      e.stopPropagation();
      send(act, act.dataset.act);
      return;
    }

    const month = t.closest('[data-month]');
    if (month) {
      e.stopPropagation();
      const view = viewOf(month);
      if (view) app.post({ type: 'month', block: view.block, shift: Number(month.dataset.month) });
      return;
    }
  }, true);

  // Inputs: answers, dates and question types are saved when they change.
  page.addEventListener('change', (e) => {
    const t = e.target;
    if (!(t instanceof Element)) return;
    if (t.matches('input[data-date]')) {
      const which = t.dataset.which;
      send(t, t.dataset.date, which === 'end' ? { end: t.value } : { date: t.value });
    } else if (t.matches('[data-answer]')) {
      send(t, 'form.answer', { line: Number(t.dataset.answer), text: t.value.replace(/\s*\n\s*/g, ' ').trim() });
    } else if (t.matches('select[data-type]')) {
      send(t, 'form.type', { line: Number(t.dataset.type), value: t.value });
    }
  });

  /* ------------------------------------------- dragging events to a day */

  let dragEvent = null;

  page.addEventListener('dragstart', (e) => {
    const ev = e.target instanceof Element && e.target.closest('[data-event-line]');
    if (!ev) return;
    e.stopPropagation();
    dragEvent = ev;
    ev.classList.add('is-dragging');
    e.dataTransfer.effectAllowed = 'move';
    e.dataTransfer.setData('text/plain', 'tsuzuri-event');
  });

  function dayUnder(e) {
    const day = e.target instanceof Element && e.target.closest('[data-day]');
    return day && dragEvent && day.closest('[data-calendar]') === dragEvent.closest('[data-calendar]') ? day : null;
  }

  function clearDay() {
    page.querySelectorAll('.tsz-day.is-drop-target').forEach((d) => d.classList.remove('is-drop-target'));
  }

  page.addEventListener('dragover', (e) => {
    if (!dragEvent) return;
    const day = dayUnder(e);
    clearDay();
    if (!day) return;
    e.preventDefault();
    e.stopPropagation();
    day.classList.add('is-drop-target');
  });

  page.addEventListener('drop', (e) => {
    if (!dragEvent) return;
    const day = dayUnder(e);
    if (day) {
      e.preventDefault();
      e.stopPropagation();
      send(dragEvent, 'calendar.set', {
        line: Number(dragEvent.dataset.eventLine),
        date: day.dataset.day,
        text: dragEvent.dataset.eventTitle,
      });
    }
  });

  page.addEventListener('dragend', () => {
    if (dragEvent) dragEvent.classList.remove('is-dragging');
    dragEvent = null;
    clearDay();
  });

  app.isEditingView = () => Boolean(editing);
})();
