#!/bin/bash
# DC-002.75 -- compilar y enlazar el POC de los PRESUPUESTOS DE CACHE: el mapa y el personaje del motor
# de DC-002.70, con los datos entrando por los caches del AssetManager y los presupuestos de consola.
#
# POR QUE UN SCRIPT PROPIO. Igual que build-estado-b-270.sh: este POC importa el backend de render
# (d2core/d2render/pvr) y ademas `d2core/d2asset` (los presupuestos y los caches), que son paquetes del
# MOTOR y no de la stdlib minima. Se arma un arbol de ENLACE con enlaces simbolicos a la gox del motor
# y una orden.txt = stdlib + el cierre de lo que este POC importa. El cierre NO se escribe a mano: sale
# de `go list -deps` (con -e, porque d2render/pvr importa "kos", que el host no resuelve).
#
# LOS DOS ARCHIVOS COMPARTIDOS (nivel.go y personaje.go) NO viven en este directorio: se copian AL
# DIRECTORIO DE BUILD desde el POC del personaje (la referencia de DC-002.38), y se compara su sha256
# contra la copia del verificador del host. Es lo que hace que la comparacion de pixeles contra
# DC-002.70 sea contra EL MISMO codigo de dibujado y no contra una copia que se pudo desincronizar.
#
# Uso:  scripts/console/build-presupuestos-275.sh [dir_salida]
#       BASE_GOX=...    arbol del motor compilado (por defecto /tmp/gox-std)
#       SALTAR_GEN=1    reusar el arbol de enlace ya armado
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
SRC="$REPO/console/dc/poc/presupuestos"
POC_REF="$REPO/console/dc/poc/personaje-mapa"
VERIF="$REPO/console/dc/verif-personaje-238"
BASE_GOX="${BASE_GOX:-/tmp/gox-std}"
OUT="${1:-/tmp/dcbuild/presup275}"
GEN="$OUT/gox-enlace"
PREFIJO="github.com/OpenDiablo2/OpenDiablo2"
SRCB="$OUT/src"

[ -f "$BASE_GOX/orden.txt" ] || { echo "falta el arbol del motor en $BASE_GOX (corre build-motor-all.sh)" >&2; exit 1; }
mkdir -p "$OUT"

echo "== 0/6 los archivos compartidos (los mismos del POC y del verificador)"
for f in nivel.go personaje.go; do
    if ! cmp -s "$POC_REF/$f" "$VERIF/$f"; then
        echo "   FALLO: $POC_REF/$f y $VERIF/$f NO son el mismo archivo" >&2
        exit 1
    fi
    echo "   $f $(sha256sum "$POC_REF/$f" | cut -c1-16)"
done

echo "== 1/6 los paquetes del motor que se compilan aca (d2cache, d2asset y pvr, con los presupuestos)"
for p in d2common/d2cache d2core/d2asset d2core/d2render/pvr; do
    mapfile -t SRC_P < <(ls "$REPO/$p"/*.go | grep -v '_test\.go$')
    sh-elf-gccgo $GODC_GOFLAGS -I "$BASE_GOX" \
        -fgo-pkgpath="$PREFIJO/$p" \
        -c "${SRC_P[@]}" \
        -o "$BASE_GOX/$PREFIJO/$p.gox"
    printf '   %-28s %9s B\n' "$(basename "$p").gox" "$(stat -c%s "$BASE_GOX/$PREFIJO/$p.gox")"
done

# el cierre del POC, con `go list -deps` sobre los paquetes del MOTOR que importa
PAQUETES="$(cd "$REPO" && go list -deps -e -tags dreamcast \
    ./d2core/d2asset ./d2core/d2render/pvr \
    ./d2common/d2fileformats/d2ds1 ./d2common/d2fileformats/d2dt1 ./d2common/d2fileformats/d2mpq \
    ./d2common/d2fileformats/d2cof ./d2common/d2fileformats/d2dcc ./d2common/d2fileformats/d2dc6 \
    ./d2common/d2path ./d2common/d2math/d2vector 2>/dev/null |
    grep "$PREFIJO" | sed "s|.*$PREFIJO/||" | sort | tr '\n' ' ')"
[ -n "${PAQUETES// /}" ] || { echo "la lista de paquetes quedo vacia (go list -deps fallo?)" >&2; exit 1; }

echo "== 2/6 el arbol de enlace ($GEN)"
if [ "${SALTAR_GEN:-0}" != "1" ]; then
    rm -rf "$GEN"
    mkdir -p "$GEN"
    # enlaces simbolicos, no copias: los .gox del motor son decenas de MB y no se tocan desde aca
    cp -rs "$BASE_GOX/." "$GEN"/
fi
rm -f "$GEN/orden.txt"
cp "$BASE_GOX/orden.txt" "$GEN/orden.txt"
for p in $PAQUETES; do
    f="$GEN/$PREFIJO/$p.gox"
    [ -f "$f" ] || { echo "   FALTA $p.gox en el arbol del motor: no se puede enlazar lo que no existe" >&2; exit 1; }
    grep -qxF "$f" "$GEN/orden.txt" || echo "$f" >> "$GEN/orden.txt"
done
echo "   paquetes del motor en el enlace: $(printf '%s\n' $PAQUETES | wc -l)"
echo "   objetos en orden.txt: $(wc -l < "$GEN/orden.txt")"

echo "== 3/6 el directorio de fuentes del POC (con los dos compartidos)"
rm -rf "$SRCB"
mkdir -p "$SRCB"
cp "$SRC"/*.go "$SRCB"/
cp -r "$SRC/csrc" "$SRCB"/
cp "$POC_REF/nivel.go" "$POC_REF/personaje.go" "$SRCB"/
echo "   $(ls "$SRCB"/*.go | wc -l) archivos .go: $(cd "$SRCB" && ls *.go | tr '\n' ' ')"

echo "== 4/6 compilando y enlazando contra los .gox del motor"
GODC_GOX="$GEN" bash "$HERE/build-dc.sh" "$SRCB" "$OUT" > "$OUT/build.log" 2>&1 || {
    echo "   FALLO el build (ver $OUT/build.log)"; tail -25 "$OUT/build.log"; exit 1; }

echo "== 5/6 la imagen con los DOS MPQ en /JUEGO (el POC los lee del disco)"
D2DIR="${D2_DIR:-$HOME/.wine-d2/drive_c/Games/Diablo II}"
MPQ_DATA="$(ls "$D2DIR" | grep -ix 'd2data.mpq' | head -1)"
MPQ_CHARS="$(ls "$D2DIR" | grep -ix 'd2char.mpq' | head -1)"
[ -n "$MPQ_DATA" ] || { echo "   no encuentro d2data.mpq en $D2DIR" >&2; exit 1; }
[ -n "$MPQ_CHARS" ] || { echo "   no encuentro d2char.mpq en $D2DIR" >&2; exit 1; }
LISTA="$OUT/iso-lista.txt"
printf '%s:/JUEGO/D2DATA.MPQ;1\n%s:/JUEGO/D2CHAR.MPQ;1\n' \
    "$D2DIR/$MPQ_DATA" "$D2DIR/$MPQ_CHARS" > "$LISTA"
MKISO_JOLIET=0 "$DC_PYTHON" "$HERE/mkiso.py" "$OUT/disc.iso" "$OUT/IP.BIN" "$OUT/1ST_READ.BIN" "@$LISTA" | tail -2
"$DC_PYTHON" "$HERE/mkgdi.py" "$OUT" | tail -2

echo "== 6/6 lo que salio"
for f in prog.elf 1ST_READ.BIN disc.iso; do
    [ -f "$OUT/$f" ] && printf '   %-14s %10s B\n' "$f" "$(stat -c%s "$OUT/$f")"
done
printf '   codigo (.text)  %10s B\n' "$(sh-elf-size -A "$OUT/prog.elf" | awk '$1==".text"{print $2}')"
printf '   rodata          %10s B\n' "$(sh-elf-size -A "$OUT/prog.elf" | awk '$1==".rodata"{print $2}')"
printf '   data+bss        %10s B\n' "$(sh-elf-size -A "$OUT/prog.elf" | awk '$1==".data"||$1==".bss"{s+=$2} END{print s}')"
[ -f "$OUT/disc.gdi" ] && echo "   imagen:         $OUT/disc.gdi"
