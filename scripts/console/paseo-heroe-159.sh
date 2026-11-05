#!/bin/bash
# DC-002.159b -- LA CORRIDA DEL HEROE CON CAPTURAS FINAS.
#
# POR QUE EXISTE. El heroe aparece a los ~50 s de corrida y el POC vive pocos segundos mas: la ventana en la
# que hay algo que mirar es corta, y la lista de capturas de `run-flycast.sh` (3, 6, 10, 15, 25, 40, 60...) pasa
# por encima sin verla. Este script corre el Gate 0 con una lista de tiempos tupida alrededor de esos 50 s.
#
# LA CRUCETA NO SE APRIETA DESDE AFUERA (medido). Se probo con `ydotool` (eventos del teclado del host) y el
# heroe no se movio ni una celda: Flycast no trae mapeo de teclado a la cruceta en su config (no hay seccion
# `[input]` en `~/.config/flycast/emu.cfg`). La caminata la hace el GUION del POC (`paseo_159.go`), que le
# contesta la cruceta al mismo servicio de entrada que usa el heroe.
#
# Uso:  bash scripts/console/paseo-heroe-159.sh <disc.gdi> [dir_salida]
#   deja las capturas y el `flycast.log` en <dir_salida> (default /tmp/paseo159).
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

IMG="${1:?uso: paseo-heroe-159.sh <disc.gdi> [dir_salida]}"
OUT="${2:-/tmp/paseo159}"

mkdir -p "$OUT"

echo "== corrida del heroe con capturas finas: $IMG -> $OUT =="

# los tiempos: el arranque de la consola y la composicion del pueblo comen ~45 s; el heroe aparece alrededor de
# los 50 y el POC aguanta unos segundos mas caminando. Se captura cada 1-2 s entre los 44 y los 62.
CAPTURAS="30 40 44 46 48 49 50 51 52 53 54 55 56 57 58 60 62" \
    bash "$HERE/run-flycast.sh" "$IMG" 62 "$OUT"

echo "== fin: capturas y log en $OUT =="
echo "   recortar la ventana del cuadro (la geometria medida es cli=210,238,640,480):"
echo "   ffmpeg -i $OUT/f52s.png -vf crop=640:480:210:238 -y $OUT/cuadro-52s.png"
