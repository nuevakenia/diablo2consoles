#!/bin/bash
# Compila un programa de Go (libgodc) para Dreamcast y arma la imagen arrancable.
#
# Uso:  scripts/console/build-dc.sh console/dc/poc/hello [salida_dir]
#
# Deja en el directorio de salida:  prog.elf, 1ST_READ.BIN, IP.BIN, disc.iso
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

SRC="${1:?uso: build-dc.sh <dir_con_main.go> [salida_dir]}"
OUT="${2:-/tmp/dcbuild/$(basename "$SRC")}"
mkdir -p "$OUT"

# Assets de Diablo II (si el POC trae bake.conf): se hornean al directorio de build, no al repo.
bash "$HERE/bake-asset.sh" "$SRC" "$OUT"

echo "== 1/4 compilando Go ($SRC) =="
sh-elf-gccgo $GODC_GOFLAGS -c "$SRC"/*.go -o "$OUT/main.o"

# Puentes a C (para funciones de KOS que el binding de Go no expone). Pueden vivir en
# $SRC/*.c o en $SRC/csrc/*.c. Se enlazan aparte: gccgo los llama con el comentario //extern.
EXTRA_OBJS=""
for f in "$SRC"/*.c "$SRC"/csrc/*.c; do
  [ -f "$f" ] || continue
  o="$OUT/$(basename "${f%.c}").o"
  kos-cc -O2 -ml -m4-single -I"$SRC" -I"$SRC/csrc" -I"$OUT/gen" -c "$f" -o "$o"
  EXTRA_OBJS="$EXTRA_OBJS $o"
done

echo "== 2/4 enlazando ELF =="
kos-cc -o "$OUT/prog.elf" "$OUT/main.o" $EXTRA_OBJS $DC_LDPATHS $DC_LIBS_PRE $DC_LIBS_POST

echo "== 3/4 binario de arranque (1ST_READ.BIN) + IP.BIN =="
sh-elf-objcopy -R .stack -O binary "$OUT/prog.elf" "$OUT/1ST_READ.BIN"
"$DC_MAKEIP" -f "$OUT/IP.BIN" >/dev/null
# el IP.BIN generado debe apuntar al binario de arranque por defecto (1ST_READ.BIN)

echo "== 4/4 imagen =="
"$DC_PYTHON" "$HERE/mkiso.py" "$OUT/disc.iso" "$OUT/IP.BIN" "$OUT/1ST_READ.BIN"
# El .cue (CD-R) hace que el emulador descramble el binario: para arrancar de verdad, GDI.
"$DC_PYTHON" "$HERE/mkgdi.py" "$OUT"

echo
echo "ELF:        $OUT/prog.elf   ($(stat -c%s "$OUT/prog.elf") bytes)"
echo "1ST_READ:   $OUT/1ST_READ.BIN   ($(stat -c%s "$OUT/1ST_READ.BIN") bytes)"
echo "imagen:     $OUT/disc.iso"
