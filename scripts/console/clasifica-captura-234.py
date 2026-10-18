#!/usr/bin/env python3
"""Clasifica las capturas de la VENTANA del emulador (DC-002.34) con numeros.

Que mide, por captura:
  - el tamano real de la ventana (del IHDR del PNG), para dejar constancia de que NO es 640x480 (medido:
    682x453 con `window:fullscreen=no` en este Flycast) -- el recorte fijo de DC-002.35 (210,210 640x480)
    no caia sobre el emulador;
  - el color dominante y su proporcion;
  - la fraccion de pixeles OSCUROS (los tres canales < 16) y de CLAROS (> 100 los tres);
  - la clase: `negra` (el motor no dibujo nada), `arranque-sega` (la pantalla de licencia del arranque de
    la consola: gris claro uniforme) o `mixta`.

Por que asi: la corrida del motor NO termina: el motor muere, KOS reinicia y la consola vuelve a arrancar,
asi que la ventana va alternando entre la pantalla de arranque de la consola y la pantalla (negra) del
motor. Un recorte a ojo no dice cual de las dos se esta mirando; estos numeros si.

Uso:  clasifica-captura-234.py <png> [<png> ...]
"""
import subprocess
import sys


def pixeles(png):
    """RGB24 crudo de la imagen entera, con ffmpeg (no hay PIL en este host)."""
    cmd = ["ffmpeg", "-v", "error", "-i", png, "-f", "rawvideo", "-pix_fmt", "rgb24", "-"]
    return subprocess.run(cmd, capture_output=True, check=True).stdout


def dimensiones(png):
    """Ancho y alto, leidos del IHDR del PNG (no hace falta PIL ni identify)."""
    with open(png, "rb") as fh:
        cab = fh.read(33)
    return int.from_bytes(cab[16:20], "big"), int.from_bytes(cab[20:24], "big")


def clasifica(png):
    buf = pixeles(png)
    total = len(buf) // 3
    ancho, alto = dimensiones(png)
    cuenta = {}
    oscuros = claros = 0
    for i in range(0, len(buf), 3):
        r, g, b = buf[i], buf[i + 1], buf[i + 2]
        clave = (r >> 4, g >> 4, b >> 4)
        cuenta[clave] = cuenta.get(clave, 0) + 1
        if r < 16 and g < 16 and b < 16:
            oscuros += 1
        if r > 100 and g > 100 and b > 100:
            claros += 1

    (rc, gc, bc), n = max(cuenta.items(), key=lambda kv: kv[1])
    pct_dom = 100.0 * n / total
    pct_osc = 100.0 * oscuros / total
    pct_cla = 100.0 * claros / total

    if pct_osc >= 95.0:
        clase = "negra (el motor no dibujo nada)"
    elif pct_dom >= 60.0 and (rc + gc + bc) / 3.0 >= 6:
        clase = "arranque-sega (gris claro uniforme)"
    elif pct_osc >= 60.0:
        clase = "negra-mayormente"
    else:
        clase = "mixta"

    print("%-40s %4dx%-4d pixeles=%-7d dom=(%2d,%2d,%2d) %5.1f%% oscuros=%5.1f%% claros=%5.1f%%  -> %s" %
          (png, ancho, alto, total, rc, gc, bc, pct_dom, pct_osc, pct_cla, clase))
    return clase


def main():
    if len(sys.argv) < 2:
        print(__doc__)
        return 2
    for png in sys.argv[1:]:
        clasifica(png)
    return 0


if __name__ == "__main__":
    sys.exit(main())
