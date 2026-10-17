#!/usr/bin/env python3
"""Mira una captura de la ventana del emulador y dice si la pantalla de la consola es el CUADRO DE ERROR
(rojo oscuro liso, PanicScreen.Draw) o algo MAS (el menu del juego): cuenta los pixeles rojo-oscuro y los
colores distintos, que es lo que separa una pantalla lisa de una con contenido dibujado.

Uso: mira-227.py <captura.png>
"""
import subprocess
import sys
from collections import Counter

PNG = sys.argv[1]
info = subprocess.run(["ffprobe", "-v", "error", "-select_streams", "v",
                       "-show_entries", "stream=width,height", "-of", "csv=p=0", PNG],
                      capture_output=True, text=True).stdout.strip()
ancho, alto = (int(v) for v in info.split(",")[:2])
datos = subprocess.run(["ffmpeg", "-v", "error", "-i", PNG, "-f", "rawvideo", "-pix_fmt", "rgb24", "-"],
                       capture_output=True).stdout

n = ancho * alto
rojo = 0
colores = Counter()

for i in range(0, len(datos), 3):
    r, g, b = datos[i], datos[i + 1], datos[i + 2]
    if r > 100 and g < 40 and b < 40:
        rojo += 1

    colores[(r >> 4, g >> 4, b >> 4)] += 1

print("CAPTURA %s %dx%d pixeles=%d rojo-oscuro=%d (%.1f%%) colores(4bits)=%d" %
      (PNG, ancho, alto, n, rojo, 100.0 * rojo / n, len(colores)))

for c, k in colores.most_common(5):
    print("   color %s: %.1f%%" % ((c[0] * 16, c[1] * 16, c[2] * 16), 100.0 * k / n))
