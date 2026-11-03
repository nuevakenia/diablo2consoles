#!/bin/bash
# DC-002.136 -- correr el POC DE LOS KNOBS POR DEFECTO en Flycast y decir, sin adornos, si ARRANCO o si murio
# cargando (`gc_external_alloc: OOM`). Es el instrumento del criterio de la tarjeta: 5 corridas seguidas sin OOM.
#
# POR QUE UN SCRIPT APARTE Y NO run-anim-132.sh: aquel se queda esperando ventana, mueve el escritorio con kwin y
# saca capturas con spectacle; para contar OOMs eso es ruido y ademas una corrida dura 3-8 minutos. Aca lo unico que
# se mira es la CONSOLA SERIE (el log del emulador) y el proceso:
#
#   ARRANCO            salio la marca del primer cuadro (JUEGO121: PRIMER-CUADRO / SPLIT PRIMER-CUADRO)
#   OOM                salio `gc_external_alloc: OOM` o `Out of memory. Requested sbrk_base` antes del primer cuadro
#   SIN-PRIMER-CUADRO  ni una cosa ni la otra dentro de la ventana (se cuelga, o tarda mas)
#
# Uso:
#   bash scripts/console/run-oom-136.sh <disc.gdi> <dir_salida> [espera_segundos]
#   ESPERA=900 CORRIDAS=1 bash scripts/console/run-oom-136.sh ...
#
# Deja en <dir_salida>: flycast.log (la consola serie entera) y resultado.txt (una linea con el veredicto y los
# numeros del monton de C en el ultimo hito de carga que imprimio el POC).
set -uo pipefail
IMG="${1:?uso: run-oom-136.sh <disc.gdi> <dir_salida> [espera]}"
OUT="${2:-/tmp/dc136-run}"
ESPERA="${3:-${ESPERA:-600}}"
FLY="${FLYCAST:-$HOME/dreamcast/dl/flycast.AppImage}"
DISP="${DISPLAY:-:0}"
TOKEN="${TOKEN:-dc136}"
POS="${POS:-1400 120}"
PX="${POS%% *}"; PY="${POS##* }"

mkdir -p "$OUT"
rm -f "$OUT/flycast.log" "$OUT/resultado.txt"

DISC_DIR="$(cd "$(dirname "$IMG")" && pwd)"
GDI="$DISC_DIR/$TOKEN.gdi"
cp -f "$IMG" "$GDI"

echo "== Flycast: $GDI (consola serie si, ventana ${PX},${PY})"
setsid env DISPLAY="$DISP" "$FLY" -config "window:fullscreen=no" \
    -config "window:width=640" -config "window:height=480" \
    -config "window:left=$PX" -config "window:top=$PY" \
    -config "Debug:SerialConsoleEnabled=true" "$GDI" >"$OUT/flycast.log" 2>&1 &
PID=$!
echo "pid=$PID"

PATRON="$TOKEN[.]gdi"
vivo() { pgrep -f "$PATRON" >/dev/null 2>&1; }

T=0
SALIO=""
while [ "$T" -lt "$ESPERA" ]; do
    if tr -d '\r' < "$OUT/flycast.log" 2>/dev/null |
        grep -qaE 'JUEGO121: PRIMER-CUADRO|JUEGO121: SPLIT PRIMER-CUADRO'; then
        SALIO="ARRANCO"; break
    fi
    if tr -d '\r' < "$OUT/flycast.log" 2>/dev/null |
        grep -qaE 'gc_external_alloc: OOM|Out of memory\. Requested sbrk_base|GC to-space overflow'; then
        SALIO="OOM"; break
    fi
    if ! vivo; then SALIO="SIN-PRIMER-CUADRO"; break; fi
    sleep 1
    T=$(( T + 1 ))
done
[ -n "$SALIO" ] || SALIO="SIN-PRIMER-CUADRO"

# SI ARRANCO, SE LE DEJA VIVIR UNOS SEGUNDOS: el criterio es "arranca", pero un OOM en los primeros cuadros (que es
# otro defecto, el de DC-002.131) tambien tiene que verse. Con ESPERA_POST=0 no se espera nada.
ESPERA_POST="${ESPERA_POST:-20}"
if [ "$SALIO" = "ARRANCO" ] && [ "$ESPERA_POST" -gt 0 ]; then
    echo "  arranco a los ${T}s: se le dejan ${ESPERA_POST}s de cuadros"
    sleep "$ESPERA_POST"
fi

# el proceso ya no hace falta: se corta con el patron del disco (nunca con un -f que pueda tocar la shell que llama)
pkill -f "$PATRON" 2>/dev/null
sleep 1
pkill -9 -f "$PATRON" 2>/dev/null

LOG="$OUT/flycast.log"
OOMS="$(tr -d '\r' < "$LOG" | grep -ac 'gc_external_alloc: OOM')"
SBRK="$(tr -d '\r' < "$LOG" | grep -ac 'Out of memory. Requested sbrk_base')"
ULTIMO="$(tr -d '\r' < "$LOG" | grep -a 'JUEGO121: MEMORIA\|ARQ-CARGA lee-volvio' | tail -1 | cut -c1-160)"
CUADROS="$(tr -d '\r' < "$LOG" | grep -ac 'JUEGO121: PRIMER-CUADRO')"
MUNDO="$(tr -d '\r' < "$LOG" | grep -a 'SOLO-MAPA' | head -1 | sed 's/.*crc32= \([0-9-]*\).*/\1/')"
TABLA="$(tr -d '\r' < "$LOG" | grep -a 'JUEGO136: TABLA-CARGA hitos' | head -1 | cut -c1-200)"

{
    printf 'RESULTADO=%s segundos=%s ooms=%s sbrkFallidos=%s primerCuadro=%s\n' \
        "$SALIO" "$T" "$OOMS" "$SBRK" "$CUADROS"
    printf 'MUNDO_crc32=%s\n' "${MUNDO:-?}"
    printf 'TABLA=%s\n' "${TABLA:-?}"
    printf 'ULTIMO_HITO=%s\n' "$ULTIMO"
    printf 'BIN=%s\n' "$(md5sum "$DISC_DIR/1ST_READ.BIN" 2>/dev/null | cut -d' ' -f1)"
} | tee "$OUT/resultado.txt"
