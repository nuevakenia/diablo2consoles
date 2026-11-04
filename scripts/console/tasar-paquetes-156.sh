#!/bin/bash
# DC-002.156: cuanto pesa cada paquete del motor (text+rodata+data) en el arbol de .gox.
#
# OJO CON LA TRAMPA: el tamano del ARCHIVO .gox NO es lo que pesa en la imagen. El .gox trae el
# `.go_export` (los descriptores de tipo y las firmas que el compilador de al lado necesita) y eso NO se
# enlaza: medido, `d2gamescreen.gox` son 6.381.316 B de archivo y 1.213.247 B de text+data. Lo que mide
# este script es lo segundo, que es lo que el enlace mete en la imagen.
#
# Uso: scripts/console/tasar-paquetes-156.sh [dir_gox] [salida.tsv]
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"
GEN="${1:-/tmp/dc155/gox}"
SAL="${2:-/tmp/dc156-pesos.tsv}"
: > "$SAL"
while read -r gox; do
    [ -f "$gox" ] || continue
    size=$(sh-elf-size -A "$gox" 2>/dev/null | awk '/^\.text/ || /^\.rodata/ || /^\.data/ { s += $2 } END { print s + 0 }')
    rel="${gox#"$GEN"/}"
    printf '%s\t%d\n' "${rel%.gox}" "$size" >> "$SAL"
done < "$GEN/orden-motor.txt"
sort -t'	' -k2 -rn "$SAL"
