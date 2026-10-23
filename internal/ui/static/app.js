"use strict";

const BITMAPS = {
  note: ["...######", "...######", "...#....#", "...#....#", "...#....#",
         ".###..###", "####.####", "####.####", ".##...##."],
  list: ["##.#######", "..........", "..........", "##.#######", "..........",
         "..........", "##.#######"],
  gear: ["...###...", ".#.###.#.", "..#####..", "####.####", "###...###",
         "####.####", "..#####..", ".#.###.#.", "...###..."],
  close: ["##...##", ".##.##.", "..###..", "..###..", ".##.##.", "##...##"],
  min: ["......", "......", "......", "......", "......", "######", "######"],
  max: ["#######", "#######", "#.....#", "#.....#", "#.....#", "#######"],
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
};

function pix(name, cell) {
  const rows = BITMAPS[name];
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
    `<defs><linearGradient id="ai" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#C35BB2"/>` +
    `<stop offset="1" stop-color="#E59BD8"/></linearGradient></defs><rect width="16" height="16" fill="url(#ai)"/>` +
    `<rect x="4" y="6.5" width="2" height="6" fill="#FFF4FC"/><rect x="7" y="3.5" width="2" height="9" fill="#FFF4FC"/>` +
    `<rect x="10" y="5.5" width="2" height="7" fill="#FFF4FC"/></svg>`;
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

function mountFrame() {
  if (!host) return;
  document.documentElement.classList.add("framed");
  const bar = document.createElement("header");
  bar.className = "frame-title";
  bar.innerHTML = `<span>Радио Дача 3Д 5.0</span>` +
    `<button class="tb-btn" data-win="min" aria-label="свернуть">${pix("min", 1)}</button>` +
    `<button class="tb-btn" data-win="max" aria-label="развернуть">${pix("max", 1)}</button>` +
    `<button class="tb-btn" data-win="close" aria-label="закрыть">${pix("close", 1)}</button>`;
  document.body.prepend(bar);
  for (const edge of ["n", "s", "e", "w", "nw", "ne", "sw", "se"]) {
    const el = document.createElement("div");
    el.className = `rz rz-${edge}`;
    el.dataset.rz = edge;
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
  bar.addEventListener("dblclick", (e) => { if (!e.target.closest("button")) tell("max"); });
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

window.lfsFrame = (max, active) => {
  const root = document.documentElement;
  root.classList.toggle("max", max);
  root.classList.toggle("inactive", !active);
  const b = document.querySelector('[data-win="max"]');
  if (b && b.dataset.state !== String(max)) {
    b.dataset.state = String(max);
    b.innerHTML = pix(max ? "restore" : "max", 1);
    b.setAttribute("aria-label", max ? "восстановить" : "развернуть");
  }
};

let readySent = false;
function frameReady() {
  if (!host || readySent) return;
  readySent = true;
  const fonts = document.fonts ? Promise.all([
    document.fonts.load("16px Tiny5", "Aя"),
    document.fonts.load('16px "Press Start 2P"', "Aя"),
  ]) : Promise.resolve();
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

function fmtDur(sec) {
  sec = Math.max(0, Math.round(sec));
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
let dismissed = "";
let login = { busy: false, waiting: false, error: "", timer: 0 };
const sigs = {};

function field(name) { return main.querySelector(`[data-f="${name}"]`); }
function setText(name, text) { const el = field(name); if (el && el.textContent !== text) el.textContent = text; }
function show(name, on) { const el = field(name); if (el) el.hidden = !on; }

function windowHTML(title, body, extra = "") {
  return `<section class="window ${extra}"><div class="titlebar"><span>${esc(title)}</span>` +
    `<span class="tb-close" aria-hidden="true"></span></div><div class="window-body">${body}</div></section>`;
}

function mountNav() {
  nav.innerHTML = [["now", "note", "Сейчас"], ["radio", "radio", "Радио"], ["queue", "list", "Очередь"],
    ["settings", "gear", "Настройки"]]
    .map(([v, icon, label]) => `<button data-act="nav" data-view="${v}">${pix(icon, 2)}<span>${label}</span>` +
      (v === "queue" ? `<span class="badge" data-badge hidden></span>` : "") + `</button>`).join("");
}

function updateNav() {
  for (const b of nav.querySelectorAll("button")) {
    if (b.dataset.view === mounted) b.setAttribute("aria-current", "page");
    else b.removeAttribute("aria-current");
  }
  const badge = nav.querySelector("[data-badge]");
  if (badge) {
    badge.hidden = !st.queueLen;
    badge.textContent = st.queueLen > 99 ? "99+" : String(st.queueLen);
  }
}

const LINK = { ok: "в сети", offline: "нет связи", auth: "нужен вход", nologin: "вход не выполнен" };

function updateStatus() {
  const who = st.loggedIn ? `${st.user} · Last.fm: ${LINK[st.link.state] || "в сети"}` : "вход не выполнен";
  const n = st.today.count;
  const cells = [who, `в очереди: ${st.queueLen}`,
    `сегодня: ${n} ${plural(n, "трек", "трека", "треков")} · ${fmtSpan(st.today.seconds)}`];
  const html = cells.map((c) => `<div>${esc(c)}</div>`).join("");
  if (statusbar.innerHTML !== html) statusbar.innerHTML = html;
}

function mountNow() {
  main.innerHTML =
    windowHTML("Сейчас играет", `<div class="now">
      <div class="art"><div>${pix("note", 20)}</div></div>
      <div class="now-info">
        <div class="chip" data-f="chip">${pix("note", 2)}<span data-f="chipText"></span></div>
        <div class="t32 ellipsis track-title" data-f="title"></div>
        <div class="ellipsis track-feat" data-f="feat"></div>
        <div class="t24 ellipsis track-artist" data-f="artist"></div>
        <div class="muted ellipsis track-album" data-f="album"></div>
        <div class="progress" data-f="progress"><div data-f="bar"></div></div>
        <div class="row now-row" data-f="times"><span class="muted" data-f="played"></span><span class="note" data-f="left"></span></div>
        <p class="muted idle-text" data-f="idle">Включите музыку в Spotify, AIMP, браузере<br>или радио в программе — трек появится здесь.</p>
        <label class="check now-check"><input type="checkbox" data-act="pause" data-f="enabled"> скробблинг включён</label>
      </div></div>`) +
    windowHTML("Недавние", `<div class="table recent"><table>
        <colgroup><col class="c-time"><col><col class="c-status"></colgroup>
        <thead><tr><th>время</th><th>трек</th><th>статус</th></tr></thead>
        <tbody data-f="recent"></tbody></table></div>
      <div class="buttons after-table"><span class="muted grow" data-f="total"></span>
        <button class="w120" data-act="refresh">обновить</button>
        <button class="w210" data-act="open" data-what="profile">${pix("ext", 2)}<span>профиль на last.fm</span></button>
      </div>`);
}

const STATUS_WORD = { playing: "играет", paused: "пауза", stopped: "остановлен" };
const SCROBBLE_WORD = { sent: "отправлен", queued: "в очереди", ignored: "отклонён" };

function updateNow() {
  const n = st.now;
  const playing = Boolean(n);
  show("chip", playing);
  show("progress", playing && !n.live);
  show("times", playing);
  show("idle", !playing);
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
    setText("played", n.length > 0 ? `прослушано ${fmtDur(n.played)} из ${fmtDur(n.length)}` : `прослушано ${fmtDur(n.played)}`);

    let left;
    if (st.paused) left = "скробблинг выключен";
    else if (n.live) left = liveNote(n);
    else if (!n.scrobblable) left = "слишком короткий трек";
    else if (n.scrobbled) left = "засчитано";
    else left = `засчитается через ${fmtDur(n.target - n.played)}`;
    setText("left", left);

    const box = field("progress");
    const frac = n.length > 0 ? n.played / n.length : (n.target > 0 ? n.played / n.target : 0);
    const inner = Math.max(0, box.clientWidth - 6);
    field("bar").style.width = `${Math.floor(inner * Math.min(1, frac) / 10) * 10}px`;
  }
  const cb = field("enabled");
  if (cb.checked === st.paused) cb.checked = !st.paused;

  const recent = st.recent.slice(0, 4);
  const sig = JSON.stringify(recent);
  if (sigs.recent !== sig) {
    sigs.recent = sig;
    field("recent").innerHTML = recent.length ? recent.map((r) =>
      `<tr><td>${clock(r.started)}</td><td>${esc(r.artist)} — ${esc(r.title)}</td>` +
      `<td class="right" title="${esc(r.reason || "")}">${SCROBBLE_WORD[r.status] || ""}</td></tr>`).join("")
      : `<tr class="empty"><td colspan="3">Здесь появятся засчитанные треки.</td></tr>`;
  }
  const c = st.all.count;
  setText("total", `всего в истории: ${num(c)} ${plural(c, "прослушивание", "прослушивания", "прослушиваний")}`);
}

function mountQueue() {
  main.innerHTML =
    windowHTML("Системное сообщение", `<div class="message">${pix("warn", 4)}<div class="message-body">
        <h2 data-f="msgTitle"></h2><p class="muted" data-f="msgText"></p></div></div>
      <div class="buttons"><button class="w180" data-act="msgAction" data-f="msgBtn"></button>
        <button class="default" data-act="dismiss">ok</button></div>`, "msg").replace("<section", '<section data-f="msg"') +
    `<section class="window"><div class="titlebar"><span data-f="qTitle"></span><span class="tb-close" aria-hidden="true"></span></div>
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
  const maxMin = Math.round(s.maxPlay / 60);
  const players = s.players.length ? `<div class="players">${s.players.map((p) =>
    `<label class="check"><input type="checkbox" data-player="${esc(p.name)}"${p.enabled ? " checked" : ""}>` +
    `<span>${esc(p.identity)}</span>${p.playing ? '<span class="note">играет</span>' : ""}</label>`).join("")}</div>`
    : `<p class="muted no-players">Плееры появятся здесь, как только в них что-нибудь заиграет.</p>`;

  main.innerHTML = windowHTML("Настройки", `
    <fieldset><legend>Аккаунт</legend><div class="account">
      <div class="avatar">${pix("heart", 2)}</div>
      <div class="who"><span class="t24">${esc(st.user)}</span>
        <span class="muted">${st.since ? `вход выполнен ${date(st.since)}` : ""}</span></div>
      <button class="w110" data-act="logout">выйти</button></div></fieldset>
    <fieldset><legend>Правила засчитывания</legend>
      <div class="rule"><span>засчитывать трек после:</span>
        <select id="percent">${opts.map(([p, label]) => `<option value="${p}"${Math.abs(p - s.percent) < 0.001 ? " selected" : ""}>${label}</option>`).join("")}</select>
        <span class="muted">или ${maxMin} ${maxMin === 1 ? "минуты" : "минут"}</span></div>
      <div class="muted rule-text">не учитывать треки короче ${Math.round(s.minLength)} секунд — так требует Last.fm</div>
      <label class="check rule-check"><input type="checkbox" id="radio"${s.radio ? " checked" : ""}> интернет-радио: засчитывать через ${maxMin} ${maxMin === 1 ? "минуту" : plural(maxMin, "минуту", "минуты", "минут")}</label>
    </fieldset>
    <fieldset><legend>Плееры, которые учитывать</legend>${players}</fieldset>
    <fieldset><legend>Система</legend>
      <label class="check autostart"${s.canAutostart ? "" : " hidden"}><input type="checkbox" id="autostart"${s.autostart ? " checked" : ""}> запускать при входе в Windows</label>
      <div class="logs"><span class="muted">журнал: ${esc(s.logDir)}</span>
        <button class="w180" data-act="open" data-what="logs">${pix("folder", 2)}<span>открыть папку</span></button></div>
    </fieldset>
    <div class="buttons settings-buttons">
      <button class="default" data-act="settingsOk">ok</button>
      <button data-act="nav" data-view="now">отмена</button>
      <button data-act="settingsApply">применить</button></div>`);
}

function updateSettings() {
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
  out.players = settings.players.map((p) => ({
    ...p, enabled: main.querySelector(`[data-player="${CSS.escape(p.name)}"]`).checked,
  }));
  return out;
}

async function saveSettings() {
  await api("/api/settings", collectSettings());
  settings = await api("/api/settings");
}

function mountLogin() {
  const ph = st.hasKeys ? "сохранён — можно не вводить" : "32 символа";
  main.innerHTML = windowHTML("Вход в Last.fm", `
    <div class="welcome">${appIcon(56)}<div>
      <h1>Добро пожаловать ${pix("heartBig", 2)}</h1>
      <p class="muted">Подключите приложение к аккаунту Last.fm,</p>
      <p class="muted">и прослушивания начнут отправляться сами.</p></div></div>
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
    </fieldset>`, "login");
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
        if (r.done) { stopLogin(); await refresh(); }
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
  if (n.target > 0) return `песня началась до включения — засчитается через ${fmtDur(n.target - n.played)}`;
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
      `<td class="muted-cell">${esc(s.url)}</td></tr>`).join("")
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
  async refresh() { await refresh(); },
  async open(el) { await api("/api/open", { what: el.dataset.what }); },
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
      text: "Прослушивания продолжат копиться в очереди и уйдут после нового входа.",
      buttons: [{ label: "да", value: "yes" }, { label: "нет", value: "no" }] });
    if (ans !== "yes") return;
    await api("/api/logout", {});
    await refresh();
  },
  async settingsOk() { await saveSettings(); view = "now"; await go(); },
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

const MOUNT = { now: mountNow, radio: mountRadio, queue: mountQueue, settings: mountSettings, login: mountLogin };
const UPDATE = { now: updateNow, radio: updateRadio, queue: updateQueue, settings: updateSettings, login: updateLogin };

async function go() {
  if (view === "settings") settings = await api("/api/settings");
  if (view === "radio") stations = await api("/api/radio/stations");
  if (view === "queue") queue = await api("/api/queue");
  render(true);
}

function render(force = false) {
  let want = st.loggedIn ? view : "login";
  if (want === "settings" && !settings) want = "now";
  if (want !== "login" && login.timer) stopLogin();
  if (force || want !== mounted) {
    mounted = want;
    for (const k in sigs) delete sigs[k];
    MOUNT[want]();
  }
  app.classList.toggle("solo", want === "login");
  nav.hidden = want === "login";
  updateNav();
  updateStatus();
  UPDATE[want]();
  frameReady();
}

let polling = false;

async function refresh() {
  if (polling) return;
  polling = true;
  try {
    st = await api("/api/state");
    syncAudio(st.radio);
    if (mounted === "queue" || (st.loggedIn && view === "queue")) queue = await api("/api/queue");
    render();
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
  main.innerHTML = windowHTML("Скробблер Last.fm", `<div class="message">${pix("warn", 4)}<div class="message-body">
    <h2>Нет доступа !</h2><p class="muted">Откройте окно через значок программы в трее или ярлык Скробблер Last.fm.</p></div></div>`);
  frameReady();
}

mountFrame();
mountNav();
const timer = setInterval(refresh, 1000);
if (!token) noAccess(); else refresh().then(() => { if (st && st.loggedIn && view !== "now") go().catch(showError); });
