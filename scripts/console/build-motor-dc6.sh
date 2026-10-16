#!/bin/bash
# Build completo del POC del motor (DC-002.3): hornea un DC6 crudo del MPQ, compila la stdlib minima
# y los paquetes del motor para SH-4, y arma el ELF arrancable.
#
# Uso:  scripts/console/build-motor-dc6.sh [dir_salida]     (por defecto /tmp/dcbuild/motor-dc6)
#       D2_DIR='/ruta/Diablo II' para decir donde esta el juego si no esta en los caminos tipicos.
#
# Despues:  scripts/console/compare-motor-dc6.sh    (corre la consola y compara contra el host)
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
SRC="$REPO/console/dc/poc/motor-dc6"
OUT="${1:-/tmp/dcbuild/motor-dc6}"
CONF="$SRC/asset.conf"

get() { sed -n "s/^$1=//p" "$CONF" | head -1; }

MPQNAME="$(get mpq)"
DC6="$(get dc6)"
NAME="$(get name)"
: "${MPQNAME:?falta mpq= en $CONF}"
: "${DC6:?falta dc6= en $CONF}"
: "${NAME:=asset}"

# --- carpeta de Diablo II: $D2_DIR, o los caminos tipicos de Wine ---
D2DIR="${D2_DIR:-}"
if [ -z "$D2DIR" ]; then
    for c in "$HOME/.wine-d2/drive_c/Games/Diablo II" \
             "$HOME/.wine/drive_c/Games/Diablo II" \
             "$HOME/Games/Diablo II"; do
        if [ -f "$c/$MPQNAME" ]; then D2DIR="$c"; break; fi
    done
fi
[ -n "$D2DIR" ] && [ -f "$D2DIR/$MPQNAME" ] || {
    echo "ERROR: no encuentro $MPQNAME. Pasa la carpeta: D2_DIR='/ruta/Diablo II' $0 $*" >&2
    exit 1
}

echo "== 1/4 horneando el DC6 crudo del MPQ ($D2DIR/$MPQNAME)"
mkdir -p "$OUT/gen"
(cd "$REPO" && go build -o "$OUT/.bake-dc6" ./console/dc/bake-dc6)
"$OUT/.bake-dc6" -mpq "$D2DIR/$MPQNAME" -dc6 "$DC6" -out "$OUT" -name "$NAME"

echo
echo "== 2/4 stdlib minima para SH-4"
bash "$HERE/build-std-gox.sh"

echo
echo "== 3/4 paquetes del motor para SH-4"
bash "$HERE/build-motor-gox.sh"

echo
echo "== 4/4 ELF del POC"
bash "$HERE/build-dc.sh" "$SRC" "$OUT"

echo
echo "elf:   $OUT/prog.elf"
echo "dc6:   $OUT/$NAME.dc6   (el mismo que consume el comparador del host)"
