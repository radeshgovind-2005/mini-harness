#!/usr/bin/env python3
"""Records docs/demo.gif by running the real code and drawing its output.

Run from the repo root:  python3 docs/make_demo.py
Needs Python 3 + Pillow and Go. Nothing here is faked: every output line comes
from cmd/worker, cmd/demo or the shell. Only the typing animation is added.
"""
import os, signal, subprocess, sys, tempfile
from PIL import Image, ImageDraw, ImageFont

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
W, H, PAD, LINE = 880, 400, 22, 24
BG, BAR, FG, DIM = (22, 24, 33), (34, 37, 52), (220, 224, 235), (120, 126, 150)
GREEN, RED, YELLOW, BLUE = (110, 220, 140), (255, 110, 120), (240, 200, 110), (120, 170, 255)
FONT = ImageFont.truetype("/System/Library/Fonts/Menlo.ttc", 15)
BOLD = ImageFont.truetype("/System/Library/Fonts/Menlo.ttc", 15, index=1)

def sh(*a, **k):
    return subprocess.run(a, check=True, capture_output=True, text=True, **k).stdout

def color(line):
    low = line.lower()
    if any(w in low for w in ("tampered", "block", "killed")): return RED
    if any(w in low for w in ("intact", "done", "current", "duplicates: 0")): return GREEN
    if "tainted" in low or "paused" in low: return YELLOW
    return FG

# --- 1. run everything for real and collect (command, [output lines]) steps per scene
def scenes():
    build = tempfile.mkdtemp()
    for name in ("worker", "demo"):
        sh("go", "build", "-o", f"{build}/{name}", f"./cmd/{name}", cwd=ROOT)
    work = tempfile.mkdtemp()
    db, fx = f"{work}/log.db", f"{work}/effects.txt"
    cmd = f"worker -db log.db -effects effects.txt"

    p = subprocess.Popen([f"{build}/worker", "-db", db, "-effects", fx, "-pause-at", "4"],
                         stdout=subprocess.PIPE, text=True)
    paused = p.stdout.readline().strip()
    first = p.pid
    os.kill(p.pid, signal.SIGKILL); p.wait()
    after_kill = open(fx).read().splitlines()
    done = sh(f"{build}/worker", "-db", db, "-effects", fx).strip()
    lines = open(fx).read().splitlines()
    dups = len(lines) - len(set(lines))
    crash = [
        (f"{cmd} -pause-at 4", [paused + "   (effect written, checkpoint not yet)"]),
        (f"kill -9 {first}", ["killed mid-step"]),
        ("wc -l < effects.txt", [f"{len(after_kill)}   (5 of 10 steps' effects on disk)"]),
        (cmd + "          # restart", [done]),
        ("wc -l < effects.txt", [f"{len(lines)}   duplicates: {dups}"]),
    ]
    def demo(s): return [l for l in sh(f"{build}/demo", s).splitlines()]
    return [
        ("1/4  Crash recovery: SIGKILL, restart, exactly-once effects", crash),
        ("2/4  Tamper evidence: one edited row is named", [("demo tamper", demo("tamper"))]),
        ("3/4  Prompt injection: blocked, logged, chain still verifies", [("demo inject", demo("inject"))]),
        ("4/4  Bitemporal memory: supersede, never delete", [("demo supersede", demo("supersede"))]),
    ]

# --- 2. draw frames
def frame(title, rows, typing=None):
    im = Image.new("RGB", (W, H), BG); d = ImageDraw.Draw(im)
    d.rectangle([0, 0, W, 38], fill=BAR)
    for i, c in enumerate(((255, 95, 87), (254, 188, 46), (40, 200, 64))):
        d.ellipse([16 + i * 22, 13, 28 + i * 22, 25], fill=c)
    d.text((100, 10), title, font=BOLD, fill=FG)
    rows = rows[-(H - 38 - PAD) // LINE:]
    y = 38 + PAD // 2
    for text, col in rows:
        d.text((PAD, y), text, font=FONT, fill=col); y += LINE
    x = PAD
    if typing is not None:                                   # cursor sits at the end of the typed text
        d.text((PAD, y), typing, font=FONT, fill=FG)
        x = PAD + int(d.textlength(typing, font=FONT))
    d.rectangle([x, y + 2, x + 9, y + 18], fill=FG)
    return im

def build_frames(data):
    frames, durs = [], []
    add = lambda im, ms: (frames.append(im), durs.append(ms))
    for title, steps in data:
        rows = []
        add(frame(title, rows), 500)
        for cmd, out in steps:
            typed = ""
            for ch in range(0, len(cmd) + 1, 3):           # typing effect, 3 chars/frame
                typed = cmd[:ch]
                add(frame(title, rows, "$ " + typed), 40)
            rows.append(("$ " + cmd, BLUE)); add(frame(title, rows), 350)
            for line in out:                                 # output, one line at a time
                rows.append((line, color(line))); add(frame(title, rows), 420)
            add(frame(title, rows), 500)
        durs[-1] = 2200                                      # hold the scene's final state
    return frames, durs

if __name__ == "__main__":
    frames, durs = build_frames(scenes())
    out = os.path.join(ROOT, "docs", "demo.gif")
    pal = [f.convert("P", palette=Image.ADAPTIVE, colors=32) for f in frames]
    pal[0].save(out, save_all=True, append_images=pal[1:], duration=durs, loop=0, optimize=True)
    print(f"wrote {out}: {len(frames)} frames, {os.path.getsize(out)//1024} KB, {sum(durs)/1000:.1f}s")
