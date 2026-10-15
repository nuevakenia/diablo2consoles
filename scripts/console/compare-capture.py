#!/usr/bin/env python3
"""Compara un asset dibujado por la Dreamcast contra la referencia que genera el horneador.

La gracia: no se compara "a ojo". El horneador (console/dc/bake) deja un PNG con el arte tal
como tiene que verse, y aca se recorta la captura de pantalla del emulador en la posicion
exacta donde el POC dibujo el asset y se comparan pixel a pixel.

Uso:
  scripts/console/compare-capture.py --captura captura.png --referencia gen/d2logo.png \
      --pantalla 2752 456 --pos 48 64

  --pantalla X Y   donde empieza la pantalla de la Dreamcast (640x480) dentro de la captura
  --pos X Y        donde el POC dibujo el asset, en coordenadas de la pantalla de la consola
  --tol N          tolerancia por canal (por defecto 8: ARGB1555 pierde 3 bits por canal)
  --fondo R G B    color de fondo del POC: los pixeles transparentes del asset tienen que
                   mostrarlo a traves

No usa PIL: decodifica con ffmpeg (siempre presente en este host).
"""
import argparse
import subprocess
import sys


def raw(png, vf=None, fmt="rgb24"):
    cmd = ["ffmpeg", "-v", "error", "-i", png]
    if vf:
        cmd += ["-vf", vf]
    cmd += ["-f", "rawvideo", "-pix_fmt", fmt, "-"]
    out = subprocess.run(cmd, capture_output=True)
    if out.returncode != 0:
        sys.exit(f"ffmpeg fallo con {png}: {out.stderr.decode()[:300]}")
    return out.stdout


def size(png):
    out = subprocess.run(
        ["ffprobe", "-v", "error", "-select_streams", "v",
         "-show_entries", "stream=width,height", "-of", "csv=p=0", png],
        capture_output=True, text=True).stdout.strip()
    w, h = out.split(",")
    return int(w), int(h)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--captura", required=True, help="PNG de la captura (pantalla completa)")
    ap.add_argument("--referencia", required=True, help="PNG de referencia que deja el horneador")
    ap.add_argument("--pantalla", nargs=2, type=int, required=True,
                    metavar=("X", "Y"), help="origen de la pantalla de la consola en la captura")
    ap.add_argument("--pos", nargs=2, type=int, required=True,
                    metavar=("X", "Y"), help="donde se dibujo el asset en la pantalla")
    ap.add_argument("--tol", type=int, default=8, help="tolerancia por canal (ARGB1555 = 8)")
    ap.add_argument("--fondo", nargs=3, type=int, default=None, metavar=("R", "G", "B"),
                    help="color de fondo del POC, para revisar la transparencia")
    args = ap.parse_args()

    W, H = size(args.referencia)
    sx, sy = args.pantalla
    px, py = args.pos

    host = raw(args.referencia, fmt="rgba")
    dc = raw(args.captura, vf=f"crop={W}:{H}:{sx + px}:{sy + py}")

    arte_total = arte_ok = dif_max = 0
    trans_total = trans_ok = 0
    for y in range(H):
        for x in range(W):
            i = (y * W + x) * 4
            j = (y * W + x) * 3
            hr, hg, hb, ha = host[i], host[i + 1], host[i + 2], host[i + 3]

            if ha == 0:  # transparente en el arte: tiene que verse el fondo
                if args.fondo is None:
                    continue
                trans_total += 1
                fr, fg, fb = args.fondo
                if (abs(dc[j] - fr) < 12 and abs(dc[j + 1] - fg) < 12 and abs(dc[j + 2] - fb) < 12):
                    trans_ok += 1
                continue

            arte_total += 1
            d = max(abs(hr - dc[j]), abs(hg - dc[j + 1]), abs(hb - dc[j + 2]))
            dif_max = max(dif_max, d)
            if d <= args.tol:
                arte_ok += 1

    print(f"referencia: {args.referencia}  {W}x{H}")
    print(f"captura:    {args.captura}  (pantalla en {sx},{sy}; asset dibujado en {px},{py})")
    if arte_total:
        print(f"  pixeles de arte:      {arte_ok}/{arte_total} "
              f"({100.0 * arte_ok / arte_total:.1f}%) iguales (tolerancia +/-{args.tol})")
        print(f"  diferencia maxima:    {dif_max} por canal")
    if trans_total:
        print(f"  pixeles transparentes: {trans_ok}/{trans_total} "
              f"({100.0 * trans_ok / trans_total:.1f}%) con el fondo visible")

    ok = arte_total > 0 and arte_ok == arte_total and (trans_total == 0 or trans_ok == trans_total)
    print("RESULTADO:", "OK" if ok else "NO COINCIDE")
    sys.exit(0 if ok else 1)


if __name__ == "__main__":
    main()
