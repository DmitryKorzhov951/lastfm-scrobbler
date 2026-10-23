#!/usr/bin/env python3
import math
import os

HERE = os.path.dirname(os.path.abspath(__file__))
OUT = os.path.join(HERE, "figma")

THEMES = {
    "default": {
        "font": "Segoe UI, Liberation Sans, Arial, sans-serif",
        "square": False,
        "lowercase": False,
        "flat_weight": False,
        "title_fx": False,
        "title_size": 28,
        "colors": {
            "bg": "#F3F3F3", "surface": "#FFFFFF", "border": "#E5E5E5", "border2": "#D1D1D1",
            "sep": "#EDEDED", "track": "#E5E5E5", "select": "#E9E9E9", "on_select": "#1A1A1A",
            "hover": "#F0F0F0",
            "text": "#1A1A1A", "text2": "#5C5C5C", "text3": "#8A8A8A",
            "accent": "#5B5BD6", "accent2": "#8E5BD6", "accent_soft": "#EEEEFC", "on_accent": "#FFFFFF",
            "ok": "#0F7B0F", "chip_bg": "#DFF6DD", "chip_fg": "#0F7B0F",
            "dot_ok": "#0F7B0F", "dot_warn": "#9D5D00",
            "badge_bg": "#FFF4CE", "badge_fg": "#9D5D00", "badge_border": "#F2DFA6",
            "info_bg": "#FFF4CE", "info_border": "#F2DFA6", "info_fg": "#1A1A1A",
            "info_icon_bg": "#9D5D00", "info_icon_fg": "#FFFFFF",
            "danger": "#C42B1C",
            "wall1": "#DDE7F6", "wall2": "#C3D2EC",
        },
    },
    "retro": {
        "font": "Tiny5, Liberation Mono, monospace",
        "square": True,
        "lowercase": False,
        "flat_weight": False,
        "title_fx": False,
        "title_size": 22,
        "note_size": 16,
        "colors": {
            "bg": "#FBE3F4", "surface": "#F6C9EA", "border": "#5C2E55", "border2": "#5C2E55",
            "sep": "#E7B3DA", "track": "#FFF8FD", "select": "#F0B8E0", "on_select": "#3A1F38",
            "hover": "#F0B8E0",
            "text": "#3A1F38", "text2": "#7A4A72", "text3": "#A77A9F",
            "accent": "#C35BB2", "accent2": "#E59BD8", "accent_soft": "#F0B8E0", "on_accent": "#FFF4FC",
            "ok": "#3A1F38", "chip_bg": "#F6C9EA", "chip_fg": "#3A1F38",
            "dot_ok": "#C35BB2", "dot_warn": "#7A4A72",
            "badge_bg": "#C35BB2", "badge_fg": "#FFF4FC", "badge_border": "#5C2E55",
            "info_bg": "#F6C9EA", "info_border": "#5C2E55", "info_fg": "#3A1F38",
            "info_icon_bg": "#C35BB2", "info_icon_fg": "#FFF4FC",
            "danger": "#A0306E",
            "wall1": "#FBD3EE", "wall2": "#E2C2F2",
        },
    },
    "brat": {
        "font": "Arial, Liberation Sans, sans-serif",
        "square": True,
        "lowercase": True,
        "flat_weight": True,
        "title_fx": True,
        "title_size": 40,
        "colors": {
            "bg": "#8ACE00", "surface": "#8ACE00", "border": "#000000", "border2": "#000000",
            "sep": "#6FA800", "track": "#6FA800", "select": "#000000", "on_select": "#8ACE00",
            "hover": "#7DBC00",
            "text": "#000000", "text2": "#203300", "text3": "#4A6B00",
            "accent": "#000000", "accent2": "#000000", "accent_soft": "#7DBC00", "on_accent": "#8ACE00",
            "ok": "#000000", "chip_bg": "#000000", "chip_fg": "#8ACE00",
            "dot_ok": "#000000", "dot_warn": "#000000",
            "badge_bg": "#000000", "badge_fg": "#8ACE00", "badge_border": "#000000",
            "info_bg": "#000000", "info_border": "#000000", "info_fg": "#8ACE00",
            "info_icon_bg": "#8ACE00", "info_icon_fg": "#000000",
            "danger": "#000000",
            "wall1": "#8ACE00", "wall2": "#7DBC00",
        },
    },
}

THEMES["retro_dark"] = dict(THEMES["retro"], colors=dict(
    THEMES["retro"]["colors"],
    bg="#141016", surface="#26202B", border="#000000", border2="#000000",
    text="#EEE8F0", text2="#A99DB0", text3="#7E7286",
    accent="#B52C44", accent2="#3A1A2A", on_accent="#F7F1F5",
    wall1="#1E1823", wall2="#2E1A26",
))

TH = THEMES["default"]
C = dict(TH["colors"])
FONT = TH["font"]


def use_theme(name):
    global TH, C, FONT
    TH = THEMES[name]
    C = dict(TH["colors"])
    FONT = TH["font"]


W = 960
NAV_W = 220
TITLE_H = 32
X0 = NAV_W + 36
X1 = W - 36
CW = X1 - X0


def esc(s):
    return str(s).replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")


def tw(s, size, weight=400):
    if TH["flat_weight"]:
        weight = 400
    narrow = set("iljtfr.,:;!|'1 ·")
    wide = set("mwшщжфюыМШЩЖФЮЫ")
    k = 0.57 if weight < 600 else 0.61
    total = 0.0
    for ch in s:
        if ch in narrow:
            total += 0.30
        elif ch in wide:
            total += 0.82
        elif ch.isupper():
            total += 0.68
        elif ch.isdigit():
            total += 0.56
        else:
            total += k
    return total * size


class Svg:
    def __init__(self, w, h):
        self.w, self.h = w, h
        self.parts = []
        self.defs = []
        self.used = {}
        if TH["title_fx"]:
            self.defs.append('<filter id="soft" x="-5%" y="-20%" width="110%" height="140%">'
                             '<feGaussianBlur stdDeviation="0.75"/></filter>')

    def uid(self, name):
        n = self.used.get(name, 0) + 1
        self.used[name] = n
        return name if n == 1 else f"{name} {n}"

    def open(self, name):
        self.parts.append(f'<g id="{esc(self.uid(name))}">')

    def close(self):
        self.parts.append("</g>")

    def rect(self, x, y, w, h, fill, rx=0, stroke=None, sw=1, name=None, extra=""):
        if TH["square"]:
            rx = 0
        a = f'x="{x}" y="{y}" width="{w}" height="{h}" fill="{fill}"'
        if rx:
            a += f' rx="{rx}"'
        if stroke:
            a += f' stroke="{stroke}" stroke-width="{sw}"'
        if name:
            a += f' id="{esc(self.uid(name))}"'
        self.parts.append(f"<rect {a} {extra}/>")

    def text(self, x, y, s, size=14, weight=400, fill=None, anchor="start", name=None,
             font=None, spacing=None):
        if TH["lowercase"]:
            s = str(s).lower()
        if TH["flat_weight"]:
            weight = 400
        a = (f'x="{x}" y="{y}" font-family="{font or FONT}" font-size="{size}" '
             f'font-weight="{weight}" fill="{fill or C["text"]}"')
        if spacing:
            a += f' letter-spacing="{spacing}"'
        if TH["title_fx"] and size >= 20:
            a += f' transform="translate({x} 0) scale(0.84 1) translate({-x} 0)" filter="url(#soft)"'
        if anchor != "start":
            a += f' text-anchor="{anchor}"'
        if name:
            a += f' id="{esc(self.uid(name))}"'
        self.parts.append(f"<text {a}>{esc(s)}</text>")

    def line(self, x1, y1, x2, y2, stroke, sw=1):
        self.parts.append(f'<line x1="{x1}" y1="{y1}" x2="{x2}" y2="{y2}" '
                          f'stroke="{stroke}" stroke-width="{sw}"/>')

    def circle(self, cx, cy, r, fill, stroke=None, sw=1):
        a = f'cx="{cx}" cy="{cy}" r="{r}" fill="{fill}"'
        if stroke:
            a += f' stroke="{stroke}" stroke-width="{sw}"'
        self.parts.append(f"<circle {a}/>")

    def path(self, d, fill="none", stroke=None, sw=1.5, extra=""):
        a = f'd="{d}" fill="{fill}"'
        if stroke:
            a += (f' stroke="{stroke}" stroke-width="{sw}" '
                  f'stroke-linecap="round" stroke-linejoin="round"')
        self.parts.append(f"<path {a} {extra}/>")

    def raw(self, s):
        self.parts.append(s)

    def render(self):
        defs = f"<defs>{''.join(self.defs)}</defs>" if self.defs else ""
        return (f'<svg xmlns="http://www.w3.org/2000/svg" width="{self.w}" height="{self.h}" '
                f'viewBox="0 0 {self.w} {self.h}">{defs}\n' + "\n".join(self.parts) + "\n</svg>\n")


def icon(s, kind, x, y, color, size=16, sw=1.5):
    k = size / 16
    s.raw(f'<g transform="translate({x},{y}) scale({k})">')
    if kind == "note":
        s.path("M6 12.5V3.6L13 2.2V11", stroke=color, sw=sw / k)
        s.circle(4.4, 12.5, 1.6, "none", color, sw / k)
        s.circle(11.4, 11, 1.6, "none", color, sw / k)
    elif kind == "list":
        for yy in (4, 8, 12):
            s.path(f"M5.5 {yy}H13.5", stroke=color, sw=sw / k)
            s.circle(2.8, yy, 0.9, color)
    elif kind == "gear":
        pts = []
        for i in range(16):
            r = 7 if i % 2 == 0 else 5.4
            a = math.pi * 2 * (i - 0.5) / 16
            pts.append(f"{8 + r * math.cos(a):.2f},{8 + r * math.sin(a):.2f}")
            a2 = math.pi * 2 * (i + 0.5) / 16
            pts.append(f"{8 + r * math.cos(a2):.2f},{8 + r * math.sin(a2):.2f}")
        s.path("M" + " L".join(pts) + "Z", stroke=color, sw=sw / k)
        s.circle(8, 8, 2.2, "none", color, sw / k)
    elif kind == "external":
        s.path("M9.5 2.5H13.5V6.5M13.5 2.5L8 8M12 9.5V13.5H2.5V4H6.5", stroke=color, sw=sw / k)
    elif kind == "check":
        s.path("M3.5 8.5L6.5 11.5L12.5 4.5", stroke=color, sw=sw / k)
    elif kind == "chevron":
        s.path("M4.5 6.5L8 10L11.5 6.5", stroke=color, sw=sw / k)
    elif kind == "pause":
        s.path("M5.5 3.5V12.5M10.5 3.5V12.5", stroke=color, sw=sw / k)
    elif kind == "window":
        s.rect(2.5, 3.5, 11, 9, "none", rx=1.5, stroke=color, sw=sw / k)
        s.path("M2.5 6H13.5", stroke=color, sw=sw / k)
    elif kind == "power":
        s.path("M8 2V7.5M4.6 4.4A5 5 0 1 0 11.4 4.4", stroke=color, sw=sw / k)
    elif kind == "folder":
        s.path("M2 4.5A1 1 0 0 1 3 3.5H6.2L7.7 5H13A1 1 0 0 1 14 6V12A1 1 0 0 1 13 13H3A1 1 0 0 1 2 12Z",
               stroke=color, sw=sw / k)
    elif kind == "key":
        s.circle(5.5, 10.5, 3, "none", color, sw / k)
        s.path("M7.7 8.3L13.5 2.5M11.5 4.5L13 6M10 6L11.3 7.3", stroke=color, sw=sw / k)
    s.raw("</g>")


def app_icon(s, x, y, size, name="Значок приложения"):
    gid = f"appgrad{len(s.defs)}"
    s.defs.append(f'<linearGradient id="{gid}" x1="0" y1="0" x2="1" y2="1">'
                  f'<stop offset="0" stop-color="{C["accent"]}"/>'
                  f'<stop offset="1" stop-color="{C["accent2"]}"/></linearGradient>')
    k = size / 16
    s.open(name)
    s.rect(x, y, size, size, f"url(#{gid})", rx=round(3.6 * k, 1))
    for bx, bh in ((4, 6), (7, 9), (10, 7)):
        s.rect(round(x + bx * k, 1), round(y + (12.5 - bh) * k, 1), round(2 * k, 1), round(bh * k, 1),
               C["on_accent"], rx=round(1 * k, 1))
    s.close()


def window(s, h, title="Скробблер Last.fm"):
    s.rect(0, 0, W, h, C["bg"], rx=8, stroke=C["border2"], name="Окно")
    s.open("Заголовок окна")
    app_icon(s, 12, 8, 16, "Значок")
    s.text(38, 21, title, size=12, fill=C["text"])
    bx = W - 46 * 3
    s.path(f"M{bx + 18} 16H{bx + 28}", stroke=C["text"], sw=1)
    s.rect(bx + 46 + 18, 11, 10, 10, "none", rx=1, stroke=C["text"], sw=1)
    cx = bx + 92 + 18
    s.path(f"M{cx} 11L{cx + 10} 21M{cx + 10} 11L{cx} 21", stroke=C["text"], sw=1)
    s.close()


def nav(s, h, selected, queue_badge=None, online=True):
    s.open("Навигация")
    items = [("Сейчас", "note"), ("Очередь", "list"), ("Настройки", "gear")]
    for i, (label, ic) in enumerate(items):
        y = 48 + i * 40
        s.open(f"Пункт «{label}»")
        sel = label == selected
        if sel:
            s.rect(8, y, NAV_W - 16, 36, C["select"], rx=4)
            s.rect(8, y + 10, 3, 16, C["accent"], rx=1.5, name="Индикатор")
        fg = C["on_select"] if sel else C["text"]
        icon(s, ic, 24, y + 10, fg)
        s.text(52, y + 23, label, size=14, weight=600 if sel else 400, fill=fg)
        if label == "Очередь" and queue_badge:
            bw = max(20, tw(str(queue_badge), 12, 600) + 12)
            s.rect(NAV_W - 20 - bw, y + 8, bw, 20, C["badge_bg"], rx=10, stroke=C["badge_border"])
            s.text(NAV_W - 20 - bw / 2, y + 22, queue_badge, size=12, weight=600,
                   fill=C["badge_fg"], anchor="middle")
        s.close()

    s.open("Аккаунт")
    s.line(16, h - 76, NAV_W - 16, h - 76, C["border"])
    s.circle(32, h - 40, 16, C["accent_soft"])
    s.text(32, h - 35, "L", size=14, weight=600, fill=C["accent"], anchor="middle")
    s.text(56, h - 44, "listener", size=14, weight=600)
    dot = C["dot_ok"] if online else C["dot_warn"]
    s.circle(60, h - 28, 3.5, dot)
    s.text(68, h - 24, "Last.fm · в сети" if online else "Last.fm · нет связи", size=12, fill=C["text2"])
    s.close()
    s.close()


def toggle(s, x, y, on, name="Переключатель"):
    s.open(name)
    if on:
        s.rect(x, y, 40, 20, C["accent"], rx=10)
        s.circle(x + 30, y + 10, 6, C["on_accent"])
    else:
        s.rect(x + 0.5, y + 0.5, 39, 19, C["surface"], rx=9.5, stroke=C["text2"])
        s.circle(x + 10, y + 10, 5, C["text2"])
    s.close()


def button(s, x, y, w, label, kind="secondary", h=32, name=None, icon_kind=None):
    s.open(name or f"Кнопка «{label}»")
    if kind == "primary":
        s.rect(x, y, w, h, C["accent"], rx=4)
        fill, weight = C["on_accent"], 600
    else:
        s.rect(x + 0.5, y + 0.5, w - 1, h - 1, C["surface"], rx=4, stroke=C["border2"])
        fill, weight = (C["danger"] if kind == "danger" else C["text"]), 400
    cx = x + w / 2
    if icon_kind:
        lw = tw(label, 14, weight)
        ix = cx - (lw + 24) / 2
        icon(s, icon_kind, ix, y + h / 2 - 8, fill)
        s.text(ix + 24, y + h / 2 + 5, label, size=14, weight=weight, fill=fill)
    else:
        s.text(cx, y + h / 2 + 5, label, size=14, weight=weight, fill=fill, anchor="middle")
    s.close()


def chip(s, x_right, y, label, kind="ok", name=None):
    fg, bg = {"ok": (C["chip_fg"], C["chip_bg"]), "warn": (C["badge_fg"], C["badge_bg"]),
              "accent": (C["accent"], C["accent_soft"])}[kind]
    w = tw(label, 12, 600) + 20
    s.open(name or f"Статус «{label}»")
    s.rect(x_right - w, y, w, 22, bg, rx=11)
    s.text(x_right - w / 2, y + 15, label, size=12, weight=600, fill=fg, anchor="middle")
    s.close()


def card(s, x, y, w, h, name):
    s.rect(x, y, w, h, C["surface"], rx=8, stroke=C["border"], name=name)


def setting_row(s, y, title, caption, h=64, name=None):
    s.open(name or f"Настройка «{title}»")
    card(s, X0, y, CW, h, "Фон")
    if caption:
        s.text(X0 + 20, y + 28, title, size=14)
        s.text(X0 + 20, y + 47, caption, size=12, fill=C["text2"])
    else:
        s.text(X0 + 20, y + h / 2 + 5, title, size=14)
    s.close()


def screen_now():
    h = 640
    s = Svg(W, h)
    window(s, h)
    nav(s, h, "Сейчас")
    s.open("Содержимое")

    s.text(X0, 84, "Сейчас", size=TH["title_size"], weight=600, name="Заголовок")
    s.text(X1 - 52, 81, "Скробблинг", size=14, anchor="end", name="Подпись переключателя")
    toggle(s, X1 - 40, 66, True, "Переключатель «Скробблинг»")

    y = 108
    s.open("Сейчас играет")
    card(s, X0, y, CW, 184, "Фон карточки")
    gid = "art"
    s.defs.append(f'<linearGradient id="{gid}" x1="0" y1="0" x2="1" y2="1">'
                  f'<stop offset="0" stop-color="{C["accent"]}"/>'
                  f'<stop offset="1" stop-color="{C["accent2"]}"/></linearGradient>')
    s.open("Обложка")
    s.rect(X0 + 20, y + 20, 144, 144, f"url(#{gid})", rx=6)
    icon(s, "note", X0 + 20 + 40, y + 20 + 40, C["on_accent"], size=64, sw=1.6)
    s.close()

    tx = X0 + 188
    s.open("Плеер")
    label = "Spotify · играет"
    cw = tw(label, 12, 600) + 30
    s.rect(tx, y + 24, cw, 22, C["accent_soft"], rx=11)
    s.circle(tx + 12, y + 35, 3.5, C["dot_ok"])
    s.text(tx + 22, y + 39, label, size=12, weight=600, fill=C["accent"])
    s.close()
    s.text(tx, y + 80, "Кукушка", size=24, weight=600, name="Название")
    s.text(tx, y + 106, "Кино", size=16, name="Исполнитель")
    s.text(tx, y + 128, "Чёрный альбом · 6:36", size=13, fill=C["text2"], name="Альбом")

    s.open("Прогресс")
    bar_w = X1 - 20 - tx
    s.rect(tx, y + 146, bar_w, 4, C["track"], rx=2, name="Дорожка")
    s.rect(tx, y + 146, round(bar_w * 0.6), 4, C["accent"], rx=2, name="Заполнение")
    s.text(tx, y + 170, "Прослушано 1:48 из нужных 3:00", size=12, fill=C["text2"])
    s.text(X1 - 20, y + 170, "засчитается через 1:12", size=12, weight=600, fill=C["accent"], anchor="end")
    s.close()
    s.close()

    y = 312
    tile_w = (CW - 32) / 3
    tiles = [("Засчитано за сеанс", "23", None), ("В очереди", "0", "всё отправлено"),
             ("Last.fm", "в сети", None)]
    for i, (cap, val, note) in enumerate(tiles):
        x = X0 + i * (tile_w + 16)
        s.open(f"Плитка «{cap}»")
        card(s, x, y, tile_w, 88, "Фон")
        s.text(x + 20, y + 28, cap, size=12, fill=C["text2"])
        if cap == "Last.fm":
            s.circle(x + 26, y + 61, 5, C["dot_ok"])
            s.text(x + 38, y + 68, val, size=20, weight=600)
        else:
            s.text(x + 20, y + 68, val, size=28, weight=600)
        if note:
            s.text(x + 20 + tw(val, 28, 600) + 10, y + 66, note, size=12, fill=C["ok"])
        s.close()

    y = 420
    s.open("Недавние")
    card(s, X0, y, CW, 200, "Фон карточки")
    s.text(X0 + 20, y + 30, "Недавние", size=16, weight=600)
    s.open("Ссылка «Профиль на Last.fm»")
    s.text(X1 - 42, y + 30, "Профиль на Last.fm", size=13, fill=C["accent"], anchor="end")
    icon(s, "external", X1 - 36, y + 18, C["accent"], size=14)
    s.close()
    rows = [("18:42", "Кино — Группа крови"), ("18:37", "Земфира — Хочешь?"),
            ("18:33", "Сплин — Выхода нет"), ("18:29", "Аквариум — Город золотой")]
    for i, (t, track) in enumerate(rows):
        ry = y + 46 + i * 38
        s.open("Строка")
        s.line(X0 + 20, ry, X1 - 20, ry, C["sep"])
        s.text(X0 + 20, ry + 24, t, size=13, fill=C["text2"])
        s.text(X0 + 80, ry + 24, track, size=14)
        chip(s, X1 - 20, ry + 8, "отправлен", "ok")
        s.close()
    s.close()

    s.close()
    return s


def screen_queue():
    h = 640
    s = Svg(W, h)
    window(s, h)
    nav(s, h, "Очередь", queue_badge="4", online=False)
    s.open("Содержимое")
    s.text(X0, 84, "Очередь", size=TH["title_size"], weight=600, name="Заголовок")
    s.text(X0, 112, "Прослушивания, которые Last.fm ещё не принял. Они хранятся на диске и не потеряются.",
           size=14, fill=C["text2"], name="Пояснение")

    y = 132
    s.open("Предупреждение")
    s.rect(X0, y, CW, 52, C["info_bg"], rx=4, stroke=C["info_border"])
    s.circle(X0 + 24, y + 26, 8, C["info_icon_bg"])
    s.text(X0 + 24, y + 31, "!", size=12, weight=700, fill=C["info_icon_fg"], anchor="middle")
    s.text(X0 + 44, y + 31, "Нет связи с Last.fm.", size=14, weight=600, fill=C["info_fg"])
    s.text(X0 + 44 + tw("Нет связи с Last.fm.", 14, 600) + 6, y + 31,
           "Следующая попытка через 2 мин.", size=14, fill=C["info_fg"])
    button(s, X1 - 12 - 156, y + 10, 156, "Повторить сейчас")
    s.close()

    y = 204
    rows = [("Сегодня, 18:52", "Кино — Кукушка", "3"), ("Сегодня, 18:47", "Сплин — Романс", "3"),
            ("Сегодня, 18:43", "ДДТ — Что такое осень", "4"),
            ("Сегодня, 18:39", "Мумий Тролль — Утекай", "4")]
    s.open("Таблица")
    card(s, X0, y, CW, 44 + len(rows) * 44, "Фон таблицы")
    s.open("Шапка")
    s.text(X0 + 20, y + 27, "Время", size=12, weight=600, fill=C["text2"])
    s.text(X0 + 150, y + 27, "Трек", size=12, weight=600, fill=C["text2"])
    s.text(X1 - 20, y + 27, "Попыток", size=12, weight=600, fill=C["text2"], anchor="end")
    s.close()
    for i, (t, track, n) in enumerate(rows):
        ry = y + 44 + i * 44
        s.open("Строка")
        s.line(X0 + 20, ry, X1 - 20, ry, C["sep"])
        s.text(X0 + 20, ry + 27, t, size=13, fill=C["text2"])
        s.text(X0 + 150, ry + 27, track, size=14)
        s.text(X1 - 20, ry + 27, n, size=14, anchor="end")
        s.close()
    s.close()

    y = y + 44 + len(rows) * 44 + 24
    button(s, X0, y, 172, "Отправить сейчас", "primary")
    button(s, X0 + 184, y, 172, "Очистить очередь", "danger")
    s.text(X0, y + 64, "Записи старше 14 дней Last.fm не принимает — они удаляются автоматически.",
           size=12, fill=C["text2"], name="Примечание")
    s.close()
    return s


def screen_settings():
    h = 940
    s = Svg(W, h)
    window(s, h)
    nav(s, h, "Настройки")
    s.open("Содержимое")
    s.text(X0, 84, "Настройки", size=TH["title_size"], weight=600, name="Заголовок")

    def section(y, label):
        s.text(X0, y, label, size=14, weight=600, name=f"Раздел «{label}»")

    section(124, "Аккаунт")
    y = 136
    s.open("Аккаунт")
    card(s, X0, y, CW, 72, "Фон")
    s.circle(X0 + 40, y + 36, 20, C["accent_soft"])
    s.text(X0 + 40, y + 42, "L", size=16, weight=600, fill=C["accent"], anchor="middle")
    s.text(X0 + 72, y + 32, "listener", size=14, weight=600)
    s.text(X0 + 72, y + 51, "Вход выполнен 12 октября 2026", size=12, fill=C["text2"])
    button(s, X1 - 20 - 96, y + 20, 96, "Выйти")
    s.close()

    section(240, "Правила засчитывания")
    y = 252
    setting_row(s, y, "Засчитывать трек после", "Правило Last.fm: половина трека или 4 минуты")
    s.open("Список «Порог»")
    dx = X1 - 20 - 196
    s.rect(dx + 0.5, y + 16.5, 195, 31, C["surface"], rx=4, stroke=C["border2"])
    s.text(dx + 12, y + 37, "половины трека", size=14)
    icon(s, "chevron", dx + 196 - 28, y + 24, C["text2"])
    s.close()
    y += 68
    setting_row(s, y, "Не учитывать треки короче", "Last.fm не принимает треки короче 30 секунд")
    s.text(X1 - 20, y + 37, "30 секунд", size=14, fill=C["text2"], anchor="end", name="Значение")
    y += 68
    setting_row(s, y, "Интернет-радио", "Засчитывать потоки без длительности через 4 минуты")
    toggle(s, X1 - 20 - 40, y + 22, True)

    section(484, "Плееры")
    s.text(X0 + tw("Плееры", 14, 600) + 10, 484, "— какие учитывать", size=14, fill=C["text2"])
    y = 496
    players = [("S", "Spotify", "играет сейчас", True, "#1DB954"),
               ("C", "Google Chrome", "", True, "#4285F4"),
               ("E", "Microsoft Edge", "", False, "#0C59A4"),
               ("М", "Медиаплеер Windows", "", True, "#C239B3")]
    for i, (letter, name, note, on, color) in enumerate(players):
        ry = y + i * 56
        s.open(f"Плеер «{name}»")
        card(s, X0, ry, CW, 52, "Фон")
        tile, fg = (C["accent"], C["on_accent"]) if TH["square"] else (color, "#FFFFFF")
        s.rect(X0 + 20, ry + 14, 24, 24, tile, rx=6)
        s.text(X0 + 32, ry + 31, letter, size=13, weight=700, fill=fg, anchor="middle")
        s.text(X0 + 56, ry + 31, name, size=14)
        if note:
            s.text(X0 + 56 + tw(name, 14) + 10, ry + 31, note, size=12, weight=600, fill=C["accent"])
        toggle(s, X1 - 20 - 40, ry + 16, on)
        s.close()

    section(744, "Система")
    y = 756
    setting_row(s, y, "Запускать при входе в Windows", "Скробблер работает в фоне, значок — в трее")
    toggle(s, X1 - 20 - 40, y + 22, True)
    y += 68
    setting_row(s, y, "Журнал работы", "%LOCALAPPDATA%\\lastfm-scrobbler\\lastfm-scrobbler.log")
    button(s, X1 - 20 - 156, y + 16, 156, "Открыть папку", icon_kind="folder")
    s.close()
    return s


def screen_login():
    h = 640
    s = Svg(W, h)
    window(s, h)
    s.open("Первый запуск")
    cw, cx = 460, (W - 460) / 2
    card(s, cx, 64, cw, 524, "Карточка")
    app_icon(s, W / 2 - 28, 96, 56)
    s.text(W / 2, 192, "Добро пожаловать", size=24, weight=600, anchor="middle", name="Заголовок")
    s.text(W / 2, 220, "Чтобы отправлять прослушивания, подключите", size=14, fill=C["text2"],
           anchor="middle", name="Пояснение")
    s.text(W / 2, 240, "приложение к своему аккаунту Last.fm.", size=14, fill=C["text2"],
           anchor="middle", name="Пояснение")

    lx, rx = cx + 36, cx + cw - 36
    fw = rx - lx
    s.open("Шаг 1")
    s.text(lx, 288, "1. Ключи приложения", size=14, weight=600)
    s.text(rx - 20, 288, "Получить ключи", size=13, fill=C["accent"], anchor="end")
    icon(s, "external", rx - 14, 276, C["accent"], size=14)
    for i, (label, value, placeholder) in enumerate((("API key", "", "32 символа"),
                                                       ("Shared secret", "••••••••••••••••••••", ""))):
        fy = 304 + i * 64
        s.open(f"Поле «{label}»")
        s.text(lx, fy + 8, label, size=12, fill=C["text2"])
        s.rect(lx + 0.5, fy + 16.5, fw - 1, 31, C["surface"], rx=4, stroke=C["border2"])
        if i == 0:
            s.rect(lx + 1, fy + 46, fw - 2, 2, C["accent"], name="Фокус")
        s.text(lx + 12, fy + 37, value or placeholder, size=14,
               fill=C["text"] if value else C["text3"])
        s.close()
    s.close()

    s.open("Шаг 2")
    s.text(lx, 452, "2. Вход в Last.fm", size=14, weight=600)
    button(s, lx, 468, fw, "Войти через Last.fm", "primary", h=40, icon_kind="key")
    s.text(W / 2, 540, "Откроется браузер. Разрешите доступ и вернитесь сюда —", size=12,
           fill=C["text2"], anchor="middle")
    s.text(W / 2, 558, "пароль от Last.fm программе не нужен.", size=12, fill=C["text2"], anchor="middle")
    s.close()
    s.close()
    return s


def screen_tray(now=("Кино — Кукушка", "Spotify · засчитается через 1:12")):
    global TH, C, FONT
    w, h = 560, 440
    s = Svg(w, h)
    s.defs.append('<linearGradient id="wall" x1="0" y1="0" x2="1" y2="1">'
                  f'<stop offset="0" stop-color="{C["wall1"]}"/><stop offset="1" stop-color="{C["wall2"]}"/>'
                  '</linearGradient>')
    s.defs.append('<filter id="shadow" x="-20%" y="-20%" width="140%" height="140%">'
                  '<feDropShadow dx="0" dy="8" stdDeviation="12" flood-color="#000" flood-opacity="0.18"/>'
                  '</filter>')
    s.rect(0, 0, w, h, "url(#wall)", name="Рабочий стол")

    s.open("Панель задач")
    s.rect(0, h - 48, w, 48, "#F3F3F3", name="Фон")
    s.line(0, h - 48, w, h - 48, "#E0E0E0")
    s.path(f"M{w - 232} {h - 21}L{w - 227} {h - 26}L{w - 222} {h - 21}", stroke=C["text"], sw=1.2)
    s.rect(w - 210, h - 40, 32, 32, "#E4E4E4", rx=4, name="Подсветка значка")
    app_icon(s, w - 202, h - 32, 16, "Значок скробблера")
    s.path(f"M{w - 166} {h - 26}A9 9 0 0 1 {w - 150} {h - 26}M{w - 163} {h - 22}A5 5 0 0 1 {w - 153} {h - 22}",
           stroke=C["text"], sw=1.3)
    s.circle(w - 158, h - 18, 1.3, C["text"])
    s.path(f"M{w - 136} {h - 27}H{w - 132}L{w - 127} {h - 31}V{h - 13}L{w - 132} {h - 17}H{w - 136}Z",
           stroke=C["text"], sw=1.2)
    sysfont = THEMES["default"]["font"]
    s.text(w - 16, h - 28, "18:52", size=12, anchor="end", font=sysfont)
    s.text(w - 16, h - 12, "12.10.2026", size=12, anchor="end", font=sysfont)
    s.close()

    entries = [
        (now[0], "info"),
        (now[1], "info"),
        ("-", None),
        ("Открыть окно", "default"),
        ("Приостановить скробблинг", "item"),
        ("Запускать при входе в Windows", "checked"),
        ("Профиль на Last.fm", "item"),
        ("-", None),
        ("Выход", "item"),
    ]
    item_h, sep_h, pad = 36, 9, 4
    menu_h = pad * 2 + sum(sep_h if e[0] == "-" else item_h for e in entries)
    mw = 320
    mx = w - 24 - mw
    my = h - 48 - 12 - menu_h

    saved = (TH, C, FONT)
    lower = TH["lowercase"]
    use_theme("default")
    if lower:
        entries = [(lbl.lower(), kind) for lbl, kind in entries]

    s.open("Меню в трее")
    s.rect(mx, my, mw, menu_h, C["surface"], rx=8, stroke=C["border"], name="Фон меню",
           extra='filter="url(#shadow)"')
    y = my + pad
    for label, kind in entries:
        if label == "-":
            s.line(mx + 1, y + 4.5, mx + mw - 1, y + 4.5, C["border"])
            y += sep_h
            continue
        s.open(f"Пункт «{label}»")
        if kind == "default":
            s.rect(mx + 4, y, mw - 8, item_h, C["hover"], rx=4, name="Наведение")
        if kind == "checked":
            icon(s, "check", mx + 14, y + 10, C["text"])
        fill = C["text3"] if kind == "info" else C["text"]
        weight = 600 if kind == "default" else 400
        s.text(mx + 40, y + 23, label, size=14, weight=weight, fill=fill)
        s.close()
        y += item_h
    s.close()
    TH, C, FONT = saved
    s.text(16, 28, "Наведите на значок — подсказка покажет трек; правая кнопка — меню.",
           size=TH.get("note_size", 12), fill=C["text2"], name="Пояснение")
    return s


R_LIGHT = {
    "desk": "#FBE3F4",
    "face": "#F6C9EA",
    "face2": "#F0B8E0",
    "face_hi": "#FBDDF2",
    "title": "#C35BB2",
    "title2": "#E59BD8",
    "hi": "#FFF4FC",
    "lo": "#C57FB6",
    "dk": "#5C2E55",
    "text": "#3A1F38",
    "text2": "#7A4A72",
    "field": "#FFF8FD",
    "accent": "#C35BB2",
    "accent_text": "#C35BB2",
    "on_accent": "#FFF4FC",
    "ring": "#5C2E55",
}
R_DARK = {
    "desk": "#141016",
    "face": "#26202B",
    "face2": "#3A2E40",
    "face_hi": "#3D3443",
    "title": "#8E1F33",
    "title2": "#3A1A2A",
    "hi": "#5E5263",
    "lo": "#151017",
    "dk": "#000000",
    "text": "#EEE8F0",
    "text2": "#A99DB0",
    "field": "#0E0B10",
    "accent": "#B52C44",
    "accent_text": "#F2738C",
    "on_accent": "#F7F1F5",
    "ring": "#A99DB0",
}
RETRO = {"retro": R_LIGHT, "retro_dark": R_DARK}
R = dict(R_LIGHT)
PIX = "Tiny5, Liberation Mono, monospace"
P2P = "'Press Start 2P', Liberation Mono, monospace"

TINY_W = {}
for chars, k in (("ilj.:!·' I", 0.25), ("frt,;()/\\", 0.375),
                 ("JLTVXYZГЗТХ?-", 0.5),
                 ("илйъ%ABCDEFGHKNOPQRSUАБВЕЁИЙКЛНОПРСУЦЧЬЭЯ", 0.625),
                 ("mwдмжфшщыюMWДМФШЩЪ—«»", 0.75), ("ЫЮ", 0.875), ("Ж", 1.0)):
    for ch in chars:
        TINY_W[ch] = k


def rtw(s, size=16, font=PIX, spacing=0):
    s = str(s)
    if font == P2P:
        return len(s) * size + spacing * len(s)
    return sum(TINY_W.get(ch, 0.5) for ch in s) * size + spacing * len(s)


BITMAPS = {
    "note": ["...######", "...######", "...#....#", "...#....#", "...#....#",
             ".###..###", "####.####", "####.####", ".##...##."],
    "list": ["##.#######", "..........", "..........", "##.#######", "..........",
             "..........", "##.#######"],
    "gear": ["...###...", ".#.###.#.", "..#####..", "####.####", "###...###",
             "####.####", "..#####..", ".#.###.#.", "...###..."],
    "check": [".......#", "......##", "#....##.", "##..##..", ".####...", "..##...."],
    "close": ["##...##", ".##.##.", "..###..", "..###..", ".##.##.", "##...##"],
    "min": ["......", "......", "......", "......", "......", "######", "######"],
    "max": ["#######", "#######", "#.....#", "#.....#", "#.....#", "#######"],
    "down": ["#######", ".#####.", "..###..", "...#..."],
    "ext": ["..#####", "....###", "...##.#", "..##..#", ".##....", "##....."],
    "warn": ["....#....", "...###...", "...#.#...", "..##.##..", "..##.##..",
             ".###.###.", ".#######.", "####.####", "#########"],
    "key": [".###.....", "#...#....", "#...#####", "#...#.#.#", ".###....."],
    "folder": ["###......", "#..#####.", "#.......#", "#.......#", "#.......#", "#########"],
    "heart": [".##.##.", "#######", "#######", ".#####.", "..###..", "...#..."],
    "heart_big": [".##...##.", "####.####", "#########", "#########", ".#######.",
                  "..#####..", "...###...", "....#...."],
}


def pix(s, kind, x, y, cell, color, name=None):
    if name:
        s.open(name)
    for j, row in enumerate(BITMAPS[kind]):
        i = 0
        while i < len(row):
            if row[i] == "#":
                k = i
                while k < len(row) and row[k] == "#":
                    k += 1
                s.rect(x + i * cell, y + j * cell, (k - i) * cell, cell, color)
                i = k
            else:
                i += 1
    if name:
        s.close()


def pix_size(kind, cell):
    rows = BITMAPS[kind]
    return max(len(r) for r in rows) * cell, len(rows) * cell


def bevel(s, x, y, w, h, tl, br, tl2, br2):
    for rx, ry, rw, rh, c in ((x, y, w, 1, tl), (x, y, 1, h, tl), (x, y + h - 1, w, 1, br),
                              (x + w - 1, y, 1, h, br), (x + 1, y + 1, w - 2, 1, tl2),
                              (x + 1, y + 1, 1, h - 2, tl2), (x + 1, y + h - 2, w - 2, 1, br2),
                              (x + w - 2, y + 1, 1, h - 2, br2)):
        s.rect(rx, ry, rw, rh, c)


def raised(s, x, y, w, h, fill=None, name=None):
    if name:
        s.open(name)
    s.rect(x, y, w, h, fill or R["face"])
    bevel(s, x, y, w, h, R["hi"], R["dk"], R["face_hi"], R["lo"])
    if name:
        s.close()


def sunken(s, x, y, w, h, fill=None, name=None):
    if name:
        s.open(name)
    s.rect(x, y, w, h, fill or R["field"])
    bevel(s, x, y, w, h, R["lo"], R["hi"], R["dk"], R["face_hi"])
    if name:
        s.close()


def r_text(s, x, y, text, size=16, fill=None, anchor="start", name=None, font=PIX, spacing=None):
    s.text(x, y, text, size=size, fill=fill or R["text"], anchor=anchor, name=name,
           font=font, spacing=spacing)


def r_mid(y, h, size=16):
    return y + h / 2 + size * 0.625 / 2


def r_titlebar(s, x, y, w, title, buttons=("close",), h=20):
    gid = f"tb{len(s.defs)}"
    s.defs.append(f'<linearGradient id="{gid}" x1="0" y1="0" x2="1" y2="0">'
                  f'<stop offset="0" stop-color="{R["title"]}"/>'
                  f'<stop offset="1" stop-color="{R["title2"]}"/></linearGradient>')
    s.open("Заголовок")
    s.rect(x, y, w, h, f"url(#{gid})")
    r_text(s, x + 6, r_mid(y, h), title.upper(), fill=R["on_accent"], spacing=1)
    bx = x + w - 2
    names = {"close": "закрыть", "min": "свернуть", "max": "развернуть"}
    for kind in reversed(buttons):
        bw, bh = 18, h - 4
        bx -= bw + 2
        s.open(f"Кнопка окна «{names[kind]}»")
        raised(s, bx, y + 2, bw, bh)
        gw, gh = pix_size(kind, 1)
        pix(s, kind, bx + (bw - gw) // 2, y + 2 + (bh - gh) // 2, 1, R["text"])
        s.close()
    s.close()


def r_dialog(s, x, y, w, h, title, buttons=("close",), name=None):
    s.open(name or f"Окно «{title}»")
    raised(s, x, y, w, h, name="Рамка")
    r_titlebar(s, x + 3, y + 3, w - 6, title, buttons)
    s.close()
    return y + 26


def r_button(s, x, y, w, label, h=28, default=False, pressed=False, icon=None, name=None):
    s.open(name or f"Кнопка «{label}»")
    if default:
        s.rect(x - 1, y - 1, w + 2, h + 2, R["ring"], name="Обводка по умолчанию")
    if pressed:
        sunken(s, x, y, w, h, R["face2"])
    else:
        raised(s, x, y, w, h)
    cx = x + w / 2 + (1 if pressed else 0)
    if icon:
        iw, ih = pix_size(icon, 2)
        ix = round(cx - (iw + 8 + rtw(label)) / 2)
        pix(s, icon, ix, y + (h - ih) // 2, 2, R["text"])
        r_text(s, ix + iw + 8, r_mid(y, h), label)
    else:
        r_text(s, cx, r_mid(y, h), label, anchor="middle")
    s.close()


def r_checkbox(s, x, y, checked, label, note=None, name=None):
    s.open(name or f"Флажок «{label}»")
    sunken(s, x, y, 16, 16)
    if checked:
        pix(s, "check", x + 4, y + 5, 1, R["text"])
    r_text(s, x + 26, r_mid(y, 16), label)
    if note:
        r_text(s, x + 26 + rtw(label) + 12, r_mid(y, 16), note, fill=R["accent_text"])
    s.close()


def r_progress(s, x, y, w, frac, h=22, name="Прогресс"):
    s.open(name)
    sunken(s, x, y, w, h)
    n = int((w - 6) * frac) // 10
    for i in range(n):
        s.rect(x + 3 + i * 10, y + 3, 8, h - 6, R["accent"])
    s.close()


def r_field(s, x, y, w, text, placeholder=False, h=26, name=None):
    s.open(name or "Поле")
    sunken(s, x, y, w, h)
    r_text(s, x + 8, r_mid(y, h), text, fill=R["text2"] if placeholder else R["text"])
    s.close()


def r_dropdown(s, x, y, w, text, h=26, name=None):
    s.open(name or f"Список «{text}»")
    sunken(s, x, y, w, h)
    r_text(s, x + 8, r_mid(y, h), text)
    raised(s, x + w - h + 2, y + 2, h - 4, h - 4)
    gw, gh = pix_size("down", 1)
    pix(s, "down", x + w - h + 2 + (h - 4 - gw) // 2, y + 2 + (h - 4 - gh) // 2, 1, R["text"])
    s.close()


def r_group(s, x, y, w, h, label):
    s.open(f"Группа «{label}»")
    s.rect(x, y, w, h, "none", stroke=R["lo"])
    s.rect(x + 1, y + 1, w, h, "none", stroke=R["hi"])
    s.rect(x + 10, y - 8, rtw(label) + 12, 16, R["face"])
    r_text(s, x + 16, r_mid(y - 8, 16), label, fill=R["accent_text"])
    s.close()


def r_table(s, x, y, w, cols, rows, row_h=40, select=None):
    sunken(s, x, y, w, len(rows) * row_h + 34)
    fixed = sum(c[0] for c in cols if c[0])
    xs, cx = [], x + 3
    for cw_, label, align in cols:
        cw_ = cw_ or (w - 6 - fixed)
        xs.append((cx, cw_, align))
        cx += cw_
    s.open("Шапка")
    for (cx, cw_, align), (_, label, _) in zip(xs, cols):
        raised(s, cx, y + 3, cw_, 24)
        r_text(s, cx + 8, r_mid(y + 3, 24), label)
    s.close()
    for i, row in enumerate(rows):
        ry = y + 30 + i * row_h
        s.open("Строка")
        fg = R["text"]
        if i == select:
            s.rect(x + 3, ry, w - 6, row_h, R["accent"], name="Выделение")
            fg = R["on_accent"]
        elif i < len(rows) - 1:
            s.rect(x + 3, ry + row_h - 1, w - 6, 1, R["face"])
        for (cx, cw_, align), cell in zip(xs, row):
            color = fg
            if isinstance(cell, tuple):
                cell, color = cell[0], (fg if i == select else cell[1])
            if align == "end":
                r_text(s, cx + cw_ - 12, r_mid(ry, row_h), cell, fill=color, anchor="end")
            else:
                r_text(s, cx + 8, r_mid(ry, row_h), cell, fill=color)
        s.close()
    return y + len(rows) * row_h + 34


def r_window(s, w, h, title, status):
    s.open("Окно")
    raised(s, 0, 0, w, h, name="Рамка окна")
    r_titlebar(s, 4, 4, w - 8, title, ("min", "max", "close"), h=22)
    s.rect(4, 30, w - 8, h - 30 - 30, R["desk"], name="Рабочая область")
    s.open("Строка состояния")
    x = 6
    for text, width in status:
        if width is None:
            width = w - 6 - x - 2
        sunken(s, x, h - 27, width, 22, R["face"])
        r_text(s, x + 8, r_mid(h - 27, 22), text)
        x += width + 4
    s.close()
    s.close()


def r_nav(s, selected, badge=None):
    s.open("Навигация")
    items = [("Сейчас", "note"), ("Очередь", "list"), ("Настройки", "gear")]
    for i, (label, ic) in enumerate(items):
        y = 44 + i * 48
        sel = label == selected
        s.open(f"Пункт «{label}»")
        if sel:
            sunken(s, 12, y, 188, 40, R["face2"])
        else:
            raised(s, 12, y, 188, 40)
        iw, ih = pix_size(ic, 2)
        pix(s, ic, 26, y + (40 - ih) // 2, 2, R["accent_text"] if sel else R["text"])
        r_text(s, 58, r_mid(y, 40, 24), label, size=24)
        if badge and label == "Очередь":
            bw = rtw(badge) + 14
            s.rect(190 - bw, y + 11, bw, 18, R["accent"])
            r_text(s, 190 - bw / 2, r_mid(y + 11, 18), badge, fill=R["on_accent"], anchor="middle")
        s.close()
    s.close()


def r_art(s, x, y, size):
    s.open("Обложка")
    sunken(s, x, y, size, size)
    gid = f"art{len(s.defs)}"
    s.defs.append(f'<linearGradient id="{gid}" x1="0" y1="0" x2="1" y2="1">'
                  f'<stop offset="0" stop-color="{R["title2"]}"/>'
                  f'<stop offset="1" stop-color="{R["title"]}"/></linearGradient>')
    s.rect(x + 3, y + 3, size - 6, size - 6, f"url(#{gid})")
    cell = (size - 30) // 9
    gw, gh = pix_size("note", cell)
    pix(s, "note", x + (size - gw) // 2, y + (size - gh) // 2, cell, R["on_accent"])
    s.close()


RW = 960
RX0 = 216
RX1 = RW - 12
RCW = RX1 - RX0
STATUS_OK = [("listener · Last.fm: в сети", 300), ("в очереди: 0", 160), ("сегодня: 23 трека · 1 ч 23 мин", None)]


def retro_now():
    h = 640
    s = Svg(RW, h)
    r_window(s, RW, h, "Скробблер Last.fm", STATUS_OK)
    r_nav(s, "Сейчас")
    s.open("Содержимое")

    y, dh = 44, 268
    top = r_dialog(s, RX0, y, RCW, dh, "Сейчас играет")
    r_art(s, RX0 + 14, top + 10, 210)
    tx = RX0 + 242
    s.open("Плеер")
    label = "Spotify · играет"
    raised(s, tx, top + 10, rtw(label) + 40, 26)
    pix(s, "note", tx + 8, top + 14, 2, R["accent_text"])
    r_text(s, tx + 32, r_mid(top + 10, 26), label)
    s.close()
    s.open("Название")
    r_text(s, tx, top + 76, "ringtone (remix)", size=32)
    r_text(s, tx, top + 98, "[feat. Charli XCX, Rico Nasty, Kero Kero Bonito]")
    s.close()
    r_text(s, tx, top + 128, "100 gecs", size=24, name="Исполнитель")
    r_text(s, tx, top + 150, "1000 gecs and the Tree of Clues · 3:35", fill=R["text2"], name="Альбом")
    r_progress(s, tx, top + 160, RX1 - 14 - tx, 62 / 215)
    r_text(s, tx, top + 202, "прослушано 1:02 из 3:35", fill=R["text2"])
    r_text(s, RX1 - 14, top + 202, "засчитается через 0:45", fill=R["accent_text"], anchor="end")
    r_checkbox(s, tx, top + 214, True, "скробблинг включён")

    y, dh = 326, 258
    top = r_dialog(s, RX0, y, RCW, dh, "Недавние")
    s.open("Список")
    ok = ("отправлен", R["text2"])
    r_table(s, RX0 + 12, top + 8, RCW - 24, [(80, "время", "start"), (None, "трек", "start"),
                                             (130, "статус", "end")],
            [("18:41", "Lana Del Rey — Summertime Sadness", ok), ("18:37", "MGK — bloody valentine", ok),
             ("18:34", "Ninajirachi — iPod Touch", ok), ("18:32", "Charli XCX — 360", ok)],
            row_h=34, select=0)
    s.close()
    by = y + dh - 12 - 28
    r_text(s, RX0 + 12, r_mid(by, 28), "всего в истории: 1 284 прослушивания", fill=R["text2"],
           name="Итог по истории")
    r_button(s, RX1 - 12 - 210, by, 210, "профиль на last.fm", icon="ext")
    r_button(s, RX1 - 12 - 210 - 12 - 120, by, 120, "обновить")
    s.close()
    return s


def retro_queue():
    h = 640
    s = Svg(RW, h)
    r_window(s, RW, h, "Скробблер Last.fm",
             [("listener · Last.fm: нет связи", 300), ("в очереди: 4", 160), ("сегодня: 23 трека · 1 ч 23 мин", None)])
    r_nav(s, "Очередь", badge="4")
    s.open("Содержимое")

    y, dh = 44, 136
    top = r_dialog(s, RX0, y, RCW, dh, "Системное сообщение")
    s.open("Сообщение")
    pix(s, "warn", RX0 + 22, top + 16, 4, R["accent_text"])
    r_text(s, RX0 + 80, top + 34, "Нет связи с Last.fm !", size=16, font=P2P)
    r_text(s, RX0 + 80, top + 60, "Следующая попытка через 2 мин. Прослушивания хранятся на диске.",
           fill=R["text2"])
    by = y + dh - 12 - 28
    r_button(s, RX1 - 14 - 96, by, 96, "ok", default=True)
    r_button(s, RX1 - 14 - 96 - 12 - 180, by, 180, "повторить сейчас")
    s.close()

    y, dh = 194, 312
    top = r_dialog(s, RX0, y, RCW, dh, "Очередь — 4 прослушивания")
    lx, lw = RX0 + 12, RCW - 24
    s.open("Таблица")
    bottom = r_table(s, lx, top + 8, lw, [(170, "время", "start"), (None, "трек", "start"),
                                          (110, "попыток", "end")],
                     [(("сегодня, 19:01", R["text2"]), "Charli XCX — Von dutch", "2"),
                      (("сегодня, 18:59", R["text2"]), "MGK — my ex's best friend", "2"),
                      (("сегодня, 18:55", R["text2"]), "Lana Del Rey — Young and Beautiful", "3"),
                      (("сегодня, 18:51", R["text2"]), "Ninajirachi — All I Am", "4")],
                     select=1)
    s.close()
    by = bottom + 14
    r_button(s, lx, by, 200, "отправить сейчас", default=True)
    r_button(s, lx + 212, by, 170, "удалить запись")
    r_button(s, lx + 394, by, 200, "очистить очередь")
    r_text(s, lx, by + 50, "Записи старше 14 дней Last.fm не принимает — они удаляются сами.",
           fill=R["text2"])
    s.close()
    return s


def retro_settings():
    h = 640
    s = Svg(RW, h)
    r_window(s, RW, h, "Скробблер Last.fm", STATUS_OK)
    r_nav(s, "Настройки")
    s.open("Содержимое")
    top = r_dialog(s, RX0, 44, RCW, 546, "Настройки")
    gx, gw = RX0 + 16, RCW - 32

    y = top + 18
    r_group(s, gx, y, gw, 66, "Аккаунт")
    s.open("Пользователь")
    sunken(s, gx + 14, y + 16, 36, 36, R["face2"])
    pix(s, "heart", gx + 25, y + 28, 2, R["accent_text"])
    r_text(s, gx + 62, y + 34, "listener", size=24)
    r_text(s, gx + 62, y + 54, "вход выполнен 12.10.2026", fill=R["text2"])
    s.close()
    r_button(s, gx + gw - 16 - 110, y + 20, 110, "выйти")

    y += 86
    r_group(s, gx, y, gw, 118, "Правила засчитывания")
    r_text(s, gx + 16, r_mid(y + 16, 26), "засчитывать трек после:")
    r_dropdown(s, gx + 224, y + 16, 220, "половины трека")
    r_text(s, gx + 460, r_mid(y + 16, 26), "или 4 минут", fill=R["text2"])
    r_text(s, gx + 16, y + 66, "не учитывать треки короче 30 секунд — так требует Last.fm",
           fill=R["text2"])
    r_checkbox(s, gx + 16, y + 84, True, "интернет-радио: засчитывать через 4 минуты")

    y += 138
    r_group(s, gx, y, gw, 96, "Плееры, которые учитывать")
    players = [("Spotify", True, "играет"), ("AIMP", True, None), ("Firefox", True, None)]
    for i, (name, on, note) in enumerate(players):
        col, row = i % 2, i // 2
        r_checkbox(s, gx + 16 + col * 330, y + 24 + row * 36, on, name, note)

    y += 116
    r_group(s, gx, y, gw, 104, "Система")
    r_checkbox(s, gx + 16, y + 24, True, "запускать при входе в Windows")
    r_text(s, gx + 16, r_mid(y + 60, 28), "журнал: %LOCALAPPDATA%\\lastfm-scrobbler", fill=R["text2"])
    r_button(s, gx + gw - 16 - 180, y + 60, 180, "открыть папку", icon="folder")

    by = y + 104 + 16
    r_button(s, gx + gw - 3 * 110 - 2 * 10, by, 110, "ok", default=True)
    r_button(s, gx + gw - 2 * 110 - 10, by, 110, "отмена")
    r_button(s, gx + gw - 110, by, 110, "применить")
    s.close()
    return s


def retro_login():
    h = 640
    s = Svg(RW, h)
    r_window(s, RW, h, "Скробблер Last.fm", [("вход не выполнен", 300), ("в очереди: 0", None)])
    s.open("Первый запуск")
    dw, dh = 600, 484
    dx, dy = (RW - dw) // 2, 30 + (h - 60 - dh) // 2
    top = r_dialog(s, dx, dy, dw, dh, "Вход в Last.fm")
    s.open("Приветствие")
    app_icon(s, dx + 24, top + 18, 56)
    s.open("Заголовок")
    hello = "Добро пожаловать"
    r_text(s, dx + 100, top + 40, hello, size=16, font=P2P)
    hw, hh = pix_size("heart_big", 2)
    pix(s, "heart_big", dx + 100 + rtw(hello, 16, P2P) + 14, top + 40 - hh + 2, 2, R["text"],
        name="Сердечко")
    s.close()
    r_text(s, dx + 100, top + 66, "Подключите приложение к аккаунту Last.fm,", fill=R["text2"])
    r_text(s, dx + 100, top + 86, "и прослушивания начнут отправляться сами.", fill=R["text2"])
    s.close()

    gx, gw = dx + 20, dw - 40
    y = top + 128
    r_group(s, gx, y, gw, 160, "1. Ключи приложения")
    r_text(s, gx + 16, r_mid(y + 18, 26), "API key")
    r_field(s, gx + 150, y + 18, gw - 166, "32 символа", placeholder=True, name="Поле «API key»")
    r_text(s, gx + 16, r_mid(y + 58, 26), "Shared secret")
    r_field(s, gx + 150, y + 58, gw - 166, "••••••••••••••••••••", name="Поле «Shared secret»")
    r_button(s, gx + gw - 16 - 200, y + 106, 200, "получить ключи", icon="ext")

    y += 182
    r_group(s, gx, y, gw, 128, "2. Вход")
    r_button(s, gx + 16, y + 22, gw - 32, "войти через last.fm", h=36, default=True, icon="key")
    r_text(s, gx + gw / 2, y + 86, "Откроется браузер: разрешите доступ и вернитесь сюда.",
           fill=R["text2"], anchor="middle")
    r_text(s, gx + gw / 2, y + 108, "Пароль от Last.fm программе не нужен.", fill=R["text2"],
           anchor="middle")
    s.close()
    return s


RETRO_SCREENS = {
    "1-tray-menu": lambda: screen_tray(("100 gecs — ringtone (remix)", "Spotify · засчитается через 0:45")),
    "2-now-playing": retro_now,
    "3-queue": retro_queue,
    "4-settings": retro_settings,
    "5-first-run": retro_login,
}
RETRO_DARK_SCREENS = {k: v for k, v in RETRO_SCREENS.items() if k != "1-tray-menu"}


SCREENS = {
    "1-tray-menu": screen_tray,
    "2-now-playing": screen_now,
    "3-queue": screen_queue,
    "4-settings": screen_settings,
    "5-first-run": screen_login,
}

if __name__ == "__main__":
    for theme, sub, screens in (("default", "", SCREENS), ("brat", "brat", SCREENS),
                                ("retro", "retro", RETRO_SCREENS),
                                ("retro_dark", "retro-dark", RETRO_DARK_SCREENS)):
        use_theme(theme)
        R.clear()
        R.update(RETRO.get(theme, R_LIGHT))
        out = os.path.join(OUT, sub)
        os.makedirs(out, exist_ok=True)
        for name, fn in screens.items():
            svg = fn()
            open(os.path.join(out, f"{name}.svg"), "w", encoding="utf-8").write(svg.render())
            print(f"{theme:8} {name}: {svg.w}x{svg.h}")
