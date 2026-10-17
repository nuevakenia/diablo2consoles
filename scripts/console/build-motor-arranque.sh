#!/bin/bash
# DC-002.13: enlaza EL MOTOR ENTERO y arranca su entry point de verdad en la consola.
#
# Por que un script propio y no build-dc.sh: build-dc.sh enlaza la stdlib minima y los .gox de los paquetes
# que el POC importe. Este enlaza los 142 paquetes del MOTOR (los que compila build-motor-all.sh, listados
# en orden-motor.txt) MAS la stdlib minima y los terceros (orden.txt): los 205 .gox. Es el unico binario del
# repo que lleva el motor entero adentro, y por eso tambien el que mide CUANTO PESA el motor.
#
# Uso:  scripts/console/build-motor-arranque.sh [dir_salida] [dir_gox]
#       D2 de Diablo II no hace falta: este arranque no hornea assets, justamente porque el motor los busca
#       en disco y ese es el punto que se mide.
#
# Antes:  bash scripts/console/build-std-gox.sh        (la stdlib minima + terceros)
#         bash scripts/console/build-motor-all.sh      (los 142 del motor -> orden-motor.txt)
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
POC="${POC:-motor-arranque}"
SRC="$REPO/console/dc/poc/$POC"
OUT="${1:-/tmp/dcbuild/$POC}"
GEN="${2:-${GODC_GOX:-$DC_ROOT/gox}}"

[ -f "$GEN/orden.txt" ] || {
    echo "falta $GEN/orden.txt (la stdlib minima): corre scripts/console/build-std-gox.sh" >&2
    exit 1
}
[ -f "$GEN/orden-motor.txt" ] || {
    echo "falta $GEN/orden-motor.txt (el motor): corre scripts/console/build-motor-all.sh '$GEN'" >&2
    exit 1
}
[ -f "$SRC/main.go" ] || { echo "no encuentro $SRC/main.go" >&2; exit 1; }

mkdir -p "$OUT"

N_MOTOR="$(wc -l < "$GEN/orden-motor.txt")"
N_STD="$(wc -l < "$GEN/orden.txt")"

echo "== 1/4 compilando el arranque (POC $POC) =="
sh-elf-gccgo $GODC_GOFLAGS -I "$GEN" -c "$SRC"/*.go -o "$OUT/main.o"

echo "== 2/4 el C: los puentes de la stdlib minima + los del POC =="
STD_C_OBJS=""
for f in "$REPO/console/dc/std/csrc"/*.c; do
    [ -f "$f" ] || continue
    o="$OUT/stdlib-$(basename "${f%.c}").o"
    ZLIB_INC="${ZLIB_INC:-$DC_ROOT/kos-ports/zlib/inst/include}"
    kos-cc -O2 -ml -m4-single -I"$ZLIB_INC" -c "$f" -o "$o"
    STD_C_OBJS="$STD_C_OBJS $o"
done

EXTRA_OBJS=""
for f in "$SRC"/*.c "$SRC"/csrc/*.c; do
    [ -f "$f" ] || continue
    o="$OUT/$(basename "${f%.c}").o"
    kos-cc -O2 -ml -m4-single -I"$SRC" -I"$SRC/csrc" -c "$f" -o "$o"
    EXTRA_OBJS="$EXTRA_OBJS $o"
done

echo "== 3/4 enlazando el MOTOR ENTERO ($N_MOTOR paquetes del motor + $N_STD de stdlib/terceros) =="
# DC-002.14: DC_LDEXTRA existe para medir el RECORTE del binario (-Wl,--gc-sections, que necesita los .gox
# compilados con -ffunction-sections -fdata-sections) sin duplicar este script. Vacio por defecto: el
# binario de siempre (el que mide la tarjeta DC-002.13) sale con la MISMA linea de enlace que antes.
# shellcheck disable=SC2046,SC2086
kos-cc -o "$OUT/prog.elf" "$OUT/main.o" $EXTRA_OBJS $STD_C_OBJS \
    $(tr '\n' ' ' < "$GEN/orden.txt") $(tr '\n' ' ' < "$GEN/orden-motor.txt") \
    ${DC_LDEXTRA:-} $DC_LDPATHS $DC_LIBS_PRE $DC_LIBS_POST

echo "== 4/4 binario de arranque (1ST_READ.BIN) + IP.BIN + imagen =="
sh-elf-objcopy -R .stack -O binary "$OUT/prog.elf" "$OUT/1ST_READ.BIN"
"$DC_MAKEIP" -f "$OUT/IP.BIN" >/dev/null
"$DC_PYTHON" "$HERE/mkiso.py" "$OUT/disc.iso" "$OUT/IP.BIN" "$OUT/1ST_READ.BIN"
"$DC_PYTHON" "$HERE/mkgdi.py" "$OUT"

# prog-emu.elf: el MISMO binario sin el debug ni el .go_export, para correrlo en Flycast.
#
# POR QUE (medido, no supuesto): el cargador de ELF de Flycast RECHAZA cualquier archivo de mas de 16 MB
# -- `if (size == 0 || size > 16_MB) return false;` en core/reios/reios_elf.cpp:18 -- y lo unico que
# contesta es "Flycast has stopped: Failed to open ELF". Con 205 .gox adentro el ELF sale de 43.876.988
# bytes (el debug son 14 de esos y el .go_export otros 11,9, ninguno de los dos se carga en la consola).
# Sacandolos queda en 13.707.552 bytes y el emulador lo carga. El que se carga de verdad son las
# secciones ALLOC: eso lo informa el `sh-elf-size -A` de abajo.
sh-elf-objcopy -R .go_export -R .comment --strip-debug "$OUT/prog.elf" "$OUT/prog-emu.elf"
sh-elf-strip -s "$OUT/prog-emu.elf"

echo
echo "ELF:        $OUT/prog.elf   ($(stat -c%s "$OUT/prog.elf") bytes, con debug: para el GDB y el 1ST_READ)"
echo "ELF (emu):  $OUT/prog-emu.elf   ($(stat -c%s "$OUT/prog-emu.elf") bytes: el que abre Flycast)"
echo "1ST_READ:   $OUT/1ST_READ.BIN   ($(stat -c%s "$OUT/1ST_READ.BIN") bytes)"
echo "secciones que SE CARGAN en los 16 MB:"
sh-elf-size -A "$OUT/prog.elf" | sed 's/^/   /'
