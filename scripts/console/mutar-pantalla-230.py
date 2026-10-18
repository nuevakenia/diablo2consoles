#!/usr/bin/env python3
"""MUTANTE del corpus de DC-002.30 (scripts/console/checar-pantalla-230.sh).

QUE ROMPE: el PISO de 800x600 de la pantalla logica del motor (`d2common/d2display.Clamp`). El control
del host de la tarjeta exige que TODA combinacion de config (1512x982 / 800x600 / 640x480 / 320x240) y
de pantalla del renderer (640x480 / 320x240) resuelva a 800x600: es lo que hace que el campo `Display`
del config NO sea una palanca. El mutante saca el piso y entonces el 320x240 SI aparece resuelto: el
control tiene que ponerse ROJO. Si el mutante queda verde, el hueco es del corpus.

Toca el repo (una linea de `d2common/d2display/display.go`) y deja una COPIA del archivo original en el
scratch para restaurarla: el corpus exige que el archivo quede con el mismo sha256 con el que empezo.

Uso: python3 scripts/console/mutar-pantalla-230.py --sin-piso | --restaurar | --estado
"""
import argparse
import hashlib
import os
import shutil
import sys

AQUI = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.abspath(os.path.join(AQUI, "..", ".."))
ARCHIVO = os.path.join(REPO, "d2common", "d2display", "display.go")
RESPALDO = "/home/dgalli/.hermes/cache/scratch/dc230-mutante/display.go.sano"

ORIGINAL = """// Clamp raises a size to the 800x600 minimum.
func Clamp(s Size) Size {
	if s.W < BaseW {
		s.W = BaseW
	}

	if s.H < BaseH {
		s.H = BaseH
	}

	return s
}"""

# el mutante: sin piso. El resto del archivo queda igual.
MUTANTE = """// Clamp raises a size to the 800x600 minimum.
// MUTANTE DC-002.30: SIN PISO (el corpus de la tarjeta tiene que cazar esto).
func Clamp(s Size) Size {
	return s
}"""


def sha(ruta):
    with open(ruta, "rb") as fh:
        return hashlib.sha256(fh.read()).hexdigest()


def leer():
    with open(ARCHIVO) as fh:
        return fh.read()


def escribir(t):
    with open(ARCHIVO, "w") as fh:
        fh.write(t)


def puesto(t):
    return "MUTANTE DC-002.30" in t


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--sin-piso", action="store_true", help="saca el piso de Clamp")
    ap.add_argument("--restaurar", action="store_true", help="deja el archivo como estaba")
    ap.add_argument("--estado", action="store_true", help="dice si el mutante esta puesto")
    a = ap.parse_args()

    t = leer()

    if a.estado:
        print("display.go sha256=%s" % sha(ARCHIVO)[:16])
        print("  el piso de Clamp:        %s" % ("SACADO (mutante puesto)" if puesto(t) else "puesto (sano)"))
        return 0

    if a.restaurar:
        if not os.path.exists(RESPALDO):
            print("no tengo respaldo en %s" % RESPALDO, file=sys.stderr)
            return 1
        shutil.copy2(RESPALDO, ARCHIVO)
        print("restaurado desde %s: sha256=%s" % (RESPALDO, sha(ARCHIVO)[:16]))
        return 0

    if a.sin_piso:
        if puesto(t):
            print("el mutante ya estaba puesto (sha256=%s)" % sha(ARCHIVO)[:16])
            return 0
        if ORIGINAL not in t:
            print("no encuentro el Clamp original: el archivo cambio", file=sys.stderr)
            return 1
        os.makedirs(os.path.dirname(RESPALDO), exist_ok=True)
        shutil.copy2(ARCHIVO, RESPALDO)
        escribir(t.replace(ORIGINAL, MUTANTE))
        print("mutante puesto (respaldo en %s): sha256=%s" % (RESPALDO, sha(ARCHIVO)[:16]))
        return 0

    ap.print_help()
    return 2


if __name__ == "__main__":
    sys.exit(main())
