#!/bin/bash
# Corre el POC del motor en Flycast y compara su resumen "MOTOR-DC6 ..." contra el que da el mismo
# codigo del motor corriendo en el host, sobre los MISMOS bytes del DC6.
#
# La evidencia es el diff: las lineas del resumen son los numeros (tamano, offsets, opacos y el hash
# FNV-1a de los indices decodificados de cada frame). Si una linea difiere, el decodificador no hizo
# lo mismo en las dos plataformas.
#
# Uso:  scripts/console/compare-motor-dc6.sh [dir_de_build] [nombre_asset]
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
OUT="${1:-/tmp/dcbuild/motor-dc6}"
NAME="${2:-d2logo}"
SECS="${SECS:-20}"

[ -f "$OUT/prog.elf" ] || { echo "falta $OUT/prog.elf: corre scripts/console/build-motor-dc6.sh" >&2; exit 1; }
[ -f "$OUT/$NAME.dc6" ] || { echo "falta el DC6 horneado $OUT/$NAME.dc6" >&2; exit 1; }

echo "== host: el mismo d2dc6.Load + el mismo resumen, en x86-64"
(cd "$REPO" && go run ./console/dc/motorcmp -dc6 "$OUT/$NAME.dc6") > "$OUT/host.txt" 2> "$OUT/host.err"
if [ $? -ne 0 ]; then
    echo "el comparador del host fallo:"; sed -n '1,10p' "$OUT/host.err"; exit 1
fi

echo "== consola: el mismo codigo, compilado para SH-4, en Flycast"
CAPTURE="${CAPTURE:-none}" bash "$HERE/run-flycast.sh" "$OUT/prog.elf" "$SECS" "$OUT/run" > "$OUT/flycast.out" 2>&1
# las lineas de la consola serie terminan en \r y el emulador mete avisos suyos: se filtra
grep -a '^MOTOR-DC6' "$OUT/run/flycast.log" | tr -d '\r' > "$OUT/consola.txt"

# solo las lineas del resumen (sin la de arranque, la de origen y la de fin)
for f in host consola; do
    grep -E '^MOTOR-DC6 (version=|frame )' "$OUT/$f.txt" > "$OUT/$f.resumen.txt"
done

N_HOST=$(wc -l < "$OUT/host.resumen.txt")
N_CONSOLA=$(wc -l < "$OUT/consola.resumen.txt")

echo
echo "== resumen del host ($N_HOST lineas) y de la consola ($N_CONSOLA lineas)"
echo "   host.txt:     $OUT/host.txt"
echo "   consola.txt:  $OUT/consola.txt"

if [ "$N_CONSOLA" -eq 0 ]; then
    echo
    echo "LA CONSOLA NO IMPRIMIO NADA: el POC no arranco o se cayo. Ultimas lineas del log:"
    grep -aE 'Fatal|EXCEPTION|SH4|MOTOR-DC6' "$OUT/run/flycast.log" | tail -10
    exit 1
fi

if ! diff -u "$OUT/host.resumen.txt" "$OUT/consola.resumen.txt"; then
    echo
    echo "== DISTINTO: el decodificador no dio lo mismo en las dos plataformas"
    exit 1
fi

echo
echo "== IGUAL: $N_HOST lineas identicas (mismo DC6, mismo codigo del motor, x86-64 y SH-4) =="
grep -c '^MOTOR-DC6 frame ' "$OUT/host.resumen.txt" | sed 's/^/   frames comparados: /'
