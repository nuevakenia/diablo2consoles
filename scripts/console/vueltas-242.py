#!/usr/bin/env python3
"""Parte un log de Flycast en VUELTAS del arranque (cada muerte del motor reinicia la consola) y resume cada
una: fuentes del disco, assets del MPQ, y en que linea muere. Sin interpretar: los numeros crudos.

Uso:  vueltas-242.py <flycast.log> [<flycast.log> ...]
"""
import re
import sys

MUERTE = re.compile(r"^(panic:|fatal error:|Out of memory|Data address error|Unhandled exception)")


def vueltas(path):
    with open(path, "rb") as fh:
        ls = fh.read().decode("utf-8", "replace").replace("\r", "").split("\n")

    cortes = [i for i, l in enumerate(ls) if "MOTOR-ARRANQUE arranca" in l] or [0]
    cortes.append(len(ls))

    for n, (a, b) in enumerate(zip(cortes, cortes[1:]), start=1):
        trozo = ls[a:b]
        fuentes = [l.split("/cd/juego/")[-1].rstrip("'") for l in trozo if "Adding source: '/cd/juego" in l]
        assets = [l for l in trozo if "Loaded /cd/juego" in l]
        muerte = next((l for l in trozo if MUERTE.match(l)), "(sin muerte en la ventana)")
        print("  vuelta %d: %d fuente(s) del disco %s / %d asset(s) %s" % (
            n, len(fuentes), fuentes, len(assets), [l.split("->")[-1].strip() for l in assets[:3]]))
        print("             muere: %s" % muerte)
        print("             ultima linea: %s" % trozo[-2][:120] if len(trozo) > 1 else "")


def main():
    for path in sys.argv[1:]:
        print("== %s" % path)
        vueltas(path)
        print()
    return 0


if __name__ == "__main__":
    sys.exit(main())
