"use strict";

const $ = (id) => document.getElementById(id);
const esc = (s) => String(s ?? "").replace(/[&<>"']/g, (c) =>
  ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);

const STATUS = { playing: "играет", paused: "пауза", stopped: "остановлен" };
const SENT = { sent: "отправлен", queued: "в очереди", ignored: "отклонён", lastfm: "в last.fm", local: "сохранён" };
const MONTHS = ["января", "февраля", "марта", "апреля", "мая", "июня", "июля", "августа",
  "сентября", "октября", "ноября", "декабря"];

let data = null;
let dataAt = 0;
let tab = "history";
let period = "week";
let lastOK = 0;

function plural(n, one, few, many) {
  const m10 = n % 10, m100 = n % 100;
  if (m10 === 1 && m100 !== 11) return one;
  if (m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14)) return few;
  return many;
}

function fmt(sec) {
  sec = Math.max(0, Math.floor(sec + 1e-6));
  const h = Math.floor(sec / 3600), m = Math.floor(sec % 3600 / 60), s = String(sec % 60).padStart(2, "0");
  return h ? `${h}:${String(m).padStart(2, "0")}:${s}` : `${m}:${s}`;
}

function span(sec) {
  const m = Math.round(sec / 60);
  if (m < 60) return `${m} мин`;
  return `${Math.floor(m / 60)} ч ${m % 60} мин`;
}

function setText(id, text) {
  const el = $(id);
  if (el.textContent !== text) el.textContent = text;
}

async function get(path) {
  const r = await fetch(path, { cache: "no-store" });
  const body = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(body.error || `ошибка ${r.status}`);
  return body;
}

function online() {
  lastOK = Date.now();
  const f = $("foot");
  f.classList.remove("err");
  f.textContent = "обновляется само";
}

function offline() {
  if (Date.now() - lastOK < 6000) return;
  const f = $("foot");
  f.classList.add("err");
  f.textContent = "нет связи с компьютером: программа выключена или телефон в другой сети";
}

function since(n) {
  return n && n.status === "playing" ? Math.max(0, (performance.now() - dataAt) / 1000) : 0;
}

function leftText(n) {
  if (data.paused) return "скробблинг выключен";
  if (n.scrobbled) return "засчитано";
  if (n.live) return n.fromStart ? "засчитается, когда песня закончится" : "";
  if (!n.scrobblable) return "слишком короткий трек";
  return `засчитается через ${fmt(Math.ceil(Math.max(0, n.left - since(n))))}`;
}

function paintClock() {
  const n = data && data.now;
  if (!n) return;
  const pos = Math.min((n.synced ? n.position : n.played) + since(n), n.length > 0 ? n.length : Infinity);
  const hasBar = n.length > 0 && !n.live;
  setText("time", hasBar ? `${fmt(pos)} / ${fmt(n.length)}` : `прослушано ${fmt(n.played + since(n))}`);
  setText("left", leftText(n));
  if (hasBar) {
    const box = $("progress");
    const inner = Math.max(0, box.clientWidth - 6);
    const fill = Math.floor(inner * Math.min(1, pos / n.length) / 8) * 8;
    const w = `${fill}px`;
    if ($("bar").style.width !== w) $("bar").style.width = w;
    paintBand(n, inner, fill);
  }
}

function outline(rows) {
  const h = rows.length;
  const w = rows[0].length;
  const on = (x, y) => y >= 0 && y < h && x >= 0 && x < w && rows[y][x] === "#";
  const out = [];
  for (let y = -1; y <= h; y++) {
    let line = "";
    for (let x = -1; x <= w; x++) {
      let near = false;
      for (let dy = -1; dy <= 1; dy++) for (let dx = -1; dx <= 1; dx++) near = near || on(x + dx, y + dy);
      line += near && !on(x, y) ? "#" : ".";
    }
    out.push(line);
  }
  return out;
}

function cells(rows, dx, dy) {
  let out = "";
  rows.forEach((row, y) => {
    for (let x = 0; x < row.length; x++) if (row[x] === "#") out += `<rect x="${x + dx}" y="${y + dy}" width="1" height="1"/>`;
  });
  return out;
}

function fringe(rows, side) {
  return rows.map((r) => {
    const at = side < 0 ? r.indexOf("#") - 1 : r.lastIndexOf("#") + 1;
    return [...r].map((_, x) => (x === at ? "#" : ".")).join("") + (at >= r.length ? "#" : "");
  });
}

function heartMarker(cell) {
  const h = HOT_ICON.heart;
  const torn = h.map((r, i) => (i === 3 ? `.${r}` : `${r}.`));
  return `<svg width="${9 * cell}" height="${8 * cell}" viewBox="0 0 9 8" shape-rendering="crispEdges" aria-hidden="true">` +
    `<g class="mh-edge">${cells(outline(h), 0, 0)}</g><g class="mh-core">${cells(h, 1, 1)}</g>` +
    `<g class="mh-cyan">${cells(fringe(torn, -1), 1, 1)}</g><g class="mh-red">${cells(fringe(torn, 1), 1, 1)}</g>` +
    `<g class="mh-torn">${cells(torn, 1, 1)}</g></svg>`;
}

function pin(el, x, w, total) {
  el.hidden = false;
  const left = `${Math.round(Math.min(Math.max(3 + x - w / 2, 0), total - w))}px`;
  if (el.style.left !== left) el.style.left = left;
}

function paintBand(n, inner, fill) {
  const m = n.moment;
  const played = $("momPlayed");
  const ahead = $("momAhead");
  const ha = $("heartA");
  const hb = $("heartB");
  if (!m || !inner) {
    for (const el of [played, ahead, ha, hb]) el.hidden = true;
    return;
  }
  if (!ha.firstChild) {
    ha.innerHTML = heartMarker(2);
    hb.innerHTML = heartMarker(2);
  }
  const a = Math.floor(inner * Math.max(0, m.start) / n.length / 8) * 8;
  const b = Math.min(inner, Math.max(a + 8, Math.ceil(inner * Math.min(m.end, n.length) / n.length / 8) * 8));
  const split = Math.min(Math.max(fill, a), b);
  stripe(played, a, split);
  stripe(ahead, split, b);
  const total = $("progress").clientWidth;
  const close = b - a < 18;
  pin(ha, close ? (a + b) / 2 : a, 18, total);
  hb.hidden = close;
  if (!close) pin(hb, b, 18, total);
}

function stripe(el, from, to) {
  el.hidden = to <= from;
  if (el.hidden) return;
  const left = `${3 + from}px`;
  const width = `${to - from}px`;
  const pos = `${-from}px 0px`;
  if (el.style.left !== left) el.style.left = left;
  if (el.style.width !== width) el.style.width = width;
  if (el.style.backgroundPosition !== pos) el.style.backgroundPosition = pos;
}

function paintMomentNote(n) {
  const m = n && n.length > 0 && !n.live ? n.moment : null;
  const box = $("momentNote");
  const html = m ? `${icon(HOT_ICON.heart)}<span>любимый момент ${fmt(m.start)}–${fmt(m.end)} · ` +
    `${m.manual ? "отмечен вами" : "чаще всего переслушиваете"}</span>` : "";
  if (box.dataset.sig !== html) {
    box.dataset.sig = html;
    box.innerHTML = html;
  }
  box.hidden = !html;
}

function renderNow() {
  const n = data.now;
  setText("user", data.user || "");
  const c = data.today.count;
  setText("today", `сегодня: ${c} ${plural(c, "трек", "трека", "треков")} · ${span(data.today.seconds)}`);
  $("chip").hidden = !n;
  $("times").hidden = !n;
  $("progress").hidden = !n || !(n.length > 0 && !n.live);
  if (!n) {
    setText("title", "Ничего не играет");
    setText("artist", "");
    setText("album", "");
    paintHots(null);
    paintLove(null);
    paintMomentNote(null);
    return;
  }
  setText("chip", `${n.player} · ${STATUS[n.status] || n.status}`);
  setText("title", n.title);
  setText("artist", n.artist);
  setText("album", n.album || "");
  paintHots(n);
  paintLove(n);
  paintMomentNote(n);
  paintClock();
}

function applyTheme(name) {
  const root = document.documentElement;
  if (name && root.dataset.theme !== name) root.dataset.theme = name;
  const color = getComputedStyle(root).getPropertyValue("--title").trim();
  const meta = document.querySelector('meta[name="theme-color"]');
  if (meta && color && meta.content !== color) meta.content = color;
}

const HOT_KIND = { track: "трек", artist: "исполнитель", album: "альбом" };
const HOT_OF = { week: "недели", month: "месяца", year: "года" };
const HOT_FOR = { week: "за неделю", month: "за месяц", year: "за год" };
const HOT_ORDER = { week: 0, month: 1, year: 2 };
const HOT_ICON = {
  star: ["...#...", "..###..", "#######", ".#####.", "..###..", ".##.##.", "##...##"],
  crown: ["#...#...#", "##.###.##", "#########", "#########", ".#######."],
  heart: [".##.##.", "#######", "#######", ".#####.", "..###..", "...#..."],
};

function icon(rows) {
  let rects = "";
  rows.forEach((row, y) => [...row].forEach((ch, x) => {
    if (ch === "#") rects += `<rect x="${x}" y="${y}" width="1" height="1"/>`;
  }));
  return `<svg width="${rows[0].length * 2}" height="${rows.length * 2}" viewBox="0 0 ${rows[0].length} ${rows.length}" ` +
    `fill="currentColor" shape-rendering="crispEdges" aria-hidden="true">${rects}</svg>`;
}

function paintHots(n) {
  const html = ["track", "artist", "album"].map((kind) => {
    let best = null;
    for (const c of (n && n.charts) || []) {
      if (c.kind === kind && (!best || c.rank < best.rank || (c.rank === best.rank && HOT_ORDER[c.period] > HOT_ORDER[best.period]))) best = c;
    }
    if (!best) return "";
    const text = best.rank === 1 ? `${HOT_KIND[kind]} ${HOT_OF[best.period]}` : `${HOT_KIND[kind]} №${best.rank} ${HOT_FOR[best.period]}`;
    return `<span class="hot${best.rank === 1 ? " top1" : ""}">${icon(HOT_ICON[best.rank === 1 ? "crown" : "star"])}<span>${esc(text)}</span></span>`;
  }).join("");
  const box = $("hots");
  if (box.dataset.sig !== html) {
    box.dataset.sig = html;
    box.innerHTML = html;
  }
  box.hidden = !html;
}

function paintLove(n) {
  const box = $("love");
  const text = n && n.loved ? (n.status === "playing" ? "любимка играет" : "любимка") : "";
  box.hidden = !text;
  if (text && box.dataset.sig !== text) {
    box.dataset.sig = text;
    box.innerHTML = `${icon(HOT_ICON.heart)}<span>${esc(text)}</span>`;
  }
}

async function loadNow() {
  try {
    data = await get("api/now");
    dataAt = performance.now();
    online();
    applyTheme(data.theme);
    document.documentElement.classList.toggle("still", data.bursts === false);
    renderNow();
  } catch (e) {
    offline();
  }
}

function dayLabel(d) {
  const today = new Date();
  const start = (x) => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime();
  const diff = Math.round((start(today) - start(d)) / 86400000);
  if (diff === 0) return "сегодня";
  if (diff === 1) return "вчера";
  return `${d.getDate()} ${MONTHS[d.getMonth()]}${d.getFullYear() !== today.getFullYear() ? " " + d.getFullYear() : ""}`;
}

function clock(d) {
  return `${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
}

async function loadHistory() {
  try {
    const list = await get("api/history?limit=60");
    online();
    if (!list.length) {
      $("history").innerHTML = `<p class="empty muted">Здесь появятся засчитанные треки.</p>`;
      return;
    }
    let html = "", day = "";
    for (const s of list) {
      const d = new Date(s.started);
      const label = dayLabel(d);
      if (label !== day) {
        day = label;
        html += `<div class="day">${esc(label)}</div>`;
      }
      html += `<div class="item"><span class="muted">${clock(d)}</span>` +
        `<span class="t">${esc(s.title)}<br><span class="a">${esc(s.artist)}</span></span>` +
        `<span class="status ${esc(s.status)}">${esc(SENT[s.status] || s.status)}</span></div>`;
    }
    if ($("history").innerHTML !== html) $("history").innerHTML = html;
  } catch (e) {
    offline();
  }
}

async function loadTop() {
  try {
    const t = await get(`api/top?period=${period}`);
    online();
    setText("periodLabel", t.period && t.period.label ? `${t.period.label} · всего ${t.total} ${plural(t.total, "прослушивание", "прослушивания", "прослушиваний")}` : "");
    const html = t.items.length ? t.items.map((it) =>
      `<div class="item"><span class="rank">${it.rank}</span>` +
      `<span class="t">${esc(it.title)}<br><span class="a">${esc(it.artist)}</span></span>` +
      `<span class="status">${it.count} ${plural(it.count, "раз", "раза", "раз")}</span></div>`).join("")
      : `<p class="empty muted">За этот период прослушиваний нет.</p>`;
    if ($("top").innerHTML !== html) $("top").innerHTML = html;
  } catch (e) {
    offline();
  }
}

function press(selector, attr, value) {
  for (const b of document.querySelectorAll(selector)) b.setAttribute("aria-pressed", String(b.dataset[attr] === value));
}

document.addEventListener("click", (e) => {
  const t = e.target.closest("[data-tab]");
  if (t) {
    tab = t.dataset.tab;
    press("[data-tab]", "tab", tab);
    $("pane-history").hidden = tab !== "history";
    $("pane-top").hidden = tab !== "top";
    (tab === "history" ? loadHistory : loadTop)();
    return;
  }
  const p = e.target.closest("[data-period]");
  if (p) {
    period = p.dataset.period;
    press("[data-period]", "period", period);
    loadTop();
  }
});

document.addEventListener("visibilitychange", () => {
  if (!document.hidden) {
    loadNow();
    (tab === "history" ? loadHistory : loadTop)();
  }
});

loadNow();
loadHistory();
setInterval(() => { if (!document.hidden) loadNow(); }, 2000);
setInterval(() => { if (!document.hidden) (tab === "history" ? loadHistory : loadTop)(); }, 20000);
setInterval(() => { if (!document.hidden) paintClock(); }, 250);
