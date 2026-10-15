#!/bin/bash
# Compila un programa en C (KOS puro, sin Go) y arma la imagen arrancable.
# Sirve para aislar: si el C anda y el Go no, el problema es libgodc/Go, no el pipeline.
# Uso:  scripts/console/build-c.sh console/dc/poc/pipeline-c [salida_dir]
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

SRC="${1:?uso: build-c.sh <dir_con_main.c> [salida_dir]}"
OUT="${2:-/tmp/dcbuild/$(basename "$SRC")}"
mkdir -p "$OUT"

echo "== 1/4 compilando C =="
kos-cc -O2 -ml -m4-single -o "$OUT/prog.elf" "$SRC"/*.c -Wl,--start-group -lkallisti -lc -lgcc -Wl,--end-group

echo "== 2/4 binario de arranque + IP.BIN =="
sh-elf-objcopy -R .stack -O binary "$OUT/prog.elf" "$OUT/1ST_READ.BIN"
"$DC_MAKEIP" -f "$OUT/IP.BIN" >/dev/null

echo "== 3/4 imagen =="
"$DC_PYTHON" "$HERE/mkiso.py" "$OUT/disc.iso" "$OUT/IP.BIN" "$OUT/1ST_READ.BIN"
echo
echo "ELF:      $OUT/prog.elf ($(stat -c%s "$OUT/prog.elf") bytes)"
echo "1ST_READ: $OUT/1ST_READ.BIN ($(stat -c%s "$OUT/1ST_READ.BIN") bytes)"
echo "imagen:   $OUT/disc.cue"
