"""Aplica zoom suave num vídeo do VHS e gera o .mp4 e o .gif finais.

    python3 zoom.py entrada.mp4 zooms.json saida        # gera saida.mp4 e saida.gif
    python3 zoom.py entrada.mp4 zooms.json saida --preview   # só um quadro por zoom

zooms.json: [[inicio_s, fim_s, centro_x, centro_y, largura], ...] em pixels do
vídeo original (1400x800). A janela mantém a proporção; entre um zoom e outro
próximo a câmera passeia direto, sem voltar para a tela inteira.
"""
import glob
import json
import os
import subprocess
import sys

from PIL import Image

W, H, FPS, RAMP = 1400, 800, 25, 0.6
src, segf, out = sys.argv[1:4]
segs = json.load(open(segf))
full = (W / 2, H / 2, W)

keys = []  # (t, cx, cy, largura)
for i, (t0, t1, cx, cy, w) in enumerate(segs):
    if not keys or keys[-1][0] < t0 - 2 * RAMP:
        keys.append((max(t0 - RAMP, 0), *full))
    keys += [(t0, cx, cy, w), (t1, cx, cy, w)]
    nxt = segs[i + 1][0] if i + 1 < len(segs) else float("inf")
    if nxt - t1 >= 2 * RAMP:
        keys.append((t1 + RAMP, *full))


def state(t):
    if not keys or t <= keys[0][0]:
        return full
    for a, b in zip(keys, keys[1:]):
        if a[0] <= t <= b[0]:
            k = (t - a[0]) / (b[0] - a[0]) if b[0] > a[0] else 1
            k = k * k * (3 - 2 * k)  # smoothstep
            return tuple(a[j] + (b[j] - a[j]) * k for j in (1, 2, 3))
    return keys[-1][1:]


def box(t):
    cx, cy, w = state(t)
    h = w * H / W
    x0 = min(max(cx - w / 2, 0), W - w)
    y0 = min(max(cy - h / 2, 0), H - h)
    return (x0, y0, x0 + w, y0 + h)


def ff(*args):
    subprocess.run(["ffmpeg", "-v", "error", "-y", *args], check=True)


if "--preview" in sys.argv:
    for i, (t0, t1, *_) in enumerate(segs):
        t, png = (t0 + t1) / 2, f"{out}_p{i}.png"
        ff("-ss", str(t), "-i", src, "-frames:v", "1", png)
        Image.open(png).convert("RGB").resize((W, H), Image.LANCZOS, box=box(t)).save(png)
    sys.exit()

frames = out + "_frames"
os.makedirs(frames, exist_ok=True)
for f in glob.glob(frames + "/*"):
    os.remove(f)
ff("-i", src, "-vf", f"fps={FPS}", f"{frames}/raw%05d.png")
for i, f in enumerate(sorted(glob.glob(frames + "/raw*.png"))):
    Image.open(f).convert("RGB").resize((W, H), Image.LANCZOS, box=box(i / FPS)).save(f"{frames}/z{i:05d}.png")
    os.remove(f)
ff("-framerate", str(FPS), "-i", f"{frames}/z%05d.png", "-c:v", "libx264", "-pix_fmt", "yuv420p",
   "-crf", "23", "-movflags", "+faststart", out + ".mp4")
# GIF abaixo de 10 MB: 10 fps, 960 px, paleta de 64 cores
ff("-i", out + ".mp4", "-vf", "fps=10,scale=960:-1:flags=lanczos,split[a][b];"
   "[a]palettegen=max_colors=64:stats_mode=diff[p];[b][p]paletteuse=dither=none:diff_mode=rectangle", out + ".gif")
for f in glob.glob(frames + "/*"):
    os.remove(f)
os.rmdir(frames)
