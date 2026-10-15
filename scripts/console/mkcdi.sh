#!/bin/bash
# Arma la imagen de disco que lee el GDEMU (y la Dreamcast real): un .cdi autoarrancable.
#
# Por que mkdcdisc y no las herramientas del toolchain: mkdcdisc hace todo el camino de una
# (extrae el binario del ELF, lo scrambla como corresponde a un CD-R, arma el IP.BIN con el
# 1ST_READ.BIN, arma el ISO y lo empaqueta en el .cdi de DiscJuggler). El .gdi que usamos para
# el ciclo rapido del emulador lleva sectores de 2048 y el GDEMU necesita 2352.
#
# Uso:  scripts/console/mkcdi.sh <prog.elf> <salida.cdi> ["nombre del disco"]
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DC_ROOT="${DC_ROOT:-$HOME/dreamcast}"
DEPS="$DC_ROOT/deps"
MKDC="$DEPS/bin/mkdcdisc"

ELF="${1:?uso: mkcdi.sh <prog.elf> <salida.cdi> [nombre]}"
OUT="${2:?falta la salida .cdi}"
NAME="${3:-DC-001 POC}"

if [ ! -x "$MKDC" ]; then
    echo "no esta mkdcdisc; lo compilo (scripts/console/build-mkdcdisc.sh)"
    bash "$HERE/build-mkdcdisc.sh"
fi

export LD_LIBRARY_PATH="$DEPS/lib"

echo "== armando $OUT desde $ELF =="
# -N sin padding: si no, rellena el disco entero y la imagen pasa de 700 MB (al GDEMU le da
#    igual, pero para copiar a la SD es incomodo).
"$MKDC" -e "$ELF" -o "$OUT" -n "$NAME" -N

echo "imagen: $OUT ($(stat -c%s "$OUT") bytes)"
