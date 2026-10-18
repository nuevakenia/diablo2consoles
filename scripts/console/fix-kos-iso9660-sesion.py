#!/usr/bin/env python3
"""DC-002.27: parche IDEMPOTENTE al driver iso9660 de KOS para que monte la SESION DE ALTA DENSIDAD de un
GD-ROM (el arbol ISO9660 de un disco de verdad empieza en el LBA 45000) y sume su base a las lecturas.

POR QUE (medido, no supuesto):
  1. El driver arma la base de la sesion con `cdrom_locate_data_track()` sobre el TOC de la zona de BAJA
     densidad -- o sea la pista 1, la que tiene el ISO que le pasamos -- pero TODAS las lecturas del arbol
     suman solo el 150 de siempre, que vale para una sesion que empieza en el LBA 0. Con un disco de
     540.428.288 bytes (263.881 sectores) el arbol NO entra en la zona de baja (45.000 sectores = 92 MB) y
     las lecturas de mas alla del FAD 45.150 las contesta la zona de ALTA (la pista 3, que empieza en el
     LBA 45.000): /JUEGO/PATCH_D2.MPQ (LBA 261.428 -> FAD 261.578) volvia 41 41 85 86... en vez de
     4d 50 51 1a (su cabecera), mientras que /1ST_READ.BIN (LBA 25) se leia bien (37 d0 02 01). O sea: no
     es el emulador leyendo de menos, es el driver leyendo con la base de la sesion equivocada.
  2. Un GD-ROM de verdad tiene la sesion del juego en la zona de ALTA (por eso el arranque lee el sector de
     boot en el FAD fijo 45.150 = el sector 0 del ISO). Con la base de la sesion en 45.000 todas las
     lecturas caen donde tienen que caer y el disco entero (515 MB) es visible en /cd.

QUE HACE:
  - si el TOC de la zona de alta tiene una pista de datos, ESA es la sesion (base 45000); si no (un CD-R
    comun), queda la de la zona de baja (base 0) como siempre;
  - guarda la base de la sesion en LBA (`session_lba`) y la suma en las lecturas del arbol: los
    directorios, los datos de los archivos (los dos caminos: DMA y cache) y el readdir.

Con la sesion de baja el resultado es byte por byte el de antes (session_lba = 0): no cambia nada de lo que
ya andaba (el romdisk, /rd, el arranque desde el ISO, todo lo de DC-002.19 a DC-002.26).

Uso:  scripts/console/fix-kos-iso9660-sesion.py [ruta_al_kos]     (por defecto $KOS_BASE)
      scripts/console/fix-kos-iso9660-sesion.py --ver             (solo dice si esta puesto)
      scripts/console/fix-kos-iso9660-sesion.py --revertir        (deja el archivo como estaba)
Despues hay que RECOMPILAR KOS (make -C $KOS_BASE) para que la lib nueva la use el enlace.
"""
import os
import sys

MARCA = "DC-002.27"
REL = "kernel/arch/dreamcast/fs/fs_iso9660.c"

DECL_VIEJO = "    CDROM_TOC   toc;"
DECL_NUEVO = "    CDROM_TOC   toc;\n    CDROM_TOC   toc_alta;"

BLOQUE_V1 = """    if((i = cdrom_read_toc(&toc, false)) != 0)
        return i;

    if(!(session_base = cdrom_locate_data_track(&toc))) {
        /* DC-002.27: un GD-ROM puede tener TODA la sesion de datos en la zona de ALTA densidad (el
           arbol ISO9660 empieza en el LBA 45000) y ahi el TOC de la de baja no tiene ninguna pista
           de datos. Sin esto el driver se rendia y /cd no existia. */
        if((i = cdrom_read_toc(&toc, true)) != 0)
            return i;

        if(!(session_base = cdrom_locate_data_track(&toc)))
            return -1;
    }

    session_lba = session_base - 150;"""

BLOQUE_V2 = """    if((i = cdrom_read_toc(&toc, false)) != 0)
        return i;

    /* DC-002.27: la sesion de datos de un GD-ROM vive en la zona de ALTA densidad (el arbol ISO9660
       empieza en el LBA 45000 y el arranque lee el sector de boot en el FAD 45150): si el TOC de la de
       alta trae una pista de datos, ESA es la sesion. Si no (un CD-R comun), queda la de la zona de baja,
       como siempre. */
    session_base = 0;
    if(cdrom_read_toc(&toc_alta, true) == 0) {
        session_base = cdrom_locate_data_track(&toc_alta);
    }

    if(!session_base) {
        if(!(session_base = cdrom_locate_data_track(&toc)))
            return -1;
    }

    session_lba = session_base - 150;"""

BLOQUE_ORIGINAL = """    if((i = cdrom_read_toc(&toc, false)) != 0)
        return i;

    if(!(session_base = cdrom_locate_data_track(&toc)))
        return -1;"""

DECL_SESSION = """
/* DC-002.27: base de la sesion en LBA (0 en la zona de baja densidad, 45000 en la de alta). Las
   lecturas del arbol suman esta base: el +150 de siempre solo vale para una sesion que empieza en
   el LBA 0, y un GD-ROM de verdad tiene el arbol en la zona de alta. */
static uint32 session_lba = 0;"""

# CAMBIOS de un archivo SIN parchear (el estado del KOS instalado)
CAMBIOS = [
    (
        "la base de la sesion en LBA, al lado de session_base",
        "/* Root FS session location (in sectors) */\nstatic uint32 session_base = 0;",
        "/* Root FS session location (in sectors) */\nstatic uint32 session_base = 0;\n" + DECL_SESSION,
    ),
    ("la variable del TOC de la zona de alta", DECL_VIEJO, DECL_NUEVO),
    ("la sesion de datos se busca en la zona de alta densidad", BLOQUE_ORIGINAL, BLOQUE_V2),
    (
        "los directorios se leen con la base de la sesion",
        "        c = biread(dir_extent);",
        "        c = biread(session_lba + dir_extent);",
    ),
    (
        "los datos de un archivo (camino DMA) se leen con la base de la sesion",
        "            c = cdrom_read_sectors_ex(outbuf,\n"
        "                fd->first_extent + (fd->ptr / 2048) + 150,\n"
        "                thissect,\n"
        "                CDROM_READ_DMA);",
        "            c = cdrom_read_sectors_ex(outbuf,\n"
        "                session_lba + fd->first_extent + (fd->ptr / 2048) + 150,\n"
        "                thissect,\n"
        "                CDROM_READ_DMA);",
    ),
    (
        "los datos de un archivo (camino del cache) se leen con la base de la sesion",
        "            c = bdread(fd->first_extent + fd->ptr / 2048);",
        "            c = bdread(session_lba + fd->first_extent + fd->ptr / 2048);",
    ),
    (
        "el readdir se lee con la base de la sesion",
        "        c = biread(fd->first_extent + fd->ptr / 2048);",
        "        c = biread(session_lba + fd->first_extent + fd->ptr / 2048);",
    ),
]

# CAMBIOS para pasar del primer parche (v1: solo miraba la zona de alta si la de baja no tenia datos) al
# definitivo (v2: la de alta manda cuando tiene pista de datos). El v1 no alcanzaba: el .gdi de esta placa
# DEBE tener la pista 1 de datos (Flycast: "Track 1 must be a data track"), asi que la zona de baja siempre
# tiene una pista de datos y el v1 nunca miraba la de alta.
CAMBIOS_DESDE_V1 = [
    ("la variable del TOC de la zona de alta", DECL_VIEJO, DECL_NUEVO),
    ("la zona de alta manda si trae pista de datos", BLOQUE_V1, BLOQUE_V2),
]


def aplica(fuente, cambios, que):
    for descripcion, viejo, nuevo in cambios:
        n = fuente.count(viejo)
        if n != 1:
            sys.exit("KOS-ISO9660: el patron de '%s' (%s) aparece %d veces (se esperaba 1): el archivo no "
                     "es el que se midio, no lo toco" % (descripcion, que, n))

        fuente = fuente.replace(viejo, nuevo)
        print("KOS-ISO9660: %s" % descripcion)

    return fuente


def main():
    args = [a for a in sys.argv[1:] if not a.startswith("--")]
    kos = args[0] if args else os.environ.get("KOS_BASE", os.path.expanduser("~/dreamcast/kos"))
    ruta = os.path.join(kos, REL)

    if not os.path.isfile(ruta):
        sys.exit("no encuentro %s" % ruta)

    with open(ruta) as fh:
        fuente = fh.read()

    puesto = MARCA in fuente
    es_v1 = BLOQUE_V1 in fuente

    if "--ver" in sys.argv:
        print("KOS-ISO9660: %s (%s)" % ("PARCHEADO" if puesto else "sin parchear", ruta))
        return 0

    if "--revertir" in sys.argv:
        if not puesto:
            print("KOS-ISO9660: no estaba parcheado, nada que revertir")
            return 0

        if es_v1:
            fuente = aplica(fuente, [(d, n, v) for d, v, n in CAMBIOS_DESDE_V1], "revertir v1")

        fuente = aplica(fuente, [(d, n, v) for d, v, n in CAMBIOS], "revertir")

        with open(ruta, "w") as fh:
            fh.write(fuente)

        print("KOS-ISO9660: revertido %s (recompilar KOS: make -C %s)" % (ruta, kos))
        return 0

    if puesto:
        if es_v1:
            fuente = aplica(fuente, CAMBIOS_DESDE_V1, "actualizar de v1 a v2")

            with open(ruta, "w") as fh:
                fh.write(fuente)

            print("KOS-ISO9660: actualizado a v2 %s (recompilar KOS: make -C %s)" % (ruta, kos))
        else:
            print("KOS-ISO9660: ya estaba parcheado (%s)" % ruta)

        return 0

    fuente = aplica(fuente, CAMBIOS, "aplicar")

    with open(ruta, "w") as fh:
        fh.write(fuente)

    print("KOS-ISO9660: parcheado %s (recompilar KOS: make -C %s)" % (ruta, kos))

    return 0


if __name__ == "__main__":
    sys.exit(main())
