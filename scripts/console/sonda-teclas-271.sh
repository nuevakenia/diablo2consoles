#!/bin/bash
# DC-002.71: SONDA DEL MAPEO DEL EMULADOR. Una corrida POR TECLA, para no tener que adivinar a que tecla
# pertenece cada boton que aparece en el log.
#
# POR QUE UNA CORRIDA POR TECLA (medido): mandando una secuencia de teclas con marcas (la cruceta) entre
# medio, los botones que aparecian en el log no quedaban alineados con las teclas de forma inequivoca (dos
# corridas dieron resultados que no coinciden: el emulador procesa la entrada en su propio loop y las teclas
# se solapan con los tiempos del POC). Con UNA tecla por corrida no hay nada que alinear: si el POC imprime un
# boton, es de esa tecla.
#
# Uso:  scripts/console/sonda-teclas-271.sh <disc.gdi> <dir_salida> [tecla...]
#       (por defecto prueba las teclas candidatas del teclado del emulador)
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMG="${1:?uso: sonda-teclas-271.sh <disc.gdi> <dir_salida> [tecla...]}"
DIR="${2:?uso: sonda-teclas-271.sh <disc.gdi> <dir_salida> [tecla...]}"
shift 2
TECLAS="${*:-z x c v y s d f a q w e r enter espacio}"
mkdir -p "$DIR"

for k in $TECLAS; do
    OUT="$DIR/$k"
    mkdir -p "$OUT"
    echo "================ tecla del teclado: $k ================"
    GUION_TECLAS="$k 3" bash "$HERE/run-flycast-271.sh" "$IMG" "$OUT" > "$OUT/corrida.txt" 2>&1
    # el estado del control sin las lineas de "nada apretado" y sin los latidos repetidos
    grep -a 'HITO271: CONTROL cuadro\|HITO271: TECLA\|HITO271: MOUSE\|HITO271: FLANCO' "$OUT/flycast.log" 2>/dev/null |
        tr -d '\r' |
        grep -v 'botones= 0 apretados= - palanca= 0 0 gatillos= 0 0' |
        sed 's/^HITO271: /  /' | head -8
    echo
done
