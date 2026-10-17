#!/usr/bin/env python3
"""DC-002.14: cuanto del binario se CARGA en los 16 MB de la consola (medido del ELF).

Lo que decide si el motor entra no es el tamano del archivo (el ELF de 43.876.988 bytes trae 14 MB de debug
y 11,9 MB de .go_export que NO se cargan) ni el 1ST_READ.BIN (13.706.888 bytes, que es la imagen con las
secciones que el objcopy deja): son las secciones con bandera ALLOC, y el dato sale del propio ELF.

Uso:  scripts/console/medir-cargado.py <elf>        (bytes que se cargan; medido: 13.910.992 en DC-002.13)
"""
import re
import subprocess
import sys

ELF = sys.argv[1] if len(sys.argv) > 1 else "/tmp/dcbuild/motor-arranque/prog.elf"
FLAGS = re.compile(r"[A-Z]+")


def cargado(elf):
    """Suma de las secciones ALLOC (bandera 'A'): las unicas que el cargador pone en RAM."""
    out = subprocess.run(["sh-elf-readelf", "-SW", elf], capture_output=True, text=True).stdout
    total = 0
    detalle = []
    for linea in out.splitlines():
        p = linea.replace("[", " ").replace("]", " ").split()
        if len(p) < 7 or not p[0].isdigit():
            continue
        nombre = p[1]
        # la columna Flg es el primer token de puras mayusculas (AX, A, WA, ...); los otros son numeros hex
        banderas = next((t for t in p[6:10] if FLAGS.fullmatch(t)), "")
        if "A" not in banderas:
            continue
        try:
            tam = int(p[5], 16)
        except ValueError:
            continue
        total += tam
        detalle.append((nombre, tam))
    return total, detalle


total, detalle = cargado(ELF)
print(f"{ELF}: {total} bytes con bandera ALLOC")
for nombre, tam in detalle:
    print(f"   {tam:>9}  {nombre}")
