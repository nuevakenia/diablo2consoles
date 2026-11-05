#!/usr/bin/env python3
"""DC-002.159o -- compara DOS capturas del emulador pixel a pixel, dentro del area de video.

POR QUE EXISTE. La tarjeta pide comparar el cuadro del pueblo del emulador contra el binario de referencia
(pixeles, no impresiones). Las capturas de `run-flycast.sh` son fotos de la pantalla entera (spectacle), asi que
primero se recorta el area de video del emulador y despues se cuentan los pixeles que difieren.

Uso:
    comparar-cuadros-159o.py <a.png> <b.png> [--box X0,X1,Y0,Y1] [--excluir X0,Y0,X1,Y1]

  --box       el recorte del area de video (por defecto 0,627,28,512: el mismo de `pixeles-emulador.py`)
  --excluir   una caja que NO se compara (el recuadro del heroe, que camina y no es parte del mapa)

Devuelve: pixeles comparados, pixeles distintos, la caja de las diferencias y el veredicto.
"""
import sys

import numpy as np
from PIL import Image


def caja(texto):
    return [int(x) for x in texto.split(",")]


def main():
    args = sys.argv[1:]
    if len(args) < 2:
        print(__doc__)
        return 1

    a, b = args[0], args[1]
    x0, x1, y0, y1 = 0, 627, 28, 512
    excluir = None

    i = 2
    while i < len(args):
        if args[i] == "--box":
            x0, x1, y0, y1 = caja(args[i + 1])
            i += 2
        elif args[i] == "--excluir":
            excluir = caja(args[i + 1])
            i += 2
        else:
            print("argumento desconocido:", args[i])
            return 2

    ia = np.asarray(Image.open(a).convert("RGB"))[y0:y1, x0:x1].astype(np.int16)
    ib = np.asarray(Image.open(b).convert("RGB"))[y0:y1, x0:x1].astype(np.int16)

    if ia.shape != ib.shape:
        print("FALLA: las dos capturas no tienen el mismo tamano:", ia.shape, ib.shape)
        return 3

    dif = np.abs(ia - ib).max(axis=2) > 0
    total = int(dif.size)
    distintos = int(dif.sum())

    print("archivo A: %s" % a)
    print("archivo B: %s" % b)
    print("area comparada: x %d..%d  y %d..%d  (%d pixeles)" % (x0, x1, y0, y1, total))

    fuera = 0

    if excluir is not None:
        ex0, ey0, ex1, ey1 = excluir
        m = dif.copy()
        m[max(0, ey0 - y0):max(0, ey1 - y0), max(0, ex0 - x0):max(0, ex1 - x0)] = False
        fuera = int(m.sum())
        print("excluida la caja x %d..%d y %d..%d: pixeles distintos FUERA de ella = %d" % (ex0, ex1, ey0, ey1, fuera))

    if distintos == 0:
        print("VERDE: las dos capturas son IDENTICAS pixel a pixel (%d pixeles comparados)" % total)
        return 0

    ys, xs = np.where(dif)
    print("pixeles distintos: %d de %d (%.4f %%)" % (distintos, total, 100.0 * distintos / total))
    print("caja de las diferencias: x %d..%d  y %d..%d" % (xs.min() + x0, xs.max() + x0, ys.min() + y0, ys.max() + y0))

    return 0


if __name__ == "__main__":
    sys.exit(main())
