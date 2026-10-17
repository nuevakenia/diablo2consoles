#!/usr/bin/env python3
"""DC-002.14: EL PESO DE CADA PAQUETE, medido en su .gox (sin solapes y sin los archivos de debug).

Por que NO se mide con el ELF enlazado: ahi los tamanos de los simbolos de .text se solapan (21.242 de
21.266 arrancan dentro del anterior; la suma da 2,86x el tamano de la seccion) porque gccgo mete cada
paquete en una sola .text. Y por que NO se mide con el TAMANO DEL ARCHIVO .gox: traen .debug_* y .go_export
que no se cargan (un .gox del motor de 204.672 bytes puede tener 40.000 de codigo).

Lo que SI es medible y exacto: sumar las SECCIONES ALLOC de cada .gox (cada byte una sola vez). Eso es lo
que ese paquete le va a pedir a la RAM como maximo; el enlace despues puede tirar algo (COMDAT) o fusionar.

Uso:  scripts/console/pesar-gox.py <dir_gox> [cuantos]
"""
import re
import subprocess
import sys
from pathlib import Path

GEN = sys.argv[1] if len(sys.argv) > 1 else "/tmp/gox-214-gc"
CUANTOS = int(sys.argv[2]) if len(sys.argv) > 2 else 40
FLAGS = re.compile(r"[A-Z]+")


def cargado(gox):
    """(bytes ALLOC, detalle [(seccion, bytes)]) de un .gox."""
    out = subprocess.run(["sh-elf-readelf", "-SW", gox], capture_output=True, text=True).stdout
    total, detalle = 0, []
    for linea in out.splitlines():
        p = linea.replace("[", " ").replace("]", " ").split()
        if len(p) < 7 or not p[0].isdigit():
            continue
        banderas = next((t for t in p[6:10] if FLAGS.fullmatch(t)), "")
        if "A" not in banderas:          # solo lo que se carga: ni debug, ni .group, ni .rela
            continue
        try:
            tam = int(p[5], 16)
        except ValueError:
            continue
        total += tam
        detalle.append((p[1], tam))
    return total, detalle


filas = []
for gox in sorted(Path(GEN).rglob("*.gox")):
    total, detalle = cargado(str(gox))
    filas.append((total, str(gox.relative_to(GEN))[:-4], detalle,
                  int(subprocess.run(["stat", "-c%s", str(gox)], capture_output=True, text=True).stdout)))

filas.sort(reverse=True)
suma = sum(f[0] for f in filas)
print(f"gox en {GEN}: {len(filas)}   bytes ALLOC sumados: {suma} ({suma / 1048576:.2f} MB)")
print(f"(para comparar: el ELF enlazado de DC-002.13 carga 13.910.992 bytes)")
print()
print(f"{'bytes':>9}  {'archivo':>8}  {'gox':>8}  paquete")
for total, nombre, detalle, tam in filas[:CUANTOS]:
    print(f"{total:>9}  {tam:>8}  {len(detalle):>8}  {nombre}")

print()
print("== el detalle de los 12 mas pesados (las secciones que los componen)")
for total, nombre, detalle, tam in filas[:12]:
    print(f"== {nombre}: {total} bytes en {len(detalle)} secciones")
    for sec, b in sorted(detalle, key=lambda x: -x[1])[:6]:
        print(f"   {b:>9}  {sec[:90]}")
