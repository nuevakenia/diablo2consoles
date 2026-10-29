#!/bin/bash
# DC-002.61 -- la captura de la VENTANA del emulador (no del escritorio entero).
#
# POR QUE EXISTE: `spectacle -f` (el modo de run-flycast.sh) fotografia el escritorio COMPUESTO, y en esta
# maquina el emulador queda DEBAJO de otras ventanas: las capturas de la primera corrida de esta tarjeta
# salieron con el editor, cero pixeles del POC. Aca se hace lo contrario: se busca la ventana del emulador
# con kdotool, se la ACTIVA y se captura SOLO esa ventana (`spectacle -a`), asi la imagen es del POC aunque
# haya otras ventanas arriba.
#
# Uso:  scripts/console/captura-ventana-261.sh <dir_salida> <seg1> <seg2> ...
#       Saca <dir>/w<seg>s.png por cada segundo pedido, contando desde que el script arranca.
set -uo pipefail

OUT="${1:?uso: captura-ventana-261.sh <dir_salida> <seg1> <seg2> ...}"
shift

mkdir -p "$OUT"

KDOTOOL="${KDOTOOL:-/usr/sbin/kdotool}"

ventana() {
    # la ventana del emulador (el AppImage se llama Flycast; el proceso hijo, Flycast-rend)
    "$KDOTOOL" search --name "Flycast" 2>/dev/null | head -1
}

t0=$(date +%s)

for seg in "$@"; do
    ahora=$(date +%s)
    falta=$(( seg - (ahora - t0) ))
    if [ "$falta" -gt 0 ]; then
        sleep "$falta"
    fi

    W=$(ventana)
    if [ -z "$W" ]; then
        echo "w${seg}s: no encontre la ventana del emulador"
        continue
    fi

    "$KDOTOOL" windowactivate "$W" >/dev/null 2>&1
    sleep 0.4

    spectacle -b -n -a -o "$OUT/w${seg}s.png" >/dev/null 2>&1

    if [ -f "$OUT/w${seg}s.png" ]; then
        echo "w${seg}s: OK $(stat -c%s "$OUT/w${seg}s.png") bytes (ventana $W)"
    else
        echo "w${seg}s: spectacle no dejo archivo"
    fi
done
