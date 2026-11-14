#!/usr/bin/env python3
import os

HERE = os.path.dirname(os.path.abspath(__file__))

FONT = "DejaVu Sans, Segoe UI, Liberation Sans, Arial, sans-serif"
DARK = "#243447"
HEAD = "#3d5372"
GRAY = "#6b7a8f"
EXT_FILL = "#eef2f7"
PROC_FILL = "#ffffff"
STORE_FILL = "#fff4e0"


def esc(s):
    return s.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")


def lines(x, y, rows, size=13, fill=GRAY, anchor="start", style=""):
    y0 = y - (len(rows) - 1) * (size + 4)
    out = [f'<text x="{x}" y="{y0}" font-size="{size}" fill="{fill}" '
           f'text-anchor="{anchor}" {style}>']
    for i, r in enumerate(rows):
        dy = 0 if i == 0 else size + 4
        out.append(f'<tspan x="{x}" dy="{dy}">{esc(r)}</tspan>')
    out.append('</text>')
    return "".join(out)


def external(x, y, w, h, title, sub=None):
    p = [f'<rect x="{x}" y="{y}" width="{w}" height="{h}" fill="{EXT_FILL}" '
         f'stroke="{DARK}" stroke-width="2.5"/>']
    ty = y + h / 2 + (0 if sub is None else -6)
    p.append(f'<text x="{x+w/2}" y="{ty}" font-size="18" font-weight="700" '
             f'fill="{DARK}" text-anchor="middle">{esc(title)}</text>')
    if sub:
        p.append(f'<text x="{x+w/2}" y="{ty+22}" font-size="13" fill="{GRAY}" '
                 f'text-anchor="middle">{esc(sub)}</text>')
    return "\n".join(p)


def process(x, y, w, h, num, title, sub=None):
    p = [f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="12" fill="{PROC_FILL}" '
         f'stroke="{DARK}" stroke-width="2.5"/>',
         f'<circle cx="{x+30}" cy="{y+28}" r="17" fill="{HEAD}"/>',
         f'<text x="{x+30}" y="{y+34}" font-size="17" font-weight="700" fill="#ffffff" '
         f'text-anchor="middle">{num}</text>']
    ty = y + h / 2 + (2 if sub is None else -6)
    p.append(f'<text x="{x+w/2+18}" y="{ty}" font-size="17" font-weight="700" '
             f'fill="{DARK}" text-anchor="middle">{esc(title)}</text>')
    if sub:
        p.append(f'<text x="{x+w/2+18}" y="{ty+21}" font-size="13" fill="{GRAY}" '
                 f'text-anchor="middle">{esc(sub)}</text>')
    return "\n".join(p)


def store(x, y, w, h, ident, title, sub=None):
    p = [f'<path d="M{x+w} {y} H{x} V{y+h} H{x+w}" fill="{STORE_FILL}" '
         f'stroke="{DARK}" stroke-width="2.5" stroke-linejoin="round"/>',
         f'<line x1="{x+52}" y1="{y}" x2="{x+52}" y2="{y+h}" stroke="{DARK}" stroke-width="2.5"/>',
         f'<text x="{x+26}" y="{y+h/2+6}" font-size="16" font-weight="700" fill="{DARK}" '
         f'text-anchor="middle">{ident}</text>']
    cx = x + 52 + (w - 52) / 2
    ty = y + h / 2 + (5 if sub is None else -4)
    p.append(f'<text x="{cx}" y="{ty}" font-size="16" font-weight="700" fill="{DARK}" '
             f'text-anchor="middle">{esc(title)}</text>')
    if sub:
        p.append(f'<text x="{cx}" y="{ty+19}" font-size="12.5" fill="{GRAY}" '
                 f'text-anchor="middle">{esc(sub)}</text>')
    return "\n".join(p)


def flow(points, label=None, lx=0, ly=0, anchor="start"):
    pts = " ".join(f"{x},{y}" for x, y in points)
    p = [f'<polyline points="{pts}" fill="none" stroke="{DARK}" stroke-width="2.5" '
         f'marker-end="url(#arr)"/>']
    if label:
        p.append(lines(lx, ly, label, anchor=anchor))
    return "\n".join(p)


MARKER = ('<defs><marker id="arr" viewBox="0 0 10 10" refX="9" refY="5" '
          'markerWidth="7" markerHeight="7" orient="auto-start-reverse">'
          f'<path d="M0,0 L10,5 L0,10 z" fill="{DARK}"/></marker></defs>')


def context_body():
    W, H = 1200, 360
    p = [MARKER]

    p.append(external(40, 60, 260, 80, "Медиаплеер", "медиаэлементы Windows"))
    p.append(external(40, 190, 260, 110, "Пользователь"))
    p.append(external(900, 60, 260, 110, "Сервис Last.fm"))
    p.append(process(440, 30, 320, 250, "0", "Скробблер", "lastfm-scrobbler"))

    p.append(flow([(300, 100), (440, 100)],
                  ["состояние,", "метаданные трека"], 380, 90, "middle"))
    p.append(flow([(300, 215), (440, 215)],
                  ["команды,", "ключи приложения"], 380, 205, "middle"))
    p.append(flow([(440, 258), (300, 258)],
                  ["состояние, отчёты"], 380, 284, "middle"))
    p.append(flow([(760, 95), (900, 95)],
                  ["прослушивания,", "«сейчас играет»"], 830, 85, "middle"))
    p.append(flow([(900, 140), (760, 140)],
                  ["ключ сессии,", "итог обработки"], 830, 175, "middle"))

    p.append(lines(40, H - 22, ["Прямоугольник — внешняя сущность, "
                                "скруглённый блок — процесс."], size=14))
    return W, H, "\n".join(p)


def level1_body():
    W, H = 1160, 880
    cx, cw = 540, 340
    ax = cx + 160
    p = [MARKER]

    p.append(external(cx, 40, cw, 70, "Медиаплеер", "медиаэлементы Windows"))
    p.append(process(cx, 170, cw, 80, "1", "Слежение за плеерами"))
    p.append(process(cx, 310, cw, 80, "2", "Учёт прослушивания"))
    p.append(store(cx, 450, cw, 70, "D2", "Очередь прослушиваний", "queue.json"))
    p.append(process(cx, 580, cw, 80, "3", "Отправка в сервис"))
    p.append(external(cx, 720, cw, 70, "Сервис Last.fm"))

    chain = [((110, 170), ["состояние и метаданные"]),
             ((250, 310), ["снимок состояния"]),
             ((390, 450), ["прослушивание"]),
             ((520, 580), ["пакет до 50 записей"]),
             ((660, 720), ["подписанный запрос"])]
    for (y1, y2), label in chain:
        p.append(flow([(ax, y1), (ax, y2)], label, ax + 16, (y1 + y2) / 2 + 5))

    p.append(flow([(ax - 60, 720), (ax - 60, 660)],
                  ["итог обработки"], ax - 76, 695, "end"))

    p.append(flow([(cx + cw, 350), (950, 350), (950, 620), (cx + cw, 620)],
                  ["«сейчас играет»", "минует очередь"], 966, 490))

    p.append(external(40, 40, 240, 70, "Пользователь"))
    p.append(process(40, 170, 240, 80, "4", "Управление"))
    p.append(store(40, 330, 240, 70, "D1", "Настройки и сессия", "config.json, session.json"))

    p.append(flow([(160, 110), (160, 170)], ["команды"], 176, 145))
    p.append(flow([(160, 250), (160, 330)], ["ключи,", "ключ сессии"], 176, 295))
    p.append(flow([(280, 365), (cx, 365)], ["пороги и фильтры"], 410, 352, "middle"))
    p.append(flow([(160, 400), (160, 615), (cx, 615)], ["ключ сессии"], 350, 602, "middle"))

    p.append(lines(40, H - 22,
                   ["Прямоугольник — внешняя сущность, скруглённый блок — процесс, "
                    "блок, открытый справа, — хранилище данных."], size=14))
    return W, H, "\n".join(p)


def svg_file(W, H, body):
    return (f'<svg xmlns="http://www.w3.org/2000/svg" width="{W}" height="{H}" '
            f'viewBox="0 0 {W} {H}" font-family="{FONT}">\n'
            f'<rect width="{W}" height="{H}" fill="#ffffff"/>\n{body}\n</svg>\n')


if __name__ == "__main__":
    out = os.path.join(os.path.dirname(HERE), "docs", "img")
    os.makedirs(out, exist_ok=True)
    for name, fn in [("dfd-context", context_body), ("dfd-level1", level1_body)]:
        W, H, body = fn()
        open(f"{out}/{name}.svg", "w", encoding="utf-8").write(svg_file(W, H, body))
        print(f"{name}: {W}x{H}")
