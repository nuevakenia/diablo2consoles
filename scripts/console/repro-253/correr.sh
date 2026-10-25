#!/bin/bash
# Escalera de reproduccion de DC-002.53 (2026-10-25).
#
# PARA QUE: separar de una vez "KOS/Flycast roto", "Go/libgodc roto" y "el POC hace algo antes del video
# que rompe pvr_init_defaults". Cada eslabon es un programa chiquito que se compila y se corre en Flycast en
# ~3 minutos, en vez de los 15 de un build del POC entero.
#
# USO:
#   scripts/console/repro-253/correr.sh            # corre los tres eslabones
#   scripts/console/repro-253/correr.sh kos        # solo el de KOS pelado
#   scripts/console/repro-253/correr.sh minimo     # solo el Go minimo
#   scripts/console/repro-253/correr.sh churn      # solo el Go con churn de monton
#
# RESULTADO MEDIDO EL 2026-10-25 (con KOS y libgodc pristinos, Flycast):
#   kos    -> arranca, pvr_init_defaults OK, 64 KB de VRAM, 60 escenas, sale limpio ("arch: shutting down")
#   minimo -> arranca, pvr_init_defaults OK, 60 pintadas por el shim del POC, sale limpio
#   churn  -> igual que minimo pero antes pide 1 MB, 40 temporales de 96 KB y 2 GC: TAMBIEN anda
# Los tres pasan. O sea: ni KOS, ni Flycast, ni libgodc, ni el estado del monton. El problema esta en lo que
# el POC hace antes de encender el video, o en el tamano/forma de SU binario (ver evidencia dc-002.53).
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../../.." && pwd)"
. "$REPO/scripts/console/env.sh"

QUE="${1:-todos}"

kos() {
    local out=/tmp/repro253-kos
    rm -rf "$out"; mkdir -p "$out"
    kos-cc -c "$HERE/kos-pelado/main.c" -o "$out/main.o" || return 1
    kos-cc -o "$out/prog.elf" "$out/main.o" $DC_LDPATHS -lkos $DC_LIBS_POST || return 1
    empaquetar "$out"
    correr "$out"
}

minimo() {
    local out=/tmp/repro253-minimo
    rm -rf "$out"
    SALTAR_STDLIB=1 bash "$REPO/scripts/console/build-dc.sh" "$HERE/go-minimo" "$out" >/dev/null 2>&1 || { echo "FALLO el build"; return 1; }
    correr "$out"
}

churn() {
    local out=/tmp/repro253-churn
    rm -rf "$out"
    SALTAR_STDLIB=1 bash "$REPO/scripts/console/build-dc.sh" "$HERE/go-churn" "$out" >/dev/null 2>&1 || { echo "FALLO el build"; return 1; }
    correr "$out"
}

empaquetar() {
    local out=$1
    sh-elf-objcopy -R .stack -O binary "$out/prog.elf" "$out/1ST_READ.BIN"
    "$DC_MAKEIP" -f "$out/IP.BIN" >/dev/null
    "$DC_PYTHON" "$REPO/scripts/console/mkiso.py" "$out/disc.iso" "$out/IP.BIN" "$out/1ST_READ.BIN"
    "$DC_PYTHON" "$REPO/scripts/console/mkgdi.py" "$out"
}

correr() {
    local out=$1
    LIBGL_ALWAYS_SOFTWARE=1 CAPTURAS=1 INTERVALO=1.0 ESPERA_ARRANQUE=80 \
        bash "$REPO/scripts/console/run-flycast-249.sh" "$out/disc.gdi" "$out/corrida" >/dev/null 2>&1
    echo "--- $out ---"
    grep -a "PRUEBA\|MINIMO\|CHURN\|HITO250\|Unhandled" "$out/corrida/flycast.log" 2>/dev/null | tr -d '\r' | head -12
}

case "$QUE" in
    kos)    kos ;;
    minimo) minimo ;;
    churn)  churn ;;
    todos)  kos; minimo; churn ;;
    *) echo "uso: $0 [kos|minimo|churn|todos]"; exit 2 ;;
esac
