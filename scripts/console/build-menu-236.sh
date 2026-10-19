#!/bin/bash
# DC-002.36 -- build del MENU PRINCIPAL en la Dreamcast: los DC6 del menu (del MPQ del juego, por el
# HOST) -> el disco de la consola -> el POC que los lee, los decodifica con el decodificador del motor y
# los pinta con el PVR.
#
# Que arma (y por que asi):
#   1. LOS ARCHIVOS DEL MENU y EL PLAN DE DIBUJO, en el host: `console/dc/menu-host-236 -mpqdir ... -out`
#      saca cada asset crudo del MPQ que le toca (medido en el inventario de la tarjeta) y `-componer`
#      arma el plan + las referencias PNG con el codigo del motor. Los dos van al MISMO directorio, que
#      es lo que viaja al disco.
#   2. los .gox de la stdlib minima y del MOTOR (d2dc6 y lo que arrastra) para SH-4.
#   3. el ELF del POC (build-dc.sh) y su ISO/GDI de arranque, con /MENU adentro (mkdisco-236.sh).
#
# Uso:  scripts/console/build-menu-236.sh [dir_salida]
#       SALTAR_HOST=1  -> reusar lo que ya este en <salida>/MENU (no re-extrae)
#       SALTAR_STDLIB=1 / SALTAR_MOTOR=1 -> reusar los .gox de $GODC_GOX
#       SIN_ASSET=FONDO.DC6 -> el MUTANTE: ese asset no viaja al disco (criterio 3 de la tarjeta)
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
POC="menu-dc-236"
SRC="$REPO/console/dc/poc/$POC"
OUT="${1:-${DC36_OUT:-$HOME/.hermes/cache/scratch/dc36/build}}"
GEN="${GODC_GOX:-$DC_ROOT/gox}"

D2DIR="${D2_DIR:-$HOME/.wine-d2/drive_c/Games/Diablo II}"

mkdir -p "$OUT" "$OUT/MENU"

echo "== 0/5 los assets del menu + el plan de dibujo (host, con el codigo del motor)"
if [ "${SALTAR_HOST:-0}" = "1" ] && [ -f "$OUT/MENU/PLAN.TXT" ]; then
    echo "   (salteado: SALTAR_HOST=1 y ya esta $OUT/MENU/PLAN.TXT)"
else
    ( cd "$REPO" && go run ./console/dc/menu-host-236 -mpqdir "$D2DIR" -out "$OUT/MENU" ) | sed 's/^/   /'
    ( cd "$REPO" && go run ./console/dc/menu-host-236 -menu "$OUT/MENU" -componer -foco 0 ) | sed 's/^/   /'
fi

echo
echo "== 1/5 stdlib minima + paquetes del motor para SH-4"
if [ "${SALTAR_STDLIB:-0}" = "1" ]; then
    echo "   (salteada: SALTAR_STDLIB=1)"
else
    bash "$HERE/build-std-gox.sh"
fi

if [ "${SALTAR_MOTOR:-0}" = "1" ]; then
    echo "   (paquetes del motor salteados: SALTAR_MOTOR=1)"
else
    PAQUETES="d2common/d2datautils d2common/d2fileformats/d2dc6" bash "$HERE/build-motor-gox.sh" "$GEN"
fi

echo
echo "== 2/5 el ELF del POC"
TMPPOC="$OUT/poc-src"
rm -rf "$TMPPOC"
mkdir -p "$TMPPOC"
cp -r "$SRC/." "$TMPPOC/"

# EL ROMDISK: los 20 assets + el plan VIAJAN ADENTRO DEL ELF, montados en /rd.
#
# POR QUE (medido en DC-002.36): el mismo POC leyendo los assets DEL DISCO no es confiable en este host --
# el disco con la carpeta /MENU no arranca (0 marcas del constructor de C de libgodc: el binario ni corre;
# ver mkdisco-236.sh), y el disco con los mismos 20 archivos en la RAIZ arranca con un binario y con otro
# de 352 bytes mas grande NO (medido), asi que el arranque depende de una combinacion de layout del ISO y
# del binario que no esta medida. El romdisk saca al disco de la ecuacion: los archivos van adentro del
# ELF (build-dc.sh los empaqueta con genromfs + bin2o cuando existe <poc>/romdisk) y libgodc lo monta en
# /rd (runtime/romdisk_init.c). El POC busca sus assets en /rd, y despues en el disco, en ese orden.
ROM="$TMPPOC/romdisk"
mkdir -p "$ROM"
for f in "$OUT"/MENU/*; do
    [ -f "$f" ] || continue   # solo archivos: el subdirectorio de referencias no viaja
    if [ -n "${SIN_ASSET:-}" ] && [ "$(basename "$f")" = "$SIN_ASSET" ]; then
        echo "   MUTANTE: NO viaja $SIN_ASSET (ni al romdisk)"
        continue
    fi
    cp -f "$f" "$ROM/"
done
echo "   romdisk: $(ls "$ROM" | wc -l) archivos del menu, $(du -sh "$ROM" | cut -f1) -> /rd"

bash "$HERE/build-dc.sh" "$TMPPOC" "$OUT"

echo
echo "== 3/5 la ISO del disco con /MENU"
SIN_ASSET="${SIN_ASSET:-}" SIN_CARPETA="${SIN_CARPETA:-1}" bash "$HERE/mkdisco-236.sh" "$OUT"

echo
echo "== 4/5 lo que quedo"
ls -la "$OUT"/prog.elf "$OUT"/disc.iso "$OUT"/disc.gdi 2>/dev/null | sed 's/^/   /'
echo "   MENU: $(ls "$OUT/MENU" | wc -l) archivos, $(du -sh "$OUT/MENU" | cut -f1)"
echo
echo "== 5/5 la referencia del host (la que se compara contra la captura)"
ls -la "$OUT"/MENU/referencias/referencia-*.png 2>/dev/null | sed 's/^/   /'
