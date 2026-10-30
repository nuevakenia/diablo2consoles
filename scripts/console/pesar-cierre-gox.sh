#!/bin/bash
# Suma .text + .rodata de los .gox que cierran sobre un paquete, para saber cuanto pesa un POC que lo importe.
# Uso: scripts/console/pesar-cierre-gox.sh <arbol_gox> <paquete>...
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
GEN="${1:?falta el arbol de .gox}"
shift
PREFIJO="github.com/OpenDiablo2/OpenDiablo2"

PAQ=$(cd "$REPO" && go list -deps -e -tags dreamcast "$@" 2>/dev/null | grep "$PREFIJO" | sed "s|.*$PREFIJO/||")

T=0; R=0; N=0; FALTAN=""
for p in $PAQ; do
    f="$GEN/$PREFIJO/$p.gox"
    if [ ! -f "$f" ]; then FALTAN="$FALTAN $p"; continue; fi
    # OJO: los .gox traen -ffunction-sections / -fdata-sections, asi que las secciones NO se llaman
    # ".text" sino ".text.<simbolo>": hay que sumar por PREFIJO o la cuenta da cero.
    linea=$(sh-elf-size -A "$f" | awk '
        $1 ~ /^\.text/ {t+=$2}
        $1 ~ /^\.rodata/ {r+=$2}
        END{printf "%d %d", t+0, r+0}')
    t=$(echo "$linea" | cut -d' ' -f1)
    r=$(echo "$linea" | cut -d' ' -f2)
    T=$((T+t)); R=$((R+r)); N=$((N+1))
    printf '   %-52s %9s %9s\n' "$p" "$t" "$r"
done

echo "---"
echo "   paquetes del motor: $N   text: $T   rodata: $R   suma: $((T+R)) B"
[ -n "$FALTAN" ] && echo "   SIN .gox:$FALTAN"
exit 0
