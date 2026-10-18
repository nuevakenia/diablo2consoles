#!/usr/bin/env python3
"""DC-002.29: PARCHEAR BYTES EN SU LUGAR (por direccion) de un ELF ya enlazado.

POR QUE: el defecto de la tarjeta es de LAYOUT, asi que el instrumento no puede agregar ni mover un byte.
Este escribe UNOS POCOS BYTES en una direccion (una instruccion, un literal) y deja el binario del mismo
tamano y con las mismas direcciones.

USO
    python3 scripts/console/parchar-memoria-229.py <prog.elf> <salida.elf> <vma_hex> <bytes_hex>
    # p.ej. la longitud del write del aviso: mov #75,r5 esta en 0x8c0126f0 -> el byte del inmediato es 0x8c0126f1
    python3 scripts/console/parchar-memoria-229.py prog.elf out.elf 8c0126f1 04
"""
import os
import re
import subprocess
import sys

ENV = "source %s/../console/env.sh && " % os.path.dirname(os.path.abspath(__file__))


def secciones(elf):
    out = subprocess.run(["bash", "-c", ENV + "sh-elf-objdump -h %s" % elf], capture_output=True, text=True).stdout
    secs = {}
    for line in out.splitlines():
        m = re.match(r"\s*\d+\s+(\S+)\s+([0-9a-f]{8})\s+([0-9a-f]{8})\s+([0-9a-f]{8})\s+([0-9a-f]{8})", line)
        if m:
            secs[m.group(1)] = (int(m.group(3), 16), int(m.group(5), 16), int(m.group(2), 16))
    return secs


def main():
    elf, salida, vma, bytes_hex = sys.argv[1:5]
    vma = int(vma, 16)
    nuevos = bytes.fromhex(bytes_hex)
    secs = secciones(elf)
    for nombre, (svma, soff, stam) in secs.items():
        if svma <= vma and vma + len(nuevos) <= svma + stam:
            off = soff + (vma - svma)
            datos = bytearray(open(elf, "rb").read())
            antes = bytes(datos[off:off + len(nuevos)])
            datos[off:off + len(nuevos)] = nuevos
            open(salida, "wb").write(bytes(datos))
            print("PARCHE %s: 0x%08x (seccion %s, offset de archivo 0x%x) %s -> %s"
                  % (os.path.basename(elf), vma, nombre, off, antes.hex(), nuevos.hex()))
            return
    sys.exit("la direccion 0x%08x no cae en ninguna seccion cargable de %s" % (vma, elf))


if __name__ == "__main__":
    main()
