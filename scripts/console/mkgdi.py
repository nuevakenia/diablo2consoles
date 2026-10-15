#!/usr/bin/env python3
"""Arma un GDI (imagen de GD-ROM, el formato de la consola real y del GDEMU) a partir del
ISO de 2048 bytes por sector que genera mkiso.py.

Por que GDI y no .cue: Flycast trata un .cue/.iso como CD-R (disc->type != GdRom) y en ese
camino **descrambla** el binario de arranque antes de ejecutarlo (reios.cpp:
`descrambl = disc->type != GdRom; ... descrambl_buffer(...)`). Nuestro 1ST_READ.BIN va sin
scramble, asi que quedaba descramblado = basura ejecutandose y excepcion del SH-4 a los ~2,5 s.
Con un GDI el disco es GdRom, no hay descrambling, y ademas es lo que lee el GDEMU.

El GDI que espera Flycast (core/imgread/gdi.cpp):
  - al menos 3 pistas
  - pista 1: CTRL=4 (datos),  pista 2: CTRL=0 (audio),  pista 3: CTRL=4 (datos) y FADS=45000
  - tamano de sector 2352 o 2048
Y al arrancar lee el sector de boot desde el FAD fijo 45150 (= pista 3 + 150), o sea el
principio de la pista 3: ahi va el ISO con el IP.BIN en el offset 0.

Uso:  scripts/console/mkgdi.py <dir_de_build>          (usa <dir>/disc.iso)
Genera: <dir>/disc.gdi + track01.iso + track02.raw + track03.iso
"""
import os
import shutil
import sys

SECTOR = 2048
# la pista de audio va en 2352: es lo que mide un sector de audio de verdad
AUDIO_SECTOR = 2352


def main():
    if len(sys.argv) < 2:
        sys.exit("uso: mkgdi.py <dir_de_build>   (tiene que existir <dir>/disc.iso)")

    d = sys.argv[1]
    iso = os.path.join(d, "disc.iso")
    if not os.path.isfile(iso):
        sys.exit(f"no encuentro {iso}")

    t1 = os.path.join(d, "track01.iso")
    t2 = os.path.join(d, "track02.raw")
    t3 = os.path.join(d, "track03.iso")
    gdi = os.path.join(d, "disc.gdi")

    # Pista 1 (sesion 1, zona de BAJA densidad): aca vive el arbol ISO9660. Ojo con el detalle
    # del emulador: reios lee el PVD en baseFad+16, pero los directorios y los archivos los lee
    # en "FAD 150 + lba" FIJO (isofs.cpp: disc->ReadSectors(150 + lba, ...)), que es la pista 1.
    # Asi que la pista 1 lleva el mismo ISO completo (en un GD-ROM real la zona de baja densidad
    # tiene justamente el ISO con el binario de arranque).
    if os.path.exists(t1):
        os.remove(t1)
    try:
        os.link(iso, t1)
    except OSError:
        shutil.copyfile(iso, t1)

    # pista 2 (audio): un sector basta, no se reproduce nada.
    with open(t2, "wb") as f:
        f.write(b"\0" * AUDIO_SECTOR)

    # pista 3 (sesion 2, datos): la que se arranca. Es el ISO tal cual (con el IP.BIN en 0).
    if os.path.exists(t3):
        os.remove(t3)
    try:
        os.link(iso, t3)  # mismo contenido, sin duplicar 750 KB
    except OSError:
        shutil.copyfile(iso, t3)

    with open(gdi, "w") as f:
        f.write("3\n")
        f.write("1 0 4 %d track01.iso 0\n" % SECTOR)
        f.write("2 450 0 %d track02.raw 0\n" % AUDIO_SECTOR)
        f.write("3 45000 4 %d track03.iso 0\n" % SECTOR)

    print(f"GDI: {gdi} ({os.path.getsize(iso)} bytes de datos en la pista 3, arranca en FAD 45150)")


if __name__ == "__main__":
    main()
