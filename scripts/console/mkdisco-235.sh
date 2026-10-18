#!/bin/bash
# DC-002.35: la ISO del disco con los archivos del juego en /JUEGO, y su GDI.
#
# Que lleva el disco:
#   /JUEGO/PATCH_D2.MPQ  el MPQ que el POC abre (16.384 entradas: entra en la consola)
#   /JUEGO/PAL.PL2       la paleta, EXTRAIDA del MPQ grande en el host: en la consola d2data.mpq no
#                        se puede abrir todavia (65.536 entradas revientan el semiespacio, DC-002.28/33)
# y el arranque de siempre (IP.BIN + 1ST_READ.BIN, los dos ya generados por build-dc.sh).
#
# Uso:  scripts/console/mkdisco-235.sh [dir_de_build]
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

B="${1:-$HOME/.hermes/cache/scratch/dc35/build}"
D2DIR="${D2_DIR:-$HOME/.wine-d2/drive_c/Games/Diablo II}"
MPQ_CHICO="${MPQ_CHICO:-patch_d2.mpq}"
# La paleta: el que llama puede fijarla con PAL=...; si no, se toma la primera que dejo el horneador del
# host en $B/host (su nombre depende del POC: build-dc6-menu-disco.sh la llama <poc>.pal y otras corridas
# la llamaron pal235.pal). Antes esto estaba fijo en $B/host/pal235.pal y una corrida con otro nombre
# abortaba con "falta la paleta" pareciendo un problema del disco.
if [ -z "${PAL:-}" ]; then
    PAL="$(ls -1 "$B"/host/*.pal 2>/dev/null | head -1)"
fi
PAL="${PAL:-$B/host/pal235.pal}"

[ -f "$B/IP.BIN" ] || { echo "falta $B/IP.BIN (corre build-dc.sh antes)"; exit 1; }
[ -f "$B/1ST_READ.BIN" ] || { echo "falta $B/1ST_READ.BIN (corre build-dc.sh antes)"; exit 1; }
[ -f "$PAL" ] || { echo "falta la paleta $PAL (la extrae build-dc6-menu-disco.sh del MPQ del host)"; exit 1; }

LISTA="$B/iso-lista.txt"
# El nombre REAL del archivo en el disco: `patch_D2.mpq` (con D mayuscula). d2mpq lo abre igual porque
# usa openIgnoreCase, pero el ISO9660 no: aca hace falta el nombre exacto, tal como lo escribe el juego.
MPQ_REAL="$(ls "$D2DIR" | grep -ix "$MPQ_CHICO" | head -1)"
[ -n "$MPQ_REAL" ] || MPQ_REAL="$(ls "$D2DIR" | grep -i "^$(basename "${MPQ_CHICO%.mpq}").mpq$" | head -1)"
[ -n "$MPQ_REAL" ] || { echo "no encuentro $MPQ_CHICO en $D2DIR"; exit 1; }

{
    printf '%s:/JUEGO/PATCH_D2.MPQ;1\n' "$D2DIR/$MPQ_REAL"
    printf '%s:/JUEGO/PAL.PL2;1\n' "$PAL"
} > "$LISTA"

# MKISO_JOLIET=0: SIN el arbol Joliet. Medido en esta tarjeta (y ya sabido por DC-002.27): con el arbol
# Joliet que pycdlib escribe por defecto, el driver iso9660 de KOS monta Joliet ("joliet level 3
# extensions detected") y despues NO encuentra NINGUNA ruta -- el POC moria en
# "MPQ-ABRIR FALLO: file does not exist" con el archivo presente en el disco. Con el ISO9660 pelado
# (nombres en mayusculas y `;1`) la ruta abre. Por defecto, pelado: es lo que la consola lee.
MKISO_JOLIET="${MKISO_JOLIET:-0}" "$DC_PYTHON" "$HERE/mkiso.py" "$B/disc.iso" "$B/IP.BIN" "$B/1ST_READ.BIN" "@$LISTA" | tail -3
"$DC_PYTHON" "$HERE/mkgdi.py" "$B" | tail -3

echo
echo "ISO:  $(stat -c%s "$B/disc.iso") bytes"
echo "GDI:  $B/disc.gdi"
