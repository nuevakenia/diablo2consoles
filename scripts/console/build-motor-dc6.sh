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
# El POC se elige por variable: por defecto el de DC-002.3 (motor-dc6). motor-pvr usa el mismo flujo
# con otro POC (scripts/console/build-motor-pvr.sh), asi que la barrera del hito no cambia.
POC="${POC:-motor-dc6}"
SRC="$REPO/console/dc/poc/$POC"
OUT="${1:-/tmp/dcbuild/$POC}"
CONF="$SRC/asset.conf"

get() { sed -n "s/^$1=//p" "$CONF" | head -1; }

MPQNAME="$(get mpq)"
DC6="$(get dc6)"
PAL="$(get pal)"
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
BAKE=(-mpq "$D2DIR/$MPQNAME" -dc6 "$DC6" -out "$OUT" -name "$NAME")
# si el POC declara pal=... en su asset.conf, se hornea tambien la paleta (DC-002.9: el color)
if [ -n "$PAL" ]; then
    BAKE+=(-pal "$PAL")
fi
"$OUT/.bake-dc6" "${BAKE[@]}"

echo
echo "== 2/4 stdlib minima para SH-4"
# SALTAR_STDLIB=1 usa la stdlib minima que YA este compilada en $GODC_GOX en vez de rehacerla (el
# build de la stdlib arranca con un `rm -rf` del directorio de .gox). Para que sirve: cuando en la
# misma maquina hay otra sesion trabajando el repo y esa gox es COMPARTIDA, rehacerla le rompe el
# build a la otra sesion. Este build de DC-002.9 corrio asi, con su propia gox
# (GODC_GOX=/tmp/dcgox-dc0029) para no tocar la compartida. Medido: la stdlib minima son 41
# paquetes y se compila en 7 s, asi que saltearla es una comodidad, no una necesidad.
if [ "${SALTAR_STDLIB:-0}" = "1" ]; then
    echo "   (salteada: SALTAR_STDLIB=1, se usa la ya compilada en ${GODC_GOX:-$DC_ROOT/gox})"
else
    bash "$HERE/build-std-gox.sh"
fi

echo
echo "== 3/4 paquetes del motor para SH-4"
bash "$HERE/build-motor-gox.sh"

echo
echo "== 4/4 ELF del POC"
# POC_ASSET_CRUDO=1: este POC trae asset.conf (bytes CRUDOS), o sea que el guard de build-dc.sh que
# avisa "esto no se arma por aca" tiene que dejar pasar la llamada. El aviso existe porque armar un POC
# crudo con build-dc.sh a mano falla con un error que manda a buscar el problema donde no esta.
POC_ASSET_CRUDO=1 bash "$HERE/build-dc.sh" "$SRC" "$OUT"

echo
echo "elf:   $OUT/prog.elf"
echo "dc6:   $OUT/$NAME.dc6   (el mismo que consume el comparador del host)"
