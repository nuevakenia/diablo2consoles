#!/bin/bash
# DC-002.96 -- compilar y enlazar el POC de la SOURCE DE CONSOLA: los seis formatos con dueno y el monton
# que vuelve al nivel de partida.
#
# POR QUE UN SCRIPT PROPIO. Igual que build-zonas-277.sh y build-loadfile-279.sh: el POC importa paquetes
# del MOTOR (los decodificadores, d2mpq, el loader) que no son stdlib, mas el paquete NUEVO de esta tarjeta
# (`console/dc/fuenteconsola`). Se arma un arbol de ENLACE con enlaces simbolicos a la gox del motor y una
# orden.txt = stdlib + el cierre de lo que este POC importa; el cierre NO se escribe a mano, sale de
# `go list -deps` (con -e, porque d2render/pvr y compania no resuelven en el host).
#
# SE RECOMPILAN EN EL ARBOL BASE los paquetes que esta tarjeta mide, para que lo que corre en Flycast sea el
# codigo de hoy y no un .gox viejo (el defecto de DC-002.90, medido): los seis decodificadores, d2mpq, la
# cadena del loader y `console/dc/fuenteconsola`, que es el paquete de la tarjeta.
#
# EL MAPA ES EL MISMO CODIGO. `nivel.go` NO vive en este POC: se copia al directorio de compilacion desde el
# POC del personaje (la referencia de DC-002.38) y se compara su sha256 contra la copia del verificador del
# host, igual que en DC-002.70/.75/.77/.81. Asi el crc32 del lienzo del pueblo tiene que dar 3507647456.
#
# Uso:  scripts/console/build-fuente-296.sh [dir_salida]
#       BASE_GOX=...   arbol del motor compilado (por defecto /tmp/gox-med)
#       NO_LIBERA=1    EL MUTANTE: el dueno no devuelve el tramo (el monton tiene que quedar tomado)
#       SALTAR_GEN=1   reusar el arbol de enlace ya armado
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
SRC="$REPO/console/dc/poc/fuente-consola"
POC_REF="$REPO/console/dc/poc/personaje-mapa"
VERIF="$REPO/console/dc/verif-personaje-238"
BASE_GOX="${BASE_GOX:-/tmp/gox-med}"
OUT="${1:-/tmp/dcbuild/fuente296}"
GEN="$OUT/gox-enlace"
PREFIJO="github.com/OpenDiablo2/OpenDiablo2"
SRCB="$OUT/src"

# LOS PAQUETES QUE ESTE BUILD RECOMPILA EN EL ARBOL, en orden (cada uno se compila contra los .gox de los
# anteriores). La lista se usa DOS veces: la lee el chequeo del arbol de .gox (DC-002.91) para no quejarse de
# los que este mismo script rehace, y la recorre el paso 1/6. Una sola lista, porque dos se separan (la
# leccion de DC-002.90).
#
# El ULTIMO es el paquete de esta tarjeta. Va al final a proposito: importa d2mpq, d2loader y
# d2loader/asset, asi que necesita esos .gox ya compilados.
PAQUETES_MOTOR="d2common/d2datautils
 d2common/d2fileformats/d2dt1
 d2common/d2fileformats/d2ds1
 d2common/d2fileformats/d2dc6
 d2common/d2fileformats/d2cof
 d2common/d2fileformats/d2mpq
 d2common/d2cache
 d2common/d2loader/asset
 d2common/d2loader/asset/types
 d2common/d2loader/mpq
 d2common/d2loader/filesystem
 d2common/d2loader
 console/dc/fuenteconsola"

[ -f "$BASE_GOX/orden.txt" ] || { echo "falta el arbol del motor en $BASE_GOX (corre build-motor-all.sh)" >&2; exit 1; }
mkdir -p "$OUT"

# ============================== EL ARBOL DE .gox (DC-002.91) ==============================
# ANTES de compilar: si un .gox del arbol es mas viejo que su fuente, gccgo muere por un simbolo que SI
# existe en el repo y el mensaje apunta al CODIGO. El chequeo dice el paquete, las dos fechas y la receta, y
# aborta. Los paquetes de PAQUETES_MOTOR no abortan: los rehace este mismo build.
BASE_GOX="$BASE_GOX" RECOMPILA="$PAQUETES_MOTOR" "$HERE/revisar-arbol-gox.sh"

echo "== 0/6 el archivo compartido del mapa (el mismo del POC y del verificador)"
for f in nivel.go; do
    if ! cmp -s "$POC_REF/$f" "$VERIF/$f"; then
        echo "   FALLO: $POC_REF/$f y $VERIF/$f NO son el mismo archivo" >&2
        exit 1
    fi
    echo "   $f $(sha256sum "$POC_REF/$f" | cut -c1-16)"
done

echo "== 1/6 los paquetes que se compilan aca (los seis decodificadores, d2mpq, el loader y la fuente)"
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
    printf '   %-34s %9s B\n' "$(basename "$p").gox" "$(stat -c%s "$BASE_GOX/$PREFIJO/$p.gox")"
done

# el cierre del POC, con `go list -deps` sobre los paquetes que importa
PAQUETES="$(cd "$REPO" && go list -deps -e -tags dreamcast \
    ./console/dc/fuenteconsola \
    ./d2common/d2fileformats/d2cof ./d2common/d2fileformats/d2dc6 \
    ./d2common/d2fileformats/d2ds1 ./d2common/d2fileformats/d2dt1 ./d2common/d2fileformats/d2mpq \
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

echo "== 3/6 el directorio de fuentes del POC (con el mapa compartido)"
rm -rf "$SRCB"
mkdir -p "$SRCB"
cp "$SRC"/*.go "$SRCB"/
cp -r "$SRC/csrc" "$SRCB"/
cp "$POC_REF/nivel.go" "$SRCB"/

# NO_LIBERA=1 es EL MUTANTE: la fuente no tiene liberador y el tramo se queda tomado. Con el mutante el
# monton NO vuelve a la base, que es lo que tiene que pasar para que el bloque con dueno signifique algo.
NO_LIBERA_VAL=false
[ "${NO_LIBERA:-0}" = "1" ] && NO_LIBERA_VAL=true
{
    printf 'package main\n\n'
    printf '// Palanca generada por build-fuente-296.sh. El porque, en knobs.go.\n'
    printf 'const noLibera = %s\n' "$NO_LIBERA_VAL"
} > "$SRCB/knobs.go"
echo "   palancas: noLibera=$NO_LIBERA_VAL"
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
