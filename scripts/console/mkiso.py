#!/usr/bin/env python3
"""Arma una imagen de GD-ROM arrancable para Dreamcast.

Reemplaza al mkisofs -G del flujo clasico (en este host no hay mkisofs ni genisoimage):
  1) construye un ISO9660 normal con pycdlib,
  2) inyecta el IP.BIN en los primeros 16 sectores (32 KB) = area de sistema del ISO,
que es exactamente lo que hace `mkisofs -G ip.bin`.

Uso:  python3 mkiso.py <salida.iso> <IP.BIN> <archivo1> [archivo2 ...]
Los archivos se agregan en la raiz del ISO con su nombre en mayusculas.
"""
import sys
import pycdlib

def main():
    if len(sys.argv) < 4:
        print(__doc__)
        return 2
    out, ipbin, files = sys.argv[1], sys.argv[2], sys.argv[3:]

    iso = pycdlib.PyCdlib()
    iso.new(interchange_level=3, joliet=3, vol_ident="D2CONSOLES",
            sys_ident="SEGA SEGAKATANA", vol_set_ident="SEGA GD-ROM")
    for f in files:
        import os
        name = os.path.basename(f).upper()
        iso.add_file(f, iso_path="/%s;1" % name)
    iso.write(out)
    iso.close()

    with open(ipbin, "rb") as fh:
        ip = fh.read()
    with open(out, "rb") as fh:
        data = fh.read()
    if len(ip) != 32768:
        print("AVISO: IP.BIN mide %d bytes, se esperaban 32768" % len(ip))
    with open(out, "wb") as fh:
        fh.write(ip[:32768].ljust(32768, b"\0") + data[32768:])

    # Flycast no abre un .iso suelto (lo manda a libcdio y falla): hay que envolverlo en un .cue.
    # Con MODE1/2048 el emulador lee los sectores de 2048 tal cual, sin rearmar EDC/ECC.
    import os
    cue = os.path.splitext(out)[0] + ".cue"
    with open(cue, "w") as fh:
        fh.write('FILE "%s" BINARY\n  TRACK 01 MODE1/2048\n    INDEX 01 00:00:00\n' % os.path.basename(out))
    print("CUE: %s (TRACK 01 MODE1/2048 -> %s)" % (cue, os.path.basename(out)))
    print("ISO: %s (%d bytes, con IP.BIN en el area de sistema)" % (out, len(ip[:32768]) + len(data[32768:])))
    return 0

if __name__ == "__main__":
    sys.exit(main())
