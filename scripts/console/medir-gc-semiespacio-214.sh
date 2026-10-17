#!/bin/bash
# DC-002.14: LA ESCALERA DEL SEMIESPACIO DEL GC (medida, una configuracion por corrida en Flycast).
#
# Por que una escalera y no un numero: `gc_init` pide 2 x GC_SEMISPACE_SIZE (runtime/gc_heap.c:37-38) y al
# binario de 13.910.992 bytes le quedan 610.304 libres. Con eso, la unica configuracion del runtime que
# entra es 2 x 256 KB = 524.288. Los valores intermedios no son adorno: dicen CUAL es la peticion que falla
# (el diff del mensaje es el tamano del memalign que no consiguio RAM), y con eso se ve que el fallo se
# corre de la primera peticion a la segunda.
#
# Uso:  bash scripts/console/medir-gc-semiespacio-214.sh [lista de KB]   (por defecto: 2048 1024 512 256)
#         GOX=<dir>   el gox del motor (por defecto /tmp/gox-0028, el que mide DC-002.13)
#         SEGS=<n>    segundos de cada corrida (por defecto 15)
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

KBS="${*:-2048 1024 512 256}"
GOX="${GOX:-/tmp/gox-0028}"
SEGS="${SEGS:-15}"
OUT=/tmp/dcbuild/motor-arranque
RES=/tmp/dcbuild/gc-semiespacio-214.txt
: > "$RES"

arranque() { # $1 = dir de la corrida; veredicto de 4 estados (medido, no supuesto)
    # El estado NO es binario: con el semiespacio grande el runtime muere en gc_init; con 1024 KB pasa
    # gc_init y muere despues en gc_external_alloc (RAM otra vez); con 512 KB o menos pasa las dos y se
    # cae en un panic de Go del init (eso ya NO es RAM: es el defecto siguiente, y es lo que se quiere ver).
    if grep -qa 'gc_init: alloc failed' "$1/flycast.log"; then
        echo "cae-gc-init diff $(grep -a -o 'diff [0-9]*' "$1/flycast.log" | tail -1 | cut -d' ' -f2)"
    elif grep -qa 'gc_external_alloc' "$1/flycast.log"; then
        echo "cae-gc-external diff $(grep -a -o 'diff [0-9]*' "$1/flycast.log" | tail -1 | cut -d' ' -f2)"
    elif grep -qa 'panic:' "$1/flycast.log"; then
        echo "pasa-el-techo-de-RAM (panic del init: $(grep -a -m1 'panic:' "$1/flycast.log" | tr -d '\r'))"
    else
        echo "arranca $(grep -ac 'MOTOR-ARRANQUE' "$1/flycast.log") marcas"
    fi
}

for KB in $KBS; do
    echo "== GC_SEMISPACE_SIZE_KB=$KB  (el GC pide $((2 * KB * 1024)) bytes)"
    python3 "$HERE/fix-libgodc-gc.py" "$KB" --build | sed 's/^/   /'
    bash "$HERE/build-motor-arranque.sh" "$OUT" "$GOX" > /tmp/dc-gc-relink-$KB.log 2>&1 || {
        echo "   EL ENLACE FALLO (ver /tmp/dc-gc-relink-$KB.log):" >&2
        tail -3 /tmp/dc-gc-relink-$KB.log >&2
        exit 1
    }
    cargado="$(python3 "$HERE/medir-cargado.py" "$OUT/prog.elf" | head -1 | sed 's/.*: \([0-9]*\) bytes.*/\1/')"
    rm -rf "/tmp/dc-run-gc-$KB"
    CAPTURE=none bash "$HERE/run-flycast.sh" "$OUT/prog-emu.elf" "$SEGS" "/tmp/dc-run-gc-$KB" > /dev/null 2>&1
    v="$(arranque "/tmp/dc-run-gc-$KB")"
    echo "   cargado: $cargado   arranque: $v"
    echo "KB=$KB cargado=$cargado pide=$((2 * KB * 1024)) $v" >> "$RES"
    if [ "${v%% *}" = "arranca" ]; then
        grep -a 'MOTOR-ARRANQUE' "/tmp/dc-run-gc-$KB/flycast.log" | tr -d '\r' | head -4 | sed 's/^/      /'
    else
        grep -a -E 'Out of memory|fatal error|panic:|Memory:' "/tmp/dc-run-gc-$KB/flycast.log" |
            tr -d '\r' | head -4 | sed 's/^/      /'
    fi
done

echo
echo "== RESUMEN (guardado en $RES)"
cat "$RES"
