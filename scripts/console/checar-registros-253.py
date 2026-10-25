#!/usr/bin/env python3
"""checar-registros-253.py -- barre un ELF de SH-4 buscando funciones que rompen la ABI.

POR QUE
--------
El 2026-10-24/25, tras eliminar el falso "solape pila/semiespacio", el POC seguia cayendo dentro de
`pvr_init` con el PC adentro del area de pilas y registros callee-saved (r8, r11) pisados. La firma
(un `jsr @r8` que debia ir a `dbglog` y salto a la pila) apunta a una funcion que no respeta los
registros que, segun la ABI de SH-4, tiene que guardar y restituir: r8-r13 (enteros) y fr12-fr15 (FPU).

QUE HACE
--------
Recorre el desensamblado, funcion por funcion, y para cada registro callee-saved cuenta:
  - guardados  : `mov.l rN,@-r15`        (y `fmov.s frN,@-r15` para FPU)
  - restituciones: `mov.l @r15+,rN`
  - otras escrituras: cualquier instruccion cuyo DESTINO sea rN
Una funcion esta MAL si:
  - escribe rN en algun lado que no sea la restitucion y no lo guardo nunca, o
  - restituye rN mas veces de las que lo guardo (se lleva basura del marco del que llamo).

USO
---
  scripts/console/checar-registros-253.py <prog.elf> [-v]

Sale con 1 si encuentra violaciones, 0 si el binario esta limpio.
"""
import re
import subprocess
import sys

REGS = [f"r{n}" for n in range(8, 14)]
FREGS = [f"fr{n}" for n in range(12, 16)]

# separa "opcode  operandos"
INS = re.compile(r"^\s*([0-9a-f]+):\s+[0-9a-f ]+\t(\S+)\s*(.*)$")


def desensamblar(elf):
    out = subprocess.run(
        ["sh-elf-objdump", "-d", "--no-show-raw-insn", elf],
        capture_output=True, text=True, check=True,
    ).stdout
    return out.splitlines()


def destino(op, ops, reg):
    """True si `reg` es destino de la instruccion."""
    if not ops:
        return False
    # mov.l rX,@-r15  / fmov.s frX,@-r15  -> guardado
    # mov.l @r15+,rX                      -> restitucion
    partes = [p.strip() for p in ops.split(",")]
    if op.startswith("mov") or op.startswith("add") or op.startswith("sub") \
       or op.startswith("and") or op.startswith("or") or op.startswith("xor") \
       or op.startswith("sh") or op.startswith("ext") or op.startswith("not") \
       or op.startswith("neg") or op.startswith("swap") or op.startswith("movt") \
       or op.startswith("mov"):
        if len(partes) == 2:
            return partes[1] == reg
        return False
    if op in ("lds", "lds.l", "sts", "sts.l", "fldi", "fmov", "float", "ftrc",
              "flds", "fsts", "ldc", "stc", "mova"):
        return False
    # resto (tst, cmp, bt, bt.s, jsr, rts, bra, nop, ...) no escriben registros de datos
    if op.startswith("tst") or op.startswith("cmp") or op.startswith("dt"):
        return False
    return False


def revisar(elf, verboso=False):
    lineas = desensamblar(elf)
    func = None
    guardados = {}
    restituciones = {}
    otras = {}
    violaciones = []

    def cerrar(nombre):
        if not nombre:
            return
        for reg in REGS + FREGS:
            g = guardados.get(reg, 0)
            r = restituciones.get(reg, 0)
            o = otras.get(reg, 0)
            if o and not g:
                violaciones.append((nombre, reg, "escribe sin guardar", o))
            elif r > g:
                violaciones.append((nombre, reg, f"restituye {r} y guarda {g}", r - g))

    for ln in lineas:
        m = re.match(r"^([0-9a-f]+) <(.+)>:$", ln.strip())
        if m:
            cerrar(func)
            func = m.group(2)
            guardados, restituciones, otras = {}, {}, {}
            continue
        mi = INS.match(ln)
        if not mi or not func:
            continue
        _, op, ops = mi.groups()
        ops = ops.split("!")[0].strip()          # saca el comentario del literal
        for reg in REGS:
            if op == "mov.l" and ops == f"{reg},@-r15":
                guardados[reg] = guardados.get(reg, 0) + 1
            elif op == "mov.l" and ops == f"@r15+,{reg}":
                restituciones[reg] = restituciones.get(reg, 0) + 1
            elif destino(op, ops, reg):
                otras[reg] = otras.get(reg, 0) + 1
        for reg in FREGS:
            if op == "fmov.s" and ops == f"{reg},@-r15":
                guardados[reg] = guardados.get(reg, 0) + 1
            elif op == "fmov.s" and ops == f"@r15+,{reg}":
                restituciones[reg] = restituciones.get(reg, 0) + 1
            elif op in ("fmov", "fldi0", "fldi1", "float", "ftrc") and ops.endswith(reg):
                otras[reg] = otras.get(reg, 0) + 1
    cerrar(func)

    return violaciones


def main():
    if len(sys.argv) < 2:
        print(__doc__)
        return 2
    elf = sys.argv[1]
    verboso = "-v" in sys.argv
    v = revisar(elf, verboso)
    if not v:
        print(f"OK: {elf} sin violaciones de registro callee-saved")
        return 0
    print(f"{len(v)} violacion(es) en {elf}:")
    for nombre, reg, que, n in v[:60]:
        print(f"  {nombre:44s} {reg:4s} {que} ({n})")
    return 1


if __name__ == "__main__":
    sys.exit(main())
