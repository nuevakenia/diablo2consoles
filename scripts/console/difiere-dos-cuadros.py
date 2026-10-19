#!/usr/bin/env python3
"""difiere-dos-cuadros.py <a.png> <b.png> [--caja x0,y0,x1,y1] [--umbral N]

Cuenta los pixeles que cambian entre dos cuadros y la caja donde cambian. Sirve para la animacion con el
personaje QUIETO: si las dos capturas son del mismo punto del mundo y la unica diferencia es el dibujo del
personaje, la caja del cambio es la caja del personaje y el numero es "cuantos pixeles se movieron".
"""
import sys

import numpy as np
from PIL import Image


def cargar(p):
    return np.asarray(Image.open(p).convert("RGB")).astype(np.int16)


def main():
    a_p, b_p = sys.argv[1], sys.argv[2]
    caja = None
    umbral = 0

    for i, arg in enumerate(sys.argv):
        if arg == "--caja" and i + 1 < len(sys.argv):
            caja = [int(v) for v in sys.argv[i + 1].split(",")]
        if arg == "--umbral" and i + 1 < len(sys.argv):
            umbral = int(sys.argv[i + 1])

    a, b = cargar(a_p), cargar(b_p)

    if a.shape != b.shape:
        print("tamanos distintos:", a.shape, b.shape)
        return 1

    if caja:
        x0, y0, x1, y1 = caja
        a, b = a[y0:y1, x0:x1], b[y0:y1, x0:x1]

    d = np.abs(a - b).sum(axis=2)
    m = d > umbral

    print("cuadros %s vs %s  tamano %dx%d" % (a_p, b_p, a.shape[1], a.shape[0]))
    print("pixeles que cambian (umbral %d) = %d de %d (%.2f%%)" % (
        umbral, int(m.sum()), m.size, m.sum() * 100.0 / m.size))

    if m.any():
        ys, xs = np.where(m)
        print("caja del cambio = (%d,%d)-(%d,%d) = %dx%d" % (
            xs.min(), ys.min(), xs.max(), ys.max(), xs.max() - xs.min() + 1, ys.max() - ys.min() + 1))
        print("diferencia maxima por pixel = %d" % int(d.max()))
    else:
        print("caja del cambio = (ninguna)")

    return 0


sys.exit(main())
