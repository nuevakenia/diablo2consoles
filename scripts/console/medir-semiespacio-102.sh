#!/bin/bash
# DC-002.102 -- LA ESCALERA DEL SEMIESPACIO DEL COMPOSITOR CON EL VIDEO ENCENDIDO.
#
# POR QUE EXISTE. DC-002.93 midio la escalera del MOTOR (2048/1536/1024/768) y dejo escrito que el valor
# "es del toolchain: un valor a la vez" -- libgodc es una biblioteca ESTATICA, asi que el semiespacio se
# fija en el vendor, se rehace `libgodc.a` y se reenlaza. Lo que NO estaba medido es el valor del
# COMPOSITOR DEL PUEBLO con el video encendido (el POC `mapa-102`), que es otro binario.
#
# LO QUE MIDE, por cada valor de la escalera:
#   1. el censo del arranque (imagen, los dos semiespacios, el monton de C que queda: `medir-hueco-gc-293.sh`)
#   2. cuanto monton de C hay en cada paso (`mallinfo` del shim del POC: usado/libre/arena/sbrk)
#   3. si el pueblo SE COMPONE (`MAPA102: MAPA tiles= ... crcGrilla= -1596781041`) y cuanto tarda
#   4. donde muere, si muere, con el pedido y el margen al techo
#
# Uso:  bash scripts/console/medir-semiespacio-102.sh 1792 [segundos]
#       bash scripts/console/medir-semiespacio-102.sh 2048 1536 1792 1664   (varios valores)
#       SAL=/tmp/dc102/escalera   directorio de las corridas (por defecto /tmp/dc102/escalera)
#
# DEJA el vendor en el ULTIMO valor que corrio (es lo que hay que recordar: el semiespacio es del
# toolchain). Para volver al valor de la rama: python3 scripts/console/fix-libgodc-gc.py 1792 --build
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)"
SAL="${SAL:-/tmp/dc102/escalera}"
SEGS_DEF="${SEGS:-140}"
cd "$REPO" || exit 1
# shellcheck source=env.sh
. "$HERE/env.sh" >/dev/null 2>&1

[ "$#" -ge 1 ] || { echo "uso: medir-semiespacio-102.sh <KB> [KB ...]   (SEGS=segundos, SAL=dir)" >&2; exit 2; }
mkdir -p "$SAL"
printf 'KB\tsemiespacios B\t1ST_READ\tmontonC B\tcomponer\tcrcGrilla\tms\tdesenlace\n' > "$SAL/tabla.tsv"

for KB in "$@"; do
    OUT="$SAL/kb$KB"
    rm -rf "$OUT"
    mkdir -p "$OUT"

    echo "== semiespacio $KB KB"
    if ! python3 "$HERE/fix-libgodc-gc.py" "$KB" --build > "$OUT/vendor.log" 2>&1; then
        echo "   el runtime NO se rehizo (ver $OUT/vendor.log)" >&2
        continue
    fi

    if ! SALTAR_MOTOR=1 bash "$HERE/build-mapa-102.sh" "$OUT" > "$OUT/build.log" 2>&1; then
        echo "   el build NO salio (ver $OUT/build.log)" >&2
        tail -5 "$OUT/build.log" >&2
        continue
    fi

    BIN="$(stat -c%s "$OUT/1ST_READ.BIN")"
    CAPTURE=none bash "$HERE/run-flycast.sh" "$OUT/disc.gdi" "${SEGS:-$SEGS_DEF}" "$OUT/run" > "$OUT/run.out" 2>&1
    LOG="$OUT/run/flycast.log"
    bash "$HERE/medir-hueco-gc-293.sh" leer "$LOG" "$BIN" > "$OUT/censo.txt" 2>&1

    MONTON="$(sed -n 's/.*monton de C despues.*= \([0-9]*\) B.*/\1/p' "$OUT/censo.txt")"
    CRC="$(grep -a 'MAPA102: MAPA tiles=' "$LOG" | sed 's/\r$//' | sed -n 's/.*crcGrilla= \(-\?[0-9]*\).*/\1/p' | tail -1)"
    MS="$(grep -a 'MAPA102: MAPA tiles=' "$LOG" | sed 's/\r$//' | sed -n 's/.*ms= \([0-9]*\).*/\1/p' | tail -1)"

    if grep -qa "MAPA102: CUADRO" "$LOG"; then DES="CUADRO EN PANTALLA"
    elif [ -n "$CRC" ]; then DES="compone; muere despues ($(grep -a 'Out of memory\|fatal error' "$LOG" | head -1 | tr -d '\r'))"
    elif grep -qa "gc_init: alloc failed" "$LOG"; then DES="muere en gc_init"
    elif [ "$(grep -ac 'gc_collect v=' "$LOG")" -gt 200 ]; then DES="thrashing del recolector (mas de 200 recolecciones, sin componer)"
    elif grep -qa "gc_external_alloc: OOM" "$LOG"; then DES="muere por monton de C antes de componer"
    elif grep -qa "SH4 exception\|Unhandled" "$LOG"; then DES="muere por excepcion"
    else DES="no se supo (ver $LOG)"
    fi

    printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$KB" "$((2 * KB * 1024))" "$BIN" "${MONTON:-?}" \
        "$([ -n "$CRC" ] && echo si || echo NO)" "${CRC:-?}" "${MS:-?}" "$DES" >> "$SAL/tabla.tsv"
    echo "   $DES"
    sed -n '1,40p' "$OUT/censo.txt" | sed 's/^/   /'
done

echo
echo "== la tabla ($SAL/tabla.tsv)"
column -t -s $'\t' "$SAL/tabla.tsv"
