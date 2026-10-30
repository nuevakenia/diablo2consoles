#!/bin/bash
# DC-002.72 -- compilar y enlazar la PRUEBA del audio: un sonido DEL MOTOR (un WAV de d2Sfx.mpq) tocado por
# el AICA a traves de d2interface.AudioProvider (el backend d2core/d2audio/aica).
#
# POR QUE UN SCRIPT PROPIO (y no alcanza build-dc.sh): este POC importa un paquete del MOTOR
# (d2core/d2audio/aica) que no es stdlib: su .gox lo produce la compilacion del motor (build-motor-all.sh), y
# el paquete del contrato (d2interface) y el resolutor de rutas (d2soundpath) tambien son del motor. Se arma,
# igual que build-input-271.sh, un arbol de ENLACE con enlaces simbolicos a la gox del motor y una orden.txt =
# stdlib + SOLO los paquetes del cierre de lo que este POC importa. El cierre NO se escribe a mano: sale de
# `go list -deps`.
#
# EL .c DEL AICA no se compila aca: d2audio_* viven en console/dc/std/csrc/sonido_dc.c y build-dc.sh los
# compila siempre. El POC solo trae su shim de consola serie y reloj (csrc/shim.c).
#
# LA IMAGEN LLEVA EL MPQ DE LOS SONIDOS: el POC lee /cd/JUEGO/D2SFX.MPQ del disco (48,9 MB, 2330 WAV). Sin ese
# archivo en la imagen el POC no tiene efecto del motor que probar (y lo dice: "MPQ FALLO").
#
# Uso:  scripts/console/build-aica-272.sh [dir_salida]
#       BASE_GOX=...    arbol del motor compilado (por defecto /tmp/gox-std)
#       SALTAR_GEN=1    reusar el arbol de enlace ya armado
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
SRC="$REPO/console/dc/poc/audio-aica"
BASE_GOX="${BASE_GOX:-/tmp/gox-std}"
OUT="${1:-/tmp/dcbuild/aica272}"
GEN="$OUT/gox-enlace"
PREFIJO="github.com/OpenDiablo2/OpenDiablo2"

[ -f "$BASE_GOX/orden.txt" ] || { echo "falta el arbol del motor en $BASE_GOX (corre build-motor-all.sh)" >&2; exit 1; }
mkdir -p "$OUT"

echo "== 1/6 el paquete del audio (aica.gox)"
mkdir -p "$BASE_GOX/$PREFIJO/d2core/d2audio"
# OJO: gccgo NO filtra por sufijo como el go de verdad, asi que los _test.go (que piden `testing`, que la
# consola no tiene) se excluyen A MANO -- el mismo filtro que hace build-motor-all.sh. Sin esto la
# compilacion del paquete muere en "import file 'testing' not found".
FUENTES="$(ls "$REPO"/d2core/d2audio/aica/*.go |
    grep -v _test.go |
    grep -vE '_(darwin|windows|js|wasm|plan9|freebsd|netbsd|openbsd|linux|android|ios|solaris|aix)\.go$' |
    grep -vE '_(386|amd64|arm|arm64|ppc64|ppc64le|mips|mipsle|mips64|mips64le|riscv64|s390x|loong64|wasm)\.go$')"
# shellcheck disable=SC2086
sh-elf-gccgo $GODC_GOFLAGS -I "$BASE_GOX" \
    -fgo-pkgpath="$PREFIJO/d2core/d2audio/aica" \
    -c $FUENTES \
    -o "$BASE_GOX/$PREFIJO/d2core/d2audio/aica.gox"
echo "   $(printf '%s\n' $FUENTES | wc -l) archivos: $(for f in $FUENTES; do basename "$f"; done | tr '\n' ' ')"
echo "   aica.gox $(stat -c%s "$BASE_GOX/$PREFIJO/d2core/d2audio/aica.gox") B"

# el cierre del POC: el paquete del audio y sus dependencias del motor (contrato, MPQ, rutas de sonido)
PAQUETES="$(cd "$REPO" && go list -deps -e -tags dreamcast \
    ./d2core/d2audio/aica 2>/dev/null |
    grep "$PREFIJO" | sed "s|.*$PREFIJO/||" | sort | tr '\n' ' ')"
[ -n "${PAQUETES// /}" ] || { echo "la lista de paquetes quedo vacia (go list -deps fallo?)" >&2; exit 1; }

echo "== 2/6 el arbol de enlace ($GEN)"
if [ "${SALTAR_GEN:-0}" != "1" ]; then
    mkdir -p "$GEN"
    # enlaces simbolicos, no copias: los .gox del motor son decenas de MB y no se tocan desde aca
    cp -rs "$BASE_GOX/." "$GEN"/ 2>/dev/null || true
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

echo "== 3/6 compilando y enlazando contra los .gox del motor"
GODC_GOX="$GEN" bash "$HERE/build-dc.sh" "$SRC" "$OUT" > "$OUT/build.log" 2>&1 || {
    echo "   FALLO el build (ver $OUT/build.log)"; tail -25 "$OUT/build.log"; exit 1; }
grep -E "^(ELF|1ST_READ|imagen)" "$OUT/build.log" | sed 's/^/   /'

echo "== 4/6 la imagen con el MPQ de los sonidos en /JUEGO"
D2DIR="${D2_DIR:-$HOME/.wine-d2/drive_c/Games/Diablo II}"
MPQ_SFX="$(ls "$D2DIR" | grep -ix 'd2sfx.mpq' | head -1)"
[ -n "$MPQ_SFX" ] || { echo "   no encuentro d2Sfx.mpq en $D2DIR" >&2; exit 1; }
LISTA="$OUT/iso-lista.txt"
printf '%s:/JUEGO/D2SFX.MPQ;1\n' "$D2DIR/$MPQ_SFX" > "$LISTA"
MKISO_JOLIET=0 "$DC_PYTHON" "$HERE/mkiso.py" "$OUT/disc.iso" "$OUT/IP.BIN" "$OUT/1ST_READ.BIN" "@$LISTA" | tail -2
"$DC_PYTHON" "$HERE/mkgdi.py" "$OUT" | tail -2

echo "== 5/6 lo que entro al enlace"
for p in $PAQUETES; do
    printf '   %-46s %10s B\n' "$p" "$(stat -L -c%s "$GEN/$PREFIJO/$p.gox")"
done

echo "== 6/6 lo que salio"
for f in prog.elf 1ST_READ.BIN disc.iso; do
    [ -f "$OUT/$f" ] && printf '   %-14s %10s B\n' "$f" "$(stat -c%s "$OUT/$f")"
done
printf '   codigo (.text)  %10s B\n' "$(sh-elf-size -A "$OUT/prog.elf" | awk '$1==".text"{print $2}')"
printf '   rodata          %10s B\n' "$(sh-elf-size -A "$OUT/prog.elf" | awk '$1==".rodata"{print $2}')"
printf '   data+bss        %10s B\n' "$(sh-elf-size -A "$OUT/prog.elf" | awk '$1==".data"||$1==".bss"{s+=$2} END{print s}')"
[ -f "$OUT/disc.gdi" ] && echo "   imagen:         $OUT/disc.gdi"
