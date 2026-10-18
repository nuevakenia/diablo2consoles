#!/usr/bin/env python3
"""DC-002.35: los numeros del HOST para el primer hito visible.

Calcula, con el MISMO polinomio que la consola (CRC-32 IEEE, el de zlib y el de hash/crc32 de Go):
  - el CRC-32 de los bytes del DC6 tal como esta en el MPQ;
  - el CRC-32 de los indices compuestos (el frame compuesto, ya decodificado);
  - el CRC-32 de la paleta ARGB1555 (dos bytes por color, el bajo primero, el mismo orden que sube
    la consola a la VRAM).
Los indices compuestos y la paleta los escribe `go run ./console/dc/bake -out <dir>` (el horneador
del host): aca solo se les pasa el CRC y se leen los numeros del .h, que es lo que el POC de la
consola tiene que repetir con SUS bytes (los que leyo del disco).

Uso:  python3 scripts/console/crc-dc6-235.py <dir_host> <name>
      (espera <dir_host>/<name>.dc6 y <dir_host>/gen/<name>.h)
"""
import re
import sys
import zlib


def crc(b):
    return zlib.crc32(b) & 0xFFFFFFFF


def main():
    if len(sys.argv) < 3:
        print(__doc__)
        return 2

    dir_host, name = sys.argv[1], sys.argv[2]

    raw = open("%s/%s.dc6" % (dir_host, name), "rb").read()
    print("HOST DC6 bytes=%d crc32=%08x" % (len(raw), crc(raw)))

    h = open("%s/gen/%s.h" % (dir_host, name), encoding="utf-8").read()

    w = int(re.search(r"#define %s_W (\d+)" % name.upper(), h).group(1))
    hh = int(re.search(r"#define %s_H (\d+)" % name.upper(), h).group(1))
    pw = int(re.search(r"#define %s_PW (\d+)" % name.upper(), h).group(1))
    ph = int(re.search(r"#define %s_PH (\d+)" % name.upper(), h).group(1))

    idx = bytes(int(v, 16) for v in re.findall(r"0x([0-9a-fA-F]{2})", h.split("idx[")[1].split("};")[0]))
    pal = [int(v, 16) for v in re.findall(r"0x([0-9a-fA-F]{4})", h.split("pal[")[1].split("};")[0])]

    print("HOST compuesto w=%d h=%d pw=%d ph=%d indices=%d crc32=%08x" % (w, hh, pw, ph, len(idx), crc(idx)))

    pb = b"".join(bytes((v & 0xFF, v >> 8)) for v in pal)
    print("HOST paleta ARGB1555 colores=%d bytes=%d crc32=%08x" % (len(pal), len(pb), crc(pb)))

    return 0


if __name__ == "__main__":
    sys.exit(main())
