#!/usr/bin/env python3
"""DC-002.14: DE DONDE SALE EL PESO DEL BINARIO DEL MOTOR (medido, por paquete).

Por que existe: DC-002.13 midio que el motor entero enlazado NO entra en los 16 MB de la consola (faltan
1.486.848 bytes de RAM para el GC). Para recortar hay que saber DONDE esta el peso, y eso NO se mide con el
tamano de los .gox (traen debug y enganan): se mide en el ELF enlazado, con el tamano de cada simbolo
(`sh-elf-nm -S`), atribuido al paquete que lo define.

Uso:  scripts/console/pesar-motor.py <elf>        (por defecto el del POC motor-arranque)

Los nombres de gccgo vienen mangleados: `_github_0com_1OpenDiablo2_1OpenDiablo2_1d2core_1d2records.Tabla`
es `github.com/OpenDiablo2/OpenDiablo2/d2core/d2records` + el simbolo (`_0` -> `.`, `_1` -> `/`).
"""
import subprocess
import sys
from collections import defaultdict

ELF = sys.argv[1] if len(sys.argv) > 1 else "/tmp/dcbuild/motor-arranque/prog-completo.elf"
NM = "sh-elf-nm"
REPO = "github.com/OpenDiablo2/OpenDiablo2/"


def desmanglear(simbolo: str) -> str:
    if not simbolo.startswith("_"):
        return simbolo
    s = simbolo[1:]
    return s.replace("_1", "/").replace("_0", ".")


def paquete(simbolo: str) -> str:
    """Devuelve la etiqueta a la que se le carga el peso de este simbolo."""
    s = desmanglear(simbolo)

    if s.startswith(REPO):
        resto = s[len(REPO):]
        if "/" in resto:
            return REPO + resto.rsplit("/", 1)[0]
        return REPO + resto.split(".")[0]

    # los blobs generados por el compilador (constantes, descriptores de tipo, gcbits): OJO, esta regla va
    # ANTES de la de la stdlib, porque `go..C3111` tambien tiene un punto y si no cae en "go"
    if s.startswith("go.") or s.startswith("type..") or s.startswith("gcbits") or s.startswith("pimt"):
        return "datos generados por el compilador (tipos, constantes, gcbits)"

    # la stdlib minima y los terceros: `fmt.Fprintf`, `bufio.Scan`, `ebiten.xyz`
    if "." in s:
        cabeza = s.split(".")[0]
        if cabeza and cabeza[0].islower() and not cabeza.startswith("_") and "/" not in cabeza:
            return cabeza

    if s.startswith("runtime") or s.startswith("_runtime"):
        return "runtime de Go (libgodc)"

    return "C (libc / KOS / libgodc)"


def main():
    salida = subprocess.run([NM, "-S", "--size-sort", ELF], capture_output=True, text=True).stdout

    peso = defaultdict(int)
    nsimb = defaultdict(int)
    total = 0

    for linea in salida.splitlines():
        p = linea.split()
        if len(p) != 4:      # las lineas de simbolos sin tamano (o basura) se saltean
            continue
        try:
            tam = int(p[1], 16)
        except ValueError:
            continue
        if tam == 0:
            continue
        etiqueta = paquete(p[3])
        peso[etiqueta] += tam
        nsimb[etiqueta] += 1
        total += tam

    print(f"ELF: {ELF}")
    print(f"peso atribuido a simbolos con tamano: {total} bytes ({total / 1048576:.2f} MB)")
    print()
    print(f"{'bytes':>10}  {'%':>5}  {'simb':>6}  paquete")
    for etiqueta, b in sorted(peso.items(), key=lambda x: -x[1]):
        print(f"{b:>10}  {100 * b / total:5.1f}  {nsimb[etiqueta]:>6}  {etiqueta}")


if __name__ == "__main__":
    main()
