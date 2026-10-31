#!/bin/bash
# DC-002.89 -- EXPERIMENTO: cuanto pesa de verdad el camino de entidades en la imagen.
#
# NO toca el repo: copia d2mapstamp y d2mapengine a un directorio de scratch, les aplica el corte y
# recompila SOLO esos dos .gox en un arbol de gox copiado, y reenlaza el POC desde el build base
# (donde el main.o y todo lo demas ya estan). Es una medicion de "que pasaria si", no un cambio de
# arquitectura: la propuesta va con el numero y la decide el humano.
#
# Variantes:
#   A  solo se saca la LLAMADA del camino de entidades en PlaceStampClippedWhere (el paquete sigue
#      importando d2mapentity/d2mapstamp: mide cuanto recoge --gc-sections por si solo)
#   B  corte de arquitectura: d2mapstamp no fabrica entidades (EntitiesWhere devuelve vacio, se van
#      newNPC/newObject) y d2mapengine no tiene el campo del factory ni el mapa de entidades
#
# Uso:  scripts/console/exp-corte-entidades-289.sh <A|B> <dir_build_base> <dir_salida>
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

VARIANTE="${1:?uso: exp-corte-entidades-289.sh <A|B> <dir_build_base> <dir_salida>}"
BASE="${2:?falta el directorio de build base}"
OUT="${3:?falta el directorio de salida}"
REPO="$(cd "$HERE/../.." && pwd)"
PREFIJO="github.com/OpenDiablo2/OpenDiablo2"
SRC="$OUT/src"

[ -f "$BASE/main.o" ] || { echo "no hay build base en $BASE" >&2; exit 1; }

echo "== 1/4 copiando el build base a $OUT"
# el script tiene que poder repetirse: se limpia lo que el mismo genero antes (no el build base)
rm -rf "$OUT/src" "$OUT/gox-enlace" "$OUT/corte" "$OUT/main.o" "$OUT/prog.elf" "$OUT/1ST_READ.BIN" 2>/dev/null || true
mkdir -p "$OUT"
cp "$BASE/prog.elf" "$OUT/prog.elf" 2>/dev/null || true
cp -a "$BASE/gox-enlace" "$OUT/gox-enlace"
cp -a "$BASE/src" "$SRC"
cp -a "$BASE"/*.o "$OUT"/        # main.o, los shim del csrc y los stdlib-*.o (el enlace los necesita)
cp "$BASE/ld-sin-eh.xc" "$OUT/ld-sin-eh.xc"
cp "$BASE/IP.BIN" "$OUT/IP.BIN"
GEN="$OUT/gox-enlace"

echo "== 2/4 fuentes del corte (variante $VARIANTE)"
CORTE="$OUT/corte"
mkdir -p "$CORTE"
cp -a "$REPO/d2core/d2map/d2mapstamp" "$CORTE/d2mapstamp"
cp -a "$REPO/d2core/d2map/d2mapengine" "$CORTE/d2mapengine"
python3 "$HERE/corte-entidades-289.py" "$VARIANTE" "$CORTE"

echo "== 3/4 recompilando los .gox tocados"
# los symlinks del arbol de enlace apuntan al arbol base: se reemplazan por archivos de verdad, si no
# gccgo escribiria A TRAVES del link y romperia el arbol base.
for p in d2core/d2map/d2mapstamp d2core/d2map/d2mapengine; do
    f="$GEN/$PREFIJO/$p.gox"
    rm -f "$f"
    mapfile -t SRC_P < <(ls "$CORTE/$(basename "$p")"/*.go | grep -v '_test\.go$' |
        grep -vE '_(darwin|windows|js|wasm|plan9|freebsd|netbsd|openbsd|linux|android|ios|solaris|aix)\.go$' |
        grep -vE '_(386|amd64|arm|arm64|ppc64|ppc64le|mips|mipsle|mips64|mips64le|riscv64|s390x|loong64|wasm)\.go$')
    sh-elf-gccgo $GODC_GOFLAGS -I "$GEN" -fgo-pkgpath="$PREFIJO/$p" -c "${SRC_P[@]}" -o "$f"
    printf '   %-16s %9s B\n' "$(basename "$p").gox" "$(stat -c%s "$f")"
done

echo "== 4/5 el POC contra los .gox nuevos y el enlace con mapa"
sh-elf-gccgo $GODC_GOFLAGS -I "$GEN" -c "$SRC"/*.go -o "$OUT/main.o"
bash "$HERE/relink-con-mapa.sh" "$OUT" "$OUT/prog-289-$VARIANTE.map" -Wl,--gc-sections
sh-elf-objcopy -R .stack -O binary "$OUT/prog.elf" "$OUT/1ST_READ.BIN"
printf '   1ST_READ.BIN %10s B (base %s B)\n' "$(stat -c%s "$OUT/1ST_READ.BIN")" "$(stat -c%s "$BASE/1ST_READ.BIN")"

# la imagen de disco, para poder correr la variante en Flycast igual que la corrida base: los MPQ van
# en /JUEGO como en build-tablas-282.sh (sin ellos el compositor no tiene tablas ni .ds1).
echo "== 5/5 la imagen con PATCH_D2.MPQ y D2DATA.MPQ en /JUEGO"
D2DIR="${D2_DIR:-$HOME/.wine-d2/drive_c/Games/Diablo II}"
MPQ_PATCH="$(ls "$D2DIR" | grep -ix 'patch_d2.mpq' | head -1)"
MPQ_DATA="$(ls "$D2DIR" | grep -ix 'd2data.mpq' | head -1)"
{
    printf '%s:/JUEGO/PATCH_D2.MPQ;1\n' "$D2DIR/$MPQ_PATCH"
    printf '%s:/JUEGO/D2DATA.MPQ;1\n' "$D2DIR/$MPQ_DATA"
} > "$OUT/iso-lista.txt"
MKISO_JOLIET=0 "$DC_PYTHON" "$HERE/mkiso.py" "$OUT/disc.iso" "$OUT/IP.BIN" "$OUT/1ST_READ.BIN" "@$OUT/iso-lista.txt" | tail -1
"$DC_PYTHON" "$HERE/mkgdi.py" "$OUT" | tail -1
