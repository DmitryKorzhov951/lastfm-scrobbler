"use strict";

const BITMAPS = {
  note: ["...######", "...######", "...#....#", "...#....#", "...#....#",
         ".###..###", "####.####", "####.####", ".##...##."],
  list: ["##.#######", "..........", "..........", "##.#######", "..........",
         "..........", "##.#######"],
  gear: ["...###...", ".#.###.#.", "..#####..", "####.####", "###...###",
         "####.####", "..#####..", ".#.###.#.", "...###..."],
  close: ["##...##", ".##.##.", "..###..", "..###..", ".##.##.", "##...##"],
  down: ["#######", ".#####.", "..###..", "...#..."],
  star: ["...#...", "..###..", "#######", ".#####.", "..###..", ".##.##.", "##...##"],
  crown: ["#...#...#", "##.###.##", "#########", "#########", ".#######."],
  mic: ["..###..", ".#####.", ".#####.", ".#####.", "#.###.#", "#.....#", ".#####.", "...#...", ".#####."],
  hash: [".#.#.", "#####", ".#.#.", "#####", ".#.#."],
  min: ["......", "......", "......", "......", "......", "######", "######"],
  max: ["#######", "#######", "#.....#", "#.....#", "#.....#", "#######"],
  mini: [".......", ".......", "#######", "#######", "#.....#", "#######"],
  pin: ["..###..", "..#.#..", "..#.#..", ".#####.", "#######", "...#...", "...#..."],
  restore: ["..######", "..######", "..#....#", "######.#", "######.#", "#....###", "#....#..", "######.."],
  ext: ["..#####", "....###", "...##.#", "..##..#", ".##....", "##....."],
  warn: ["....#....", "...###...", "...#.#...", "..##.##..", "..##.##..",
         ".###.###.", ".#######.", "####.####", "#########"],
  key: [".###.....", "#...#....", "#...#####", "#...#.#.#", ".###....."],
  folder: ["###......", "#..#####.", "#.......#", "#.......#", "#.......#", "#########"],
  heart: [".##.##.", "#######", "#######", ".#####.", "..###..", "...#..."],
  heartBig: [".##...##.", "####.####", "#########", "#########", ".#######.",
             "..#####..", "...###...", "....#...."],
  radio: [".........#..", "........#...", ".......#....", "############", "#..........#",
          "#.#.#.#.##.#", "#..#.#..##.#", "#.#.#.#....#", "#..#.#..##.#", "#..........#",
          "############"],
  play: ["#.....", "###...", "#####.", "######", "#####.", "###...", "#....."],
  stop: ["######", "######", "######", "######", "######", "######"],
  speaker: ["...#...#.", "..##....#", "####..#.#", "####..#.#", "####..#.#", "..##....#", "...#...#."],
  chart: ["......##.", "......##.", "...##.##.", "...##.##.", "##.##.##.", "##.##.##.", "##.##.##.", "#########"],
  left: ["...#", "..##", ".###", "####", ".###", "..##", "...#"],
  right: ["#...", "##..", "###.", "####", "###.", "##..", "#..."],
  phone: [".#####.", "#.....#", "#.###.#", "#.#.#.#", "#.###.#", "#.....#", "#..#..#", ".#####."],
  disc: ["..#####..", ".#.....#.", "#..###..#", "#.#...#.#", "#.#.#.#.#", "#.#...#.#", "#..###..#", ".#.....#.", "..#####.."],
  person: ["...###...", "..#####..", "..#####..", "..#####..", "...###...", ".........", ".#######.", "#########", "#########"],
};

function pix(name, cell) { return pixRows(BITMAPS[name], cell); }

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

const HEART_EDGE = outline(BITMAPS.heart);
const HEART_TORN = BITMAPS.heart.map((r, i) => (i === 3 ? `.${r}` : `${r}.`));
const HEART_LEFT = fringe(HEART_TORN, -1);
const HEART_RIGHT = fringe(HEART_TORN, 1);

function heartMarker(cell) {
  return `<svg width="${9 * cell}" height="${8 * cell}" viewBox="0 0 9 8" shape-rendering="crispEdges" aria-hidden="true">` +
    `<g class="mh-edge">${cells(HEART_EDGE, 0, 0)}</g><g class="mh-core">${cells(BITMAPS.heart, 1, 1)}</g>` +
    `<g class="mh-cyan">${cells(HEART_LEFT, 1, 1)}</g><g class="mh-red">${cells(HEART_RIGHT, 1, 1)}</g>` +
    `<g class="mh-torn">${cells(HEART_TORN, 1, 1)}</g></svg>`;
}

function pixRows(rows, cell) {
  const w = Math.max(...rows.map((r) => r.length));
  const h = rows.length;
  let rects = "";
  rows.forEach((row, y) => {
    for (let x = 0; x < row.length;) {
      if (row[x] !== "#") { x++; continue; }
      let k = x;
      while (k < row.length && row[k] === "#") k++;
      rects += `<rect x="${x}" y="${y}" width="${k - x}" height="1"/>`;
      x = k;
    }
  });
  return `<svg class="pix" width="${w * cell}" height="${h * cell}" viewBox="0 0 ${w} ${h}" ` +
    `fill="currentColor" aria-hidden="true">${rects}</svg>`;
}

function appIcon(size) {
  return `<svg class="pix" width="${size}" height="${size}" viewBox="0 0 16 16" aria-hidden="true">` +
    `<defs><linearGradient id="ai" x1="0" y1="0" x2="1" y2="1"><stop offset="0" class="ai-a"/>` +
    `<stop offset="1" class="ai-b"/></linearGradient></defs><rect width="16" height="16" fill="url(#ai)"/>` +
    `<g class="ai-bars"><rect x="4" y="6.5" width="2" height="6"/><rect x="7" y="3.5" width="2" height="9"/>` +
    `<rect x="10" y="5.5" width="2" height="7"/></g></svg>`;
}

function grip() {
  let hi = "", lo = "";
  for (let y = 0; y < 12; y++) {
    for (let x = 0; x < 12; x++) {
      const k = x + y - 11;
      if (k < 0 || k % 4 === 3) continue;
      if (k % 4 === 0) hi += `M${x} ${y}h1v1h-1z`;
      else lo += `M${x} ${y}h1v1h-1z`;
    }
  }
  return `<svg class="pix" width="12" height="12" viewBox="0 0 12 12" aria-hidden="true">` +
    `<path class="grip-hi" d="${hi}"/><path class="grip-lo" d="${lo}"/></svg>`;
}

function selectHTML(id, options, value) {
  return `<span class="select"><select id="${id}">${options.map(([v, label]) =>
    `<option value="${esc(v)}"${String(v) === String(value) ? " selected" : ""}>${esc(label)}</option>`).join("")}</select>` +
    `<span class="select-btn">${pix("down", 1)}</span></span>`;
}

const params = new URLSearchParams(location.hash.slice(1));
let token = params.get("t");
try {
  if (token) sessionStorage.setItem("lfs-token", token);
  else token = sessionStorage.getItem("lfs-token");
} catch (e) { }
let view = params.get("v") || "now";
history.replaceState(null, "", location.pathname);

async function api(path, body) {
  const opt = { headers: { "X-Token": token || "" } };
  if (body !== undefined) {
    opt.method = "POST";
    opt.headers["Content-Type"] = "application/json";
    opt.body = JSON.stringify(body);
  }
  const r = await fetch(path, opt);
  const data = await r.json().catch(() => ({}));
  if (!r.ok) {
    const err = new Error(data.error || `ошибка ${r.status}`);
    err.status = r.status;
    throw err;
  }
  return data;
}

const host = window.chrome && window.chrome.webview;
const tell = (msg) => { if (host) host.postMessage(msg); };

const THEMES = [["standard", "стандартная"], ["dark", "тёмная"], ["brat", "summer 2024"], ["glitch", "АААААААААААА памагите"], ["fallen", "fallen"]];
const THEME_FONTS = { standard: ["Tiny5", "Press Start 2P"], brat: ["Roboto Condensed"], glitch: ["Martian Mono", "Rubik Glitch"],
  fallen: ["Forum", "Handjet"] };
let themePreview = "";
let burstsPreview = null;
let themeTold = "";

function applyBursts(on) {
  document.documentElement.classList.toggle("still", on === false);
}

function applyTheme(name) {
  if (!THEMES.some(([t]) => t === name)) name = "standard";
  const root = document.documentElement;
  if (root.dataset.theme !== name) root.dataset.theme = name;
  if (themeTold !== name) {
    themeTold = name;
    tell("theme:" + name);
  }
}

function mountFrame() {
  if (!host) return;
  document.documentElement.classList.add("framed");
  const bar = document.createElement("header");
  bar.className = "frame-title";
  bar.innerHTML = `<span data-frame="title">${APP_TITLE}</span>` +
    `<button class="tb-btn tb-pin" data-win="pin" aria-label="поверх всех окон" title="поверх всех окон">${pix("pin", 1)}</button>` +
    `<button class="tb-btn" data-win="min" aria-label="свернуть" title="свернуть">${pix("min", 1)}</button>` +
    `<button class="tb-btn" data-win="mini" aria-label="мини-плеер" title="мини-плеер">${pix("mini", 1)}</button>` +
    `<button class="tb-btn" data-win="close" aria-label="закрыть" title="закрыть">${pix("close", 1)}</button>`;
  document.body.prepend(bar);
  mountMini();
  for (const edge of ["n", "s", "e", "w", "nw", "ne", "sw", "se"]) {
    const el = document.createElement("div");
    el.className = `rz rz-${edge}`;
    el.dataset.rz = edge;
    if (edge === "se") el.innerHTML = grip();
    document.body.append(el);
  }

  let drag = false;
  bar.addEventListener("mousedown", (e) => { drag = e.button === 0 && !e.target.closest("button"); });
  window.addEventListener("mousemove", (e) => {
    if (drag && (e.buttons & 1)) {
      drag = false;
      tell("drag");
    }
  });
  window.addEventListener("mouseup", () => { drag = false; });
  bar.addEventListener("dblclick", (e) => { if (!e.target.closest("button")) tell("mini"); });
  const panel = document.getElementById("mini");
  panel.addEventListener("mousedown", (e) => { drag = e.button === 0 && !e.target.closest("button"); });
  panel.addEventListener("dblclick", (e) => { if (!e.target.closest("button")) tell("mini"); });
  bar.addEventListener("click", (e) => {
    const b = e.target.closest("[data-win]");
    if (b) tell(b.dataset.win);
  });
  document.addEventListener("mousedown", (e) => {
    const edge = e.target.closest("[data-rz]");
    if (edge && e.button === 0) {
      e.preventDefault();
      tell("resize:" + edge.dataset.rz);
    }
  });
}

window.lfsFrame = (max, active, mini = false, pinned = false) => {
  const root = document.documentElement;
  const wasMini = root.classList.contains("mini");
  root.classList.toggle("max", max);
  root.classList.toggle("inactive", !active);
  applyMode(mini, pinned);
  if (mini && !wasMini) closeModal();
  if (mini !== wasMini && st) {
    render();
    refresh();
  }
};

function applyMode(mini, pinned) {
  const root = document.documentElement;
  root.classList.toggle("mini", mini);
  root.classList.toggle("pinned", pinned);
  fitMini();
  const b = document.querySelector('[data-win="mini"]');
  if (b && b.dataset.state !== String(mini)) {
    b.dataset.state = String(mini);
    b.innerHTML = pix(mini ? "max" : "mini", 1);
    const label = mini ? "обычное окно" : "мини-плеер";
    b.setAttribute("aria-label", label);
    b.title = label;
  }
  const p = document.querySelector('[data-win="pin"]');
  if (p) p.setAttribute("aria-pressed", String(pinned));
}

const APP_TITLE = "Следилка за музыкой";

function mountMini() {
  const el = document.createElement("div");
  el.id = "mini";
  el.className = "mini-panel";
  el.innerHTML = `<div class="mini-art"><img data-m="img" alt="" hidden><div data-m="ph">${pix("note", 6)}</div></div>
    <div class="mini-info">
      <div class="mini-title" data-m="tbox"><div class="mq" data-m="mq"><span data-m="t1"></span><span data-m="t2" aria-hidden="true"></span></div></div>
      <div class="mini-artist"><span class="tag mini-tag" data-m="tag" hidden>${pix("hash", 1)}<span data-m="tagText"></span></span><span class="ellipsis"><span data-m="artist"></span><span class="muted" data-m="album"></span></span></div>
      <div class="progress mini-bar" data-m="progress"><div data-m="bar"></div><span class="moment-played" data-m="momPlayed" hidden></span><span class="moment-ahead" data-m="momAhead" hidden></span><span class="mheart" data-m="heartA" hidden>${heartMarker(1)}</span><span class="mheart" data-m="heartB" hidden>${heartMarker(1)}</span></div>
      <div class="row mini-row" data-m="times"><span class="muted" data-m="time"></span><span class="note" data-m="left"></span></div>
    </div>`;
  document.body.append(el);
  wireCover(el.querySelector('[data-m="img"]'), el.querySelector('[data-m="ph"]'));
  const start = window.lfsStart;
  if (start) applyMode(Boolean(start.mini), Boolean(start.pinned));
}

const isMini = () => document.documentElement.classList.contains("mini");

const MINI_W = 480;

function fitMini() {
  const root = document.documentElement;
  const zoom = isMini() ? String(Math.round(Math.max(0.5, Math.min(1, innerWidth / MINI_W)) * 1000) / 1000) : "";
  if (root.style.zoom !== zoom) root.style.zoom = zoom === "1" ? "" : zoom;
}

window.addEventListener("resize", () => {
  fitMini();
  updateMini();
});

function miniField(name) { return document.querySelector(`#mini [data-m="${name}"]`); }
function miniText(name, text) { const el = miniField(name); if (el && el.textContent !== text) el.textContent = text; }

function coverURL(key) { return `/cover/${encodeURIComponent(key)}?t=${encodeURIComponent(token || "")}`; }

function wireCover(img, ph) {
  img.addEventListener("error", () => {
    img.hidden = true;
    img.dataset.bad = img.dataset.key || "";
    ph.hidden = false;
  });
  img.addEventListener("load", () => {
    img.hidden = false;
    ph.hidden = true;
  });
}

function setCover(img, ph, key) {
  if (!img) return;
  const want = key || "";
  if (img.dataset.key === want) return;
  img.dataset.key = want;
  if (!want || img.dataset.bad === want) {
    img.hidden = true;
    img.removeAttribute("src");
    if (ph) ph.hidden = false;
    return;
  }
  img.src = coverURL(want);
}

function marquee(text) {
  const box = miniField("tbox"), mq = miniField("mq");
  if (!box || !mq) return;
  if (mq.dataset.text !== text) {
    mq.dataset.text = text;
    miniText("t1", text);
    miniText("t2", text);
    mq.classList.remove("run");
  }
  const t1 = miniField("t1");
  const run = t1.scrollWidth > box.clientWidth + 1;
  if (run !== mq.classList.contains("run")) {
    mq.classList.toggle("run", run);
    if (run) mq.style.setProperty("--mq-time", `${Math.max(6, (t1.scrollWidth + 60) / 35)}s`);
  }
}

function updateMini() {
  const el = document.getElementById("mini");
  const titleEl = document.querySelector('[data-frame="title"]');
  if (!el || !isMini() || !st) {
    if (titleEl && titleEl.textContent !== APP_TITLE) titleEl.textContent = APP_TITLE;
    return;
  }
  const n = st.now;
  if (!n) {
    if (titleEl && titleEl.textContent !== APP_TITLE) titleEl.textContent = APP_TITLE;
    marquee("Ничего не играет");
    miniText("artist", "включите музыку — трек появится здесь");
    miniText("album", "");
    miniField("tag").hidden = true;
    miniField("progress").hidden = true;
    miniField("times").hidden = true;
    setCover(miniField("img"), miniField("ph"), "");
    coverText(miniField("ph"), "");
    return;
  }
  const hot = bestRank(n.charts);
  const word = STATUS_WORD[n.status] || n.status;
  const state = !n.loved ? word : n.status === "playing" ? loveText(n) : `${word} · ${loveText(n)}`;
  const head = `${n.player} · ${state}${hot ? ` · ${hotLabel(hot, true)}` : ""}`;
  if (titleEl && titleEl.textContent !== head) titleEl.textContent = head;
  marquee(n.title);
  miniText("artist", n.artist);
  miniText("album", n.album ? ` · ${n.album}` : "");
  const tag = n.tags && n.tags[0];
  miniField("tag").hidden = !tag;
  if (tag) miniText("tagText", tag);
  miniField("progress").hidden = !(n.length > 0 && !n.live);
  miniField("times").hidden = false;
  paintMiniClock(n);
  setCover(miniField("img"), miniField("ph"), n.cover);
  coverText(miniField("ph"), coverName(n));
}

let stAt = 0;

function since(n) {
  return n && n.status === "playing" ? Math.max(0, (performance.now() - stAt) / 1000) : 0;
}

function posNow(n) {
  const p = (n.synced ? n.position : n.played) + since(n);
  return n.length > 0 ? Math.min(p, n.length) : p;
}

function playedNow(n) { return n.played + since(n); }

function leftNow(n) { return Math.max(0, n.left - since(n)); }

function leftText(n, short) {
  if (st.paused) return "скробблинг выключен";
  if (n.live) return liveNote(n);
  if (!n.scrobblable) return short ? "слишком короткий" : "слишком короткий трек";
  if (n.scrobbled) return "засчитано";
  return `засчитается через ${fmtLeft(leftNow(n))}`;
}

function paintBar(box, bar, frac, step) {
  const inner = Math.max(0, box.clientWidth - 6);
  const w = `${Math.floor(inner * Math.min(1, Math.max(0, frac)) / step) * step}px`;
  if (bar.style.width !== w) bar.style.width = w;
}

function paintMiniClock(n) {
  const shown = posNow(n);
  const hasBar = n.length > 0 && !n.live;
  miniText("time", hasBar ? `${fmtDur(shown)} / ${fmtDur(n.length)}` : `прослушано ${fmtDur(playedNow(n))}`);
  miniText("left", leftText(n, true));
  if (hasBar) paintBar(miniField("progress"), miniField("bar"), shown / n.length, 6);
  placeMoment(miniField("progress"), { played: miniField("momPlayed"), ahead: miniField("momAhead"),
    ha: miniField("heartA"), hb: miniField("heartB"), cell: 1 }, hasBar ? n.moment : null, n.length, miniField("bar"), 6);
}

function paintNowClock(n) {
  const shown = posNow(n);
  setText("played", n.length > 0
    ? `${n.synced ? "" : "прослушано "}${fmtDur(shown)} из ${fmtDur(n.length)}`
    : `прослушано ${fmtDur(playedNow(n))}`);
  setText("left", leftText(n, false));
  const frac = n.length > 0 ? shown / n.length : (n.target > 0 ? playedNow(n) / n.target : 0);
  paintBar(field("progress"), field("bar"), frac, 10);
  paintMomentBar(n);
}

setInterval(() => {
  if (!st || !st.now || document.hidden) return;
  if (mounted === "now" && field("played")) paintNowClock(st.now);
  if (mounted === "now") {
    if (marking) paintMoment();
    paintLyricLine(st.now);
  }
  if (isMini()) paintMiniClock(st.now);
}, 200);

let readySent = false;
function frameReady() {
  if (!host || readySent) return;
  readySent = true;
  const names = THEME_FONTS[document.documentElement.dataset.theme] || THEME_FONTS.standard;
  const fonts = document.fonts ? Promise.all(names.map((f) => document.fonts.load(`16px "${f}"`, "Aя"))) : Promise.resolve();
  fonts.catch(() => {}).then(() => tell("ready"));
}

const esc = (s) => String(s ?? "").replace(/[&<>"']/g, (c) =>
  ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);

function plural(n, one, few, many) {
  const m10 = n % 10, m100 = n % 100;
  if (m10 === 1 && m100 !== 11) return one;
  if (m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14)) return few;
  return many;
}

function fmtLeft(sec) { return fmtDur(Math.ceil(Math.max(0, sec))); }

function fmtDur(sec) {
  sec = Math.max(0, Math.floor(sec + 1e-6));
  const h = Math.floor(sec / 3600), m = Math.floor(sec % 3600 / 60), s = sec % 60;
  const ss = String(s).padStart(2, "0");
  return h ? `${h}:${String(m).padStart(2, "0")}:${ss}` : `${m}:${ss}`;
}

function fmtSpan(sec) {
  const m = Math.floor(sec / 60), h = Math.floor(m / 60);
  return h ? `${h} ч ${m % 60} мин` : `${m} мин`;
}

const two = (n) => String(n).padStart(2, "0");
const clock = (iso) => { const d = new Date(iso); return `${two(d.getHours())}:${two(d.getMinutes())}`; };

function dayClock(iso) {
  const d = new Date(iso), now = new Date();
  const start = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const diff = Math.floor((start - new Date(d.getFullYear(), d.getMonth(), d.getDate())) / 86400000);
  const day = diff === 0 ? "сегодня" : diff === 1 ? "вчера" : `${two(d.getDate())}.${two(d.getMonth() + 1)}`;
  return `${day}, ${clock(iso)}`;
}

const num = (n) => String(n).replace(/\B(?=(\d{3})+(?!\d))/g, "\u00a0");

const date = (iso) => { const d = new Date(iso); return `${two(d.getDate())}.${two(d.getMonth() + 1)}.${d.getFullYear()}`; };

function splitFeat(title) {
  const m = /^(.*?)\s*([[(](?:feat\.?|ft\.?|featuring|with)\s.*)$/i.exec(title);
  return m && m[1] ? [m[1], m[2]] : [title, ""];
}

const $ = (sel, root = document) => root.querySelector(sel);
const app = $("#app"), nav = $("#nav"), main = $("#main"), statusbar = $("#status"), modal = $("#modal");

let st = null;
let queue = [];
let selected = null;
let settings = null;
let mounted = "";
let mountedLogin = null;
let dismissed = "";
let login = { busy: false, waiting: false, error: "", timer: 0 };
const sigs = {};

function field(name) { return main.querySelector(`[data-f="${name}"]`); }
function setText(name, text) { const el = field(name); if (el && el.textContent !== text) el.textContent = text; }
function show(name, on) { const el = field(name); if (el) el.hidden = !on; }

function windowHTML(title, body, extra = "", panel = "") {
  const close = panel
    ? `<button class="tb-close" data-act="closePanel" data-panel="${panel}" aria-label="закрыть окно «${esc(title)}»">${pix("close", 1)}</button>`
    : `<span class="tb-close" aria-hidden="true">${pix("close", 1)}</span>`;
  return `<section class="window ${extra}"${panel ? ` data-panel="${panel}"` : ""}><div class="titlebar"><span>${esc(title)}</span>` +
    `${close}</div><div class="window-body">${body}</div></section>`;
}

function mountNav() {
  nav.innerHTML = [["now", "note", "Сейчас"], ["top", "chart", "Топ"], ["radio", "radio", "Радио"], ["queue", "list", "Очередь"],
    ["settings", "gear", "Настройки"]]
    .map(([v, icon, label]) => `<button data-act="nav" data-view="${v}">${pix(icon, 2)}<span>${label}</span>` +
      (v === "queue" ? `<span class="badge" data-badge hidden></span>` : "") + `</button>`).join("");
}

function updateNav() {
  for (const b of nav.querySelectorAll("button")) {
    if (b.dataset.view === mounted) b.setAttribute("aria-current", "page");
    else b.removeAttribute("aria-current");
  }
  const q = nav.querySelector('[data-view="queue"]');
  if (q) q.hidden = !st.loggedIn && !st.queueLen;
  const badge = nav.querySelector("[data-badge]");
  if (badge) {
    badge.hidden = !st.queueLen;
    badge.textContent = st.queueLen > 99 ? "99+" : String(st.queueLen);
  }
}

const LINK = { ok: "в сети", offline: "нет связи", auth: "нужен вход", nologin: "вход не выполнен" };

function updateStatus() {
  const who = st.loggedIn ? `${st.user} · Last.fm: ${LINK[st.link.state] || "в сети"}` : "Last.fm: без входа · локально";
  const n = st.today.count;
  const cells = [who, st.loggedIn || st.queueLen ? `в очереди: ${st.queueLen}` : `сохранено: ${num(st.all.count)}`,
    `сегодня: ${n} ${plural(n, "трек", "трека", "треков")} · ${fmtSpan(st.today.seconds)}`];
  const html = cells.map((c) => `<div>${esc(c)}</div>`).join("");
  if (statusbar.innerHTML !== html) statusbar.innerHTML = html;
}

function mountNow() {
  main.innerHTML =
    windowHTML("Сейчас играет", `<div class="now">
      <div class="art art-note" data-f="art"><div data-f="artText">${pix("note", 20)}</div></div>
      <div class="now-info">
        <div class="chip-line"><div class="chip" data-f="chip">${pix("note", 2)}<span data-f="chipText"></span></div><span class="hot love" data-f="love" hidden>${pix("heart", 2)}<span data-f="loveText"></span></span></div>
        <div class="t32 track-title hot-line"><div class="ellipsis" data-f="title" data-act="nowCard" data-kind="track"></div><span class="hot" data-f="hotTrack" hidden></span></div>
        <div class="ellipsis track-feat" data-f="feat"></div>
        <div class="t24 track-artist hot-line"><div class="ellipsis" data-f="artist" data-act="nowCard" data-kind="artist"></div><span class="hot" data-f="hotArtist" hidden></span></div>
        <div class="muted track-album hot-line"><div class="ellipsis" data-f="album" data-act="nowCard" data-kind="album"></div><span class="hot" data-f="hotAlbum" hidden></span></div>
        <div class="tags" data-f="tags" hidden></div>
        <div class="progress" data-f="progress"><div data-f="bar"></div><span class="moment-played" data-f="momPlayed" hidden></span><span class="moment-ahead" data-f="momAhead" hidden></span><span class="mheart" data-f="heartA" hidden>${heartMarker(2)}</span><span class="mheart" data-f="heartB" hidden>${heartMarker(2)}</span></div>
        <div class="row now-row" data-f="times"><span class="muted" data-f="played"></span><span class="note" data-f="left"></span></div>
        <div class="row moment-row" data-f="momentRow">
          <span class="moment-info">${pix("heart", 2)}<span class="ellipsis" data-f="momentText"></span></span>
          <span class="moment-btns">
            <button class="small" data-act="markMoment"><span data-f="markText">отметить момент</span></button>
            <button class="small" data-act="clearMoment" data-f="clearMoment" hidden>сбросить</button>
          </span>
        </div>
        <p class="muted idle-text" data-f="idle">Включите музыку в Spotify, AIMP, браузере<br>или радио в программе — трек появится здесь.</p>
        <label class="check now-check"><input type="checkbox" data-act="pause" data-f="enabled"> скробблинг включён</label>
      </div></div>`, "", "now") +
    windowHTML("Рекомендации", `<div class="recs">
        <div class="recs-col"><div class="recs-head">${pix("person", 2)}<span>исполнители</span></div><div class="recs-list" data-f="recArtists"></div></div>
        <div class="recs-col"><div class="recs-head">${pix("note", 2)}<span>треки</span></div><div class="recs-list" data-f="recTracks"></div></div>
      </div>
      <p class="muted recs-note ellipsis" data-f="recNote"></p>`, "recs-window", "recs") +
    windowHTML("Текст песни", `<div class="lyrics" data-f="lyrics" tabindex="0"></div>
      <div class="buttons lyrics-foot"><span class="muted grow ellipsis" data-f="lyricsNote"></span>
        <button class="w210" data-act="open" data-what="lyricsdir" data-f="lyricsFolder" hidden>${pix("folder", 2)}<span>папка с текстами</span></button>
        <button class="w210" data-act="open" data-what="lyrics" data-f="lyricsSearch">${pix("ext", 2)}<span>найти в интернете</span></button>
      </div>`, "lyrics-window", "lyrics") +
    windowHTML("Недавние", `<div class="table recent"><table>
        <colgroup><col class="c-time"><col><col class="c-status"></colgroup>
        <thead><tr><th>время</th><th>трек</th><th>статус</th></tr></thead>
        <tbody data-f="recent"></tbody></table></div>
      <div class="buttons after-table"><span class="muted grow" data-f="total"></span>
        <button class="w120" data-act="refresh">обновить</button>
        <button class="w210" data-act="open" data-what="profile" data-f="profileBtn">${pix("ext", 2)}<span>профиль на last.fm</span></button>
        <button class="w210" data-act="nav" data-view="login" data-f="loginBtn">${pix("key", 2)}<span>войти в last.fm</span></button>
      </div>`, "", "recent") +
    `<div class="dock" data-f="dock" hidden></div>`;
  const box = field("lyrics");
  for (const ev of ["wheel", "pointerdown", "keydown", "touchstart"]) {
    box.addEventListener(ev, () => { lyrManual = performance.now() + LYR_MANUAL; }, { passive: true });
  }
  watchBar(field("progress"));
}

let marking = null;
let drag = null;

function canMark(n) { return Boolean(n) && n.length > 0 && !n.live; }

function effMoment(n) {
  if (!canMark(n)) return null;
  const key = trackKey(n);
  if (drag && drag.key === key && drag.to !== null) {
    return { start: Math.min(drag.from, drag.to), end: Math.max(drag.from, drag.to), draft: true };
  }
  if (marking && marking.key === key) return { start: marking.start, end: Math.max(marking.start, posNow(n)), draft: true };
  return n.moment || null;
}

function placeMoment(box, parts, m, length, bar, step) {
  const { played, ahead, ha, hb, cell } = parts;
  if (!box || !played || !ahead) return;
  const inner = Math.max(0, box.clientWidth - 6);
  if (!m || !(length > 0) || !inner) {
    for (const el of [played, ahead, ha, hb]) if (el) el.hidden = true;
    return;
  }
  const a = Math.floor(inner * Math.max(0, m.start) / length / step) * step;
  const b = Math.min(inner, Math.max(a + step, Math.ceil(inner * Math.min(m.end, length) / length / step) * step));
  const fill = parseFloat(bar && bar.style.width) || 0;
  const split = m.draft ? a : Math.min(Math.max(fill, a), b);
  stripe(played, a, split);
  stripe(ahead, split, b);
  const w = 9 * cell;
  const close = b - a < w;
  pin(ha, close ? (a + b) / 2 : a, w, box.clientWidth);
  if (hb) {
    hb.hidden = close;
    if (!close) pin(hb, b, w, box.clientWidth);
  }
}

function pin(el, x, w, total) {
  if (!el) return;
  el.hidden = false;
  const left = `${Math.round(Math.min(Math.max(3 + x - w / 2, 0), total - w))}px`;
  if (el.style.left !== left) el.style.left = left;
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

function paintMomentBar(n, m = effMoment(n)) {
  const box = field("progress");
  if (!box) return;
  placeMoment(box, { played: field("momPlayed"), ahead: field("momAhead"), ha: field("heartA"), hb: field("heartB"), cell: 2 },
    m, n ? n.length : 0, field("bar"), 10);
}

function momentText(m) {
  if (marking && m) return `отмечаю с ${fmtDur(m.start)}… в конце нажмите «готово»`;
  if (m && m.draft) return `${fmtDur(m.start)}–${fmtDur(m.end)} — отпустите кнопку мыши`;
  if (!m) return "любимого момента пока нет";
  return `${fmtDur(m.start)}–${fmtDur(m.end)} · ${m.manual ? "отмечен вами" : "чаще всего переслушиваете"}`;
}

function paintMoment() {
  const n = st && st.now;
  if (marking && (!n || marking.key !== trackKey(n))) marking = null;
  const m = effMoment(n);
  if (field("progress")) {
    field("progress").classList.toggle("pickable", canMark(n));
    paintMomentBar(n, m);
    show("momentRow", canMark(n));
    if (canMark(n)) {
      setText("momentText", momentText(m));
      setText("markText", marking ? "готово" : "отметить момент");
      show("clearMoment", Boolean(n.moment && n.moment.manual) && !marking && !drag);
    }
  }
  paintLyricsFav(m);
}

function barPos(ev, box, n) {
  const r = box.getBoundingClientRect();
  const inner = Math.max(1, r.width - 6);
  return Math.min(Math.max(ev.clientX - r.left - 3, 0), inner) / inner * n.length;
}

function watchBar(box) {
  box.addEventListener("pointerdown", (ev) => {
    const n = st && st.now;
    if (!canMark(n) || ev.button !== 0) return;
    marking = null;
    drag = { key: trackKey(n), from: barPos(ev, box, n), to: null };
    box.setPointerCapture(ev.pointerId);
  });
  box.addEventListener("pointermove", (ev) => {
    if (!drag || !st.now) return;
    drag.to = barPos(ev, box, st.now);
    paintMoment();
  });
  box.addEventListener("pointerup", () => {
    const d = drag;
    drag = null;
    const n = st && st.now;
    if (!d || !n || d.to === null || trackKey(n) !== d.key || Math.abs(d.to - d.from) < 2) {
      paintMoment();
      return;
    }
    saveMoment(n, Math.min(d.from, d.to), Math.max(d.from, d.to)).catch(showError);
  });
  box.addEventListener("pointercancel", () => { drag = null; paintMoment(); });
}

async function saveMoment(n, start, end) {
  start = Math.max(0, Math.round(start * 10) / 10);
  end = Math.min(n.length, Math.round(end * 10) / 10);
  const before = n.moment;
  n.moment = { start, end, manual: true };
  paintMoment();
  updateMini();
  try {
    await api("/api/moment", { artist: n.artist, title: n.title, length: n.length, start, end });
  } catch (e) {
    n.moment = before;
    paintMoment();
    throw e;
  }
}

function paintLyricsFav(m) {
  const box = field("lyrics");
  const d = lyr.data;
  if (!box || !box.classList.contains("synced") || !d || !d.synced || !st.now || lyr.key !== trackKey(st.now)) return;
  const sig = `${box.dataset.sig}|${m ? `${m.start}|${m.end}` : "-"}`;
  if (box.dataset.fav === sig) return;
  box.dataset.fav = sig;
  const lines = d.synced;
  const len = st.now.length || 0;
  for (const el of box.querySelectorAll(".ly[data-i]")) {
    const i = Number(el.dataset.i);
    const from = lines[i].t;
    const to = i + 1 < lines.length ? lines[i + 1].t : Math.max(len, from + 5);
    el.classList.toggle("fav", Boolean(m) && from < m.end && to > m.start);
  }
}

const PANELS = ["now", "recs", "lyrics", "recent"];
const PANEL_NAME = { now: "Сейчас играет", recs: "Рекомендации", lyrics: "Текст песни", recent: "Недавние" };
const PANEL_ICON = { now: "note", recs: "disc", lyrics: "mic", recent: "list" };

let recs = { key: "", data: null, at: 0 };
let recsBusy = false;

function recsKey() { return st && st.now ? `${st.now.artist}\u0000${st.now.title}` : "idle"; }

function recsShown() { return mounted === "now" && !isMini() && !closedSet().has("recs"); }

async function loadRecs() {
  if (recsBusy || !st || !recsShown()) return;
  const key = recsKey();
  const d = recs.key === key ? recs.data : null;
  const age = performance.now() - recs.at;
  if (d && (d.state === "found" || d.state === "none")) return;
  if (d && d.state === "loading" && age < 1500) return;
  if (d && (d.state === "error" || d.state === "nokeys") && age < 60000) return;
  recsBusy = true;
  try {
    const data = await api("/api/recs");
    recs = { key, data, at: performance.now() };
    if (mounted === "now") paintRecs();
  } catch {
    recs.at = performance.now();
  } finally {
    recsBusy = false;
  }
}

function recsMeta(plays) {
  return plays ? `<span class="rec-meta">${num(plays)} ${plural(plays, "раз", "раза", "раз")}</span>` : `<span class="rec-meta rec-new">новое</span>`;
}

function recsMsg(text) { return `<p class="recs-msg muted">${esc(text)}</p>`; }

function paintRecs() {
  const a = field("recArtists");
  const t = field("recTracks");
  if (!a || !t) return;
  const d = recs.data;
  let ah = "";
  let th = "";
  let note = "";
  if (!d || d.state === "loading") {
    ah = th = recsMsg("ищу на Last.fm…");
  } else if (d.state === "none") {
    note = "Включите музыку — здесь появятся похожие исполнители и треки с Last.fm.";
  } else if (d.state === "nokeys") {
    note = "Для рекомендаций нужны ключи приложения Last.fm — их можно ввести в настройках.";
  } else if (d.state === "error") {
    note = "Не получилось связаться с Last.fm — программа попробует ещё раз через пару минут.";
  } else {
    ah = d.artists.length ? d.artists.map((r) => `<button class="rec" data-act="openLastfm" data-artist="${esc(r.name)}">` +
      `<span class="ellipsis">${esc(r.name)}</span>${recsMeta(r.plays)}</button>`).join("")
      : recsMsg("Last.fm не знает похожих исполнителей");
    th = d.tracks.length ? d.tracks.map((r) => `<button class="rec" data-act="openLastfm" data-artist="${esc(r.artist)}" ` +
      `data-track="${esc(r.title)}"><span class="ellipsis">${esc(r.title)} ` +
      `<span class="muted">— ${esc(r.artist)}</span></span>${recsMeta(r.plays)}</button>`).join("")
      : recsMsg("Last.fm не знает похожих треков");
    note = `с Last.fm · похожее на «${splitFeat(d.title)[0]}» — ${d.artist}`;
  }
  if (a.dataset.sig !== ah) {
    a.dataset.sig = ah;
    a.innerHTML = ah;
  }
  if (t.dataset.sig !== th) {
    t.dataset.sig = th;
    t.innerHTML = th;
  }
  setText("recNote", note);
}
let panelsLocal = null;
let panelsAt = 0;

function closedSet() {
  const server = (st && st.closed) || [];
  if (panelsLocal && (server.join() === panelsLocal.join() || performance.now() - panelsAt > 4000)) panelsLocal = null;
  return new Set(panelsLocal || server);
}

function paintPanels() {
  const closed = closedSet();
  for (const sec of main.querySelectorAll("section[data-panel]")) {
    const off = closed.has(sec.dataset.panel);
    if (sec.hidden !== off) sec.hidden = off;
  }
  const dock = field("dock");
  if (!dock) return;
  const list = PANELS.filter((p) => closed.has(p));
  const html = list.length ? `<span class="muted dock-label">${list.length === PANELS.length ? "все окна закрыты:" : "закрытые окна:"}</span>` +
    list.map((p) => `<button data-act="openPanel" data-panel="${p}">` +
      `${pix(PANEL_ICON[p], 2)}<span>${PANEL_NAME[p]}</span></button>`).join("") : "";
  if (dock.dataset.sig !== html) {
    dock.dataset.sig = html;
    dock.innerHTML = html;
  }
  dock.hidden = !list.length;
}

async function setClosed(list, focus) {
  panelsLocal = PANELS.filter((p) => list.includes(p));
  panelsAt = performance.now();
  paintPanels();
  paintLyrics();
  const el = main.querySelector(focus);
  if (el) el.focus();
  loadLyrics();
  loadRecs();
  await api("/api/panels", { closed: panelsLocal });
  panelsAt = performance.now();
}

const LYR_MANUAL = 5000;
const LYR_LEAD = 0.15;
const LYR_RECHECK = { missing: 15000, error: 30000 };
const LYR_SOURCE = {
  file: "текст из файла рядом с песней", tags: "текст из тегов файла", folder: "текст из папки с текстами",
  lrclib: "текст с lrclib.net", lyricsovh: "текст с lyrics.ovh", demo: "демо: так выглядит найденный текст",
};
const reduceMotion = matchMedia("(prefers-reduced-motion: reduce)");
let lyr = { key: "", data: null, at: 0 };
let lyrBusy = false;
let lyrCur = -1;
let lyrManual = 0;

function trackKey(t) { return t ? `${t.artist}\u0000${t.title}\u0000${t.album || ""}` : ""; }

function lyricsShown() { return mounted === "now" && !isMini() && !closedSet().has("lyrics"); }

function followable(n) { return Boolean(n) && !n.live && (n.synced || n.fromStart); }

async function loadLyrics(force) {
  if (lyrBusy || !st || !st.now || !lyricsShown()) return;
  const key = trackKey(st.now);
  const d = lyr.key === key ? lyr.data : null;
  if (d && !(force && d.state === "missing")) {
    const wait = d.state === "loading" ? 900 : LYR_RECHECK[d.state];
    if (!wait || performance.now() - lyr.at < wait) return;
  }
  lyrBusy = true;
  try {
    const data = await api("/api/lyrics");
    lyr = { key: data.state === "none" ? "" : trackKey(data), data, at: performance.now() };
    if (mounted === "now") paintLyrics();
  } catch {
    lyr.at = performance.now();
  } finally {
    lyrBusy = false;
  }
}

function lyricsFile(n) {
  return `${n.artist} - ${n.title}`.replace(/[\u0000-\u001f\\/:*?"<>|]/g, "_") + ".lrc";
}

window.addEventListener("focus", () => loadLyrics(true));

function lyricsMsg(...lines) {
  return lines.map((l, i) => `<p class="lyrics-msg${i ? " muted" : ""}">${l}</p>`).join("");
}

function lyricsView() {
  const n = st.now;
  if (!n) return { sig: "idle", html: () => lyricsMsg("Включите музыку — здесь появится текст песни."), note: "" };
  const key = trackKey(n);
  const d = lyr.key === key ? lyr.data : null;
  if (!d || d.state === "loading") return { sig: `${key}|loading`, html: () => lyricsMsg("Ищу текст песни…"), note: "" };
  if (d.state === "missing") {
    return { sig: `${key}|missing`, note: "", folder: true, html: () => lyricsMsg("Текст этой песни не нашёлся.",
      `Положите файл «${esc(lyricsFile(n))}» в папку с текстами — он появится здесь сам. Или поищите текст в интернете.`) };
  }
  if (d.state !== "found") {
    return { sig: `${key}|error`, note: "", html: () => lyricsMsg("Не получилось связаться с сайтами текстов.",
      "Программа попробует ещё раз через пару минут.") };
  }
  const source = LYR_SOURCE[d.source] || "";
  if (d.instrumental) {
    return { sig: `${key}|instrumental`, note: source,
      html: () => `<div class="lyrics-msg lyrics-instr">${pix("note", 3)}<span>песня без слов</span></div>` };
  }
  if (d.synced && d.synced.length) {
    return {
      sig: `${key}|synced|${d.synced.length}`, synced: true,
      note: `${source} · ${followable(n) ? "строки идут вместе с песней" : "плеер не сообщает, где играет песня, — без подсветки"}`,
      html: () => d.synced.map((l, i) => l.text ? `<p class="ly" data-i="${i}">${esc(l.text)}</p>`
        : `<p class="ly gap" data-i="${i}">${pix("note", 1)}</p>`).join(""),
    };
  }
  const plain = d.plain || "";
  return {
    sig: `${key}|plain|${plain.length}`, note: source,
    html: () => plain.split("\n").map((s) => s.trim() ? `<p class="ly">${esc(s)}</p>` : `<p class="ly gap"></p>`).join(""),
  };
}

function paintLyrics() {
  const box = field("lyrics");
  if (!box || !st) return;
  const v = lyricsView();
  if (box.dataset.sig !== v.sig) {
    box.dataset.sig = v.sig;
    box.innerHTML = v.html();
    box.classList.toggle("synced", Boolean(v.synced));
    box.scrollTop = 0;
    lyrCur = -1;
    lyrManual = 0;
  }
  setText("lyricsNote", v.note);
  show("lyricsFolder", Boolean(v.folder));
  show("lyricsSearch", Boolean(st.now));
  if (v.synced) {
    paintLyricsFav(effMoment(st.now));
    paintLyricLine(st.now);
  }
}

function lineAt(lines, pos) {
  let lo = 0, hi = lines.length;
  while (lo < hi) {
    const mid = (lo + hi) >> 1;
    if (lines[mid].t <= pos) lo = mid + 1;
    else hi = mid;
  }
  return lo - 1;
}

function paintLyricLine(n) {
  const box = field("lyrics");
  const d = lyr.data;
  if (!box || !n || !box.classList.contains("synced") || !d || !d.synced || lyr.key !== trackKey(n) || !followable(n)) return;
  const i = lineAt(d.synced, posNow(n) + LYR_LEAD);
  const back = lyrManual > 0 && performance.now() > lyrManual;
  if (i === lyrCur && !back) return;
  if (back) lyrManual = 0;
  if (i !== lyrCur) {
    const old = box.querySelector(".ly.cur");
    if (old) old.classList.remove("cur");
    lyrCur = i;
  }
  const el = i >= 0 ? box.querySelector(`[data-i="${i}"]`) : null;
  if (el) el.classList.add("cur");
  if (lyrManual) return;
  const top = el ? el.offsetTop - (box.clientHeight - el.offsetHeight) / 2 : 0;
  box.scrollTo({ top: Math.max(0, top), behavior: reduceMotion.matches ? "auto" : "smooth" });
}

const STATUS_WORD = { playing: "играет", paused: "пауза", stopped: "остановлен" };

const HOT_KIND = { track: "трек", artist: "исполнитель", album: "альбом" };
const HOT_OF = { week: "недели", month: "месяца", year: "года" };
const HOT_FOR = { week: "за неделю", month: "за месяц", year: "за год" };
const HOT_ORDER = { week: 0, month: 1, year: 2 };
const HOT_KINDS = ["track", "artist", "album"];

function loveText(n) { return n.status === "playing" ? "любимка играет" : "любимка"; }

function paintTags(tags) {
  const box = field("tags");
  if (!box) return;
  const list = (tags || []).slice(0, 3);
  const html = list.map((t) => `<span class="tag">${pix("hash", 2)}<span>${esc(t)}</span></span>`).join("");
  if (box.dataset.sig !== html) {
    box.dataset.sig = html;
    box.innerHTML = html;
  }
  box.hidden = !list.length;
}

function bestRank(charts, kind) {
  let best = null;
  for (const c of charts || []) {
    if (kind && c.kind !== kind) continue;
    if (!best || c.rank < best.rank || (c.rank === best.rank &&
      (kind ? HOT_ORDER[c.period] > HOT_ORDER[best.period]
        : HOT_KINDS.indexOf(c.kind) < HOT_KINDS.indexOf(best.kind) ||
          (c.kind === best.kind && HOT_ORDER[c.period] > HOT_ORDER[best.period])))) best = c;
  }
  return best;
}

function hotLabel(c, withKind) {
  if (c.rank === 1) return `${HOT_KIND[c.kind]} ${HOT_OF[c.period]}`;
  return `${withKind ? HOT_KIND[c.kind] + " " : ""}№${c.rank} ${HOT_FOR[c.period]}`;
}

function paintHot(f, charts, kind) {
  const el = field(f);
  if (!el) return;
  const c = bestRank(charts, kind);
  el.hidden = !c;
  if (!c) return;
  const html = `${pix(c.rank === 1 ? "crown" : "star", 2)}<span>${esc(hotLabel(c, false))}</span>`;
  if (el.dataset.sig !== html) {
    el.dataset.sig = html;
    el.innerHTML = html;
  }
  el.classList.toggle("top1", c.rank === 1);
}
const SCROBBLE_WORD = { sent: "отправлен", queued: "в очереди", ignored: "отклонён", lastfm: "в last.fm", local: "сохранён" };

function initial(text) {
  const letter = String(text || "").match(/\p{L}/u);
  return (letter ? letter[0] : [...String(text || "").trim()][0] || "").toUpperCase();
}

function coverText(el, text) {
  if (!el || el.dataset.t === text) return;
  el.dataset.t = text;
  el.dataset.i = initial(text);
  el.dataset.len = text.length <= 7 ? "s" : text.length <= 16 ? "m" : text.length <= 40 ? "l" : "xl";
}

function coverName(n) { return n ? (n.album || splitFeat(n.title)[0]) : ""; }

function updateNow() {
  const n = st.now;
  const playing = Boolean(n);
  coverText(field("artText"), coverName(n));
  field("art").dataset.s = playing ? n.status : "off";
  show("chip", playing);
  show("progress", playing && !n.live);
  show("times", playing);
  show("idle", !playing);
  for (const f of ["title", "artist", "album"]) field(f).classList.toggle("click", playing && (f !== "album" || Boolean(n.album)));
  if (!playing) {
    setText("title", "Ничего не играет");
    setText("feat", "");
    setText("artist", "");
    setText("album", "");
  } else {
    const [title, feat] = splitFeat(n.title);
    setText("chipText", `${n.player} · ${STATUS_WORD[n.status] || n.status}`);
    setText("title", title);
    setText("feat", feat);
    setText("artist", n.artist);
    setText("album", [n.album, n.length > 0 ? fmtDur(n.length) : ""].filter(Boolean).join(" · "));
    paintNowClock(n);
  }
  const charts = playing ? n.charts : null;
  paintHot("hotTrack", charts, "track");
  paintHot("hotArtist", charts, "artist");
  paintHot("hotAlbum", charts, "album");
  const loved = playing && Boolean(n.loved);
  show("love", loved);
  if (loved) setText("loveText", loveText(n));
  paintTags(playing ? n.tags : null);
  const cb = field("enabled");
  if (cb.checked === st.paused) cb.checked = !st.paused;

  const recent = st.recent.slice(0, 4);
  const sig = JSON.stringify(recent);
  if (sigs.recent !== sig) {
    sigs.recent = sig;
    field("recent").innerHTML = recent.length ? recent.map((r) =>
      `<tr class="click" ${cardAttrs("track", r)}><td>${clock(r.started)}</td><td>${esc(r.artist)} — ${esc(r.title)}</td>` +
      `<td class="right"${r.reason ? ` title="${esc(r.reason)}"` : ""}>${SCROBBLE_WORD[r.status] || ""}</td></tr>`).join("")
      : `<tr class="empty"><td colspan="3">Здесь появятся засчитанные треки.</td></tr>`;
  }
  const c = st.all.count;
  setText("total", `${st.loggedIn ? "всего в истории" : "на этом компьютере"}: ${num(c)} ${plural(c, "прослушивание", "прослушивания", "прослушиваний")}`);
  show("profileBtn", st.loggedIn);
  show("loginBtn", !st.loggedIn);
  paintPanels();
  paintRecs();
  paintLyrics();
  paintMoment();
}

function mountQueue() {
  main.innerHTML =
    windowHTML("Системное сообщение", `<div class="message">${pix("warn", 4)}<div class="message-body">
        <h2 data-f="msgTitle"></h2><p class="muted" data-f="msgText"></p></div></div>
      <div class="buttons"><button class="w180" data-act="msgAction" data-f="msgBtn"></button>
        <button class="default" data-act="dismiss">ok</button></div>`, "msg").replace("<section", '<section data-f="msg"') +
    `<section class="window"><div class="titlebar"><span data-f="qTitle"></span><span class="tb-close" aria-hidden="true">${pix("close", 1)}</span></div>
      <div class="window-body"><div class="table queue"><table>
        <colgroup><col class="c-day"><col><col class="c-tries"></colgroup>
        <thead><tr><th>время</th><th>трек</th><th>попыток</th></tr></thead>
        <tbody data-f="qrows"></tbody></table></div>
      <div class="buttons after-table queue-buttons">
        <button class="default w200" data-act="flush" data-f="flushBtn">отправить сейчас</button>
        <button class="w170" data-act="delete" data-f="delBtn">удалить запись</button>
        <button class="w200" data-act="clear" data-f="clearBtn">очистить очередь</button></div>
      <p class="muted queue-note">Записи старше 14 дней Last.fm не принимает — они удаляются сами.</p></div></section>`;
}

function retryText() {
  const left = (new Date(st.link.retryAt) - Date.now()) / 1000;
  if (!st.link.retryAt || left <= 0) return "вот-вот";
  if (left < 60) return "меньше чем через минуту";
  const m = Math.ceil(left / 60);
  return `через ${m} ${plural(m, "минуту", "минуты", "минут")}`;
}

function queueMessage() {
  if (!st.queueLen) return null;
  if (st.link.state === "offline") {
    return { key: "offline", title: "Нет связи с Last.fm !", btn: "повторить сейчас", act: "flush",
      text: `Следующая попытка ${retryText()}. Прослушивания хранятся на диске.` };
  }
  if (st.link.state === "auth") {
    return { key: "auth", title: "Last.fm не принял вход !", btn: "войти заново", act: "relogin",
      text: "Войдите в аккаунт заново — прослушивания подождут в очереди." };
  }
  return null;
}

function updateQueue() {
  const msg = queueMessage();
  if (!msg || msg.key !== dismissed) {
    if (!msg) dismissed = "";
  }
  const showMsg = Boolean(msg) && msg.key !== dismissed;
  show("msg", showMsg);
  if (showMsg) {
    setText("msgTitle", msg.title);
    setText("msgText", msg.text);
    setText("msgBtn", msg.btn);
    field("msgBtn").dataset.do = msg.act;
  }

  if (selected !== null && !queue.some((q) => q.id === selected)) selected = null;
  const n = queue.length;
  setText("qTitle", n ? `Очередь — ${n} ${plural(n, "прослушивание", "прослушивания", "прослушиваний")}` : "Очередь пуста");
  const sig = JSON.stringify([queue, selected]);
  if (sigs.queue !== sig) {
    sigs.queue = sig;
    field("qrows").innerHTML = n ? queue.map((q) =>
      `<tr data-act="select" data-id="${q.id}"${q.id === selected ? ' class="sel"' : ""}>` +
      `<td class="muted-cell">${dayClock(q.started)}</td><td>${esc(q.artist)} — ${esc(q.title)}</td>` +
      `<td class="right">${q.attempts}</td></tr>`).join("")
      : `<tr class="empty"><td colspan="3">Всё отправлено — в очереди ничего нет.</td></tr>`;
  }
  field("flushBtn").disabled = !n || !st.loggedIn;
  field("delBtn").disabled = selected === null;
  field("clearBtn").disabled = !n;
}

const PERCENTS = [[0.5, "половины трека"], [0.75, "трёх четвертей трека"], [0.9, "90 % трека"]];

function mountSettings() {
  const s = settings;
  const opts = PERCENTS.some(([p]) => Math.abs(p - s.percent) < 0.001) ? PERCENTS
    : [...PERCENTS, [s.percent, `${Math.round(s.percent * 100)} % трека`]];
  const percent = opts.find(([p]) => Math.abs(p - s.percent) < 0.001)[0];
  const maxMin = Math.round(s.maxPlay / 60);
  const players = s.players.length ? `<div class="players">${s.players.map((p) =>
    `<label class="check"><input type="checkbox" data-player="${esc(p.name)}"${p.enabled ? " checked" : ""}>` +
    `<span>${esc(p.identity)}</span>${p.playing ? '<span class="note">играет</span>' : ""}</label>`).join("")}</div>`
    : `<p class="muted no-players">Плееры появятся здесь, как только в них что-нибудь заиграет.</p>`;

  main.innerHTML = windowHTML("Настройки", `
    <fieldset><legend>Аккаунт</legend>${st.loggedIn ? `<div class="account">
      <div class="avatar">${pix("heart", 2)}</div>
      <div class="who"><span class="t24">${esc(st.user)}</span>
        <span class="muted">${st.since ? `вход выполнен ${date(st.since)}` : ""}</span></div>
      <button class="w110" data-act="logout">выйти</button></div>
      <div class="logs lib-row"><span class="muted lib-line" data-f="libLine"></span>
        <button class="w200" data-act="libReload">загрузить заново</button></div>` : `<div class="account">
      <div class="avatar">${pix("heart", 2)}</div>
      <div class="who"><span class="t24">без Last.fm</span>
        <span class="muted">прослушивания хранятся на этом компьютере</span></div>
      <button class="w200" data-act="nav" data-view="login">${pix("key", 2)}<span>войти в last.fm</span></button></div>
      <div class="logs lib-row"><span class="muted lib-line" data-f="libLine"></span></div>`}</fieldset>
    <fieldset><legend>Правила засчитывания</legend>
      <div class="rule"><span>засчитывать трек после:</span>
        ${selectHTML("percent", opts, percent)}
        <span class="muted">или ${maxMin} ${maxMin === 1 ? "минуты" : "минут"}</span></div>
      <div class="muted rule-text">не учитывать треки короче ${Math.round(s.minLength)} секунд — так требует Last.fm</div>
      <label class="check rule-check"><input type="checkbox" id="radio"${s.radio ? " checked" : ""}> интернет-радио: засчитывать через ${maxMin} ${maxMin === 1 ? "минуту" : plural(maxMin, "минуту", "минуты", "минут")}</label>
    </fieldset>
    <fieldset><legend>Плееры, которые учитывать</legend>${players}</fieldset>
    <fieldset><legend>Система</legend>
      <div class="logs sys-row"><label class="check autostart"${s.canAutostart ? "" : " hidden"}><input type="checkbox" id="autostart"${s.autostart ? " checked" : ""}> запускать при входе в Windows</label>
        <button class="phone-btn" data-act="phone">${pix("phone", 2)}<span>QR-код для телефона</span></button></div>
      <div class="logs sys-theme"><span>тема:</span>${selectHTML("theme", THEMES, themePreview || s.theme)}
        <label class="check bursts"${(themePreview || s.theme) === "glitch" ? "" : " hidden"}><input type="checkbox" id="bursts"${(burstsPreview ?? s.bursts) ? " checked" : ""}> рывки</label>
        <button class="w170" data-act="open" data-what="logs">${pix("folder", 2)}<span>папка журнала</span></button></div>
      <div class="logs sys-lyrics"><label for="lyricsDir">тексты песен:</label>
        <input id="lyricsDir" type="text" spellcheck="false" autocomplete="off" value="${esc(s.lyricsDir || "")}" placeholder="${esc(s.lyricsAuto || "")}">
        <button class="w170" data-act="open" data-what="lyricsdir">${pix("folder", 2)}<span>открыть папку</span></button></div>
    </fieldset>
    <div class="buttons settings-buttons">
      <button class="default" data-act="settingsOk">ok</button>
      <button data-act="nav" data-view="now">отмена</button>
      <button data-act="settingsApply">применить</button></div>`, "settings");
}

function updateSettings() {
  setText("libLine", libShort(st.library));
  const playing = st.now ? st.now.player : "";
  for (const p of settings.players) {
    const label = main.querySelector(`[data-player="${CSS.escape(p.name)}"]`)?.closest("label");
    if (!label) continue;
    const want = playing && p.identity === playing && st.now.status === "playing";
    const note = label.querySelector(".note");
    if (want && !note) label.insertAdjacentHTML("beforeend", '<span class="note">играет</span>');
    if (!want && note) note.remove();
  }
}

function collectSettings() {
  const out = { ...settings };
  out.percent = Number($("#percent").value);
  out.radio = $("#radio").checked;
  out.autostart = $("#autostart").checked;
  out.theme = $("#theme").value;
  out.bursts = $("#bursts").checked;
  out.lyricsDir = $("#lyricsDir").value.trim();
  out.players = settings.players.map((p) => ({
    ...p, enabled: main.querySelector(`[data-player="${CSS.escape(p.name)}"]`).checked,
  }));
  return out;
}

async function saveSettings() {
  await api("/api/settings", collectSettings());
  settings = await api("/api/settings");
  st.theme = settings.theme;
  st.bursts = settings.bursts;
}

function mountLogin() {
  const ph = st.hasKeys ? "сохранён — можно не вводить" : "32 символа";
  main.innerHTML = windowHTML("Вход в Last.fm", `
    <div class="welcome">${appIcon(56)}<div>
      <h1>Подключить Last.fm ${pix("heartBig", 2)}</h1>
      <p class="muted">Прослушивания начнут отправляться в ваш профиль сами,</p>
      <p class="muted">а сохранённые за последние 14 дней уйдут сразу после входа.</p>
      <p class="muted">Без входа всё по-прежнему хранится на этом компьютере.</p></div></div>
    <fieldset><legend>1. Ключи приложения</legend>
      <div class="keys">
        <label for="apikey">API key</label><input id="apikey" type="text" placeholder="${ph}" spellcheck="false" autocomplete="off">
        <label for="secret">Shared secret</label><input id="secret" type="password" placeholder="${ph}" autocomplete="off"></div>
      <div class="buttons after-table"><button class="w200" data-act="open" data-what="keys">${pix("ext", 2)}<span>получить ключи</span></button></div>
    </fieldset>
    <fieldset><legend>2. Вход</legend>
      <button class="default big" data-act="login" data-f="loginBtn">${pix("key", 2)}<span data-f="loginLabel">войти через last.fm</span></button>
      <p class="muted login-hint" data-f="loginHint"></p>
      <p class="login-error" data-f="loginErr"></p>
    </fieldset>
    <div class="buttons login-back"><button data-act="nav" data-view="settings">назад</button></div>`, "login");
}

function updateLogin() {
  setText("loginLabel", login.waiting ? "открыть страницу ещё раз" : login.busy ? "подождите…" : "войти через last.fm");
  field("loginBtn").disabled = login.busy && !login.waiting;
  setText("loginHint", login.waiting
    ? "Ждём подтверждения: разрешите доступ на странице Last.fm\nв браузере и вернитесь сюда."
    : "Откроется браузер: разрешите доступ и вернитесь сюда.\nПароль от Last.fm программе не нужен.");
  setText("loginErr", login.error);
  show("loginErr", Boolean(login.error));
}

async function doLogin() {
  if (login.waiting) { api("/api/open", { what: "auth" }).catch(showError); return; }
  const key = $("#apikey").value.trim(), secret = $("#secret").value.trim();
  login = { busy: true, waiting: false, error: "", timer: 0 };
  updateLogin();
  try {
    if (key || secret || !st.hasKeys) await api("/api/keys", { apiKey: key, secret });
    await api("/api/login/start", {});
    login.waiting = true;
    updateLogin();
    const started = Date.now();
    login.timer = setInterval(async () => {
      try {
        const r = await api("/api/login/check", {});
        if (r.done) { stopLogin(); view = "now"; await refresh(); await go(); }
      } catch (e) {
        stopLogin(e.message);
      }
      if (Date.now() - started > 10 * 60 * 1000) stopLogin("Время на подтверждение истекло — нажмите кнопку войти ещё раз.");
    }, 2000);
  } catch (e) {
    stopLogin(e.message);
  }
}

function stopLogin(error = "") {
  clearInterval(login.timer);
  login = { busy: false, waiting: false, error, timer: 0 };
  if (mounted === "login") updateLogin();
}

const TOP_PERIODS = [["week", "неделя"], ["month", "месяц"], ["year", "год"], ["all", "всё время"]];
const TOP_KINDS = [["artists", "исполнители", "artist", "исполнитель"], ["tracks", "треки", "track", "трек"],
  ["albums", "альбомы", "album", "альбом"]];
const CARD_KIND = { artists: "artist", tracks: "track", albums: "album" };

const topSel = { period: "week", kind: "artists", offset: 0 };
try {
  const saved = JSON.parse(localStorage.getItem("lfs-top") || "{}");
  if (TOP_PERIODS.some(([p]) => p === saved.period)) topSel.period = saved.period;
  if (TOP_KINDS.some(([k]) => k === saved.kind)) topSel.kind = saved.kind;
} catch (e) { }
let topData = null;
let topLoadedAt = 0;
let topPlays = -1;
let topBusy = false;
let topError = "";

function cardAttrs(kind, it) {
  return `data-act="card" data-kind="${kind}" data-artist="${esc(it.artist)}" data-title="${esc(it.title || "")}" data-album="${esc(it.album || "")}"`;
}

async function loadTop() {
  if (topBusy) return;
  topBusy = true;
  try {
    const q = new URLSearchParams({ kind: topSel.kind, period: topSel.period, offset: String(topSel.offset) });
    topData = await api(`/api/stats/top?${q}`);
    topError = "";
  } catch (e) {
    topError = e.message;
  } finally {
    topBusy = false;
    topLoadedAt = Date.now();
    topPlays = st && st.library ? st.library.plays : -1;
  }
  if (mounted === "top") updateTop();
}

function seg(items, current, act) {
  return `<div class="seg">${items.map(([v, label]) =>
    `<button data-act="${act}" data-v="${v}" aria-pressed="${v === current}">${label}</button>`).join("")}</div>`;
}

function mountTop() {
  main.innerHTML = windowHTML("Топ", `
    <div class="top-bar">${seg(TOP_PERIODS, topSel.period, "topPeriod")}${seg(TOP_KINDS, topSel.kind, "topKind")}</div>
    <div class="top-period">
      <button class="arrow" data-act="topMove" data-d="-1" data-f="topPrev" aria-label="раньше">${pix("left", 2)}</button>
      <div class="top-label"><span class="t24" data-f="topLabel"></span><span class="note" data-f="topLive">идёт</span></div>
      <button class="arrow" data-act="topMove" data-d="1" data-f="topNext" aria-label="позже">${pix("right", 2)}</button>
    </div>
    <div class="table top"><table>
      <colgroup><col class="c-rank"><col><col class="c-meter"><col class="c-count"></colgroup>
      <thead><tr><th class="right">№</th><th data-f="topHead"></th><th></th><th class="right">раз</th></tr></thead>
      <tbody data-f="topRows"></tbody></table></div>
    <div class="buttons after-table"><span class="muted grow" data-f="topTotal"></span></div>
    <p class="muted top-lib" data-f="topLib"></p>`);
}

function libText(l) {
  if (!st.loggedIn) {
    return `История с этого компьютера: ${num(st.all.count)} ${plural(st.all.count, "прослушивание", "прослушивания", "прослушиваний")}. ` +
      "Войдите в Last.fm — появится вся история профиля.";
  }
  if (!l || l.state === "off") return "Войдите в Last.fm — программа загрузит всю историю прослушиваний.";
  if (l.state === "importing") {
    return l.total ? `Загружаю историю с Last.fm: ${num(l.plays)} из ${num(l.total)} прослушиваний…` : "Загружаю историю с Last.fm…";
  }
  if (l.state === "error") {
    const left = l.retryAt ? Math.ceil((new Date(l.retryAt) - Date.now()) / 60000) : 0;
    const when = left > 0 ? `через ${left} ${plural(left, "минуту", "минуты", "минут")}` : "вот-вот";
    return `${l.error}. Повторю ${when}. Загружено ${num(l.plays)} прослушиваний.`;
  }
  return `История с Last.fm: ${num(l.plays)} ${plural(l.plays, "прослушивание", "прослушивания", "прослушиваний")}` +
    (l.syncedAt ? ` · обновлено ${dayClock(l.syncedAt)}` : "");
}

function libShort(l) {
  if (!st.loggedIn) return `на этом компьютере: ${num(st.all.count)} ${plural(st.all.count, "прослушивание", "прослушивания", "прослушиваний")}`;
  if (!l || l.state === "off") return "история Last.fm загрузится после входа";
  if (l.state === "importing") return l.total ? `загружаю историю: ${num(l.plays)} из ${num(l.total)}…` : "загружаю историю…";
  if (l.state === "error") return `история: ${l.error}`;
  const when = l.syncedAt ? ` · обновлено ${clock(l.syncedAt)}` : "";
  return `история: ${num(l.plays)} ${plural(l.plays, "прослушивание", "прослушивания", "прослушиваний")}${when}`;
}

function updateTop() {
  const lib = st.library || {};
  const stale = Date.now() - topLoadedAt > (lib.state === "importing" ? 3000 : 15000);
  if (!topBusy && (stale || (lib.plays !== topPlays && Date.now() - topLoadedAt > 3000))) loadTop();

  for (const b of main.querySelectorAll('[data-act="topPeriod"]')) b.setAttribute("aria-pressed", String(b.dataset.v === topSel.period));
  for (const b of main.querySelectorAll('[data-act="topKind"]')) b.setAttribute("aria-pressed", String(b.dataset.v === topSel.kind));
  const kind = TOP_KINDS.find(([k]) => k === topSel.kind);
  setText("topHead", kind[3]);
  setText("topLib", libText(lib));

  const t = topData && topData.kind === topSel.kind && topData.period.kind === topSel.period && topData.period.offset === topSel.offset ? topData : null;
  setText("topLabel", t ? t.period.label : "");
  setText("topLive", t && t.period.since ? `с ${date(t.period.since)}` : "идёт");
  show("topLive", Boolean(t && (t.period.live || t.period.since)));
  field("topPrev").disabled = !t || !t.period.hasPrev;
  field("topNext").disabled = !t || !t.period.hasNext;
  for (const f of ["topPrev", "topNext"]) field(f).style.visibility = topSel.period === "all" ? "hidden" : "";
  setText("topTotal", t ? `всего за ${PERIOD_WORD[topSel.period]}: ${num(t.total)} ${plural(t.total, "прослушивание", "прослушивания", "прослушиваний")}` : "");

  const sig = JSON.stringify([t, topError]);
  if (sigs.top === sig) return;
  sigs.top = sig;
  const rows = field("topRows");
  if (!t) {
    rows.innerHTML = `<tr class="empty"><td colspan="4">${esc(topError || "Считаю…")}</td></tr>`;
    return;
  }
  if (!t.items.length) {
    const msg = lib.state === "importing" ? "История ещё загружается — топ появится чуть позже."
      : lib.state === "error" && !lib.plays ? "История с Last.fm ещё не загружена — причина ниже."
      : "За этот период прослушиваний нет.";
    rows.innerHTML = `<tr class="empty"><td colspan="4">${msg}</td></tr>`;
    return;
  }
  const max = t.items[0].count;
  const ck = CARD_KIND[t.kind];
  rows.innerHTML = t.items.map((it) => {
    const name = t.kind === "artists" ? esc(it.artist)
      : t.kind === "tracks" ? `${esc(it.artist)} — ${esc(it.title)}`
      : `${esc(it.album)} <span class="muted">· ${esc(it.artist)}</span>`;
    return `<tr class="click" ${cardAttrs(ck, it)}><td class="right">${it.rank}</td><td>${name}</td>` +
      `<td><div class="meter"><div data-w="${it.count / max}"></div></div></td><td class="right">${num(it.count)}</td></tr>`;
  }).join("");
  for (const el of rows.querySelectorAll("[data-w]")) el.style.width = `${Math.max(4, Math.round(Number(el.dataset.w) * 100))}%`;
}

const PERIOD_WORD = { week: "неделю", month: "месяц", year: "год", all: "всё время" };

function saveTopSel() {
  try { localStorage.setItem("lfs-top", JSON.stringify({ period: topSel.period, kind: topSel.kind })); } catch (e) { }
}

let cardStack = [];

async function openCard(q, push = true) {
  const c = await api(`/api/stats/card?${new URLSearchParams(q)}`);
  if (push) cardStack.push(q);
  renderCard(c);
}

const CARD_TITLE = { track: "Трек", album: "Альбом", artist: "Исполнитель" };
const CARD_ICON = { track: "note", album: "disc", artist: "person" };

function times(n) { return `${num(n)} ${plural(n, "раз", "раза", "раз")}`; }

function cardList(title, kind, items) {
  if (!items || !items.length) return "";
  return `<div class="card-list"><div class="muted card-list-title">${title}</div><div class="table mini"><table>
    <colgroup><col><col class="c-count"></colgroup><tbody>${items.map((it) =>
      `<tr class="click" ${cardAttrs(kind, it)}><td>${esc(kind === "album" ? it.album : it.title)}</td>` +
      `<td class="right">${num(it.count)}</td></tr>`).join("")}</tbody></table></div></div>`;
}

function renderCard(c) {
  const name = c.kind === "track" ? c.title : c.kind === "album" ? c.album : c.artist;
  const links = [];
  if (c.kind !== "artist") {
    links.push(`<button class="link" ${cardAttrs("artist", c)}>${esc(c.artist)}</button>` +
      (c.artistCount ? `<span class="muted"> — ${times(c.artistCount)}</span>` : ""));
  }
  if (c.kind === "track" && c.album) {
    links.push(`<button class="link" ${cardAttrs("album", c)}>${esc(c.album)}</button>` +
      (c.albumCount ? `<span class="muted"> — ${times(c.albumCount)}</span>` : ""));
  }
  const body = c.count ? `
      <div class="card-count"><span class="t32">${num(c.count)}</span>
        <span>${plural(c.count, "прослушивание", "прослушивания", "прослушиваний")} за всё время</span></div>
      <div class="card-periods">
        <div><span class="muted">эта неделя</span><span class="t24">${num(c.week)}</span></div>
        <div><span class="muted">этот месяц</span><span class="t24">${num(c.month)}</span></div>
        <div><span class="muted">этот год</span><span class="t24">${num(c.year)}</span></div></div>
      <p class="muted card-dates">впервые: ${date(c.first)} · последний раз: ${dayClock(c.last)}</p>
      ${c.kind === "artist" ? cardList("любимые треки", "track", c.tracks) + cardList("альбомы", "album", c.albums) : ""}
      ${c.kind === "album" ? cardList("треки альбома", "track", c.tracks) : ""}`
    : `<p class="muted card-none">${c.kind === "track" ? "Этот трек ещё ни разу не засчитан." : "Прослушиваний пока нет."}</p>`;
  modal.innerHTML = windowHTML(CARD_TITLE[c.kind], `
    <div class="card-head"><div class="card-art" data-i="${esc(initial(name))}">${pix(CARD_ICON[c.kind], 5)}</div>
      <div class="card-names"><div class="t24 card-name">${esc(name)}</div>
        ${links.map((l) => `<div class="card-link">${l}</div>`).join("")}</div></div>
    ${body}
    <div class="buttons card-buttons">${cardStack.length > 1 ? '<button data-act="cardBack">назад</button>' : ""}
      <button class="default" data-act="modal" data-value="ok">ok</button></div>`, "card");
  modal.hidden = false;
  modal.querySelector("button.default").focus();
}

function renderPhone(p) {
  const qr = p.enabled && p.qr && p.qr.length
    ? `<div class="qr">${pixRows(p.qr, 4)}</div>`
    : `<div class="qr qr-off">${pix("phone", 8)}</div>`;
  let text;
  if (p.error) text = `<p class="phone-err">${esc(p.error)}</p>`;
  else if (p.enabled) text = `<p>Наведите камеру телефона на код — откроется страница: что играет сейчас, история прослушиваний и топ треков.</p>
      <div class="phone-url url">${esc(p.url)}</div>
      ${p.other && p.other.length ? `<p class="muted">если не открывается: ${p.other.map(esc).join(", ")}</p>` : ""}`;
  else text = `<p>Включите — и здесь появится QR-код. Телефон откроет по нему страницу: что играет сейчас, история прослушиваний и топ треков.</p>`;
  modal.innerHTML = windowHTML("Телефон", `<div class="phone-box">${qr}
      <div class="phone-text">
        <label class="check"><input type="checkbox" data-act="phoneToggle"${p.enabled ? " checked" : ""}> открывать на телефоне</label>
        ${text}
        <p class="muted">Телефон должен быть в той же сети Wi-Fi. Если Windows спросит про доступ к сети, разрешите его для частных сетей.</p>
      </div></div>
    <div class="buttons">${p.enabled ? '<button data-act="phoneReset">новая ссылка</button>' : ""}
      <button class="default" data-act="modal" data-value="ok">закрыть</button></div>`, "phone-win");
  modal.hidden = false;
  modal.querySelector("button.default").focus();
}

let stations = [];
let radioSel = null;

async function loadStations() {
  stations = await api("/api/radio/stations");
  if (mounted === "radio") updateRadio();
}

async function playStation(id) {
  await api("/api/radio/play", { id });
  radioSel = id;
  syncAudio({ ...st.radio, playing: true, station: id });
  await refresh();
}

function mountRadio() {
  main.innerHTML =
    windowHTML("Радио", `<div class="now">
      <div class="art"><div>${pix("radio", 16)}</div></div>
      <div class="now-info">
        <div class="chip" data-f="rChip">${pix("radio", 2)}<span data-f="rChipText"></span></div>
        <div class="t32 ellipsis track-title" data-f="rSong"></div>
        <div class="t24 ellipsis track-artist" data-f="rArtist"></div>
        <div class="muted ellipsis track-album" data-f="rInfo"></div>
        <p class="note radio-note" data-f="rNote"></p>
        <div class="radio-controls">
          <button class="default w150" data-act="radioPlay" data-f="rPlay">${pix("play", 2)}<span>слушать</span></button>
          <button class="w120" data-act="radioStop" data-f="rStop">${pix("stop", 2)}<span>стоп</span></button>
          <label class="volume">${pix("speaker", 2)}<input type="range" min="0" max="100" step="1" data-f="rVolume" aria-label="громкость"></label>
        </div>
      </div></div>`) +
    windowHTML("Мои станции", `<div class="table stations"><table>
        <colgroup><col class="c-name"><col></colgroup>
        <thead><tr><th>название</th><th>адрес потока</th></tr></thead>
        <tbody data-f="stations"></tbody></table></div>
      <div class="buttons after-table radio-buttons">
        <button class="w150" data-act="stationAdd">добавить</button>
        <button class="w150" data-act="stationEdit" data-f="stEdit">изменить</button>
        <button class="w150" data-act="stationDelete" data-f="stDel">удалить</button>
        <span class="muted grow radio-hint">двойной щелчок — слушать</span>
      </div>`);
  field("rVolume").value = st.radio.volume;
}

function liveNote(n) {
  if (n.scrobbled) return "засчитано";
  if (n.fromStart) return "засчитается, когда песня закончится";
  if (n.target > 0) return `песня началась до включения — засчитается через ${fmtLeft(leftNow(n))}`;
  return "песня началась до включения — её не засчитать";
}

function radioNote(r) {
  if (r.error) return r.error;
  if (!r.playing) return "";
  if (audioBlocked) return "браузер не дал включить звук — нажмите кнопку слушать";
  if (!r.connected) return "подключаюсь к станции…";
  if (!r.artist) return "станция не сообщает исполнителя — засчитывать нечего";
  if (st.paused) return "скробблинг выключен";
  const n = st.now;
  if (n && n.live && n.player === "Радио") return liveNote(n);
  return "";
}

function updateRadio() {
  const r = st.radio;
  const on = r.playing;
  setText("rChipText", !on ? "выключено" : r.connected ? "в эфире" : "подключение…");
  setText("rSong", on ? (r.song || r.name) : "Радио выключено");
  setText("rArtist", on ? r.artist : "");
  setText("rInfo", on ? [r.song && r.song !== r.name ? r.name : "", r.format].filter(Boolean).join(" · ")
    : r.count ? "Выберите станцию в списке и нажмите кнопку слушать." : "Добавьте станцию — понадобится адрес её потока.");
  setText("rNote", radioNote(r));
  field("rPlay").disabled = !stations.length;
  field("rStop").disabled = !on;
  const vol = field("rVolume");
  if (Date.now() > volumeHeld && document.activeElement !== vol && Number(vol.value) !== r.volume) vol.value = r.volume;

  if (radioSel !== null && !stations.some((s) => s.id === radioSel)) radioSel = null;
  const sig = JSON.stringify([stations, radioSel, on && r.station]);
  if (sigs.stations !== sig) {
    sigs.stations = sig;
    field("stations").innerHTML = stations.length ? stations.map((s) =>
      `<tr data-act="stationSel" data-id="${esc(s.id)}"${s.id === radioSel ? ' class="sel"' : ""}>` +
      `<td>${on && s.id === r.station ? pix("note", 2) : ""}${esc(s.name)}</td>` +
      `<td class="muted-cell url">${esc(s.url)}</td></tr>`).join("")
      : `<tr class="empty"><td colspan="2">Станций пока нет — нажмите кнопку добавить.</td></tr>`;
  }
  const has = radioSel !== null;
  field("stEdit").disabled = !has;
  field("stDel").disabled = !has;
}

function stationForm(s) {
  modal.innerHTML = windowHTML(s ? "Изменить станцию" : "Новая станция", `<div class="keys station-form">
      <label for="stName">название</label>
      <input type="text" id="stName" maxlength="80" autocomplete="off" spellcheck="false" placeholder="как подписать станцию" value="${esc(s ? s.name : "")}">
      <label for="stURL">адрес потока</label>
      <input type="text" id="stURL" maxlength="2048" autocomplete="off" spellcheck="false" placeholder="https://…" value="${esc(s ? s.url : "")}">
    </div>
    <p class="muted form-hint">Прямой адрес потока или ссылка на плейлист .m3u / .pls,
например https://ice2.somafm.com/groovesalad-128-mp3</p>
    <p class="note form-error" hidden></p>
    <div class="buttons"><button class="default" data-act="stationSave" data-id="${s ? esc(s.id) : ""}">ok</button>
      <button data-act="modal" data-value="cancel">отмена</button></div>`, "form");
  modal.hidden = false;
  $(s ? "#stURL" : "#stName").focus();
}

const audio = new Audio();
audio.preload = "none";
let audioWant = "";
let audioGen = 0;
let audioRetry = 0;
let audioBlocked = false;
let volumeHeld = 0;

function syncAudio(r) {
  const want = r && r.playing ? r.station : "";
  if (want !== audioWant) {
    audioWant = want;
    clearTimeout(audioRetry);
    if (want) startAudio(); else stopAudio();
  } else if (want && audioBlocked && audio.paused) {
    audio.play().catch(() => {});
  }
  if (r && Date.now() > volumeHeld) setVolume(r.volume);
}

function setVolume(v) {
  const x = Math.max(0, Math.min(100, Number(v) || 0)) / 100;
  if (Math.abs(audio.volume - x) > 0.001) audio.volume = x;
}

function startAudio() {
  audioGen++;
  audio.src = `/radio/stream?t=${encodeURIComponent(token || "")}&s=${encodeURIComponent(audioWant)}&n=${audioGen}`;
  audio.play().then(() => { audioBlocked = false; }).catch((e) => {
    if (e && e.name === "NotAllowedError") audioBlocked = true;
  });
}

function stopAudio() {
  audioBlocked = false;
  audio.pause();
  audio.removeAttribute("src");
  audio.load();
  tell("audio:off");
}

function retryAudio() {
  if (!audioWant) return;
  clearTimeout(audioRetry);
  audioRetry = setTimeout(() => { if (audioWant) startAudio(); }, 3000);
}

audio.addEventListener("playing", () => { audioBlocked = false; tell("audio:on"); });
audio.addEventListener("error", retryAudio);
audio.addEventListener("ended", retryAudio);

let modalResolve = null;

function ask({ title, heading, text, buttons }) {
  modal.innerHTML = windowHTML(title, `<div class="message">${pix("warn", 4)}<div class="message-body">
      <h2>${esc(heading)}</h2><p class="muted">${esc(text)}</p></div></div>
    <div class="buttons">${buttons.map((b, i) =>
      `<button class="${i === 0 ? "default" : ""}" data-act="modal" data-value="${b.value}">${esc(b.label)}</button>`).join("")}</div>`);
  modal.hidden = false;
  modal.querySelector("button").focus();
  return new Promise((resolve) => { modalResolve = resolve; });
}

function closeModal(value) {
  cardStack = [];
  modal.hidden = true;
  modal.innerHTML = "";
  if (modalResolve) modalResolve(value);
  modalResolve = null;
}

function showError(err) {
  return ask({ title: "Ошибка", heading: "Не получилось !", text: err.message || String(err),
    buttons: [{ label: "ok", value: "ok" }] });
}

const actions = {
  async nav(el) { view = el.dataset.view; await go(); },
  async refresh() { await api("/api/library/sync", {}); await refresh(); },
  async open(el) { await api("/api/open", { what: el.dataset.what }); },
  async openLastfm(el) { await api("/api/open", { what: "lastfm", artist: el.dataset.artist, track: el.dataset.track || "" }); },
  async markMoment() {
    const n = st.now;
    if (!canMark(n)) return;
    const key = trackKey(n);
    const pos = posNow(n);
    if (!marking || marking.key !== key) {
      marking = { key, start: pos };
      paintMoment();
      return;
    }
    const from = Math.min(marking.start, pos);
    let to = Math.max(marking.start, pos);
    marking = null;
    if (to - from < 3) to = Math.min(n.length, from + 15);
    await saveMoment(n, from, to);
  },
  async clearMoment() {
    const n = st.now;
    if (!n) return;
    marking = null;
    await api("/api/moment", { artist: n.artist, title: n.title, clear: true });
    await refresh();
  },
  async closePanel(el) {
    const p = el.dataset.panel;
    const c = closedSet();
    c.add(p);
    await setClosed([...c], `[data-act="openPanel"][data-panel="${p}"]`);
  },
  async openPanel(el) {
    const p = el.dataset.panel;
    const c = closedSet();
    c.delete(p);
    await setClosed([...c], `section[data-panel="${p}"] .tb-close`);
  },
  async flush() { await api("/api/queue/flush", {}); setTimeout(refresh, 700); },
  async select(el) { const id = Number(el.dataset.id); selected = selected === id ? null : id; updateQueue(); },
  dismiss() { const m = queueMessage(); dismissed = m ? m.key : ""; updateQueue(); },
  async msgAction(el) { if (el.dataset.do === "relogin") await actions.relogin(); else await actions.flush(); },
  async relogin() { await api("/api/logout", {}); await refresh(); },
  async delete() {
    const q = queue.find((x) => x.id === selected);
    if (!q) return;
    const ans = await ask({ title: "Подтверждение", heading: "Удалить запись ?",
      text: `${q.artist} — ${q.title}\nЗапись не уйдёт в Last.fm и пропадёт из истории.`,
      buttons: [{ label: "да", value: "yes" }, { label: "нет", value: "no" }] });
    if (ans !== "yes") return;
    await api("/api/queue/delete", { ids: [q.id] });
    selected = null;
    await refresh();
  },
  async clear() {
    const n = queue.length;
    const ans = await ask({ title: "Подтверждение", heading: "Очистить очередь ?",
      text: `${n} ${plural(n, "прослушивание", "прослушивания", "прослушиваний")} не уйдут в Last.fm и пропадут из истории.`,
      buttons: [{ label: "да", value: "yes" }, { label: "нет", value: "no" }] });
    if (ans !== "yes") return;
    await api("/api/queue/clear", {});
    await refresh();
  },
  async logout() {
    const ans = await ask({ title: "Подтверждение", heading: "Выйти из аккаунта ?",
      text: "Новые прослушивания будут сохраняться только на этом компьютере. Сохранённые за последние 14 дней уйдут в Last.fm после нового входа.",
      buttons: [{ label: "да", value: "yes" }, { label: "нет", value: "no" }] });
    if (ans !== "yes") return;
    await api("/api/logout", {});
    await refresh();
  },
  async settingsOk() { await saveSettings(); view = "now"; await go(); },
  async phone() { renderPhone(await api("/api/phone")); },
  async phoneToggle(el) { renderPhone(await api("/api/phone", { enabled: el.checked })); },
  async phoneReset() {
    const ans = await ask({ title: "Подтверждение", heading: "Сделать новую ссылку ?",
      text: "Старая ссылка и старый QR-код перестанут открываться — телефон придётся отсканировать заново.",
      buttons: [{ label: "да", value: "yes" }, { label: "нет", value: "no" }] });
    if (ans !== "yes") { renderPhone(await api("/api/phone")); return; }
    renderPhone(await api("/api/phone/reset", {}));
  },
  topPeriod(el) { topSel.period = el.dataset.v; topSel.offset = 0; saveTopSel(); loadTop(); updateTop(); },
  topKind(el) { topSel.kind = el.dataset.v; saveTopSel(); loadTop(); updateTop(); },
  topMove(el) { topSel.offset = Math.min(0, topSel.offset + Number(el.dataset.d)); loadTop(); updateTop(); },
  async card(el) {
    await openCard({ kind: el.dataset.kind, artist: el.dataset.artist, title: el.dataset.title, album: el.dataset.album });
  },
  async nowCard(el) {
    const n = st.now;
    if (!n || (el.dataset.kind === "album" && !n.album)) return;
    cardStack = [];
    await openCard({ kind: el.dataset.kind, artist: n.artist, title: n.title, album: n.album });
  },
  async cardBack() {
    cardStack.pop();
    const q = cardStack[cardStack.length - 1];
    if (q) await openCard(q, false);
  },
  async libReload() {
    const ans = await ask({ title: "Подтверждение", heading: "Загрузить историю заново ?",
      text: "История прослушиваний будет стёрта и заново загружена с Last.fm. Это может занять несколько минут.",
      buttons: [{ label: "да", value: "yes" }, { label: "нет", value: "no" }] });
    if (ans !== "yes") return;
    await api("/api/library/reload", {});
    topData = null;
    await refresh();
  },
  async settingsApply() { await saveSettings(); mountSettings(); },
  async login() { await doLogin(); },
  modal(el) { closeModal(el.dataset.value); },

  async radioPlay() {
    const id = stations.some((s) => s.id === radioSel) ? radioSel : (st.radio.last || (stations[0] && stations[0].id));
    if (!id) return;
    await playStation(id);
  },
  async radioStop() {
    await api("/api/radio/stop", {});
    syncAudio({ ...st.radio, playing: false });
    await refresh();
  },
  stationSel(el) { radioSel = el.dataset.id; updateRadio(); },
  stationAdd() { stationForm(null); },
  stationEdit() {
    const s = stations.find((x) => x.id === radioSel);
    if (s) stationForm(s);
  },
  async stationDelete() {
    const s = stations.find((x) => x.id === radioSel);
    if (!s) return;
    const ans = await ask({ title: "Подтверждение", heading: "Удалить станцию ?", text: s.name,
      buttons: [{ label: "да", value: "yes" }, { label: "нет", value: "no" }] });
    if (ans !== "yes") return;
    await api("/api/radio/stations/delete", { id: s.id });
    radioSel = null;
    await loadStations();
  },
  async stationSave(el) {
    const body = { id: el.dataset.id, name: $("#stName").value, url: $("#stURL").value };
    const err = modal.querySelector(".form-error");
    try {
      if (body.id) await api("/api/radio/stations/edit", body);
      else radioSel = (await api("/api/radio/stations", body)).id;
    } catch (e) {
      err.textContent = e.message;
      err.hidden = false;
      return;
    }
    closeModal("ok");
    await loadStations();
  },
};

document.addEventListener("click", async (ev) => {
  const el = ev.target.closest("[data-act]");
  if (!el || el.disabled || el.dataset.act === "pause") return;
  const fn = actions[el.dataset.act];
  if (!fn) return;
  try { await fn(el); } catch (e) { if (el.dataset.act !== "modal") showError(e); }
});

document.addEventListener("change", (ev) => {
  if (ev.target.id === "theme") {
    themePreview = ev.target.value;
    applyTheme(themePreview);
    $("#bursts").closest("label").hidden = themePreview !== "glitch";
  }
  if (ev.target.id === "bursts") {
    burstsPreview = ev.target.checked;
    applyBursts(burstsPreview);
  }
});

document.addEventListener("change", async (ev) => {
  if (ev.target.dataset.act !== "pause") return;
  try { await api("/api/pause", { paused: !ev.target.checked }); await refresh(); } catch (e) { showError(e); }
});

document.addEventListener("keydown", (ev) => {
  if (!modal.hidden && ev.key === "Escape") closeModal("no");
  if (!modal.hidden && ev.key === "Enter" && ev.target.matches("#stName, #stURL")) {
    modal.querySelector('[data-act="stationSave"]').click();
  }
});

document.addEventListener("dblclick", (ev) => {
  const row = ev.target.closest('tr[data-act="stationSel"]');
  if (row) playStation(row.dataset.id).catch(showError);
});

document.addEventListener("input", (ev) => {
  if (ev.target.dataset.f !== "rVolume") return;
  volumeHeld = Date.now() + 3000;
  setVolume(Number(ev.target.value));
});
document.addEventListener("change", async (ev) => {
  if (ev.target.dataset.f !== "rVolume") return;
  volumeHeld = Date.now() + 3000;
  try { await api("/api/radio/volume", { volume: Number(ev.target.value) }); } catch (e) { showError(e); }
});

const MOUNT = { now: mountNow, top: mountTop, radio: mountRadio, queue: mountQueue, settings: mountSettings, login: mountLogin };
const UPDATE = { now: updateNow, top: updateTop, radio: updateRadio, queue: updateQueue, settings: updateSettings, login: updateLogin };

async function go() {
  if (view === "settings") settings = await api("/api/settings");
  if (view === "radio") stations = await api("/api/radio/stations");
  if (view === "queue") queue = await api("/api/queue");
  if (view === "top") await loadTop();
  render(true);
}

function render(force = false) {
  let want = view;
  if (want === "login" && st.loggedIn) want = "now";
  if (want === "queue" && !st.loggedIn && !st.queueLen) want = "now";
  if (want === "settings" && !settings) want = "now";
  if (want !== "login" && login.timer) stopLogin();
  if (want !== "settings") {
    themePreview = "";
    burstsPreview = null;
  }
  applyTheme(themePreview || st.theme);
  applyBursts(burstsPreview ?? st.bursts);
  if (force || want !== mounted || (want === "settings" && mountedLogin !== st.loggedIn)) {
    mounted = want;
    mountedLogin = st.loggedIn;
    for (const k in sigs) delete sigs[k];
    MOUNT[want]();
  }
  app.classList.remove("solo");
  nav.hidden = false;
  updateNav();
  updateStatus();
  UPDATE[want]();
  updateMini();
  frameReady();
}

let polling = false;

async function refresh() {
  if (polling) return;
  polling = true;
  try {
    st = await api(isMini() ? "/api/state?cover=1" : "/api/state");
    stAt = performance.now();
    syncAudio(st.radio);
    if (mounted === "queue" || (st.loggedIn && view === "queue")) queue = await api("/api/queue");
    render();
    loadLyrics();
    loadRecs();
  } catch (e) {
    if (e.status === 401) {
      noAccess();
      return;
    }
  } finally {
    polling = false;
  }
}

function noAccess() {
  clearInterval(timer);
  app.classList.add("solo");
  nav.hidden = true;
  main.innerHTML = windowHTML(APP_TITLE, `<div class="message">${pix("warn", 4)}<div class="message-body">
    <h2>Нет доступа !</h2><p class="muted">Откройте окно через значок программы в трее или ярлык «Следилка за музыкой».</p></div></div>`);
  frameReady();
}

mountFrame();
mountNav();
const timer = setInterval(refresh, 1000);
if (!token) noAccess(); else refresh().then(() => { if (st && view !== "now") go().catch(showError); });
