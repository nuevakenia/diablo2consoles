#!/bin/bash
# DC-002.27: LA IMAGEN DE DISCO CON LOS ASSETS DEL JUEGO ADENTRO.
#
# Arma el ISO (y el GDI que arranca Flycast y lee el GDEMU) de un POC YA CONSTRUIDO, con la CARPETA DEL
# JUEGO en `/JUEGO` adentro del ISO9660. Es el otro camino que el romdisk para los datos del juego: el
# romdisk son 16 MB de RAM y el binario ya se lleva 13,9 MB, mientras que el disco se lee POR PARTES (el
# MPQ tiene su indice al final del archivo y el motor hace Seek + ReadAt: no hace falta que entre en RAM).
#
# Uso:  scripts/console/mkdisco.sh <dir_build> [<dir_juego>]
#   <dir_build>  trae IP.BIN y 1ST_READ.BIN (los deja build-motor-arranque.sh o build-dc.sh)
#   <dir_juego>  los MPQ del juego TAL COMO ESTAN en el PC (p.ej. ~/.wine-d2/drive_c/Games/Diablo II).
#                Se hornean todos los *.mpq que haya ahi, en /JUEGO.
#
# POR QUE LOS NOMBRES VAN EN MAYUSCULAS (y por que eso no rompe nada): el ISO9660 de nivel 3 que arma
# pycdlib solo acepta A-Z, 0-9 y _ (medido: "patch_D2.mpq" da PyCdlibInvalidInput "ISO9660 filenames must
# consist of characters A-Z, 0-9, and _"). El motor los pide en minusculas, y el iso9660 de KOS compara los
# nombres sin distinguir mayusculas. Eso NO se supone: la sonda del POC motor-disco abre las DOS formas
# ("/cd/JUEGO/PATCH_D2.MPQ" y "/cd/juego/patch_d2.mpq") y el corpus lo exige.
#
# MUTANTE: DC_SIN_JUEGO=1 -> la ISO se arma SIN la carpeta /JUEGO. Es el mutante de la tarjeta: el cuadro de
# error del motor ("file not found: /cd/juego/patch_d2.mpq") TIENE que volver.
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

DIR="${1:?uso: mkdisco.sh <dir_build> [<dir_juego>]}"
JUEGO="${2:-}"
ISO="$DIR/disc.iso"

[ -f "$DIR/IP.BIN" ] || { echo "no encuentro $DIR/IP.BIN (lo deja el build del POC)" >&2; exit 1; }
[ -f "$DIR/1ST_READ.BIN" ] || { echo "no encuentro $DIR/1ST_READ.BIN (lo deja el build del POC)" >&2; exit 1; }

ARGS=("$DIR/IP.BIN" "$DIR/1ST_READ.BIN")
BAKED=0
BYTES=0

if [ "${DC_SIN_JUEGO:-0}" = "1" ]; then
    echo "== la ISO SIN la carpeta del juego (mutante DC_SIN_JUEGO=1) =="
elif [ -z "$JUEGO" ]; then
    echo "AVISO: sin <dir_juego>: la ISO va SIN los MPQ (es el mutante de la tarjeta)" >&2
else
    for f in "$JUEGO"/*.mpq; do
        [ -f "$f" ] || continue
        # ojo: el nombre interno va en MAYUSCULAS (lo exige el ISO9660 de nivel 3) y con `;1` (la version de
        # ISO9660; el iso9660 de KOS la ignora al comparar, y el motor pide el nombre sin version)
        nombre="$(basename "$f" | tr '[:lower:]' '[:upper:]')"
        ARGS+=("$f:/JUEGO/$nombre;1")
        BAKED=$((BAKED + 1))
        BYTES=$((BYTES + $(stat -c%s "$f")))
    done
fi

"$DC_PYTHON" "$HERE/mkiso.py" "$ISO" "${ARGS[@]}" | sed 's/^/   /'
"$DC_PYTHON" "$HERE/mkgdi.py" "$DIR" | sed 's/^/   /'

echo "DISCO: $BAKED MPQ en /JUEGO ($BYTES bytes) + el binario de arranque"
echo "DISCO: $DIR/disc.iso ($(stat -c%s "$ISO") bytes) y $DIR/disc.gdi"
