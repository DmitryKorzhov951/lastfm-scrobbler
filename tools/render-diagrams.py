#!/usr/bin/env python3
import glob, os, re, struct, subprocess, sys, tempfile, zlib

HERE = os.path.dirname(os.path.abspath(__file__))
IMG = os.path.join(os.path.dirname(HERE), "docs", "img")
SP = tempfile.mkdtemp(prefix="diagrams-")


def find_chrome():
    if os.environ.get("CHROME"):
        return os.environ["CHROME"]
    patterns = [
        os.path.join(os.environ.get("PLAYWRIGHT_BROWSERS_PATH", "/opt/pw-browsers"),
                     "chromium-*/chrome-linux/chrome"),
        "/usr/bin/chromium", "/usr/bin/chromium-browser",
        "/usr/bin/google-chrome", "/usr/bin/google-chrome-stable",
    ]
    for p in patterns:
        found = sorted(glob.glob(p))
        if found:
            return found[-1]
    sys.exit("Chromium не найден. Укажите путь: CHROME=/usr/bin/chromium python3 " + __file__)


CHROME = find_chrome()
SLACK = 96
SCALE = 2


def decode(path):
    d = open(path, "rb").read()
    pos, idat, w, h, ct = 8, b"", None, None, None
    while pos < len(d):
        ln = struct.unpack(">I", d[pos:pos + 4])[0]
        typ = d[pos + 4:pos + 8]
        data = d[pos + 8:pos + 8 + ln]
        pos += 12 + ln
        if typ == b"IHDR":
            w, h, _, ct = struct.unpack(">IIBB", data[:10])
        elif typ == b"IDAT":
            idat += data
        elif typ == b"IEND":
            break
    bpp = {0: 1, 2: 3, 4: 2, 6: 4}[ct]
    raw, stride = zlib.decompress(idat), w * bpp
    out, prev, i = [], bytearray(stride), 0
    for _ in range(h):
        f = raw[i]; i += 1
        line = bytearray(raw[i:i + stride]); i += stride
        if f:
            for x in range(stride):
                a = line[x - bpp] if x >= bpp else 0
                b = prev[x]
                c = prev[x - bpp] if x >= bpp else 0
                if f == 1: line[x] = (line[x] + a) & 0xFF
                elif f == 2: line[x] = (line[x] + b) & 0xFF
                elif f == 3: line[x] = (line[x] + (a + b) // 2) & 0xFF
                else:
                    p = a + b - c
                    pa, pb, pc = abs(p - a), abs(p - b), abs(p - c)
                    line[x] = (line[x] + (a if pa <= pb and pa <= pc else b if pb <= pc else c)) & 0xFF
        out.append(bytes(line)); prev = line
    return w, h, bpp, out


def encode(path, w, h, bpp, rows):
    ct = {1: 0, 2: 4, 3: 2, 4: 6}[bpp]
    raw = b"".join(b"\x00" + r for r in rows[:h])

    def chunk(tag, data):
        return (struct.pack(">I", len(data)) + tag + data
                + struct.pack(">I", zlib.crc32(tag + data) & 0xFFFFFFFF))

    png = (b"\x89PNG\r\n\x1a\n"
           + chunk(b"IHDR", struct.pack(">IIBBBBB", w, h, 8, ct, 0, 0, 0))
           + chunk(b"IDAT", zlib.compress(raw, 9))
           + chunk(b"IEND", b""))
    open(path, "wb").write(png)


def render(name):
    svg = open(f"{IMG}/{name}.svg", encoding="utf-8").read()
    w = int(re.search(r'width="(\d+)"', svg).group(1))
    h = int(re.search(r'height="(\d+)"', svg).group(1))

    html = (f'<!doctype html><meta charset="utf-8">'
            f'<style>html,body{{margin:0;padding:0;background:#fff}}'
            f'img{{display:block}}</style>'
            f'<img src="{name}.svg" width="{w}" height="{h}">')
    open(f"{SP}/{name}.html", "w", encoding="utf-8").write(html)
    open(f"{SP}/{name}.svg", "w", encoding="utf-8").write(svg)

    tmp = f"{SP}/{name}-raw.png"
    subprocess.run([CHROME, "--headless", "--disable-gpu", "--no-sandbox",
                    "--hide-scrollbars", "--default-background-color=ffffffff",
                    f"--force-device-scale-factor={SCALE}",
                    f"--window-size={w},{h + SLACK}",
                    f"--screenshot={tmp}", f"file://{SP}/{name}.html"],
                   check=True, capture_output=True)

    iw, ih, bpp, rows = decode(tmp)
    target = h * SCALE
    encode(f"{IMG}/{name}.png", iw, min(target, ih), bpp, rows)

    last = max((y for y, r in enumerate(rows[:target])
                if any(r[x] < 240 for x in range(0, len(r), bpp * 5))), default=0)
    print(f"{name}: {iw}x{target}, содержимое до {last} строки "
          f"({'вмещается' if last < target - 4 else 'ОБРЕЗАНО'})")


if __name__ == "__main__":
    args = sys.argv[1:]
    if args and os.path.isdir(args[0]):
        IMG = os.path.abspath(args[0])
        args = args[1:]
    if not args:
        args = sorted(f[:-4] for f in os.listdir(IMG) if f.endswith(".svg"))
    for n in args:
        render(n)
