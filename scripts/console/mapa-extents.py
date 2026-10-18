#!/usr/bin/env python3
"""Mapa del ISO contra lo que la consola lee: imprimir la LBA (extent) de cada archivo y los bytes que hay
en el ISO en esa LBA y en las LBA de al lado (+-150 sectores), para cazar un corrimiento de sesion.

Uso: mapa-extents-227.py <iso> <ruta_interna> [<ruta_interna> ...]
"""
import sys

import pycdlib


def dump(fh, lba, n=16):
    fh.seek(lba * 2048)
    return " ".join("%02x" % b for b in fh.read(n))


def main():
    iso_path, rutas = sys.argv[1], sys.argv[2:]
    iso = pycdlib.PyCdlib()
    iso.open(iso_path)

    with open(iso_path, "rb") as fh:
        for ruta in rutas:
            rec = iso.get_record(iso_path=ruta)
            lba = rec.extent_location()
            print("%s lba=%d bytes=%d" % (ruta, lba, rec.data_length))
            print("   en su LBA     : %s" % dump(fh, lba))
            print("   LBA+150       : %s" % dump(fh, lba + 150))
            print("   LBA-150       : %s" % dump(fh, lba - 150))
            print("   su LBA, sector +1: %s" % dump(fh, lba + 1))

    iso.close()

    return 0


if __name__ == "__main__":
    sys.exit(main())
