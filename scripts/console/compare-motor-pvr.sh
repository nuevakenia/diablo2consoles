#!/bin/bash
# POC de DC-002.9 (motor-pvr): el motor pintando en la consola. Mide DOS cosas y las dos tienen que dar
# bien:
#
#   1) el resumen de texto (los indices decodificados y los RGBA de cada frame, con su hash FNV-1a):
#      tiene que ser IDENTICO al que da el mismo codigo del motor en el host -> compare-motor-dc6.sh
#   2) la CAPTURA de pantalla del emulador contra la referencia en PNG que deja el comparador del host
#      (el frame 0 tal como lo decodifica el motor), pixel a pixel y en las cuatro orientaciones
#      -> compare-capture.py
#
# La 1 dice "el decodificador y la paleta hicieron lo mismo en las dos plataformas". La 2 dice "y el PVR
# pinto eso mismo". Ninguna alcanza sola: el hash podria coincidir con la textura subida con las UV mal
# (se veria cualquier cosa) y la captura podria coincidir con un decodificador roto en las dos
# plataformas.
#
# Uso:  scripts/console/compare-motor-pvr.sh [dir_de_build] [nombre_asset]
#       CAPTURE=none            para no sacar capturas (solo el resumen de texto)
#       FONDO='89 0 140' POS='40 300'   si el POC cambia el fondo o donde dibuja la copia 1:1
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
OUT="${1:-/tmp/dcbuild/motor-pvr}"
NAME="${2:-d2logo}"
PVR_FRAME="${PVR_FRAME:-0}"

# El fondo violeta es el que pone el POC (kos.PvrSetBgColor(0.35, 0, 0.55)) y la copia 1:1 del asset
# esta en (40,300) de la pantalla de la consola: con esas dos cosas compare-capture.py ubica la pantalla
# del emulador dentro de la captura y sabe que pixel comparar.
FONDO="${FONDO:-89 0 140}"
POS="${POS:-40 300}"

echo "== 1) el resumen de texto: consola contra host (y las capturas, que las saca ese mismo paso)"
# shellcheck disable=SC2086
CAPTURE="${CAPTURE:-spectacle}" POC=motor-pvr bash "$HERE/compare-motor-dc6.sh" "$OUT" "$NAME" || exit 1

echo
echo "== 2) la captura contra la referencia del host (el frame $PVR_FRAME, los RGBA del motor)"
REF="$OUT/ref-frame$PVR_FRAME.png"
(cd "$REPO" && go run ./console/dc/motorcmp -dc6 "$OUT/$NAME.dc6" -pal "$OUT/$NAME.pal" \
    -png "$REF" -frame "$PVR_FRAME" | grep '^MOTOR-PVR png=') || exit 1

# la ultima captura es la mas asentada (a los 3 s el emulador todavia puede estar arrancando)
CAP="$(ls -t "$OUT"/run/f*.png 2>/dev/null | head -1)"
[ -n "$CAP" ] || { echo "no hay capturas en $OUT/run (se sacan con CAPTURE=spectacle)" >&2; exit 1; }

PY="${DC_PYTHON:-python3}"
# shellcheck disable=SC2086
"$PY" "$HERE/compare-capture.py" --captura "$CAP" --referencia "$REF" --fondo $FONDO --pos $POS
