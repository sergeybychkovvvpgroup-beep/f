#!/usr/bin/env python3
from pathlib import Path
from PIL import Image, ImageDraw, ImageFont

OUT = Path(__file__).resolve().parent
BG = "#242833"
FG = "#d8def2"
MUTED = "#a9afc7"
PINK = "#f38ba8"
MATCH = "#f5c2e7"
FONT_PATH = "/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf"
BOLD_PATH = "/usr/share/fonts/truetype/dejavu/DejaVuSansMono-Bold.ttf"
FONT = ImageFont.truetype(FONT_PATH, 18)
BOLD = ImageFont.truetype(BOLD_PATH, 18)
TITLE = ImageFont.truetype(BOLD_PATH, 19)
LINE = 24


def draw_segments(draw, x, y, segments):
    for text, color, font in segments:
        draw.text((x, y), text, fill=color, font=font)
        x += draw.textlength(text, font=font)


def save(name, size, title, header, query, rows, selected=0, bottom=False):
    image = Image.new("RGB", size, BG)
    draw = ImageDraw.Draw(image)
    draw.text((24, 15), title, fill=PINK, font=TITLE)
    x = 46
    top = 62
    footer = "Ctrl+E edit  •  Tab details  •  F1–F5 categories"
    if bottom:
        draw.text((x, top), header, fill=FG, font=FONT)
        y = top + LINE
        for i, (name_text, command) in enumerate(rows):
            marker = "│" if i == selected else " "
            draw.text((x, y), marker, fill=PINK if i == selected else FG, font=FONT)
            draw_segments(draw, x + 22, y, [(name_text, MATCH if i == selected else FG, BOLD if i == selected else FONT), (" · ", MUTED, FONT), (command, MUTED, FONT)])
            y += LINE
        draw.text((x, y + 2), "> " + query, fill=FG, font=FONT)
        draw.text((x, y + LINE + 2), footer, fill=MUTED, font=FONT)
    else:
        draw.text((x, top), header, fill=FG, font=FONT)
        draw.text((x, top + LINE), "> " + query, fill=FG, font=FONT)
        y = top + 2 * LINE
        for i, (name_text, command) in enumerate(rows):
            marker = "│" if i == selected else " "
            draw.text((x, y), marker, fill=PINK if i == selected else FG, font=FONT)
            draw_segments(draw, x + 22, y, [(name_text, MATCH if i == selected else FG, BOLD if i == selected else FONT), (" · ", MUTED, FONT), (command, MUTED, FONT)])
            y += LINE
        draw.text((x, y + 2), footer, fill=MUTED, font=FONT)
    image.save(OUT / name, optimize=True)


commands = [
    ("edge-status", "ssh -o RemoteCommand='show status' ops@192.0.2.10"),
    ("voice-status", "ssh -o ProxyJump=jump.example -o RemoteCommand='show channels' ops@192.0.2.20"),
    ("web [tunnel]", "ssh -o 'LocalForward=8443 198.51.100.20:443' ops@192.0.2.30"),
    ("site-router [overlay]", "ssh admin@198.51.100.40"),
]

save("compact-top.png", (1060, 264), "COMPACT · TOP", "COMMANDS   4 / 117", "status", commands, selected=0)
save("compact-bottom.png", (1060, 264), "COMPACT · BOTTOM", "COMMANDS   4 / 117", "status", commands, selected=0, bottom=True)
save("compact-top-address.png", (1060, 184), "COMPACT · COMMANDS", "SSH   1 / 117", "data", [("data-node", "ssh -o ProxyJump=access.example -o ServerAliveInterval=30…operator@192.0.2.50")], selected=0)
save("full-screen.png", (1060, 624), "FULL-SCREEN", "SSH   6 / 117", "", [
    ("server-console", "ssh admin@192.0.2.60"),
    ("web [tunnel]", "ssh -o 'LocalForward=8443 198.51.100.20:443' ops@192.0.2.30"),
    *commands[:2],
    ("dmz-router [jump]", "ssh -o ProxyJump=access.example admin@198.51.100.1"),
    ("site-router [overlay]", "ssh admin@198.51.100.40"),
], selected=0)
