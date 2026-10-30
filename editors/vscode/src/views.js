'use strict';
// Draws Tsuzuri's views as HTML from the data the tsuzuri binary reads out
// of each block (see internal/preview/views.go), so parsing stays in one
// place and only the look lives here.
//
// Views placed in the note (`at`) are editable. Controls carry the edit
// they make in data attributes, which media/views.js sends back:
//   data-edit="op"  text edited in place (the new text is the edit's text)
//   data-act="op"   a button or option that makes the edit when clicked
//   data-date="op"  a date picker (its value is the edit's date)
// plus data-line (a note line), data-index, data-value, data-date-value,
// data-end-value and data-text as the edit needs them. Every edit is applied
// by the binary, the same as the app's own clicks.

const DAY = 86_400_000;

const PALETTE = [
  'var(--vscode-charts-blue, #3794ff)',
  'var(--vscode-charts-green, #89d185)',
  'var(--vscode-charts-orange, #d18616)',
  'var(--vscode-charts-purple, #b180d7)',
  'var(--vscode-charts-yellow, #cca700)',
  'var(--vscode-charts-red, #f14c4c)',
];
const GREY = 'var(--vscode-descriptionForeground, #8b8b8b)';
const TONES = { done: PALETTE[1], progress: PALETTE[0], todo: GREY, blocked: PALETTE[5] };
const CHART_TYPES = ['bar', 'hbar', 'line', 'pie', 'donut'];
const QUESTION_TYPES = [
  ['', 'Short answer'],
  ['long', 'Long answer'],
  ['choice', 'Single choice'],
  ['multi', 'Multiple choice'],
  ['rating', 'Rating'],
  ['date', 'Date'],
  ['email', 'Email'],
];
const CHOICE = ['choice', 'select', 'radio'];
const MULTI = ['multi', 'checkbox', 'checkboxes'];

function esc(s) {
  return String(s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

/** data-* attributes from an object; undefined values are left out. */
function data(attrs) {
  return Object.entries(attrs)
    .filter(([, v]) => v !== undefined && v !== null)
    .map(([k, v]) => ` data-${k}="${esc(v)}"`)
    .join('');
}

function color(i) {
  return PALETTE[i % PALETTE.length];
}

/** YYYY-MM-DD as a UTC date. */
function utc(s) {
  const [y, m, d] = s.split('-').map(Number);
  return new Date(Date.UTC(y, m - 1, d || 1));
}

function iso(d) {
  return d.toISOString().slice(0, 10);
}

function todayUTC() {
  const n = new Date();
  return new Date(Date.UTC(n.getFullYear(), n.getMonth(), n.getDate()));
}

function fmt(v) {
  return Number.isInteger(v) ? String(v) : v.toFixed(1);
}

/**
 * Helpers bound to one view. `at` places it in the note (base is the note
 * line of the block's first line); without it the view is read-only.
 */
function context(inline, at) {
  const edit = Boolean(at);
  return {
    edit,
    inline,
    line: (l) => (edit ? at.base + l : undefined),
    /** Text shown rendered, edited as `raw` in place. */
    text(raw, op, attrs = {}, tag = 'span') {
      if (!edit) return `<${tag}>${inline(raw)}</${tag}>`;
      return `<${tag} class="tsz-editable" tabindex="0"${data({ edit: op, raw, ...attrs })}>${inline(raw)}</${tag}>`;
    },
    /** A button that makes an edit. */
    button(label, op, attrs = {}, cls = 'tsz-btn', title = '') {
      if (!edit) return '';
      return `<button class="${cls}" type="button"${title ? ` title="${esc(title)}"` : ''}${data({ act: op, ...attrs })}>${label}</button>`;
    },
    /** "+ New …": typing there and pressing Enter makes the edit. */
    adder(label, op, attrs = {}, placeholder = label) {
      if (!edit) return '';
      return `<div class="tsz-adder" tabindex="0"${data({ edit: op, raw: '', new: '1', ...attrs })} data-placeholder="${esc(placeholder)}" title="${esc(placeholder)}">${esc(label)}</div>`;
    },
    remove(line, title) {
      return this.button('×', 'delete', { line }, 'tsz-x', title);
    },
  };
}

/* ---------------------------------------------------------------- board */

/** A card's description: shown under its title, edited as lines of text. */
function description(card, c) {
  const lines = card.description || [];
  const shown = lines.map(c.inline).join('<br>');
  if (!c.edit) return lines.length ? `<div class="tsz-card-desc">${shown}</div>` : '';
  return `<div class="tsz-card-desc tsz-editable" tabindex="0"${data({
    edit: 'board.describe',
    raw: lines.join('\n'),
    line: c.line(card.line),
    multiline: '1',
    placeholder: 'Add a description',
  })}>${shown}</div>`;
}

/** A view's title, edited in place; an empty one shows "Add a title" on hover. */
function title(text, c, placeholder = 'Add a title') {
  if (!c.edit) return text ? `<h4 class="tsz-title">${c.inline(text)}</h4>` : '';
  return `<h4 class="tsz-title${text ? '' : ' is-empty'}">${c.text(text || '', 'title', { placeholder })}</h4>`;
}

function board(cols, c, at) {
  const cards = (col) => col.cards.map((card) => `
        <div class="tsz-card"${c.edit ? ` draggable="true" data-card-line="${c.line(card.line)}"` : ''}>
          <div class="tsz-card-title">${c.text(card.title, 'board.rename', { line: c.line(card.line) })}${c.button('×', 'board.delete', { line: c.line(card.line) }, 'tsz-x', 'Delete card')}</div>
          ${description(card, c)}
        </div>`).join('');

  const body = cols.map((col, i) => {
    const named = col.line >= 0;
    const name = named ? c.text(col.name, 'board.renamecolumn', { line: c.line(col.line) }) : c.inline(col.name);
    return `
    <section class="tsz-col" data-col="${i}" style="--tsz-accent:${TONES[col.tone] || color(i + 2)}">
      <h4 class="tsz-col-head"><span class="tsz-pill-label"><span class="tsz-dot"></span>${name}</span><span class="tsz-count">${col.cards.length}</span>${named ? c.button('×', 'board.deletecolumn', { line: c.line(col.line) }, 'tsz-x', 'Delete column') : ''}</h4>
      <div class="tsz-cards">${cards(col)}</div>
      ${c.adder('+ New', 'board.add', { index: i }, 'Card name')}
    </section>`;
  }).join('');

  const drag = c.edit ? ` data-board="${c.block}:${c.end}"` : '';
  const addColumn = c.edit ? `<section class="tsz-col tsz-col-new">${c.adder('+', 'board.column', {}, 'Column name')}</section>` : '';
  if (!cols.length && !c.edit) return '<p class="tsz-empty">Empty board. Add a "## Column" line and "- Card" lines under it.</p>';
  return `<div class="tsz-board-cols"${drag}>${body}${addColumn}</div>`;
}

/* ------------------------------------------------------------- calendar */

function calendar(cal, c, shift = 0) {
  const base = utc(cal.month);
  const month = new Date(Date.UTC(base.getUTCFullYear(), base.getUTCMonth() + shift, 1));
  const today = iso(todayUTC());
  const byDay = {};
  for (const e of cal.events) (byDay[e.date] = byDay[e.date] || []).push(e);

  const title = month.toLocaleString('en', { month: 'long', year: 'numeric', timeZone: 'UTC' });
  const head = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'].map((d) => `<div class="tsz-dow">${d}</div>`).join('');
  const cells = [];
  const start = new Date(month.getTime() - month.getUTCDay() * DAY);
  for (let week = 0; week < 6; week++) {
    const weekStart = new Date(start.getTime() + week * 7 * DAY);
    if (week > 0 && weekStart.getUTCMonth() !== month.getUTCMonth()) break;
    for (let d = 0; d < 7; d++) {
      const date = new Date(weekStart.getTime() + d * DAY);
      const key = iso(date);
      const cls = ['tsz-day'];
      if (date.getUTCMonth() !== month.getUTCMonth()) cls.push('is-outside');
      if (key === today) cls.push('is-today');
      const events = (byDay[key] || []).map((e) => `<div class="tsz-event"${c.edit ? ` draggable="true" data-event-line="${c.line(e.line)}" data-event-title="${esc(e.title)}"` : ''}>${c.text(e.title, 'calendar.set', { line: c.line(e.line), 'date-value': e.date })}${c.button('×', 'delete', { line: c.line(e.line) }, 'tsz-x', 'Delete event')}</div>`).join('');
      const add = c.edit ? `<div class="tsz-day-add" tabindex="0"${data({ edit: 'calendar.add', raw: '', new: '1', 'date-value': key })} data-placeholder="New event" title="Add an event">+</div>` : '';
      cells.push(`<div class="${cls.join(' ')}"${c.edit ? ` data-day="${key}"` : ''}><span class="tsz-daynum">${date.getUTCDate()}</span>${add}${events}</div>`);
    }
  }
  const count = `${cal.events.length} event${cal.events.length === 1 ? '' : 's'}`;
  const nav = c.edit
    ? `<span class="tsz-nav"><button class="tsz-btn" type="button" data-month="-1" title="Previous month">‹</button><button class="tsz-btn" type="button" data-month="0" title="Back to the note's month">Today</button><button class="tsz-btn" type="button" data-month="1" title="Next month">›</button></span>`
    : '';
  return `<h4 class="tsz-title tsz-cal-head">${esc(title)} <span class="tsz-muted">${count}</span>${nav}</h4><div class="tsz-grid"${c.edit ? ` data-calendar="${c.block}"` : ''}>${head}${cells.join('')}</div>`;
}

/* ------------------------------------------------------------- timeline */

function timeline(spans, c, name) {
  if (!spans.length && !c.edit) return `${title(name, c)}<p class="tsz-empty">Empty timeline. Add "Name: 2026-10-01 -> 2026-10-07" lines.</p>`;
  const rows = spans.map((s) => ({ ...s, s: utc(s.start), e: utc(s.end) }));
  const today = todayUTC();
  const min = rows.length ? Math.min(...rows.map((r) => r.s.getTime())) : today.getTime();
  const max = rows.length ? Math.max(...rows.map((r) => r.e.getTime())) : today.getTime();
  const total = (max - min) / DAY + 1;
  const pct = (t) => ((t - min) / DAY / total) * 100;
  const marker = today.getTime() >= min && today.getTime() <= max
    ? `<div class="tsz-today" style="left:${pct(today.getTime()) + 50 / total}%" title="Today"></div>`
    : '';

  const body = rows.map((r, i) => {
    const days = (r.e - r.s) / DAY + 1;
    const keep = { line: c.line(r.line), 'date-value': r.start, 'end-value': r.end, text: r.title };
    const dates = c.edit
      ? `<span class="tsz-dates">
          <input type="date" value="${r.start}" title="Start"${data({ date: 'timeline.set', which: 'start', ...keep })}>
          <span class="tsz-muted">→</span>
          <input type="date" value="${r.end}" title="End"${data({ date: 'timeline.set', which: 'end', ...keep })}>
          ${c.button('×', 'delete', { line: c.line(r.line) }, 'tsz-x', 'Delete item')}
        </span>`
      : '';
    return `<div class="tsz-row">
      <div class="tsz-label">${c.text(r.title, 'timeline.set', keep)}</div>
      <div class="tsz-track">${marker}
        <div class="tsz-bar" style="left:${pct(r.s.getTime())}%;width:${(days / total) * 100}%;--tsz-accent:${color(i)}"
          title="${esc(`${r.start} → ${r.end} (${days} day${days === 1 ? '' : 's'})`)}"></div>
      </div>
      ${dates}
    </div>`;
  }).join('');
  const axis = rows.length
    ? `<div class="tsz-row tsz-axis"><div></div><div class="tsz-track"><span>${iso(new Date(min))}</span><span>${iso(new Date(max))}</span></div></div>`
    : '';
  const start = iso(today);
  const end = iso(new Date(today.getTime() + 3 * DAY));
  return `${title(name, c)}<div class="tsz-rows">${body}${axis}</div>${c.adder('+ New item', 'timeline.add', { 'date-value': start, 'end-value': end }, 'Item name')}`;
}

/* ---------------------------------------------------------------- chart */

function chartBody(ch, c) {
  const data = ch.data;
  if (!data.length) return '<p class="tsz-empty">Empty chart. Add a value below.</p>';
  const height = ch.height * 18;
  const peak = Math.max(...data.map((d) => d.value), 0) || 1;

  if (ch.type === 'hbar') {
    return `<div class="tsz-hbars">${data.map((d, i) => `
      <div class="tsz-label">${c.inline(d.label)}</div>
      <div class="tsz-hbar-track"><div class="tsz-hbar" style="width:${(Math.max(d.value, 0) / peak) * 100}%;--tsz-accent:${color(i)}"></div></div>
      <div class="tsz-value">${fmt(d.value)}</div>`).join('')}
    </div>`;
  }

  if (ch.type === 'line') {
    const w = 800;
    const pad = 24;
    const x = (i) => (data.length === 1 ? w / 2 : pad + (i * (w - 2 * pad)) / (data.length - 1));
    const lo = Math.min(...data.map((d) => d.value), 0);
    const span = peak - lo || 1;
    const y = (v) => pad + (1 - (v - lo) / span) * (height - 2 * pad);
    const pts = data.map((d, i) => `${x(i).toFixed(1)},${y(d.value).toFixed(1)}`).join(' ');
    return `<svg class="tsz-line" viewBox="0 0 ${w} ${height + 20}" role="img">
      <line x1="${pad}" x2="${w - pad}" y1="${y(lo)}" y2="${y(lo)}" class="tsz-baseline"/>
      <polyline points="${pts}"/>
      ${data.map((d, i) => `<circle cx="${x(i)}" cy="${y(d.value)}" r="4"><title>${esc(`${d.label}: ${fmt(d.value)}`)}</title></circle>
      <text x="${x(i)}" y="${y(d.value) - 10}" class="tsz-svg-value">${esc(fmt(d.value))}</text>
      <text x="${x(i)}" y="${height + 14}" class="tsz-svg-label">${esc(d.label)}</text>`).join('')}
    </svg>`;
  }

  if (ch.type === 'pie' || ch.type === 'donut') {
    const sum = data.reduce((a, d) => a + Math.max(d.value, 0), 0) || 1;
    let at = 0;
    const stops = data.map((d, i) => {
      const from = at;
      at += (Math.max(d.value, 0) / sum) * 100;
      return `${color(i)} ${from}% ${at}%`;
    });
    return `<div class="tsz-pie-wrap">
      <div class="tsz-pie${ch.type === 'donut' ? ' is-donut' : ''}" style="background:conic-gradient(${stops.join(',')})"></div>
      <ul class="tsz-legend">${data.map((d, i) => `<li style="--tsz-accent:${color(i)}"><span class="tsz-dot"></span>${c.inline(d.label)}<span class="tsz-value">${fmt(d.value)} · ${Math.round((Math.max(d.value, 0) / sum) * 100)}%</span></li>`).join('')}</ul>
    </div>`;
  }

  return `<div class="tsz-bars" style="height:${height}px">${data.map((d, i) => `
    <div class="tsz-bar-col">
      <div class="tsz-value">${fmt(d.value)}</div>
      <div class="tsz-vbar" style="height:${(Math.max(d.value, 0) / peak) * 100}%;--tsz-accent:${color(i)}"></div>
      <div class="tsz-label">${c.inline(d.label)}</div>
    </div>`).join('')}
  </div>`;
}

function chart(ch, c) {
  const head = title(ch.title || '', c);
  if (!c.edit) return head + chartBody(ch, c);

  const types = CHART_TYPES.map((t) => c.button(t, 'chart.type', { value: t }, `tsz-pill${t === ch.type ? ' is-on' : ''}`)).join('');
  const rows = ch.data.map((d) => `
      <div class="tsz-data-row">
        ${c.text(d.label, 'chart.set', { line: c.line(d.line), value: String(d.value) })}
        ${c.text(fmt(d.value), 'chart.set', { line: c.line(d.line), text: d.label, field: 'value' })}
        ${c.remove(c.line(d.line), 'Delete value')}
      </div>`).join('');
  return `${head}<div class="tsz-pills">${types}</div>${chartBody(ch, c)}
    <details class="tsz-data"><summary>Data</summary>
      <div class="tsz-data-rows">${rows}</div>
      ${c.adder('+ Add value', 'chart.add', { field: 'pair' }, 'Label: 12')}
    </details>`;
}

/* ----------------------------------------------------------------- form */

function answerField(q, c) {
  const line = c.line(q.line);
  const options = q.options || [];
  const answer = q.answer || '';
  const off = c.edit ? '' : ' disabled';

  if (CHOICE.includes(q.kind) || MULTI.includes(q.kind)) {
    const multi = MULTI.includes(q.kind);
    const chosen = new Set(multi ? answer.split(',').map((a) => a.trim()) : [answer]);
    const opts = options.map((o, i) => {
      const on = chosen.has(o);
      const act = c.edit ? data({ act: multi ? 'form.toggle' : 'form.pick', line, index: i }) : '';
      return `<button type="button" class="tsz-opt${on ? ' is-on' : ''}"${act}${off}><span class="${multi ? 'tsz-check' : 'tsz-radio'}"></span>${c.inline(o)}</button>`;
    }).join('');
    return opts + c.adder('+ Add option', 'form.option', { line }, 'Option');
  }
  if (q.kind === 'rating') {
    const n = parseInt(answer, 10) || 0;
    const stars = [1, 2, 3, 4, 5].map((i) => `<button type="button" class="tsz-star${i <= n ? ' is-on' : ''}"${c.edit ? data({ act: 'form.rate', line, index: i }) : ''}${off} title="${i} star${i === 1 ? '' : 's'}">★</button>`).join('');
    return `<div class="tsz-stars">${stars}</div>`;
  }
  if (q.kind === 'date') {
    return `<input class="tsz-input" type="date" value="${esc(answer)}"${c.edit ? data({ answer: line }) : ''}${off}>`;
  }
  if (q.kind === 'long') {
    return `<textarea class="tsz-input" rows="3" placeholder="Your answer"${c.edit ? data({ answer: line }) : ''}${off}>${esc(answer)}</textarea>`;
  }
  const type = q.kind === 'email' ? 'email' : 'text';
  return `<input class="tsz-input" type="${type}" value="${esc(answer)}" placeholder="Your answer"${c.edit ? data({ answer: line }) : ''}${off}>`;
}

function form(f, c) {
  const out = [c.edit ? title(f.title || '', c, 'Untitled form') : `<h4 class="tsz-title">${c.inline(f.title || 'Untitled form')}</h4>`];
  if (f.description || c.edit) {
    out.push(`<p class="tsz-muted">${c.edit ? c.text(f.description || '', 'form.description', { placeholder: 'Add a description' }) : c.inline(f.description)}</p>`);
  }
  for (const q of f.questions) {
    const line = c.line(q.line);
    const required = q.required ? '<span class="tsz-required" title="Required">*</span>' : '';
    const tools = c.edit
      ? `<span class="tsz-q-tools">
          <select class="tsz-select" title="Question type"${data({ type: line })}>${QUESTION_TYPES.map(([v, l]) => `<option value="${v}"${(q.kind === v || (!v && q.kind === 'text')) ? ' selected' : ''}>${l}</option>`).join('')}</select>
          ${c.button(q.required ? 'Required' : 'Optional', 'form.required', { line }, `tsz-pill${q.required ? ' is-on' : ''}`, 'Toggle required')}
          ${c.remove(line, 'Delete question')}
        </span>`
      : '';
    out.push(`<div class="tsz-field"><div class="tsz-q">${c.text(q.text, 'form.rename', { line })}${required}${tools}</div>${answerField(q, c)}</div>`);
  }
  if (!f.questions.length && !c.edit) out.push('<p class="tsz-empty">No questions yet. Add "? Question" lines.</p>');
  out.push(c.adder('+ Add question', 'form.add', {}, 'Question'));
  return out.join('');
}

/* --------------------------------------------------------------- render */

/**
 * HTML for a view from the binary. `inline` renders Markdown in labels;
 * `at` ({base, block, end}) makes it editable; `state` holds per-view
 * display state such as which month a calendar shows.
 */
function renderView(view, inline, at, state = {}) {
  const c = context(inline, at);
  if (at) Object.assign(c, { block: at.block, end: at.end });
  switch (view.kind) {
    case 'board': return board(view.board || [], c);
    case 'calendar': return calendar(view.calendar, c, (at && state.months && state.months[at.block]) || 0);
    case 'timeline': return timeline(view.timeline || [], c, view.title || '');
    case 'chart': return chart(view.chart, c);
    case 'form': return form(view.form, c);
  }
  return '';
}

module.exports = { renderView };
