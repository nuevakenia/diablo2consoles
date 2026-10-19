#!/usr/bin/env python3
"""DC-002.49b -- ¿QUE direccion del COF se ve en la CAPTURA? (el canal de los pixeles)

POR QUE EXISTE
--------------
El canal del log (`HITO249: ANIM ... crc32`) dice que compuesto se dibujo, pero es el POC diciendo que hizo.
Este dice que se VE. Los dos tienen que coincidir, y son independientes: el del log compara crc32 contra la
tabla del host; este compara PIXELES contra las referencias del host.

COMO COMPARA (la leccion de DC-002.49)
--------------------------------------
La region NO es la pantalla entera (que da ~68 % de identidad entre dos cuadros cualesquiera y esconde todo):
es la UNION de lo que cada referencia cambia respecto del mapa solo, o sea el personaje y nada mas. Cada
referencia se baja al tamano del recorte de la captura (no se reescala la captura: no se inventan pixeles) y
se mide la identidad pixel a pixel con tolerancia por canal.

LA REGLA QUE HACE QUE ESTO NO SE ENGANE A SI MISMO
--------------------------------------------------
La captura se tiene que parecer a SU direccion MUCHO mas que a la segunda: se informa el margen
(mejor - segundo). Si el margen es chico, el numero no dice nada y el corpus lo tiene que declarar.

Las referencias tienen que estar compuestas EN EL MISMO ANCLA que la captura: el personaje se mueve al
caminar, y una referencia centrada no se parece a un personaje corrido 20 px. Las arma el que llama con
`verif-personaje-238 -pos <celda> -censo-dirs TN -png-dirs TN` (medido: el censo tarda 0,43 s en el host).

Uso:
  clasificar-captura-249b.py --host REFS --mapa REFS/mapa.png --modo TN --ventana L,T,W,H captura.png ...
"""
import argparse
import importlib.util
import os
import sys

import numpy as np

AQUI = os.path.dirname(os.path.abspath(__file__))
DIRS_DEL_CONTROL = [0, 1, 4, 7, 8, 9, 12, 15]


def cargar(nombre, archivo):
    spec = importlib.util.spec_from_file_location(nombre, os.path.join(AQUI, archivo))
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


MED = cargar("medidor238", "medir-captura-238.py")
ANIM = cargar("medidor249", "medir-anim-249.py")


def region_de_las_referencias(refs, mapa, tolerancia):
    """La union de lo que CADA direccion pinta encima del mapa solo (en pixeles logicos 640x480)."""
    x0 = y0 = 10 ** 9
    x1 = y1 = -1
    for d, img in refs.items():
        cambio = (np.abs(img.astype(np.int16) - mapa.astype(np.int16)).max(axis=2) > tolerancia)
        ys, xs = np.where(cambio)
        if len(xs) == 0:
            raise SystemExit(f"VACIA dir={d:02d}: no cambia ningun pixel respecto del mapa")
        x0, y0 = min(x0, int(xs.min())), min(y0, int(ys.min()))
        x1, y1 = max(x1, int(xs.max()) + 1), max(y1, int(ys.max()) + 1)
    return x0, y0, x1, y1


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--host", required=True, help="carpeta con pantalla-<MODO>-dNN.png de ESTE ancla")
    ap.add_argument("--mapa", required=True, help="la pantalla SIN el personaje, de ESTE ancla (-png-mapa)")
    ap.add_argument("--modo", default="TN")
    ap.add_argument("--ventana", default="", help="area cliente del emulador: L,T,W,H (la linea GEO de la corrida)")
    ap.add_argument("--tolerancia", type=int, default=40,
                    help="diferencia maxima por canal para 'mismo pixel' (40 = el reescalado del emulador, DC-002.49)")
    ap.add_argument("--dirs", default=",".join(str(d) for d in DIRS_DEL_CONTROL))
    ap.add_argument("capturas", nargs="+")
    args = ap.parse_args()

    ventana = None
    if args.ventana:
        ventana = tuple(int(x) for x in args.ventana.split(","))
        if len(ventana) != 4:
            raise SystemExit("--ventana tiene que ser L,T,W,H")

    mapa = MED.decodificar(args.mapa)
    dirs = [int(x) for x in args.dirs.split(",")]
    refs = {}
    for d in dirs:
        ruta = os.path.join(args.host, "pantalla-%s-d%02d.png" % (args.modo, d))
        if not os.path.exists(ruta):
            raise SystemExit("FALTA " + ruta)
        refs[d] = MED.decodificar(ruta)

    region = region_de_las_referencias(refs, mapa, 8)
    print("region del personaje (union de las %d direcciones, pixeles logicos): %d,%d -> %d,%d  (%dx%d)"
          % (len(dirs), region[0], region[1], region[2], region[3], region[2] - region[0], region[3] - region[1]))

    for ruta in args.capturas:
        img = MED.decodificar(ruta)
        recorte = ANIM.recorte_en_la_captura(img, ventana, region)
        if recorte is None:
            print("%-28s FALLA: la region del personaje no cae dentro de la captura" % os.path.basename(ruta))
            continue

        alto_d, ancho_d = recorte.shape[0], recorte.shape[1]
        filas = []
        for d in dirs:
            ref_crop = ANIM.recorte_de_la_referencia(refs[d], region, (ancho_d, alto_d))
            if ref_crop is None:
                continue
            filas.append((ANIM.identidad(recorte, ref_crop), d))

        filas.sort(reverse=True)
        mejor, segundo = filas[0], filas[1] if len(filas) > 1 else (0.0, -1)
        margen = mejor[0] - segundo[0]
        print("%-28s mejor=dir %02d  identidad=%.2f %%  segundo=dir %02d (%.2f %%)  margen=%.2f pts  recorte=%dx%d"
              % (os.path.basename(ruta), mejor[1], mejor[0], segundo[1], segundo[0], margen, ancho_d, alto_d))
        print("    ranking: " + "  ".join("%02d:%.2f" % (d, i) for i, d in filas))

    return 0


if __name__ == "__main__":
    sys.exit(main())
