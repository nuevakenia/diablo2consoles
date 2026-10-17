#!/usr/bin/env python3
"""Muta el twiddle de la pantalla del PVR (DC-002.26) para probar que el corpus tiene dientes.

Los dos defectos que cierra DC-002.26 estan en console/dc/std/csrc/pvr_pantalla.c:

  A. --al-reves     el lugar de los bits: la version vieja ponia la Y en los bits IMPARES del indice y la X
                    en los pares. La convencion del PVR es la de KOS (`pvr_texture.c:34`):
                    `TWIDOUT(x,y) = TWIDTAB(y) | (TWIDTAB(x) << 1)`, o sea la X en los impares. Estar al
                    reves es escribir la TRASPUESTA: el cuadro del motor se ve ROTADO 90 grados.
  B. --tabla-512    la tabla del twiddle vuelve a 512 entradas: con el motor (800x600 -> 1024x1024) el lado
                    menor es 1024 y `twid_mask` vale 1023, asi que los bloques con x0 o y0 >= 512 leen
                    FUERA de la tabla y se escriben donde no van (medido: 219 de los 475 bloques).

Uso:
    python3 scripts/console/mutar-twiddle-226.py --al-reves
    python3 scripts/console/mutar-twiddle-226.py --tabla-512
    python3 scripts/console/mutar-twiddle-226.py --restaurar     (deja el archivo como estaba)
    python3 scripts/console/mutar-twiddle-226.py --estado        (que mutaciones hay puestas)
"""
import argparse
import hashlib
import os
import sys

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
C = os.path.join(REPO, "console/dc/std/csrc/pvr_pantalla.c")

# el texto SANO (el que deja el arreglo) y el mutado, uno por defecto
INDICE_SANO = """    return (uint32_t)(twid_tabla[x0 & twid_mask] << 1)
         + (uint32_t)twid_tabla[y0 & twid_mask]
"""
INDICE_AL_REVES = """    return (uint32_t)(twid_tabla[y0 & twid_mask] << 1)
         + (uint32_t)(twid_tabla[x0 & twid_mask])
"""
CELDA_SANA = "                    conv_tile[(uint32_t)twid_tabla[xx] << 1 | fila] =\n"
CELDA_AL_REVES = "                    conv_tile[fila << 1 | (uint32_t)twid_tabla[xx]] =\n"
TABLA_SANA = "#define TWID_TABLA_N 1024\n"
TABLA_512 = "#define TWID_TABLA_N 512\n"


def leer():
    with open(C, "r", encoding="utf-8") as f:
        return f.read()


def escribir(t):
    with open(C, "w", encoding="utf-8") as f:
        f.write(t)


def estado(t):
    puestas = []
    if INDICE_AL_REVES in t and CELDA_AL_REVES in t:
        puestas.append("A (el lugar de los bits, al reves)")
    if TABLA_512 in t:
        puestas.append("B (tabla de 512)")
    return puestas


def reemplazar(t, viejo, nuevo, que):
    n = t.count(viejo)
    if nuevo in t:
        print("  ya estaba: %s" % que)
        return t
    if n != 1:
        sys.exit("ERROR: el texto de %s aparece %d veces (esperaba 1)" % (que, n))
    print("  puesto: %s" % que)
    return t.replace(viejo, nuevo, 1)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--al-reves", action="store_true", help="el lugar de los bits al reves (la rotacion)")
    ap.add_argument("--tabla-512", action="store_true", help="la tabla del twiddle vuelve a 512 entradas")
    ap.add_argument("--restaurar", action="store_true", help="deja el archivo sano")
    ap.add_argument("--estado", action="store_true", help="imprime que mutaciones hay puestas")
    args = ap.parse_args()

    t = leer()
    print("pvr_pantalla.c: %s  (%s bytes)" % (hashlib.sha256(t.encode()).hexdigest()[:16], len(t)))

    if args.estado:
        print("  mutaciones puestas:", estado(t) or "ninguna")
        return

    if args.restaurar:
        t = reemplazar(t, INDICE_AL_REVES, INDICE_SANO, "A: el indice sano")
        t = reemplazar(t, CELDA_AL_REVES, CELDA_SANA, "A: la celda sana")
        t = reemplazar(t, TABLA_512, TABLA_SANA, "B: la tabla de 1024")
        escribir(t)
        print("  estado final:", estado(leer()) or "ninguna")
        return

    if args.al_reves:
        t = reemplazar(t, INDICE_SANO, INDICE_AL_REVES, "A: el indice al reves")
        t = reemplazar(t, CELDA_SANA, CELDA_AL_REVES, "A: la celda al reves")
    if args.tabla_512:
        t = reemplazar(t, TABLA_SANA, TABLA_512, "B: la tabla de 512")
    escribir(t)
    print("  estado final:", estado(leer()) or "ninguna")


if __name__ == "__main__":
    main()
