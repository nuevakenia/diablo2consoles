#!/usr/bin/env python3
"""Cuenta los pixeles de la CAPTURA del emulador, para el criterio 2 de DC-002.35.

Que mide (con numeros, no con impresion):
  - cuantos pixeles de la captura NO son el fondo (negro) en la zona de la ventana del emulador;
  - el color dominante de esos pixeles.

Por que se hace asi: en KDE/Wayland `spectacle -f` fotografia la PANTALLA ENTERA, asi que la captura
trae el escritorio alrededor. La ventana del emulador se ubica buscando el bloque grande de pixeles
mas oscuro... o, mas simple y medible: se pasa el recorte (x,y,w,h) y se cuentan los pixeles ahi.
Uso:  python3 cuenta-captura-235.py <png> [x y w h]
"""
import subprocess
import sys


def leer_pixeles(png, x, y, w, h):
    """Decodifica el recorte a RGB24 crudo con ffmpeg (no hay PIL en este host)."""
    cmd = ["ffmpeg", "-v", "error", "-i", png, "-vf", "crop=%d:%d:%d:%d" % (w, h, x, y),
           "-f", "rawvideo", "-pix_fmt", "rgb24", "-"]
    return subprocess.run(cmd, capture_output=True, check=True).stdout


def main():
    png = sys.argv[1]
    if len(sys.argv) >= 6:
        x, y, w, h = (int(v) for v in sys.argv[2:6])
    else:
        # la ventana del emulador que deja run-flycast.sh / flycast-adelante.sh: 210,210 y 640x480
        x, y, w, h = 210, 210, 640, 480

    buf = leer_pixeles(png, x, y, w, h)
    total = w * h

    noNegro = 0
    cuenta = {}
    for i in range(0, len(buf), 3):
        r, g, b = buf[i], buf[i + 1], buf[i + 2]
        if r > 8 or g > 8 or b > 8:
            noNegro += 1
            clave = (r >> 4, g >> 4, b >> 4)
            cuenta[clave] = cuenta.get(clave, 0) + 1

    print("CAPTURA %s recorte=%d,%d %dx%d pixeles=%d noFondo=%d (%.2f%%)" %
          (png, x, y, w, h, total, noNegro, 100.0 * noNegro / total))

    dominante = sorted(cuenta.items(), key=lambda kv: -kv[1])[:5]
    for (r, g, b), n in dominante:
        print("   color dominante (rango 16): %2d,%2d,%2d  pixeles=%d" % (r, g, b, n))

    return 0


if __name__ == "__main__":
    sys.exit(main())
