#!/bin/bash
# DC-002.36: la ISO del disco con la carpeta /MENU (los DC6 del menu, las tablas de las fuentes, las
# paletas, el plan de dibujo y las cadenas) y su GDI.
#
# Por que /MENU y no los MPQ enteros: los 18 archivos del menu son 1,87 MB medidos; d2data.mpq solo pesa
# 267 MB. Los assets salen de los MPQ (del disco del juego) en el HOST, se miden y viajan crudos: es el
# mismo camino que DC-002.31 midio para los assets sueltos. Los nombres van en MAYUSCULAS y sin
# subdirectorios porque el ISO9660 de la consola resuelve el nombre exacto.
#
# SIN_ASSET=<nombre> no copia ese archivo: es el MUTANTE de la tarjeta (criterio 3).
#
# Uso:  scripts/console/mkdisco-236.sh [dir_de_build]
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

B="${1:-$HOME/.hermes/cache/scratch/dc36/build}"
M="$B/MENU"
SIN_ASSET="${SIN_ASSET:-}"

[ -f "$B/IP.BIN" ] || { echo "falta $B/IP.BIN (corre build-dc.sh antes)"; exit 1; }
[ -f "$B/1ST_READ.BIN" ] || { echo "falta $B/1ST_READ.BIN (corre build-dc.sh antes)"; exit 1; }
[ -d "$M" ] || { echo "falta $M (lo arma build-menu-236.sh con el host)"; exit 1; }
[ -f "$M/PLAN.TXT" ] || { echo "falta $M/PLAN.TXT (el plan de dibujo que arma el host)"; exit 1; }

LISTA="$B/iso-lista.txt"
: > "$LISTA"

# SIN_CARPETA=1: los archivos van a la RAIZ del ISO (/NOMBRE;1) y no a /MENU/.
#
# POR QUE (medido en DC-002.36, y es lo que hace que el menu se vea): con los 20 archivos adentro de la
# carpeta /MENU el disco NO ARRANCA en Flycast con el binario del menu: la corrida no imprime NINGUNA marca
# del arranque (ni el constructor de C de libgodc, 'ARQ-MARCA ctor-kos_startup', que sale antes de main),
# o sea que el binario no llega a ejecutarse; la pantalla queda negra. Lo mismo pasa con UN solo archivo
# chico (PAL_SKY.PAL, 768 B) en la carpeta, con nombres cortos 8.3 y con PLAN.TXT (10 sectores): es la
# CARPETA, no los nombres ni el tamano. Los 20 archivos en la RAIZ del ISO: arranca y el POC dibuja.
# El mismo binario en un ISO sin carpetas arranca; el disco de DC-002.35 (carpeta /JUEGO) arranca con SU
# binario pero NO con el del menu (cruce medido). Queda declarado como defecto aparte del arranque de disco.
DESTINO="/MENU"
[ "${SIN_CARPETA:-0}" = "1" ] && DESTINO=""

for f in "$M"/*; do
    [ -f "$f" ] || continue   # solo archivos: el subdirectorio de referencias no va al disco
    base="$(basename "$f")"
    if [ -n "$SIN_ASSET" ] && [ "$base" = "$SIN_ASSET" ]; then
        echo "   MUTANTE: NO viaja $base"
        continue
    fi
    printf '%s:%s/%s;1\n' "$f" "$DESTINO" "$base" >> "$LISTA"
done

# MKISO_JOLIET=0: SIN el arbol Joliet. Medido en DC-002.35: con Joliet el driver iso9660 de KOS monta
# Joliet y despues NO encuentra ninguna ruta (el POC moria con "file does not exist" con el archivo en
# el disco). Con el ISO9660 pelado (nombres en mayusculas y `;1`) la ruta abre.
MKISO_JOLIET="${MKISO_JOLIET:-0}" "$DC_PYTHON" "$HERE/mkiso.py" "$B/disc.iso" "$B/IP.BIN" "$B/1ST_READ.BIN" "@$LISTA" | tail -3
"$DC_PYTHON" "$HERE/mkgdi.py" "$B" | tail -3

echo
echo "ISO:  $(stat -c%s "$B/disc.iso") bytes  (/MENU: $(wc -l < "$LISTA") archivos)"
echo "GDI:  $B/disc.gdi"
