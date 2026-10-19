#!/usr/bin/env python3
"""DC-002.49b: ¿el instrumento DISTINGUE las 8 direcciones del control?

Antes de exigirle a una captura que diga "esta es la direccion 12", hay que medir si el instrumento puede: se
comparan las REFERENCIAS del host (la pantalla con el cuadro 0 de cada direccion encima) entre si, y se mira
cuanta identidad tienen. Dos direcciones que dan ~100% no se pueden separar en una captura, y el criterio
estaria mintiendo.

LA REGION QUE SE COMPARA no se inventa: es la union de lo que CADA imagen cambia respecto del mapa solo
(-png-mapa). Asi la comparacion es sobre el personaje, no sobre los 640x480 del mapa (que dominan y esconden
la diferencia: es el error que tuvo DC-002.49 cuando dio 68,45 % contra 68,30 %).

Uso:
  distinguir-dirs-249b.py --host DIR --mapa mapa.png [--modo TN] [--tolerancia 8]

DIR tiene que traer pantalla-<MODO>-dNN.png (de `verif-personaje-238 -png-dirs TN -png-ciclo DIR`).
Sale 0 siempre: los numeros son el resultado.
"""
import argparse
import importlib.util
import os
import sys

import numpy as np

AQUI = os.path.dirname(os.path.abspath(__file__))

# las 8 direcciones del COF que la cruceta puede pedir (medido con verif-personaje-238 -censo-control)
DIRS_DEL_CONTROL = [0, 1, 4, 7, 8, 9, 12, 15]


def cargar_medidor_238():
    spec = importlib.util.spec_from_file_location(
        "medidor238", os.path.join(AQUI, "medir-captura-238.py"))
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


MED = cargar_medidor_238()


def identidad(a, b, region, tolerancia):
    """Fraccion de pixeles iguales (dentro de la tolerancia por canal) DENTRO de la region."""
    x0, y0, x1, y1 = region
    if x1 <= x0 or y1 <= y0:
        return 0.0

    ra = a[y0:y1, x0:x1].astype(np.int16)
    rb = b[y0:y1, x0:x1].astype(np.int16)

    dif = np.abs(ra - rb).max(axis=2)
    iguales = int((dif <= tolerancia).sum())

    return 100.0 * iguales / float(dif.size)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--host", required=True, help="carpeta con pantalla-<MODO>-dNN.png")
    ap.add_argument("--mapa", required=True, help="la pantalla SIN el personaje (-png-mapa)")
    ap.add_argument("--modo", default="TN")
    ap.add_argument("--tolerancia", type=int, default=8,
                    help="diferencia maxima por canal para decir 'mismo pixel' (el host no reescala: 8 sobra)")
    ap.add_argument("--todas", action="store_true",
                    help="compara las 16 direcciones del COF y no solo las 8 que pide el control")
    args = ap.parse_args()

    base = MED.decodificar(args.mapa)

    dirs = list(range(16)) if args.todas else DIRS_DEL_CONTROL

    refs = {}
    cajas = {}
    for d in dirs:
        ruta = os.path.join(args.host, "pantalla-%s-d%02d.png" % (args.modo, d))
        if not os.path.exists(ruta):
            print("FALTA %s" % ruta)
            return 1
        img = MED.decodificar(ruta)
        refs[d] = img

        # la region donde ESTA direccion pinta algo (contra el mapa solo)
        cambio = (np.abs(img.astype(np.int16) - base.astype(np.int16)).max(axis=2) > args.tolerancia)
        ys, xs = np.where(cambio)
        if len(xs) == 0:
            print("VACIA dir=%02d (no cambia ningun pixel respecto del mapa)" % d)
            return 1
        cajas[d] = (int(xs.min()), int(ys.min()), int(xs.max()) + 1, int(ys.max()) + 1)
        print("DIR %02d caja=%d,%d %dx%d pixeles=%d" % (
            d, cajas[d][0], cajas[d][1],
            cajas[d][2] - cajas[d][0], cajas[d][3] - cajas[d][1], int(cambio.sum())))

    print("")
    print("IDENTIDAD ENTRE DIRECCIONES (modo %s, %d direcciones, tolerancia %d)" % (
        args.modo, len(dirs), args.tolerancia))

    peor = (101.0, None)
    for i in range(len(dirs)):
        for j in range(i + 1, len(dirs)):
            a, b = dirs[i], dirs[j]
            # la union de las dos cajas: donde CUALQUIERA de las dos pinta
            x0 = min(cajas[a][0], cajas[b][0])
            y0 = min(cajas[a][1], cajas[b][1])
            x1 = max(cajas[a][2], cajas[b][2])
            y1 = max(cajas[a][3], cajas[b][3])

            idt = identidad(refs[a], refs[b], (x0, y0, x1, y1), args.tolerancia)
            if idt < peor[0]:
                peor = (idt, (a, b))
            print("PAR %02d-%02d region=%dx%d identidad=%.2f %%" % (a, b, x1 - x0, y1 - y0, idt))

    print("")
    if peor[1] is None:
        print("una sola direccion: no hay pares")
        return 0

    # el veredicto: con el par MAS PARECIDO todavia por debajo del 90 % el instrumento discrimina
    estado = "DISCRIMINA" if peor[0] < 90.0 else "NO DISCRIMINA"
    print("PEOR PAR %02d-%02d identidad=%.2f %% -> %s (umbral 90 %%)" % (
        peor[1][0], peor[1][1], peor[0], estado))

    return 0


if __name__ == "__main__":
    sys.exit(main())
