#!/usr/bin/env python3
"""DC-002.159o -- saca de los logs de las corridas los numeros que pide la tarjeta, uno por corrida.

Uso: resumen-corridas-159o.py /tmp/cap159o2-1 /tmp/cap159o2-2 ...
"""
import glob
import os
import re
import sys


def ultimo(patron, texto, flags=0):
    hallados = re.findall(patron, texto, flags)
    return hallados[-1] if hallados else None


def resumen(directorio):
    log = os.path.join(directorio, "flycast.log")
    if not os.path.exists(log):
        return "%s: sin flycast.log" % directorio

    with open(log, "r", encoding="utf-8", errors="replace") as f:
        texto = f.read()

    excepciones = len(re.findall(r"Unhandled exception", texto))
    vivas = re.findall(r"S159N-VIVA[^\r\n]*", texto)
    cuadros = [int(n) for n in re.findall(r"HEROE159 cuadro= ?(\d+)", texto)]
    mun = re.findall(r"GATE0: ESTADO (mapa:[a-z-]+) montonLibre= (-?\d+) breakLibre= (-?\d+) montonUsado= (\d+)", texto)
    reset = [m for m in mun if m[0] == "mapa:reset-hecho"]
    sombras = ultimo(r"sombrasDT1= ?(\d+)", texto)
    ultimo_cuadro = ultimo(r"CUADRO (\d+) ms= (\d+)", texto) if False else None

    print("=" * 100)
    print(directorio)
    print("  excepciones          :", excepciones)
    print("  cuadros (ultimo HEROE):", cuadros[-1] if cuadros else "-")
    if reset:
        print("  montonUsado reset-hecho: %s B  (breakLibre %s)" % (reset[-1][3], reset[-1][2]))
    else:
        print("  montonUsado reset-hecho: NO APARECE")
    print("  sombrasDT1 (ultimo)  :", sombras)
    for v in vivas[-2:]:
        print("  ", v.strip())


for d in sys.argv[1:]:
    if os.path.isdir(d):
        resumen(d)
