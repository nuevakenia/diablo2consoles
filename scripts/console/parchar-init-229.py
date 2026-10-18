#!/usr/bin/env python3
"""DC-002.29: REPUNTA **UNA** LLAMADA de `___go_init_main` a una direccion basura (instrumento de biseccion).

POR QUE EXISTE (y que tiene de distinto del otro instrumento)
    `scripts/console/marcas-init-elf.py` agrega una seccion nueva al ELF para imprimir el INDICE de cada
    `init`. Es fino, pero AGREGA bytes al binario -- y el defecto de la tarjeta es de LAYOUT (DC-002.22:
    "un println de mas en Go cambia el defecto"), asi que un instrumento que rearma el binario puede
    esconder justo lo que quiere medir.
    Este NO agrega ni mueve un byte: reescribe UN literal del pool de `___go_init_main` (4 bytes en su
    lugar). El binario queda del MISMO tamano y con las MISMAS direcciones: solo cambia a donde salta esa
    llamada. Con eso se BISECTA el arranque: si la llamada k salta a basura, la corrida NO puede pasar de
    ahi; el ultimo k que SI se alcanza acota el `init` que mata.

COMO SE LEE
    La direccion basura va a la ZONA 3 del SH-4 (0x0F000000+), que no esta mapeada: la corrida tiene que
    morir con una excepcion del emulador que NOMBRA esa direccion (`Unhandled exception PC ...`, la forma
    que ya se uso en DC-002.24b). Si la corrida muere ANTES (o se queda muda sin nombrarla), esa llamada no
    se alcanzo.

USO
    python3 scripts/console/parchar-init-229.py <prog.elf> <salida.elf> <indice>
    python3 scripts/console/parchar-init-229.py --lista <prog.elf>          # indice -> destino
"""
import os
import re
import subprocess
import sys

ENV = "source %s/../console/env.sh && " % os.path.dirname(os.path.abspath(__file__))
INIT_SYM = "___go_init_main"
BASE_BASURA = 0x0FF00000


def sh(cmd):
    r = subprocess.run(["bash", "-c", ENV + cmd], capture_output=True, text=True)
    if r.returncode != 0:
        sys.exit("FALLO: %s\n%s%s" % (cmd, r.stdout[-2000:], r.stderr[-2000:]))
    return r.stdout


def sim(elf, nombre):
    for line in sh("sh-elf-nm %s | grep -w '%s'" % (elf, nombre)).splitlines():
        p = line.split()
        if len(p) >= 3 and p[2] == nombre:
            return int(p[0], 16)
    return None


def secciones(elf):
    secs = {}
    for line in sh("sh-elf-objdump -h %s" % elf).splitlines():
        m = re.match(r"\s*\d+\s+(\S+)\s+([0-9a-f]{8})\s+([0-9a-f]{8})\s+([0-9a-f]{8})\s+([0-9a-f]{8})", line)
        if m:
            secs[m.group(1)] = (int(m.group(3), 16), int(m.group(5), 16), int(m.group(2), 16))
    return secs


def llamadas(elf):
    """[(indice, direccion del jsr, vma del literal, direccion destino, nombre del destino)]"""
    if sim(elf, INIT_SYM) is None:
        sys.exit("no encuentro %s en %s" % (INIT_SYM, elf))
    d = sh("sh-elf-objdump -d --disassemble=%s %s" % (INIT_SYM, elf))
    out, pend = [], None
    for line in d.splitlines():
        line = line.rstrip()
        m = re.match(r"\s*([0-9a-f]+):\t[0-9a-f ]+\tmov\.l\t([0-9a-f]+)\s*<[^>]*>,\s*(r\d+)\s*!\s*([0-9a-f]+)\s*<([^>]*)>", line)
        if m:
            pend = (m.group(3), int(m.group(2), 16), int(m.group(4), 16), m.group(5))
            continue
        m2 = re.match(r"\s*([0-9a-f]+):\t[0-9a-f ]+\tjsr\t@(r\d+)", line)
        if m2 and pend and pend[0] == m2.group(2):
            out.append((len(out) + 1, int(m2.group(1), 16), pend[1], pend[2], pend[3]))
            pend = None
    return out


def vma_a_off(secs, vma):
    for nombre, (svma, soff, stam) in secs.items():
        if ".debug" in nombre or nombre.startswith(".strtab") or nombre.startswith(".symtab"):
            continue
        if svma <= vma < svma + stam:
            return soff + (vma - svma), nombre
    return None, None


def main():
    if "--lista" in sys.argv:
        elf = [a for a in sys.argv[1:] if not a.startswith("--")][0]
        ll = llamadas(elf)
        print("%d llamadas en %s" % (len(ll), INIT_SYM))
        for k, j, lit, dst, nom in ll:
            print("   %3d  0x%08x  %s" % (k, dst, nom))
        return

    elf, salida = [a for a in sys.argv[1:] if not a.startswith("--")][:2]
    explícito = None
    if "--destino" in sys.argv:
        # --destino <k>=<hex>: repunta la llamada k a una direccion ELEGIDA (un testigo: ver la tarjeta)
        k_txt = sys.argv[sys.argv.index("--destino") + 1]
        k, hexa = k_txt.split("=")
        explícito = int(hexa, 16)
    else:
        k = sys.argv[sys.argv.index("--indice") + 1] if "--indice" in sys.argv else None
    if k is None:
        sys.exit("uso: parchar-init-229.py <prog.elf> <salida.elf> --indice <k> | --destino <k>=<hex>")
    k = int(k)
    secs = secciones(elf)
    ll = llamadas(elf)
    if not 1 <= k <= len(ll):
        sys.exit("el indice %d esta fuera de 1..%d" % (k, len(ll)))
    _, _, lit_vma, dst, nom = ll[k - 1]
    basura = explícito if explícito is not None else BASE_BASURA + 4 * k

    datos = bytearray(open(elf, "rb").read())
    off, sec = vma_a_off(secs, lit_vma)
    if off is None:
        sys.exit("el literal de la llamada %d (0x%08x) no cae en ninguna seccion" % (k, lit_vma))
    antes = int.from_bytes(datos[off:off + 4], "little")
    if antes != dst:
        sys.exit("el literal de %d no vale lo que dice el desensamblado (0x%08x contra 0x%08x)" % (k, antes, dst))
    datos[off:off + 4] = basura.to_bytes(4, "little")
    open(salida, "wb").write(bytes(datos))

    print("PARCHE init %d/%d: el literal en 0x%08x (%s, offset de archivo 0x%x) pasa de 0x%08x (%s) a 0x%08x"
          % (k, len(ll), lit_vma, sec, off, dst, nom, basura))


if __name__ == "__main__":
    main()
