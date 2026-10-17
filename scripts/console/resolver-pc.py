#!/usr/bin/env python3
"""Resuelve los PC que imprime la sonda del PC (`runtime/sonda_pc.c`, tarjeta DC-002.24b) contra el ELF.

POR QUE UN SCRIPT Y NO `addr2line` A MANO
    El ELF del arranque esta enlazado en 0x8c010000 (la RAM de la consola) y el `addr2line` del toolchain
    NO encuentra la linea asi: contesta `?? ??:0`. El nombre de la funcion si se resuelve (por la tabla de
    simbolos), asi que este script hace las dos cosas:

      1. el simbolo que CONTIENE el PC (por `sh-elf-nm -n`, el mas cercano hacia abajo) y el offset
      2. la linea, probando el PC tal cual y el PC menos la base de carga (`--base`, 0x8c010000 por defecto)

USO
    # los PC de un flycast.log (saca los de una linea o de todas las 'SONDA-PC')
    python3 scripts/console/resolver-pc.py /tmp/dcbuild/dc24b-sonda/prog-emu.elf /tmp/dc-run-24bsonda1/flycast.log
    # o PC sueltos
    python3 scripts/console/resolver-pc.py <elf> 0x8c013348 0x8c03f680
    # los del 'top' de la linea t=N y los que gritan RACHA/FUERA (por defecto: todos)
    python3 scripts/console/resolver-pc.py <elf> <log> --solo-alarmas
"""
import bisect
import os
import re
import subprocess
import sys

BASE_DEF = 0x8C010000


def nm(elf, tool_prefix="sh-elf-"):
    out = subprocess.run([tool_prefix + "nm", "-n", elf], capture_output=True, text=True).stdout
    syms = []
    for line in out.splitlines():
        p = line.split()
        if len(p) == 3:
            try:
                syms.append((int(p[0], 16), p[2]))
            except ValueError:
                pass
    syms.sort()
    return syms


def addr2line(elf, addr, tool_prefix="sh-elf-"):
    r = subprocess.run([tool_prefix + "addr2line", "-f", "-C", "-e", elf, hex(addr)],
                       capture_output=True, text=True).stdout.splitlines()
    if len(r) >= 2 and r[1] not in ("??:0", ""):
        return r[0] + "  " + r[1]
    return None


def pc_de_log(path, solo_alarmas):
    pcs = []
    rx = re.compile(r"(8c[0-9a-f]{6})")
    for t in open(path, "r", errors="replace"):
        if "SONDA-PC" not in t:
            continue
        if solo_alarmas and ("RACHA" not in t and "FUERA" not in t):
            continue
        pcs.extend(m.group(1) for m in rx.finditer(t))
    return pcs


def censo(path, elf):
    """El censo de las lineas 'SONDA-PC t=N top <pc>:<n> ...': agrupa por SIMBOLO (que funcion se lleva el
    tiempo del SH-4) y por PC. Devuelve (total_muestras, por_pc, por_simbolo)."""
    rx = re.compile(r"(8c[0-9a-f]{6}):(\d+)")
    por_pc = {}
    ventanas = 0
    for t in open(path, "r", errors="replace"):
        if "SONDA-PC t=" not in t:
            continue
        ventanas += 1
        for m in rx.finditer(t):
            por_pc[m.group(1)] = por_pc.get(m.group(1), 0) + int(m.group(2))
    syms = nm(elf)
    addrs = [s[0] for s in syms]
    por_simbolo = {}
    for pc, n in por_pc.items():
        a = int(pc, 16)
        i = bisect.bisect_right(addrs, a) - 1
        nom = syms[i][1] if i >= 0 else "?"
        # los simbolos de gccgo vienen con el prefijo _github_0com_1... : se acorta para la tabla
        nom = re.sub(r"^_(?:github_0com_1|d2c_)", "", nom).replace("_1", "/")
        por_simbolo[nom] = por_simbolo.get(nom, 0) + n
    return ventanas, por_pc, por_simbolo, syms, addrs


def imprime_censo(path, elf):
    ventanas, por_pc, por_simbolo, syms, addrs = censo(path, elf)
    total = sum(por_pc.values())
    print("== censo de la sonda en %s: %d ventanas, %d muestras contadas (top de cada ventana)"
          % (path, ventanas, total))
    print("   %6s  %9s  %s" % ("%", "muestras", "SINBOLO (funcion)"))
    for nom, n in sorted(por_simbolo.items(), key=lambda kv: -kv[1]):
        print("   %5.1f%%  %9d  %s" % (100.0 * n / total, n, nom))
    print("   --- por PC (los 12 que mas se repiten en el top):")
    for pc, n in sorted(por_pc.items(), key=lambda kv: -kv[1])[:12]:
        a = int(pc, 16)
        i = bisect.bisect_right(addrs, a) - 1
        nom, base = (syms[i][1], syms[i][0]) if i >= 0 else ("?", 0)
        print("   %5.1f%%  %9d  0x%s  %s +%d  %s" % (100.0 * n / total, n, pc, nom, a - base,
                                                      addr2line(elf, a) or addr2line(elf, a - BASE_DEF) or "?"))


def main():
    if len(sys.argv) < 3:
        sys.exit(__doc__)
    elf = sys.argv[1]
    args = sys.argv[2:]
    solo_alarmas = "--solo-alarmas" in args
    censo_on = "--censo" in args
    args = [a for a in args if not a.startswith("--")]
    if censo_on:
        imprime_censo(args[0], elf)
        return
    if len(args) == 1 and os.path.isfile(args[0]):
        pcs = pc_de_log(args[0], solo_alarmas)
        print("== %d PC sacados de %s" % (len(pcs), args[0]))
    else:
        pcs = [a[2:] if a.lower().startswith("0x") else a for a in args]

    syms = nm(elf)
    addrs = [s[0] for s in syms]
    vistos = set()
    for pc in pcs:
        if pc in vistos:
            continue
        vistos.add(pc)
        a = int(pc, 16)
        i = bisect.bisect_right(addrs, a) - 1
        sym, base = (syms[i][1], syms[i][0]) if i >= 0 else ("?", 0)
        linea = addr2line(elf, a) or addr2line(elf, a - BASE_DEF) or "?"
        print("0x%s  %-40s +%-6d  %s" % (pc, sym, a - base, linea))


main()
