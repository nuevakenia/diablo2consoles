#!/bin/bash
# DC-002.87b -- EL BARRIDO DE ACOMODO, CON EL VOLCADO DE REGISTROS PUESTO.
#
# Por que existe. DC-002.87 midio que el desenlace del primer cuadro lo decide el acomodo del binario, pero
# su barrido corrio sin la sonda (DC_ROOT sin exportar: control negativo). Y DC-002.87b midio que, con la
# sonda vieja, la corrida se quedaba MUDA a los ~28 s (la consola de KOS se apaga sola al agotar su tope en
# `scif_write`) mientras el cuadro SI se pintaba: o sea que "no hay linea de CUADRO" no era "no pinto".
# Este script corre el barrido con el runtime instrumentado y con la sonda que escribe el SCIF a mano, y
# deja UN log por intento, con una linea de resumen por intento (cuadro, excepciones, PC fuera de RAM).
#
# Uso:  scripts/console/barrido-287b.sh <etiqueta> <segundos> <bytes_relleno> [bytes_relleno...]
#       RECOLECCIONES=n   recolecciones forzadas al arrancar, iguales para todos los intentos del barrido
#       RESERVA=n         bytes que se le quitan al monton de C
#       BASE=<dir>        directorio de un build del cooperativo ya hecho (por defecto /tmp/dc287/A)
#       DC_ROOT=<dir>     vendor instrumentado (por defecto /tmp/dc287/dcroot)
# Los logs quedan en $BASE/logs (y el crudo de cada corrida en el directorio del intento).
set -uo pipefail
TAG="${1:?uso: barrido-287b.sh <etiqueta> <segundos> <bytes> [bytes...]}"
SECS="${2:?segundos}"
shift 2
[ "$#" -ge 1 ] || { echo "falta al menos un relleno" >&2; exit 1; }
BASE="${BASE:-/tmp/dc287/A}"
export DC_ROOT="${DC_ROOT:-/tmp/dc287/dcroot}"
export RESERVA="${RESERVA:-0}"
export FALLA="${FALLA:-0}"
REC="${RECOLECCIONES:-0}"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOWN="$BASE/logs"; mkdir -p "$DOWN"
for B in "$@"; do
    ET="$TAG-$B"
    bash "$HERE/repro-layout-287.sh" "$B" "$BASE" "$SECS" "$REC" > "$DOWN/$ET.build.log" 2>&1
    OUT="$BASE/relleno-$B-r$RESERVA-f$FALLA-g$REC"
    L="$OUT/run/flycast.log"
    if [ -f "$L" ]; then
        cp "$L" "$DOWN/$ET.log"
        T=$(tr -d '\r' < "$L")
        EXC=$(printf '%s\n' "$T" | grep -ac 'SONDA-REG FALLO' || true)
        FUERA=$(printf '%s\n' "$T" | grep -ac 'SONDA-PC FUERA' || true)
        SALTO=$(printf '%s\n' "$T" | grep -a 'SONDA-PC t=' | tail -1 | sed 's/.*saltados=\([0-9]*\).*/\1/')
        CU=$(printf '%s\n' "$T" | grep -a 'COOP276: CUADRO' | tail -1 | sed 's/COOP276: CUADRO //')
        TT=$(printf '%s\n' "$T" | grep -a 'SONDA-PC t=' | tail -1 | sed 's/.*t=\([0-9]*\).*/\1/')
        printf '%-16s bin=%-8s exc=%-2s fuera=%-2s t=%-4s saltados=%-5s | %s\n' "$ET" \
            "$(stat -c%s "$OUT/1ST_READ.BIN" 2>/dev/null || echo 0)" "$EXC" "$FUERA" "${TT:-?}" \
            "${SALTO:-?}" "${CU:-SIN-CUADRO}"
        printf '%s\n' "$T" | grep -a 'SONDA-REG FALLO' | head -1
    else
        printf '%-16s SIN LOG (ver %s)\n' "$ET" "$DOWN/$ET.build.log"
    fi
    # el .iso de cada intento son 540 MB (disc.iso y los dos trackNN.iso son el MISMO inodo por los hardlinks
    # de mkgdi.py): se guardan el BIN y el log, no el disco.
    for f in "$OUT/disc.iso" "$OUT/track01.iso" "$OUT/track03.iso"; do rm -f "$f"; done
done
