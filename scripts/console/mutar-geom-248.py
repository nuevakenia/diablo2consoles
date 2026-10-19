#!/usr/bin/env python3
"""mutar-geom-248.py -- EL MUTANTE de DC-002.48: vuelve a poner el paso de celda del defecto.

El defecto era `nivel.go` con `ladoCeldaX = 32` / `ladoCeldaY = 16` (el tamaño de UN SUBTILE) donde el
motor usa 80x40 (la celda entera). Este script pone el archivo en cualquiera de las dos formas, en TODAS
las copias de `nivel.go` del repo (las cuatro tienen que quedar iguales: el proyecto compara su sha256):

    --defecto  <dir>   deja las cuatro copias con 32x16 (el defecto)
    --arreglo  <dir>   deja las cuatro copias con 80x40 (lo correcto, el paso del motor)
    --ver      <dir>   dice en que estado estan las copias (y si son iguales entre si)

Se corre sobre un arbol APARTE (una copia congelada del checkout), no sobre el que se esta midiendo:

    rsync -a --exclude .git ~/Proyectos/diablo2consoles/ /tmp/dc248-mut/
    python3 scripts/console/mutar-geom-248.py --defecto /tmp/dc248-mut

Con el defecto puesto, `go run ./console/dc/verif-acto1-237` tiene que decir:
    GEOM248: VEREDICTO TILES-CORRIDOS        y  GEOM248: LIENZO cobertura max=5  pixelesSobre>0
Con el arreglo:
    GEOM248: VEREDICTO ENCAJA                y  GEOM248: LIENZO cobertura max=1  pixelesSobre=0
"""
import argparse
import hashlib
import pathlib
import re
import sys

COPIAS = [
    "console/dc/poc/personaje-mapa/nivel.go",
    "console/dc/poc/acto1-disco/nivel.go",
    "console/dc/verif-acto1-237/nivel.go",
    "console/dc/verif-personaje-238/nivel.go",
]

PATRON = re.compile(r"^\tladoCelda([XY])\s+=\s+(\d+)\s*//", re.MULTILINE)

DEFECTO = {"X": "32", "Y": "16"}
ARREGLO = {"X": "80", "Y": "40"}


def leer(ruta):
    return ruta.read_text(encoding="utf-8")


def estado(texto):
    m = PATRON.findall(texto)
    if len(m) != 2:
        return None
    return {k: v for k, v in m}


def escribir(ruta, texto, valores):
    def sub(m):
        return "\tladoCelda%s      = %s //" % (m.group(1), valores[m.group(1)])

    nuevo, n = PATRON.subn(sub, texto)
    if n != 2:
        raise SystemExit("no pude reescribir %s (encontre %d lineas ladoCelda*)" % (ruta, n))
    ruta.write_text(nuevo, encoding="utf-8")


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("arbol", help="raiz del arbol (una copia del repo)")
    g = ap.add_mutually_exclusive_group(required=True)
    g.add_argument("--defecto", action="store_true", help="pone 32x16 (el defecto)")
    g.add_argument("--arreglo", action="store_true", help="pone 80x40 (lo correcto)")
    g.add_argument("--ver", action="store_true", help="solo informa el estado")
    args = ap.parse_args()

    raiz = pathlib.Path(args.arbol).expanduser().resolve()
    if not raiz.is_dir():
        raise SystemExit("no existe el arbol %s" % raiz)

    valores = DEFECTO if args.defecto else ARREGLO if args.arreglo else None
    huellas = {}

    for rel in COPIAS:
        ruta = raiz / rel
        if not ruta.exists():
            print("FALTA %s" % ruta)
            continue

        if valores is not None:
            escribir(ruta, leer(ruta), valores)

        texto = leer(ruta)
        st = estado(texto)
        huellas[rel] = hashlib.sha256(texto.encode()).hexdigest()
        print("%-44s ladoCeldaX=%s ladoCeldaY=%s sha256=%s" % (
            rel, st["X"] if st else "?", st["Y"] if st else "?", huellas[rel][:16]))

    iguales = len(set(huellas.values())) == 1 and len(huellas) == len(COPIAS)
    print("copias iguales entre si: %s (%d de %d)" % ("SI" if iguales else "NO", len(huellas), len(COPIAS)))

    if not iguales:
        sys.exit(1)


main()
