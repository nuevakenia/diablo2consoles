#!/bin/bash
# DC-002.70 -- compilar y enlazar el POC del ESTADO B: el mapa y el personaje del motor dibujados por
# d2interface.Surface con el backend PVR (d2core/d2render/pvr) y su Renderer.
#
# POR QUE UN SCRIPT PROPIO (y no alcanza build-dc.sh): este POC importa el backend de render (d2render/pvr),
# que es un paquete del MOTOR y no de la stdlib minima: sus .gox los produce la compilacion del motor. Se
# arma, igual que build-pvr-surface.sh, un arbol de ENLACE con enlaces simbolicos a la gox del motor y una
# orden.txt = stdlib + SOLO los paquetes del cierre de lo que este POC importa. El cierre NO se escribe a
# mano: sale de `go list -deps` (con -e, porque d2render/pvr importa "kos", que el host no resuelve).
#
# LOS DOS ARCHIVOS COMPARTIDOS (nivel.go y personaje.go) NO se copian al repo: se copian AL DIRECTORIO DE
# BUILD desde el POC del personaje (que es la referencia), y se compara su sha256 con la copia del
# verificador del host. Asi la comparacion de pixeles contra el POC es contra EL MISMO codigo, y no hay dos
# copias que se puedan desincronizar en silencio.
#
# Uso:  scripts/console/build-estado-b-270.sh [dir_salida]
#       BASE_GOX=...    arbol del motor compilado (por defecto /tmp/gox-std)
#       SALTAR_GEN=1    reusar el arbol de enlace ya armado
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
SRC="$REPO/console/dc/poc/estado-b"
POC_REF="$REPO/console/dc/poc/personaje-mapa"
VERIF="$REPO/console/dc/verif-personaje-238"
BASE_GOX="${BASE_GOX:-/tmp/gox-std}"
OUT="${1:-/tmp/dcbuild/estado270}"
GEN="$OUT/gox-enlace"
PREFIJO="github.com/OpenDiablo2/OpenDiablo2"
SRCB="$OUT/src"

[ -f "$BASE_GOX/orden.txt" ] || { echo "falta el arbol del motor en $BASE_GOX (corre build-motor-all.sh)" >&2; exit 1; }
mkdir -p "$OUT"

echo "== 0/5 los archivos compartidos (los mismos del POC y del verificador)"
for f in nivel.go personaje.go; do
    if ! cmp -s "$POC_REF/$f" "$VERIF/$f"; then
        echo "   FALLO: $POC_REF/$f y $VERIF/$f NO son el mismo archivo" >&2
        exit 1
    fi
    echo "   $f $(sha256sum "$POC_REF/$f" | cut -c1-16)"
done

echo "== 1/5 el paquete del backend (pvr.gox, con el Renderer de DC-002.70)"
sh-elf-gccgo $GODC_GOFLAGS -I "$BASE_GOX" \
    -fgo-pkgpath="$PREFIJO/d2core/d2render/pvr" \
    -c "$REPO"/d2core/d2render/pvr/*.go \
    -o "$BASE_GOX/$PREFIJO/d2core/d2render/pvr.gox"
echo "   pvr.gox $(stat -c%s "$BASE_GOX/$PREFIJO/d2core/d2render/pvr.gox") B"

# el cierre del POC, con `go list -deps` sobre los paquetes del MOTOR que importa (los dos archivos
# compartidos traen d2ds1/d2dt1/d2mpq/d2cof/d2dcc/d2dc6/d2path/d2vector; el main trae el backend)
PAQUETES="$(cd "$REPO" && go list -deps -e -tags dreamcast \
    ./d2core/d2render/pvr \
    ./d2common/d2fileformats/d2ds1 ./d2common/d2fileformats/d2dt1 ./d2common/d2fileformats/d2mpq \
    ./d2common/d2fileformats/d2cof ./d2common/d2fileformats/d2dcc ./d2common/d2fileformats/d2dc6 \
    ./d2common/d2path ./d2common/d2math/d2vector 2>/dev/null |
    grep "$PREFIJO" | sed "s|.*$PREFIJO/||" | sort | tr '\n' ' ')"
[ -n "${PAQUETES// /}" ] || { echo "la lista de paquetes quedo vacia (go list -deps fallo?)" >&2; exit 1; }

echo "== 2/5 el arbol de enlace ($GEN)"
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

echo "== 3/5 el directorio de fuentes del POC (con los dos compartidos)"
rm -rf "$SRCB"
mkdir -p "$SRCB"
cp "$SRC"/*.go "$SRCB"/
cp -r "$SRC/csrc" "$SRCB"/
cp "$POC_REF/nivel.go" "$POC_REF/personaje.go" "$SRCB"/
echo "   $(ls "$SRCB"/*.go | wc -l) archivos .go: $(cd "$SRCB" && ls *.go | tr '\n' ' ')"

echo "== 4/5 compilando y enlazando contra los .gox del motor"
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
