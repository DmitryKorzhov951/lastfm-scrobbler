#!/usr/bin/env python3
import os

import dfd

HERE = os.path.dirname(os.path.abspath(__file__))

FONT = "DejaVu Sans, Segoe UI, Liberation Sans, Arial, sans-serif"
DARK = "#243447"
HEAD = "#3d5372"
GRAY = "#6b7a8f"
LINE = "#243447"

BOX_W = 320
HEAD_H = 46
ROW_H = 33
GAP = 170


def esc(s):
    return s.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")


def entity(x, y, title, attrs):
    h = HEAD_H + ROW_H * len(attrs)
    p = [f'<rect x="{x}" y="{y}" width="{BOX_W}" height="{h}" rx="8" '
         f'fill="#ffffff" stroke="{DARK}" stroke-width="2.5"/>',
         f'<path d="M{x} {y+HEAD_H} V{y+8} a8,8 0 0 1 8,-8 H{x+BOX_W-8} '
         f'a8,8 0 0 1 8,8 V{y+HEAD_H} Z" fill="{HEAD}"/>',
         f'<text x="{x+BOX_W/2}" y="{y+30}" font-size="19" font-weight="700" '
         f'fill="#ffffff" text-anchor="middle">{esc(title)}</text>']
    for i, (name, typ, key) in enumerate(attrs):
        ry = y + HEAD_H + ROW_H * i
        if i:
            p.append(f'<line x1="{x+1}" y1="{ry}" x2="{x+BOX_W-1}" y2="{ry}" '
                     f'stroke="#d5dbe3" stroke-width="1"/>')
        p.append(f'<text x="{x+16}" y="{ry+22}" font-size="16" '
                 f'font-weight="{"700" if key else "400"}" fill="{DARK}">{esc(name)}</text>')
        p.append(f'<text x="{x+BOX_W-16}" y="{ry+22}" font-size="12.5" fill="{GRAY}" '
                 f'text-anchor="end">{esc(typ)}</text>')
    return "\n".join(p), h


def hlink(x1, x2, y, label, left="1", right="∞"):
    mid = (x1 + x2) / 2
    return f'''<line x1="{x1}" y1="{y}" x2="{x2}" y2="{y}" stroke="{LINE}" stroke-width="2.5"/>
<circle cx="{x1}" cy="{y}" r="4.5" fill="{LINE}"/>
<circle cx="{x2}" cy="{y}" r="4.5" fill="{LINE}"/>
<text x="{mid}" y="{y-14}" font-size="15" fill="{GRAY}" text-anchor="middle" font-style="italic">{esc(label)}</text>
<text x="{x1+14}" y="{y+24}" font-size="17" font-weight="700" fill="{DARK}">{left}</text>
<text x="{x2-14}" y="{y+24}" font-size="17" font-weight="700" fill="{DARK}" text-anchor="end">{right}</text>'''


def vlink(x, y1, y2, label, top="1", bottom="∞"):
    mid = (y1 + y2) / 2
    return f'''<line x1="{x}" y1="{y1}" x2="{x}" y2="{y2}" stroke="{LINE}" stroke-width="2.5"/>
<circle cx="{x}" cy="{y1}" r="4.5" fill="{LINE}"/>
<circle cx="{x}" cy="{y2}" r="4.5" fill="{LINE}"/>
<text x="{x-16}" y="{y1+24}" font-size="17" font-weight="700" fill="{DARK}" text-anchor="end">{top}</text>
<text x="{x-16}" y="{y2-14}" font-size="17" font-weight="700" fill="{DARK}" text-anchor="end">{bottom}</text>
<text x="{x+18}" y="{mid+5}" font-size="15" fill="{GRAY}" font-style="italic">{esc(label)}</text>'''


def er_body():
    cy = 160
    ents = [
        ("ПЛЕЕР", [("имя", "строка", True),
                   ("состояние", "перечисление", False),
                   ("позиция", "длительность", False)]),
        ("ТРЕК", [("исполнитель", "строка", True),
                  ("название", "строка", True),
                  ("альбом", "строка", False),
                  ("длительность", "время", False)]),
        ("ПРОСЛУШИВАНИЕ", [("исполнитель", "строка", True),
                           ("название", "строка", True),
                           ("время начала", "момент", True),
                           ("альбом", "строка", False),
                           ("длительность", "целое, с", False)]),
        ("ЗАПИСЬ ОЧЕРЕДИ", [("прослушивание", "ссылка", False),
                            ("время постановки", "момент", False),
                            ("число попыток", "целое", False)]),
    ]
    xs = [40 + i * (BOX_W + GAP) for i in range(4)]
    W = xs[-1] + BOX_W + 40

    parts, geom = [], []
    for x, (title, attrs) in zip(xs, ents):
        h = HEAD_H + ROW_H * len(attrs)
        y = cy - h / 2
        svg, _ = entity(x, y, title, attrs)
        parts.append(svg)
        geom.append((x, y, h))

    ux, uy = xs[2], 380
    usvg, uh = entity(ux, uy, "ПОЛЬЗОВАТЕЛЬ",
                      [("имя", "строка", True), ("ключ сессии", "строка", False)])
    parts.append(usvg)

    parts.append(hlink(xs[0] + BOX_W, xs[1], cy, "проигрывает"))
    parts.append(hlink(xs[1] + BOX_W, xs[2], cy, "порождает"))
    parts.append(hlink(xs[2] + BOX_W, xs[3], cy, "хранится как", right="1"))
    parts.append(vlink(ux + BOX_W / 2, geom[2][1] + geom[2][2], uy,
                       "слушает", top="∞", bottom="1"))

    H = uy + uh + 62
    parts.append(f'<text x="40" y="{H-22}" font-size="15" fill="{GRAY}">'
                 f'Полужирным выделены ключевые атрибуты. '
                 f'1 — один, ∞ — много.</text>')
    return W, H, "\n".join(parts)


def flow_body():
    bw, bh, y, gap = 290, 110, 70, 70
    xs = [40 + i * (bw + gap) for i in range(4)]
    W = xs[-1] + bw + 40
    blocks = [
        ("Медиаплеер", "состояние и метаданные", "медиаэлементы Windows", "#eef2f7"),
        ("Подсчёт времени", "сколько трек звучал", "правила Last.fm", "#eef2f7"),
        ("Очередь на диске", "неотправленные записи", "файл queue.json", "#fff4e0"),
        ("Сервис Last.fm", "профиль пользователя", "пакет до 50 записей", "#eef2f7"),
    ]
    parts = ['<defs><marker id="arr" viewBox="0 0 10 10" refX="9" refY="5" '
             'markerWidth="7" markerHeight="7" orient="auto-start-reverse">'
             f'<path d="M0,0 L10,5 L0,10 z" fill="{LINE}"/></marker></defs>']
    for i, (x, (t1, t2, t3, fill)) in enumerate(zip(xs, blocks)):
        parts.append(f'<rect x="{x}" y="{y}" width="{bw}" height="{bh}" rx="10" '
                     f'fill="{fill}" stroke="{DARK}" stroke-width="2.5"/>')
        parts.append(f'<text x="{x+bw/2}" y="{y+38}" font-size="20" font-weight="700" '
                     f'fill="{DARK}" text-anchor="middle">{esc(t1)}</text>')
        parts.append(f'<text x="{x+bw/2}" y="{y+64}" font-size="15" fill="{DARK}" '
                     f'text-anchor="middle">{esc(t2)}</text>')
        parts.append(f'<text x="{x+bw/2}" y="{y+88}" font-size="13" fill="{GRAY}" '
                     f'text-anchor="middle">{esc(t3)}</text>')
        if i:
            parts.append(f'<line x1="{xs[i-1]+bw+10}" y1="{y+bh/2}" x2="{x-10}" y2="{y+bh/2}" '
                         f'stroke="{LINE}" stroke-width="3" marker-end="url(#arr)"/>')
    H = y + bh + 80
    parts.append(f'<text x="{W/2}" y="{y+bh+56}" font-size="16" fill="{DARK}" '
                 f'text-anchor="middle">Прослушивание записывается на диск до отправки, '
                 f'поэтому не теряется при обрыве связи.</text>')
    return W, H, "\n".join(parts)


def svg_file(W, H, body):
    return (f'<svg xmlns="http://www.w3.org/2000/svg" width="{W}" height="{H}" '
            f'viewBox="0 0 {W} {H}" font-family="{FONT}">\n'
            f'<rect width="{W}" height="{H}" fill="#ffffff"/>\n{body}\n</svg>\n')


def svg_embed(W, H, body, title):
    floor = min(900, round(W * 0.62))
    return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} {H}" '
            f'role="img" aria-label="{esc(title)}" font-family="{FONT}" '
            f'style="width:100%;min-width:{floor}px;height:auto;display:block">\n'
            f'<rect width="{W}" height="{H}" fill="#ffffff"/>\n{body}\n</svg>')


def figure(name, W, H, body, title):
    uniq = body.replace('"arr"', f'"arr-{name}"').replace("url(#arr)", f"url(#arr-{name})")
    return "\n".join([
        f'<figure style="margin:0 0 2rem;max-width:{W}px">',
        '  <div style="overflow-x:auto;border:1px solid #e2e6ec;border-radius:10px">',
        svg_embed(W, H, uniq, title),
        '  </div>',
        f'  <figcaption style="font:14px/1.5 system-ui,sans-serif;color:#6b7a8f;'
        f'margin-top:.6rem">{esc(title)}</figcaption>',
        '</figure>'])


root = os.path.join(os.path.dirname(HERE), "docs")
os.makedirs(f"{root}/img", exist_ok=True)
os.makedirs(f"{root}/embed", exist_ok=True)

diagrams = {
    "data-model": (er_body(), "Модель данных приложения lastfm-scrobbler"),
    "data-flow": (flow_body(), "Путь данных: плеер, подсчёт времени, очередь, Last.fm"),
    "dfd-context": (dfd.context_body(), "Контекстная диаграмма потоков данных (уровень 0)"),
    "dfd-level1": (dfd.level1_body(), "Диаграмма потоков данных, уровень 1"),
}

for name, ((W, H, body), title) in diagrams.items():
    open(f"{root}/img/{name}.svg", "w", encoding="utf-8").write(svg_file(W, H, body))
    print(f"{name}: {W}x{H}")

combined = []
preview = ['<!doctype html><html lang="ru"><head><meta charset="utf-8">',
           '<meta name="viewport" content="width=device-width,initial-scale=1">',
           '<title>Схемы информационной модели</title>',
           '<style>body{margin:0;padding:24px;background:#fafbfc;'
           'font:16px/1.5 system-ui,sans-serif;color:#243447}'
           'main{max-width:1200px;margin:0 auto}h1{font-size:22px}</style>',
           '</head><body><main><h1>Схемы информационной модели</h1>']

for name, ((W, H, body), title) in diagrams.items():
    snippet = figure(name, W, H, body, title)
    open(f"{root}/embed/{name}.html", "w", encoding="utf-8").write(snippet + "\n")
    combined.append(snippet)
    preview.append(snippet)

open(f"{root}/embed/diagrams.html", "w", encoding="utf-8").write("\n\n".join(combined) + "\n")
preview.append('</main></body></html>')
open(f"{root}/embed/preview.html", "w", encoding="utf-8").write("\n".join(preview) + "\n")

print("код для вставки:", ", ".join(f"{n}.html" for n in diagrams),
      "+ diagrams.html, preview.html")
