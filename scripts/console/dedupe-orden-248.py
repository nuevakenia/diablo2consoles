#!/usr/bin/env python3
"""dedupe-orden-248.py <dir_capa>: saca las lineas repetidas de orden.txt conservando el orden.

Por que: `build-motor-gox.sh` AGREGA lineas al orden.txt de la capa; si una corrida anterior ya las habia
puesto (o si la capa se copio de otra), el archivo queda con la misma ruta dos veces y el enlace mete el
mismo .gox dos veces: el linker muere con `multiple definition of ...`. Medido en esta tarjeta.
"""
import pathlib
import sys

p = pathlib.Path(sys.argv[1]) / "orden.txt"
lineas = [l for l in p.read_text().splitlines() if l.strip()]
vistas, salida, repetidas = set(), [], 0
for l in lineas:
    if l in vistas:
        repetidas += 1
        continue
    vistas.add(l)
    salida.append(l)
p.write_text("\n".join(salida) + "\n")
print("%s: %d lineas -> %d (saque %d repetidas)" % (p, len(lineas), len(salida), repetidas))
