#!/bin/bash
# Desglose de la IMAGEN del binario por paquete: .text, .rodata, .go_export, .data, .bss.
#
# Por que existe: para decidir donde recortar hay que saber que pesa. El .go_export NO lleva el flag
# ALLOC, asi que no viaja en el binario ni ocupa RAM: contarlo como "gordo" es un error de lectura
# (son 11,4 MB que mueren en el enlace).
#
# Uso:  scripts/console/medir-imagen.sh [dir_gox] [salida]
#       por defecto: /tmp/gox-med y /tmp/td-desglose.txt
#
# Antes hay que tener los .gox del motor: scripts/console/build-std-gox.sh <dir> &&
# scripts/console/build-motor-all.sh <dir> <dir_log>
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

GEN="${1:-/tmp/gox-med}"
OUT="${2:-/tmp/td-desglose.txt}"
: > "$OUT"

if [ ! -d "$GEN" ]; then
    echo "ERROR: no existe el arbol de .gox: $GEN" >&2
    echo "       corre: scripts/console/build-std-gox.sh $GEN && scripts/console/build-motor-all.sh $GEN /tmp/log-med" >&2
    exit 1
fi

for f in $(find "$GEN" -name "*.gox"); do
    p="$(basename "$f" .gox)"
    sh-elf-objdump -h "$f" 2>/dev/null | gawk -v pkg="$p" '
        $1 ~ /^[0-9]+$/ {
            n = $2
            v = strtonum("0x" $3)
            if (n ~ /^\.go_export/)      gexp[pkg] += v
            else if (n ~ /^\.rodata/)    ro[pkg]   += v
            else if (n ~ /^\.text/)      tx[pkg]   += v
            else if (n ~ /^\.data/)      da[pkg]   += v
            else if (n ~ /^\.bss/)       bs[pkg]   += v
        }
        END { printf "%d %d %d %d %d %s\n", tx[pkg], ro[pkg], gexp[pkg], da[pkg], bs[pkg], pkg }
    '
done > "$OUT"

echo "=== por seccion (todo el arbol de .gox):"
gawk '{t+=$1;r+=$2;e+=$3;d+=$4;b+=$5} END {
  printf "  text        %10d B  %6.2f MB\n", t, t/1048576
  printf "  rodata      %10d B  %6.2f MB   <- lo unico ALLOC de datos\n", r, r/1048576
  printf "  go_export   %10d B  %6.2f MB   <- NO ALLOC: no va al binario\n", e, e/1048576
  printf "  data        %10d B  %6.2f MB\n", d, d/1048576
  printf "  bss         %10d B  %6.2f MB\n", b, b/1048576
  printf "  IMAGEN (text+rodata+data+bss) %d B  %.2f MB\n", t+r+d+b, (t+r+d+b)/1048576
}' "$OUT"

echo
echo "=== top 20 por imagen (text+rodata):"
gawk '{printf "%9d %9d %9d  %s\n", $1+$2, $1, $2, $6}' "$OUT" | sort -rn | head -20
echo
echo "detalle por paquete en: $OUT"
