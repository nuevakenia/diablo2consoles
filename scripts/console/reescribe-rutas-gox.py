#!/usr/bin/env python3
"""Reescribe las rutas absolutas de orden.txt / orden-motor.txt al copiar una capa de .gox.

POR QUE HACE FALTA (medido, DC-002.34): los dos orden*.txt guardan la RUTA ABSOLUTA de cada .gox. Al copiar
la capa con `cp -a` a otro directorio, los archivos quedan listados con la ruta vieja, y el enlace mete los
DOS juegos de .gox: el linker muere con `multiple definition of ..._d2mpq.New..f` (first defined here en la
capa vieja). No es un problema del motor: es de la copia.

Uso:  reescribe-rutas-gox.py <dir_viejo> <dir_nuevo> <dir_capa>
"""
import pathlib
import sys


def main():
    viejo, nuevo, capa = sys.argv[1:4]
    for nombre in ("orden.txt", "orden-motor.txt"):
        p = pathlib.Path(capa) / nombre
        if not p.exists():
            print("no existe", p)
            continue
        texto = p.read_text()
        n = texto.count(viejo + "/")
        p.write_text(texto.replace(viejo + "/", nuevo + "/"))
        print("reescrito %s: %d rutas %s -> %s" % (p, n, viejo, nuevo))
    return 0


if __name__ == "__main__":
    sys.exit(main())
