#!/usr/bin/env python3
"""DC-002.29: BUSCAR LA SECUENCIA SH-4 ILEGAL EN UN BINARIO ENLAZADO.

POR QUE: Flycast (el recompilador) mata la corrida del binario rearmado con
    `Fatal: SH4 branch instruction in delay slot`
o sea: el SH-4 ejecuto una RAMA dentro de un DELAY SLOT. En el SH-4 eso no esta definido (el manual manda
excepcion de instruccion ilegal); en Flycast es fatal y el emulador se detiene. La secuencia esta en el
binario, asi que se puede buscar SIN correr nada: rama que usa delay slot seguida, en el delay slot
(los 2 bytes que siguen), de otra rama.

QUE BUSCA (una entrada = un sospechoso):
  - un salto con delay slot: BRA/BSR disp12 (0xAxxx/0xBxxx), JMP @Rn (0x4n2B), JSR @Rn (0x4n0B),
    RTS (0x000B), RTE (0x002B), BT/S (0x8Dnn), BF/S (0x8Fnn)
  - y en los 2 bytes siguientes, CUALQUIER rama (incluidas BT 0x89nn y BF 0x8Bnn, que no llevan delay slot).

OJO CON LA LECTURA: los charcos de literales (pool) viven adentro de .text, asi que un binario grande
tiene falsos positivos: la lista dice DONDE esta cada par y en que simbolo cae, para poder compararlo entre
dos binarios (el que anda y el rearmado) y contra la direccion que reporte la corrida.

USO
    python3 scripts/console/buscar-ramas-ilegales-229.py <prog.elf> [vma_ini vma_fin]
"""
import os
import re
import subprocess
import sys

AQUI = os.path.dirname(os.path.abspath(__file__))
ENV = ". %s/env.sh >/dev/null 2>&1 && " % AQUI


def corre(cmd):
    return subprocess.run(["bash", "-c", ENV + cmd], capture_output=True, text=True).stdout


def secciones(elf):
    """-> [(nombre, vma, offset_de_archivo, tamano)] leyendo `objdump -h`"""
    out = corre("sh-elf-objdump -h %s" % elf)
    secs = []
    for linea in out.splitlines():
        m = re.match(r"\s*\d+\s+(\S+)\s+([0-9a-f]{8})\s+([0-9a-f]{8})\s+([0-9a-f]{8})\s+([0-9a-f]{8})", linea)
        if m:
            tam, vma, off = int(m.group(2), 16), int(m.group(3), 16), int(m.group(5), 16)
            secs.append((m.group(1), vma, off, tam))
    return secs


def simbolos(elf):
    out = corre("sh-elf-nm -n %s" % elf)
    syms = []
    for linea in out.splitlines():
        partes = linea.split()
        if len(partes) >= 3:
            try:
                syms.append((int(partes[0], 16), partes[2]))
            except ValueError:
                pass
    return syms


def dueno(syms, vma):
    izq, der = 0, len(syms) - 1
    mejor = (0, "?")
    while izq <= der:
        med = (izq + der) // 2
        if syms[med][0] <= vma:
            mejor = syms[med]
            izq = med + 1
        else:
            der = med - 1
    return "%s+0x%x" % (mejor[1], vma - mejor[0])


def es_rama_con_delay(op):
    if 0xA000 <= op <= 0xAFFF:
        return True                      # BRA disp12
    if 0xB000 <= op <= 0xBFFF:
        return True                      # BSR disp12
    if op in (0x000B, 0x002B):
        return True                      # RTS / RTE
    if (op & 0xF0FF) in (0x402B, 0x400B):
        return True                      # JMP @Rn / JSR @Rn
    if (op & 0xFF00) in (0x8D00, 0x8F00):
        return True                      # BT/S / BF/S
    return False


def es_rama(op):
    if es_rama_con_delay(op):
        return True
    if (op & 0xFF00) in (0x8900, 0x8B00):
        return True                      # BT / BF (sin delay slot)
    if (op & 0xF0FF) in (0x0003, 0x002B):
        return True                      # BSRF / BRAF
    return False


def main():
    elf = sys.argv[1]
    ini = int(sys.argv[2], 16) if len(sys.argv) > 2 else None
    fin = int(sys.argv[3], 16) if len(sys.argv) > 3 else None
    texto = open(elf, "rb").read()
    syms = simbolos(elf)

    hallados = 0
    for nombre, vma, off, tam in secciones(elf):
        if nombre != ".text":
            continue
        for i in range(0, tam - 4, 2):
            a = int.from_bytes(texto[off + i:off + i + 2], "little")
            b = int.from_bytes(texto[off + i + 2:off + i + 4], "little")
            if ini is not None and not (ini <= vma + i <= fin):
                continue
            if es_rama_con_delay(a) and es_rama(b):
                hallados += 1
                print("%08x  %04x %04x  %s" % (vma + i, a, b, dueno(syms, vma + i)))
    print("== %d pares rama+rama-en-delay-slot en .text de %s" % (hallados, os.path.basename(elf)))


if __name__ == "__main__":
    main()
