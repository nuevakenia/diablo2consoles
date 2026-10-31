#!/bin/bash
# DC-002.79 -- compilar y enlazar el POC de la LECTURA COMPLETA (LoadFile con archivos grandes).
#
# POR QUE UN SCRIPT PROPIO. Igual que build-zonas-277.sh y build-presupuestos-275.sh: el POC importa
# paquetes del MOTOR (d2asset con la lectura de consola) que no son stdlib. Se arma un arbol de ENLACE con
# enlaces simbolicos a la gox del motor y una orden.txt = stdlib + el cierre de lo que este POC importa. El
# cierre NO se escribe a mano: sale de `go list -deps` (con -e, porque hay paquetes que el host no resuelve).
#
# SE RECOMPILAN LOS PAQUETES QUE CAMBIA ESTA TARJETA, en el arbol BASE, para que lo que corre en Flycast sea
# el codigo de hoy y no un .gox viejo:
#   d2mpq            `TamanoDeArchivo`, `LeerEn` y la palanca del prefetch (lectura_entera.go)
#   d2loader/asset   la interfaz opcional SourceDeLecturaEntera
#   d2loader/mpq     la fuente MPQ que sabe el tamano y lee entero
#   d2loader         `TamanoDeArchivo` / `LeerEnteroEn`
#   d2asset          `LoadFile` por el camino de la consola (lectura_consola.go)
#
# Uso:  scripts/console/build-loadfile-279.sh [dir_salida]
#       BASE_GOX=...      arbol del motor compilado (por defecto /tmp/gox-med)
#       LECTURA=vieja|nueva   la palanca de la tarjeta (vieja = `io.ReadAll`, la prueba del ANTES)
#       MODO=archivo|buffer   LoadFile (buffer nuevo por archivo) o LoadFileEn (buffer reusado)
#       LIBERA=1          libera con free() el crudo de cada .dt1 despues del crc32
#       SALTAR_GEN=1      reusar el arbol de enlace ya armado
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
SRC="$REPO/console/dc/poc/loadfile-grande"
BASE_GOX="${BASE_GOX:-/tmp/gox-med}"
OUT="${1:-/tmp/dcbuild/loadfile279}"
GEN="$OUT/gox-enlace"
PREFIJO="github.com/OpenDiablo2/OpenDiablo2"
SRCB="$OUT/src"

# LOS PAQUETES DEL MOTOR QUE ESTE BUILD RECOMPILA EN EL ARBOL, en orden (cada uno se compila contra los
# .gox de los anteriores). La lista se usa DOS veces: la lee el chequeo del arbol de .gox (DC-002.91) para
# no quejarse de los que este mismo script rehace, y la recorre el paso 1/6. Una sola lista, porque dos se
# separan (la leccion de DC-002.90).
PAQUETES_MOTOR="d2common/d2datautils
 d2common/d2fileformats/d2dt1
 d2common/d2cache
 d2common/d2fileformats/d2mpq
 d2common/d2loader/asset
 d2common/d2loader/asset/types
 d2common/d2loader/mpq
 d2common/d2loader/filesystem
 d2common/d2loader
 d2core/d2asset"

[ -f "$BASE_GOX/orden.txt" ] || { echo "falta el arbol del motor en $BASE_GOX (corre build-motor-all.sh)" >&2; exit 1; }
mkdir -p "$OUT"

# ============================== EL ARBOL DE .gox (DC-002.91) ==============================
# ANTES de compilar: si un .gox del arbol es mas viejo que su fuente, gccgo muere por un simbolo que SI
# existe en el repo y el mensaje apunta al CODIGO (medido en DC-002.90: `zlib.NuevoInflador` con
# /tmp/gox-med del 30/10 13:22 contra la stdlib del 30/10 19:24). El chequeo dice el paquete, las dos
# fechas y la receta, y aborta. Los paquetes de PAQUETES_MOTOR no abortan: los rehace este mismo build.
BASE_GOX="$BASE_GOX" RECOMPILA="$PAQUETES_MOTOR" "$HERE/revisar-arbol-gox.sh"


echo "== 1/6 los paquetes del motor que se compilan aca (d2mpq, el loader y d2asset)"
for p in $PAQUETES_MOTOR; do
    mapfile -t SRC_P < <(ls "$REPO/$p"/*.go | grep -v '_test\.go$' |
        grep -vE '_(darwin|windows|js|wasm|plan9|freebsd|netbsd|openbsd|linux|android|ios|solaris|aix)\.go$' |
        grep -vE '_(386|amd64|arm|arm64|ppc64|ppc64le|mips|mipsle|mips64|mips64le|riscv64|s390x|loong64|wasm)\.go$' |
        grep -vE '/(marca_carga_host)\.go$')
    mkdir -p "$(dirname "$BASE_GOX/$PREFIJO/$p.gox")"
    sh-elf-gccgo $GODC_GOFLAGS -I "$BASE_GOX" \
        -fgo-pkgpath="$PREFIJO/$p" \
        -c "${SRC_P[@]}" \
        -o "$BASE_GOX/$PREFIJO/$p.gox"
    printf '   %-34s %9s B\n' "$p.gox" "$(stat -c%s "$BASE_GOX/$PREFIJO/$p.gox")"
done

# el cierre del POC, con `go list -deps` sobre los paquetes del MOTOR que importa
PAQUETES="$(cd "$REPO" && go list -deps -e -tags dreamcast \
    ./d2core/d2asset \
    ./d2common/d2fileformats/d2ds1 ./d2common/d2fileformats/d2mpq \
    ./d2common/d2interface ./d2common/d2enum 2>/dev/null |
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

echo "== 3/6 el directorio de fuentes del POC"
rm -rf "$SRCB"
mkdir -p "$SRCB"
cp "$SRC"/*.go "$SRCB"/
cp -r "$SRC/csrc" "$SRCB"/

# LECTURA=nueva prende el camino de esta tarjeta; sin la palanca queda el de siempre (`io.ReadAll`), que es
# la prueba del ANTES. MODO=buffer lee con `LoadFileEn` en un buffer reusado; si no, `LoadFile`.
LECTURA_VAL=false
[ "${LECTURA:-vieja}" = "nueva" ] && LECTURA_VAL=true
MODO_BUFFER_VAL=false
[ "${MODO:-archivo}" = "buffer" ] && MODO_BUFFER_VAL=true
LIBERA_VAL=false
[ "${LIBERA:-0}" = "1" ] && LIBERA_VAL=true
{
    printf 'package main\n\n'
    printf '// Palancas generadas por build-loadfile-279.sh. El porque de cada una, en knobs.go.\n'
    printf 'const lecturaDeConsola = %s\n' "$LECTURA_VAL"
    printf 'const modoBuffer = %s\n' "$MODO_BUFFER_VAL"
    printf 'const liberaCrudo = %s\n' "$LIBERA_VAL"
} > "$SRCB/knobs.go"
echo "   palancas: lecturaDeConsola=$LECTURA_VAL modoBuffer=$MODO_BUFFER_VAL liberaCrudo=$LIBERA_VAL"
echo "   $(ls "$SRCB"/*.go | wc -l) archivos .go: $(cd "$SRCB" && ls *.go | tr '\n' ' ')"

echo "== 4/6 compilando y enlazando contra los .gox del motor"
GODC_GOX="$GEN" bash "$HERE/build-dc.sh" "$SRCB" "$OUT" > "$OUT/build.log" 2>&1 || {
    echo "   FALLO el build (ver $OUT/build.log)"; tail -25 "$OUT/build.log"; exit 1; }

echo "== 5/6 la imagen con d2data.mpq en /JUEGO (el POC lo lee del disco)"
D2DIR="${D2_DIR:-$HOME/.wine-d2/drive_c/Games/Diablo II}"
MPQ_DATA="$(ls "$D2DIR" | grep -ix 'd2data.mpq' | head -1)"
[ -n "$MPQ_DATA" ] || { echo "   no encuentro d2data.mpq en $D2DIR" >&2; exit 1; }
LISTA="$OUT/iso-lista.txt"
printf '%s:/JUEGO/D2DATA.MPQ;1\n' "$D2DIR/$MPQ_DATA" > "$LISTA"
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
