"use strict";

(() => {
  const PAL = [[255, 26, 75], [255, 0, 204], [122, 0, 255], [0, 64, 255], [0, 240, 255], [57, 255, 136], [255, 230, 0], [255, 255, 255]];
  const NOTE = ["...######", "...######", "...#....#", "...#....#", "...#....#",
    ".###..###", "####.####", "####.####", ".##...##."];
  const root = document.documentElement;
  const calm = matchMedia("(prefers-reduced-motion: reduce)");
  let painted = false;
  let next = null;
  let shown = null;

  function random(seed) {
    return () => {
      seed = (seed + 0x6D2B79F5) | 0;
      let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
      t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
      return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
    };
  }

  function canvas(w, h) {
    const c = document.createElement("canvas");
    c.width = w;
    c.height = h;
    return c;
  }

  const css = ([r, g, b], a) => `rgba(${r},${g},${b},${a})`;

  function posterize(d) {
    for (let i = 0; i < d.length; i += 4) {
      for (let k = 0; k < 3; k++) {
        const v = d[i + k];
        d[i + k] = v > 200 ? 255 : v > 120 ? 190 : v > 50 ? 90 : 0;
      }
    }
  }

  function drip(d, w, h, rnd, clusters, density) {
    for (let k = 0; k < clusters; k++) {
      const cx = rnd() * w;
      const spread = 6 + rnd() * w * 0.12;
      const n = (density * (0.5 + rnd())) | 0;
      for (let j = 0; j < n; j++) {
        const x = Math.max(0, Math.min(w - 1, (cx + (rnd() - 0.5) * 2 * spread) | 0));
        let y = (rnd() * h * 0.85) | 0;
        const i0 = (y * w + x) * 4;
        let col = [d[i0], d[i0 + 1], d[i0 + 2]];
        if (col[0] + col[1] + col[2] < 90) {
          if (rnd() < 0.6) continue;
          col = PAL[(rnd() * PAL.length) | 0];
        }
        const len = (4 + rnd() * h * 0.55) | 0;
        const wd = 1 + ((rnd() * 2.4) | 0);
        for (let t = 0; t < len && y < h; t++, y++) {
          const f = 1 - (t / len) * 0.75;
          for (let dx = 0; dx < wd && x + dx < w; dx++) {
            const i = (y * w + x + dx) * 4;
            d[i] = col[0] * f;
            d[i + 1] = col[1] * f;
            d[i + 2] = col[2] * f;
          }
          if (rnd() < 0.03) {
            const i = (y * w + x) * 4;
            if (d[i] + d[i + 1] + d[i + 2] > 200) col = [d[i], d[i + 1], d[i + 2]];
          }
        }
      }
    }
  }

  function shift(d, w, h, rnd, bands, max) {
    for (let k = 0; k < bands; k++) {
      const y0 = (rnd() * h) | 0;
      const bh = 1 + ((rnd() * h * 0.05) | 0);
      const s = ((((rnd() - 0.5) * 2 * max * w) | 0) % w + w) % w * 4;
      for (let y = y0; y < Math.min(h, y0 + bh); y++) {
        const a = y * w * 4;
        const row = d.slice(a, a + w * 4);
        d.set(row.subarray(w * 4 - s), a);
        d.set(row.subarray(0, w * 4 - s), a + s);
      }
    }
  }

  function split(d, w, h, rnd, bands, max) {
    for (let k = 0; k < bands; k++) {
      const y0 = (rnd() * h) | 0;
      const bh = 2 + ((rnd() * h * 0.3) | 0);
      const o = 1 + ((rnd() * max) | 0);
      for (let y = y0; y < Math.min(h, y0 + bh); y++) {
        const a = y * w * 4;
        for (let x = w - 1; x >= o; x--) d[a + x * 4] = d[a + (x - o) * 4];
        for (let x = 0; x < w - o; x++) d[a + x * 4 + 2] = d[a + (x + o) * 4 + 2];
      }
    }
  }

  function noise(d, w, h, rnd, part) {
    const n = (w * h * part) | 0;
    for (let k = 0; k < n; k++) {
      const i = (((rnd() * h) | 0) * w + ((rnd() * w) | 0)) * 4;
      const c = PAL[(rnd() * PAL.length) | 0];
      d[i] = c[0];
      d[i + 1] = c[1];
      d[i + 2] = c[2];
    }
  }

  function blocks(g, c, w, h, rnd, n) {
    for (let k = 0; k < n; k++) {
      const s = rnd() < 0.6 ? 8 : 16;
      const bw = s * (1 + ((rnd() * 5) | 0));
      const sx = ((rnd() * (w - bw)) / 8 | 0) * 8;
      const sy = ((rnd() * (h - s)) / 8 | 0) * 8;
      const dx = Math.max(0, Math.min(w - bw, sx + (((rnd() - 0.5) * 12) | 0) * 8));
      const dy = Math.max(0, Math.min(h - s, sy + (((rnd() - 0.5) * 4) | 0) * 8));
      g.drawImage(c, sx, sy, bw, s, dx, dy, bw, s);
    }
  }

  function sortLine(d, start, step, count, lo) {
    const lum = (i) => d[i] * 0.3 + d[i + 1] * 0.59 + d[i + 2] * 0.11;
    let k = 0;
    while (k < count) {
      while (k < count && lum(start + k * step) <= lo) k++;
      const s = k;
      while (k < count && lum(start + k * step) > lo) k++;
      if (k - s < 3) continue;
      const px = [];
      for (let j = s; j < k; j++) {
        const i = start + j * step;
        px.push([d[i], d[i + 1], d[i + 2], lum(i)]);
      }
      px.sort((a, b) => b[3] - a[3]);
      for (let j = 0; j < px.length; j++) {
        const i = start + (s + j) * step;
        d[i] = px[j][0];
        d[i + 1] = px[j][1];
        d[i + 2] = px[j][2];
      }
    }
  }

  function art(w, h, rnd) {
    const c = canvas(w, h);
    const g = c.getContext("2d", { willReadFrequently: true });
    const pick = () => PAL[(rnd() * (PAL.length - 1)) | 0];
    g.fillStyle = "#000";
    g.fillRect(0, 0, w, h);
    g.lineCap = "round";
    for (let i = 0; i < 46; i++) {
      g.strokeStyle = css(pick(), 0.6 + rnd() * 0.4);
      g.lineWidth = 1 + rnd() * h * (i < 10 ? 0.16 : 0.04);
      g.beginPath();
      g.moveTo(rnd() * w, rnd() * h);
      g.bezierCurveTo(rnd() * w, rnd() * h, rnd() * w, rnd() * h, rnd() * w, rnd() * h);
      g.stroke();
    }
    for (let i = 0; i < 14; i++) {
      g.fillStyle = `rgba(0,0,0,${0.6 + rnd() * 0.4})`;
      g.beginPath();
      g.moveTo(rnd() * w, rnd() * h);
      for (let k = 0; k < 3; k++) g.lineTo(rnd() * w, rnd() * h);
      g.fill();
    }
    for (let i = 0; i < (w * h) / 450; i++) {
      g.fillStyle = css(PAL[(rnd() * PAL.length) | 0], 0.6 + rnd() * 0.4);
      g.fillRect((rnd() * w) | 0, (rnd() * h) | 0, 1 + ((rnd() * 2) | 0), 2 + ((rnd() * 14) | 0));
    }
    const img = g.getImageData(0, 0, w, h);
    const d = img.data;
    posterize(d);
    for (let k = 0; k < 4; k++) {
      const x0 = (rnd() * w) | 0;
      const x1 = Math.min(w, x0 + ((w * (0.1 + rnd() * 0.3)) | 0));
      const lo = 20 + rnd() * 50;
      for (let x = x0; x < x1; x++) sortLine(d, x * 4, w * 4, h, lo);
    }
    drip(d, w, h, rnd, 6, 110);
    shift(d, w, h, rnd, 6, 0.2);
    split(d, w, h, rnd, 5, 5);
    noise(d, w, h, rnd, 0.006);
    g.putImageData(img, 0, 0);
    blocks(g, c, w, h, rnd, 22);
    return c;
  }

  function note(rnd) {
    const s = 48;
    const c = canvas(s, s);
    const g = c.getContext("2d", { willReadFrequently: true });
    g.fillStyle = "#000";
    g.fillRect(0, 0, s, s);
    g.fillStyle = "#fff";
    NOTE.forEach((row, y) => {
      for (let x = 0; x < row.length; x++) if (row[x] === "#") g.fillRect(6 + x * 4, 4 + y * 4, 4, 4);
    });
    const img = g.getImageData(0, 0, s, s);
    const d = img.data;
    for (let x = 0; x < s; x++) {
      for (let y = s - 1; y >= 0; y--) {
        if (d[(y * s + x) * 4] < 200) continue;
        if (rnd() < 0.3) break;
        const col = PAL[(rnd() * (PAL.length - 1)) | 0];
        const len = 2 + ((rnd() * (s - y)) | 0);
        for (let t = 1; t < len && y + t < s; t++) {
          const i = ((y + t) * s + x) * 4;
          const f = 1 - (t / len) * 0.7;
          d[i] = col[0] * f;
          d[i + 1] = col[1] * f;
          d[i + 2] = col[2] * f;
        }
        break;
      }
    }
    shift(d, s, s, rnd, 4, 0.12);
    split(d, s, s, rnd, 3, 2);
    noise(d, s, s, rnd, 0.02);
    g.putImageData(img, 0, 0);
    return c;
  }

  function strip(rnd) {
    const w = 160;
    const h = 12;
    const c = canvas(w, h);
    const g = c.getContext("2d", { willReadFrequently: true });
    g.fillStyle = "#000";
    g.fillRect(0, 0, w, h);
    for (let y = 0; y < h;) {
      const bh = 1 + ((rnd() * 3) | 0);
      for (let x = 0; x < w;) {
        const len = 2 + ((rnd() * 28) | 0);
        if (rnd() > 0.3) {
          g.fillStyle = css(PAL[(rnd() * PAL.length) | 0], 0.5 + rnd() * 0.5);
          g.fillRect(x, y, len, bh);
        }
        x += len;
      }
      y += bh;
    }
    const img = g.getImageData(0, 0, w, h);
    const d = img.data;
    posterize(d);
    for (let y = 0; y < h; y++) sortLine(d, y * w * 4, 4, w, 25);
    split(d, w, h, rnd, 3, 4);
    noise(d, w, h, rnd, 0.02);
    g.putImageData(img, 0, 0);
    return c;
  }

  function make() {
    const rnd = random((Math.random() * 4294967296) >>> 0);
    const tall = innerHeight > innerWidth;
    return {
      art: `url(${art(tall ? 200 : 320, tall ? 360 : 220, rnd).toDataURL("image/png")})`,
      note: `url(${note(rnd).toDataURL("image/png")})`,
      strip: `url(${strip(rnd).toDataURL("image/png")})`,
    };
  }

  const idle = window.requestIdleCallback || ((f) => setTimeout(f, 300));

  function paint(set) {
    root.style.setProperty("--glitch-art", set.art);
    root.style.setProperty("--glitch-note", set.note);
    root.style.setProperty("--glitch-strip", set.strip);
  }

  function show(set) {
    paint(set);
    shown = set;
    painted = true;
    next = null;
    idle(() => {
      if (root.dataset.theme === "glitch" && !next) next = make();
    });
  }

  function check() {
    if (root.dataset.theme === "glitch" && !painted) show(make());
  }

  const later = () => 13000 + Math.random() * 4000;

  function cut() {
    const old = shown;
    const fresh = next || make();
    show(fresh);
    if (!old) return;
    setTimeout(() => paint(old), 70);
    setTimeout(() => paint(fresh), 130);
  }

  function burst() {
    if (root.dataset.theme === "glitch" && !root.classList.contains("still") && !document.hidden && !calm.matches) {
      root.style.setProperty("--tear-y", `${(Math.random() * 92) | 0}%`);
      root.style.setProperty("--tear-h", `${4 + ((Math.random() * 26) | 0)}px`);
      root.style.setProperty("--tear-x", `${(Math.random() * 100) | 0}%`);
      root.classList.add("glitching");
      setTimeout(cut, 140);
      setTimeout(() => root.classList.remove("glitching"), 450);
    }
    setTimeout(burst, later());
  }

  new MutationObserver(check).observe(root, { attributes: true, attributeFilter: ["data-theme"] });
  check();
  setTimeout(burst, later());
})();
