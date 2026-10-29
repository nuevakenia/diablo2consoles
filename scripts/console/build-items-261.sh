#!/bin/bash
# DC-002.61 -- LOS ITEMS DEL SUELO: los 41 objetos del DS1 con su grafico del MPQ, el boton de accion para
# recogerlos y el inventario de 4 casillas con su panel.
#
# QUE CAMBIA RESPECTO DE build-monstruo-250.sh (y por que)
# ---------------------------------------------------------------------------------------------------
# `console/dc/poc/personaje-mapa/` trae DOS archivos nuevos de esta tarjeta -- `objetos.go` (los items, el
# inventario y el panel) y `fuente.go` (la letra de 5x7 del panel) -- con TRES interruptores APAGADOS en el
# arbol:
#
#   var items = false       -> true: toma los registros de objetos del DS1, les busca su capa DCC en el MPQ,
#                                  los pinta en el suelo, y el boton A recoge el que esta al lado en el
#                                  inventario de 4 casillas (panel con la tecla I / boton X).
#   var itemsSordo = false  -> true: EL MUTANTE del criterio 3 (el boton se lee y se informa, no recoge).
#   var itemsAuto = false   -> true: EL INSTRUMENTO (la cruceta y el boton de accion simulados, para poder
#                                  medir sin teclado; el camino es el real: el mismo `est.Avanzar` del motor,
#                                  el mismo radio de recogida y el mismo inventario).
#   var panelAuto = false   -> true: EL INSTRUMENTO del panel (lo abre y lo cierra solo).
#
# El resto de la tarjeta NO cambia una linea: el mapa, el personaje, el bicho y el golpe son los mismos. El
# script compila una COPIA del POC (con las sed ya aplicadas) y le pasa POC_DIR a build-monstruo-250.sh, que
# es el script de la casa: la ISO con los tres MPQ, la verificacion de los tres archivos compartidos con el
# host y el censo salen de ahi.
#
# Uso:  scripts/console/build-items-261.sh [dir_salida]
#       ITEMS=0            -> el BASELINE: el binario de DC-002.50 SIN items (para comparar los pixeles del
#                             criterio 2 y para probar que el suelo no aparece cuando el interruptor esta off)
#       BESTIA=0           -> sin el bicho (el POC de DC-002.49 con los items)
#       ITEMS_AUTO=1       -> enciende el instrumento: camina hasta el item mas cercano y lo recoge (3 veces)
#       PANEL_AUTO=1       -> enciende el instrumento del panel (abre/cierra solo)
#       ITEMS_SORDO=1      -> el MUTANTE: las mismas pulsaciones, nada se recoge
#       SALTAR_STDLIB=1 / SALTAR_MOTOR=1 + GODC_GOX=~/dreamcast/gox  -> reusar los .gox ya compilados
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
POC="personaje-mapa"
OUT="${1:-${DC261_OUT:-/tmp/dcbuild/items-261}}"
SRC="$REPO/console/dc/poc/$POC"
COPIA="$OUT/poc-items"

mkdir -p "$OUT"
rm -rf "$COPIA"
mkdir -p "$COPIA"
cp -r "$SRC/." "$COPIA/"

echo "== 0/2 los interruptores de los items (en la COPIA, el arbol no se toca)"
if [ "${ITEMS:-1}" = "1" ]; then
    sed -i 's|^var items = false$|var items = true|' "$COPIA/objetos.go"
    grep -q '^var items = true$' "$COPIA/objetos.go" || {
        echo "   FALLO: no pude encender los items (no encontre 'var items = false')"; exit 1; }
    echo "   ITEMS=ON: los 41 objetos del DS1 en el suelo, el boton A recoge y el panel de 4 casillas"
else
    echo "   ITEMS=0: el BASELINE (el binario de DC-002.50, sin una linea de items)"
fi

if [ "${ITEMS_SORDO:-0}" = "1" ]; then
    sed -i 's|^var itemsSordo = false$|var itemsSordo = true|' "$COPIA/objetos.go"
    grep -q '^var itemsSordo = true$' "$COPIA/objetos.go" || {
        echo "   FALLO: no pude poner el mutante (no encontre 'var itemsSordo = false')"; exit 1; }
    echo "   MUTANTE: itemsSordo=true (el boton se lee y se informa, nada se recoge)"
fi

if [ "${ITEMS_AUTO:-0}" = "1" ]; then
    sed -i 's|^var itemsAuto = false$|var itemsAuto = true|' "$COPIA/objetos.go"
    grep -q '^var itemsAuto = true$' "$COPIA/objetos.go" || {
        echo "   FALLO: no pude encender el instrumento (no encontre 'var itemsAuto = false')"; exit 1; }
    echo "   ITEMS_AUTO=ON: la cruceta y el boton de accion simulados (80/400 ms) hasta recoger 3 items"
fi

if [ "${PANEL_AUTO:-0}" = "1" ]; then
    sed -i 's|^var panelAuto = false$|var panelAuto = true|' "$COPIA/objetos.go"
    grep -q '^var panelAuto = true$' "$COPIA/objetos.go" || {
        echo "   FALLO: no pude encender el panel automatico (no encontre 'var panelAuto = false')"; exit 1; }
    echo "   PANEL_AUTO=ON: el panel se abre y se cierra solo (1,2 s cada 2,5 s)"
fi

if [ "${MEDIR:-0}" = "1" ]; then
    sed -i 's|^var medirPantalla = false$|var medirPantalla = true|' "$COPIA/objetos.go"
    grep -q '^var medirPantalla = true$' "$COPIA/objetos.go" || {
        echo "   FALLO: no pude encender el instrumento de pixeles (no encontre 'var medirPantalla = false')"; exit 1; }
    echo "   MEDIR=ON: cuenta los pixeles no negros de la pantalla en los cuadros 60 y 240"
fi

echo
echo "== 1/2 el resto del camino es el de la casa: POC_DIR=<copia> build-monstruo-250.sh $OUT"
POC_DIR="$COPIA" BESTIA="${BESTIA:-1}" GOLPEAUTO="${GOLPEAUTO:-0}" GOLPE_SORDO="${GOLPE_SORDO:-0}" \
    bash "$HERE/build-monstruo-250.sh" "$OUT"

echo
echo "== 2/2 los numeros de ESTA tarjeta"
echo "items=$(grep -c '^var items = true$' "$COPIA/objetos.go") \
itemsAuto=$(grep -c '^var itemsAuto = true$' "$COPIA/objetos.go") \
itemsSordo=$(grep -c '^var itemsSordo = true$' "$COPIA/objetos.go") \
panelAuto=$(grep -c '^var panelAuto = true$' "$COPIA/objetos.go") \
bestia=$(grep -c '^var bestia = true$' "$COPIA/main.go")"
echo "ELF: $OUT/prog.elf ($(stat -c%s "$OUT/prog.elf") bytes)"
echo "GDI: $OUT/disc.gdi"
