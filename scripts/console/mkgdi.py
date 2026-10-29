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

    # Pista 2 (audio): un sector basta, no se reproduce nada.
    #
    # DC-002.61: DONDE VA ESTA PISTA IMPORTA. El arranque vive en el sector 25 del ISO (FAD 175 =
    # 150 + 25) y el ejecutable ocupa los FAD 175..175+N. Si el sector de audio cae dentro de ese
    # rango, el emulador le contesta ese FAD a la pista de audio (readSector recorre las pistas de
    # atras para adelante y el audio gana) y devuelve 2048 B de ceros de un sector de 2352: queda un
    # agujero de un sector en medio del .text, en RAM. Medido con el FAD 600 (LBA 450, el valor
    # viejo): borraba el offset 0xD4800 del binario, que caia dentro de _pvr_allocate_buffers; el
    # flujo se iba a la zona vacia, no ejecutaba el epilogo, la pila quedaba 0x20 abajo y el rts
    # saltaba a 0x8c42e020. Como el agujero esta siempre en el mismo offset pero el codigo que lo
    # habita cambia con cada compilacion, el sintoma se movia con el tamano del binario.
    # Por eso el audio va bien lejos del ejecutable y antes de la zona de alta.
    AUDIO_LBA = 30000
    binario = os.path.join(d, "1ST_READ.BIN")
    if os.path.isfile(binario):
        sectores_bin = (os.path.getsize(binario) + SECTOR - 1) // SECTOR
        fin_bin_fad = 150 + 25 + sectores_bin
        if 150 + AUDIO_LBA <= fin_bin_fad:
            sys.exit(f"el audio en LBA {AUDIO_LBA} (FAD {150 + AUDIO_LBA}) cae dentro del ejecutable "
                     f"(termina en FAD {fin_bin_fad}): correrlo mas arriba")
    with open(t2, "wb") as f:
        f.write(b"\0" * AUDIO_SECTOR)

    # Pista 3 (sesion 2, datos): la que se arranca. Es el ISO tal cual (con el IP.BIN en 0).
    if os.path.exists(t3):
        os.remove(t3)
    try:
        os.link(iso, t3)  # mismo contenido, sin duplicar 512 MB
    except OSError:
        shutil.copyfile(iso, t3)

    # cuantos sectores ocupa la pista 1 (el ISO): la pista 2 va justo despues
    sectores_datos = (os.path.getsize(iso) + SECTOR - 1) // SECTOR

    # DC-002.27: DONDE VIVE LA SESION DE DATOS.
    #
    # El .gdi tiene que llevar la pista 1 de DATOS (Flycast lo exige: "Track 1 must be a data track", medido
    # al intentar dejar la zona de baja solo con audio). Y la pista 1 lleva el ISO entero, pero esa zona de
    # BAJA densidad de un GD-ROM son los primeros 45.000 sectores (92 MB): el arbol de un disco con los MPQ
    # adentro no entra ahi (el MPQ esta en el LBA 261.428). Las lecturas de mas alla del FAD 45.150 las
    # contesta la zona de ALTA, o sea la pista 3 -- que es el mismo ISO y empieza en el LBA 45.000, igual
    # que en un GD-ROM de verdad (por eso el arranque lee el sector de boot en el FAD 45.150 = el sector 0
    # del ISO).
    #
    # Medido: sin eso, /JUEGO/PATCH_D2.MPQ se leia 41 41 85 86 ... en vez de 4d 50 51 1a (su cabecera),
    # mientras que /1ST_READ.BIN (LBA 25) se leia bien: el defecto no era del emulador, era del driver
    # leyendo con la base de la sesion equivocada. Lo arregla
    # scripts/console/fix-kos-iso9660-sesion.py (la sesion de la zona de alta manda cuando trae pista de
    # datos). GDI_BAJA=datos no cambia nada del archivo -- se deja para el mutante del corpus, que revierte
    # el parche de KOS en vez del .gdi.
    with open(gdi, "w") as f:
        f.write("3\n")
        f.write("1 0 4 %d track01.iso 0\n" % SECTOR)
        f.write("2 %d 0 %d track02.raw 0\n" % (AUDIO_LBA, AUDIO_SECTOR))
        f.write("3 45000 4 %d track03.iso 0\n" % SECTOR)

    print(f"GDI: {gdi} ({os.path.getsize(iso)} bytes = {sectores_datos} sectores; el arranque lee en FAD "
          f"45150; la sesion de datos se monta de la zona de alta)")


if __name__ == "__main__":
    main()
