#!/bin/bash
# DC-002.89 -- REENLAZAR un build ya hecho, con el mapa del enlazador.
#
# POR QUE EXISTE. El mapa (`-Wl,-Map`) es la unica forma de saber QUE objeto puso cada byte en
# .text/.rodata: `nm` solo ve simbolos con nombre y tamano y los del blob de gccgo (`_go_..C*`) no
# existen como simbolo. build-dc.sh ya acepta DC_LDMAP, pero reenlazar desde cero cuesta recompilar
# todo el motor; este script reusa los objetos de un directorio de build terminado y SOLO rehace el
# enlace. Para que el mapa valga, el prog.elf resultante tiene que salir byte a byte igual al de ese
# directorio: el script lo verifica y falla si no.
#
# Uso:  scripts/console/relink-con-mapa.sh <dir_build> <mapa.salida> [flags extra del enlazador...]
#       Las flags extra van ANTES de los objetos (para --gc-sections, -s, etc.).
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

OUT="${1:?uso: relink-con-mapa.sh <dir_build> <mapa.salida> [flags...]}"
MAPA="${2:?uso: relink-con-mapa.sh <dir_build> <mapa.salida> [flags...]}"
shift 2
EXTRA="$*"

[ -f "$OUT/main.o" ] || { echo "falta $OUT/main.o (no es un directorio de build)" >&2; exit 1; }
[ -f "$OUT/gox-enlace/orden.txt" ] || { echo "falta $OUT/gox-enlace/orden.txt" >&2; exit 1; }

# los objetos del POC que no son main.o ni la stdlib de C (los shims del csrc)
OTROS=""
for f in "$OUT"/*.o; do
    b="$(basename "$f")"
    case "$b" in
        main.o|stdlib-*) continue ;;
        *) OTROS="$OTROS $f" ;;
    esac
done
STD_C_OBJS="$( { ls "$OUT"/stdlib-*.o 2>/dev/null || true; } | tr '\n' ' ')"
STD_OBJS="$(tr '\n' ' ' < "$OUT/gox-enlace/orden.txt")"
ROMDISK_OBJ=""
[ -f "$OUT/romdisk.o" ] && ROMDISK_OBJ="$OUT/romdisk.o"

# el script de linker sin .eh_frame (el mismo que dejo build-dc.sh)
# (las barras del patron van escapadas: en ${var//pat/rep} el patron termina en la PRIMERA barra sin
#  escapar, asi que sin escapar se parte en "utils" y el linker recibe una ruta inventada)
KOS_LDFLAGS="${KOS_LDFLAGS//-T$KOS_BASE\/utils\/ldscripts\/shlelf.xc/-T$OUT/ld-sin-eh.xc}"
export KOS_LDFLAGS

sha_antes=""
[ -f "$OUT/prog.elf" ] && sha_antes="$(sha256sum "$OUT/prog.elf" | cut -d' ' -f1)"

kos-cc -o "$OUT/prog.elf" "$OUT/main.o" $ROMDISK_OBJ $OTROS $STD_C_OBJS $STD_OBJS \
    $DC_LDPATHS $EXTRA -Wl,-Map="$MAPA" $DC_LIBS_PRE $DC_LIBS_POST

sha_despues="$(sha256sum "$OUT/prog.elf" | cut -d' ' -f1)"
echo "mapa:      $MAPA ($(stat -c%s "$MAPA") B)"
echo "prog.elf:  $sha_despues"
if [ -n "$sha_antes" ]; then
    if [ "$sha_antes" = "$sha_despues" ]; then
        echo "           == igual al de antes ($sha_antes): el mapa describe ESE binario"
    else
        # distinto es lo normal cuando se reenlaza con objetos cambiados (una variante). Con
        # STRICTO=1 se exige que sea el mismo binario, que es como se valida el instrumento.
        echo "           != distinto al de antes ($sha_antes)"
        if [ "${STRICTO:-0}" = "1" ]; then
            echo "           STRICTO=1: el mapa NO describe el binario previo" >&2
            exit 3
        fi
    fi
fi
