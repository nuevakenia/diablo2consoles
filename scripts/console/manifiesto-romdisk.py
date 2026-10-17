#!/usr/bin/env python3
"""DC-002.19: escribe un ".manifiesto" por CADA directorio de un romdisk.

POR QUE: el FS ROM de KOS no deja ABRIR un directorio (medido: fs_open("/rd") da INVALID y fs_readdir no
llega), asi que os.ReadDir de la consola sale de un ".manifiesto" que se hornea DENTRO del romdisk junto
con los archivos, y que no puede desincronizarse porque lo genera el mismo build.

POR QUE RECURSIVO: scripts/console/build-dc.sh escribe el manifiesto de la RAIZ del romdisk (una linea por
entrada de $SRC/romdisk/*), y con eso alcanza para los POC cuyo romdisk es plano. El romdisk del ARRANQUE
(DC-002.19) tiene directorios adentro (`OpenDiablo2/config.json` y los archivos del juego en `juego/`), y
os.ReadDir("/rd/juego") lee "/rd/juego/.manifiesto": sin un manifiesto POR DIRECTORIO, un subdirectorio se
veria VACIO.

FORMATO (el mismo de build-dc.sh): nombre TAB bytes TAB archivo|directorio, una linea por entrada. El
tamano de un DIRECTORIO se declara 0 (el tamano de un directorio lo pone el sistema de archivos que lo
hospeda, no es un dato del contenido, y os.ReadDir no lo usa). El propio ".manifiesto" no se lista.

Uso:  python3 scripts/console/manifiesto-romdisk.py <directorio_del_romdisk>
"""
import os
import sys


def main():
    if len(sys.argv) != 2:
        print("uso: manifiesto-romdisk.py <directorio_del_romdisk>", file=sys.stderr)
        return 2

    raiz = sys.argv[1]
    if not os.path.isdir(raiz):
        print("no es un directorio: %s" % raiz, file=sys.stderr)
        return 1

    total = 0
    for base, dirs, archivos in os.walk(raiz):
        dirs.sort()
        archivos.sort()
        lineas = ["%s\t0\tdirectorio" % d for d in dirs]
        for a in archivos:
            if a == ".manifiesto":
                continue
            lineas.append("%s\t%d\tarchivo" % (a, os.path.getsize(os.path.join(base, a))))

        with open(os.path.join(base, ".manifiesto"), "w") as fh:
            fh.write("".join(l + "\n" for l in lineas))

        total += len(lineas)
        print("   manifiesto %s: %d entradas" % (base, len(lineas)))

    print("   manifiestos: %d entradas en total" % total)
    return 0


if __name__ == "__main__":
    sys.exit(main())
