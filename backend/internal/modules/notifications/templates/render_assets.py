#!/usr/bin/env python3
"""Перерисовывает картинки писем: banner.png и icon-*.png.

Запуск из корня репозитория: python3 backend/internal/modules/notifications/templates/render_assets.py
Нужны Google Chrome (растрирует SVG, грузит Tenor Sans с Google Fonts) и Pillow.
Иконки и цвета статусов те же, что в профиле сайта (frontend/src/pages/public/Profile.tsx).
"""
import json
import math
import pathlib
import subprocess
import tempfile

from PIL import Image

HERE = pathlib.Path(__file__).resolve().parent
REPO = HERE.parents[4]
CHROME = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
SCALE = 2

SEA = "#010014"
CARD = "#0E1629"
GOLD = "#C7AE7F"
GOLD_LINE = "#7D7151"

# Тона и глифы как у .pf-status и ICON в профиле. lock - только для письма с кодом входа.
TONES = {
    "wait": ("#8194AE", "rgba(129,148,174,0.45)", "rgba(129,148,174,0.12)"),
    "work": ("#D8C9A2", GOLD_LINE, "rgba(199,174,127,0.12)"),
    "ok": ("#8FC7A0", "rgba(143,199,160,0.45)", "rgba(143,199,160,0.12)"),
    "bad": ("#C46F62", "rgba(196,111,98,0.45)", "rgba(196,111,98,0.12)"),
}
ICONS = {
    "lock": ("work", "M6 11h12v9H6z M8.5 11V8a3.5 3.5 0 0 1 7 0v3 M12 14.5v2"),
    "processing": ("wait", "M12 3a9 9 0 1 0 9 9M12 7v5l3 2"),
    "new": ("work", "M3 7l9 6 9-6M3 7h18v10H3z"),
    "review": ("work", "M2 12s3.5-6 10-6 10 6 10 6-3.5 6-10 6-10-6-10-6zM12 9.5a2.5 2.5 0 1 0 0 5 2.5 2.5 0 0 0 0-5z"),
    "published": ("ok", "M12 3a9 9 0 1 0 9 9 9 9 0 0 0-9-9zM8 12.5l2.5 2.5L16 9.5"),
    "rejected": ("bad", "M12 3a9 9 0 1 0 9 9 9 9 0 0 0-9-9zM9 9l6 6M15 9l-6 6"),
}
ICON_SIZE = 56
BANNER_W, BANNER_H = 600, 210
FONT = '<link href="https://fonts.googleapis.com/css2?family=Tenor+Sans&display=block" rel="stylesheet">'


def shoot(html, width, height, out):
    with tempfile.TemporaryDirectory() as work:
        page = pathlib.Path(work) / "page.html"
        page.write_text(html, encoding="utf-8")
        shot = pathlib.Path(work) / "shot.png"
        subprocess.run([
            CHROME, "--headless=new", "--hide-scrollbars", "--disable-gpu",
            f"--force-device-scale-factor={SCALE}", f"--window-size={width},{height}",
            "--virtual-time-budget=6000", f"--screenshot={shot}", page.as_uri(),
        ], check=True, capture_output=True)
        image = Image.open(shot).convert("RGB").crop((0, 0, width * SCALE, height * SCALE))
    # Палитра вместо truecolor: баннер уходит вложением в каждое письмо.
    image.quantize(colors=96, method=Image.Quantize.MEDIANCUT, dither=Image.Dither.NONE).save(out, optimize=True)
    print(f"{out.name}: {out.stat().st_size // 1024} КБ")


def albers(lon, lat):
    # Равновеликая коническая Альберса, как на школьных картах России.
    phi1, phi2, phi0, lon0 = map(math.radians, (52, 64, 45, 100))
    n = (math.sin(phi1) + math.sin(phi2)) / 2
    c = math.cos(phi1) ** 2 + 2 * n * math.sin(phi1)
    rho0 = math.sqrt(c - 2 * n * math.sin(phi0)) / n
    lon = lon + 360 if lon < 0 else lon
    theta = n * (math.radians(lon) - lon0)
    rho = math.sqrt(c - 2 * n * math.sin(math.radians(lat))) / n
    return rho * math.sin(theta), rho0 - rho * math.cos(theta)


def russia_path(box_x, box_y, box_w, box_h):
    features = json.loads((REPO / "frontend/public/russia-regions.geojson").read_text(encoding="utf-8"))["features"]
    polygons = []
    for feature in features:
        geometry = feature["geometry"]
        polygons += [geometry["coordinates"]] if geometry["type"] == "Polygon" else geometry["coordinates"]
    rings = [[albers(lon, lat) for lon, lat in ring] for polygon in polygons for ring in polygon]
    xs = [x for ring in rings for x, _ in ring]
    ys = [y for ring in rings for _, y in ring]
    scale = min(box_w / (max(xs) - min(xs)), box_h / (max(ys) - min(ys)))
    left = box_x + (box_w - (max(xs) - min(xs)) * scale) / 2
    top = box_y + (box_h - (max(ys) - min(ys)) * scale) / 2
    parts = []
    for ring in rings:
        points, last = [], None
        for x, y in ring:
            point = (left + (x - min(xs)) * scale, top + (max(ys) - y) * scale)
            if last is None or abs(point[0] - last[0]) + abs(point[1] - last[1]) > 0.6:
                points.append(point)
                last = point
        if len(points) > 2:
            parts.append("M" + "L".join(f"{x:.1f} {y:.1f}" for x, y in points) + "Z")
    return "".join(parts)


def banner():
    logo = (REPO / "frontend/public/logo.svg").read_text(encoding="utf-8")
    logo = logo.replace("<svg ", '<svg class="logo" ', 1)
    html = f"""<!doctype html><html><head><meta charset="utf-8">{FONT}<style>
html,body{{margin:0;background:{SEA};}}
.b{{position:relative;width:{BANNER_W}px;height:{BANNER_H}px;overflow:hidden;
  background:radial-gradient(70% 90% at 72% 55%, rgba(199,174,127,0.10), transparent 70%), {SEA};}}
.map{{position:absolute;inset:0;-webkit-mask-image:linear-gradient(90deg,transparent 40%,#000 66%);mask-image:linear-gradient(90deg,transparent 40%,#000 66%);}}
.logo{{position:absolute;left:34px;top:38px;height:134px;width:auto;}}
.t{{position:absolute;left:124px;top:66px;font:400 30px/1.12 'Tenor Sans',sans-serif;color:#fff;letter-spacing:0.2px;}}
.t span{{display:block;color:{GOLD};font-size:30px;}}
.rule{{position:absolute;left:124px;top:148px;width:52px;height:1px;background:{GOLD_LINE};}}
</style></head><body><div class="b">
<svg class="map" viewBox="0 0 {BANNER_W} {BANNER_H}" width="{BANNER_W}" height="{BANNER_H}">
<path d="{russia_path(250, 12, 340, 186)}" fill="rgba(199,174,127,0.05)" stroke="{GOLD_LINE}" stroke-width="0.5" stroke-linejoin="round" opacity="0.9"/>
</svg>{logo}<div class="t">Россия -<span>страна Героев</span></div><div class="rule"></div>
</div></body></html>"""
    shoot(html, BANNER_W, BANNER_H, HERE / "banner.png")


def icon(name):
    tone, glyph = ICONS[name]
    color, line, wash = TONES[tone]
    html = f"""<!doctype html><html><head><meta charset="utf-8"><style>
html,body{{margin:0;background:{CARD};}}
.i{{width:{ICON_SIZE}px;height:{ICON_SIZE}px;box-sizing:border-box;border-radius:50%;
  border:1px solid {line};background:{wash};display:flex;align-items:center;justify-content:center;}}
</style></head><body><div class="i"><svg viewBox="0 0 24 24" width="26" height="26" fill="none" stroke="{color}"
stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="{glyph}"/></svg></div></body></html>"""
    shoot(html, ICON_SIZE, ICON_SIZE, HERE / f"icon-{name}.png")


if __name__ == "__main__":
    banner()
    for name in ICONS:
        icon(name)
