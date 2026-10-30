#!/bin/bash
# DC-002.71 -- compilar y enlazar la PRUEBA de la entrada: el control de la Dreamcast leido por
# d2interface.InputService (el backend d2core/d2input/maple) y dibujado por el Renderer del PVR.
#
# POR QUE UN SCRIPT PROPIO (y no alcanza build-dc.sh): este POC importa DOS paquetes del MOTOR
# (d2core/d2input/maple y d2core/d2render/pvr), que no son stdlib: sus .gox los produce la compilacion del
# motor (build-motor-all.sh), y el paquete del contrato (d2interface) y los enums (d2enum) tambien son del
# motor. Se arma, igual que build-estado-b-270.sh, un arbol de ENLACE con enlaces simbolicos a la gox del motor
# y una orden.txt = stdlib + SOLO los paquetes del cierre de lo que este POC importa. El cierre NO se escribe a
# mano: sale de `go list -deps` (con -e, porque d2render/pvr importa "kos", que el host no resuelve).
#
# EL .c DEL CONTROL no se compila aca: d2c_control_buscar/d2c_control_leer viven en
# console/dc/std/csrc/control_dc.c y build-dc.sh los enlaza siempre. Una lectura del maple, un solo lugar.
#
# Uso:  scripts/console/build-input-271.sh [dir_salida]
#       BASE_GOX=...    arbol del motor compilado (por defecto /tmp/dc271/gox)
#       SALTAR_GEN=1    reusar el arbol de enlace ya armado
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
SRC="$REPO/console/dc/poc/input-maple"
BASE_GOX="${BASE_GOX:-/tmp/dc271/gox}"
OUT="${1:-/tmp/dcbuild/input271}"
GEN="$OUT/gox-enlace"
PREFIJO="github.com/OpenDiablo2/OpenDiablo2"

[ -f "$BASE_GOX/orden.txt" ] || { echo "falta el arbol del motor en $BASE_GOX (corre build-motor-all.sh)" >&2; exit 1; }
mkdir -p "$OUT"

echo "== 1/5 el paquete de la entrada (maple.gox)"
mkdir -p "$BASE_GOX/$PREFIJO/d2core/d2input"
# OJO: gccgo NO filtra por sufijo como el go de verdad, asi que los _test.go (que piden `testing`, que la
# consola no tiene) y los archivos de otra plataforma se excluyen A MANO -- el mismo filtro que hace
# build-motor-all.sh. Sin esto la compilacion del paquete muere en "import file 'testing' not found", que
# manda a buscar el problema en el paquete cuando esta en la lista de fuentes.
FUENTES="$(ls "$REPO"/d2core/d2input/maple/*.go |
    grep -v _test.go |
    grep -vE '_(darwin|windows|js|wasm|plan9|freebsd|netbsd|openbsd|linux|android|ios|solaris|aix)\.go$' |
    grep -vE '_(386|amd64|arm|arm64|ppc64|ppc64le|mips|mipsle|mips64|mips64le|riscv64|s390x|loong64|wasm)\.go$')"
sh-elf-gccgo $GODC_GOFLAGS -I "$BASE_GOX" \
    -fgo-pkgpath="$PREFIJO/d2core/d2input/maple" \
    -c $FUENTES \
    -o "$BASE_GOX/$PREFIJO/d2core/d2input/maple.gox"
echo "   $(printf '%s\n' $FUENTES | wc -l) archivos: $(printf '%s ' $(for f in $FUENTES; do basename "$f"; done))"
echo "   maple.gox $(stat -c%s "$BASE_GOX/$PREFIJO/d2core/d2input/maple.gox") B"

# el cierre del POC: los dos paquetes del motor que importa, con sus dependencias
PAQUETES="$(cd "$REPO" && go list -deps -e -tags dreamcast \
    ./d2core/d2input/maple ./d2core/d2render/pvr 2>/dev/null |
    grep "$PREFIJO" | sed "s|.*$PREFIJO/||" | sort | tr '\n' ' ')"
[ -n "${PAQUETES// /}" ] || { echo "la lista de paquetes quedo vacia (go list -deps fallo?)" >&2; exit 1; }

echo "== 2/5 el arbol de enlace ($GEN)"
if [ "${SALTAR_GEN:-0}" != "1" ]; then
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
echo "   paquetes del motor en el enlace: $(printf '%s\n' $PAQUETES | wc -l) -> $PAQUETES"
echo "   objetos en orden.txt: $(wc -l < "$GEN/orden.txt")"

echo "== 3/5 compilando y enlazando contra los .gox del motor"
GODC_GOX="$GEN" bash "$HERE/build-dc.sh" "$SRC" "$OUT" > "$OUT/build.log" 2>&1 || {
    echo "   FALLO el build (ver $OUT/build.log)"; tail -25 "$OUT/build.log"; exit 1; }
grep -E "^(ELF|1ST_READ|imagen)" "$OUT/build.log" | sed 's/^/   /'

echo "== 4/5 lo que entro al enlace"
for p in $PAQUETES; do
    printf '   %-46s %10s B\n' "$p" "$(stat -L -c%s "$GEN/$PREFIJO/$p.gox")"
done

echo "== 5/5 lo que salio"
for f in prog.elf 1ST_READ.BIN; do
    [ -f "$OUT/$f" ] && printf '   %-14s %10s B\n' "$f" "$(stat -c%s "$OUT/$f")"
done
printf '   codigo (.text)  %10s B\n' "$(sh-elf-size -A "$OUT/prog.elf" | awk '$1==".text"{print $2}')"
printf '   rodata          %10s B\n' "$(sh-elf-size -A "$OUT/prog.elf" | awk '$1==".rodata"{print $2}')"
printf '   data+bss        %10s B\n' "$(sh-elf-size -A "$OUT/prog.elf" | awk '$1==".data"||$1==".bss"{s+=$2} END{print s}')"
[ -f "$OUT/disc.gdi" ] && echo "   imagen:         $OUT/disc.gdi"
