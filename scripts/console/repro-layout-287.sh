#!/bin/bash
# DC-002.87 -- REHACER EL POC DEL COOPERATIVO CON OTRO ACOMODO Y CORRERLO, en un solo paso.
#
# POR QUE. El salto a basura del primer cuadro lo decide el acomodo del binario (DC-002.84: mismo codigo de
# juego, 32 B de diferencia, un build pinta y el otro muere). Para cazar el cuadro que muere con el volcado
# de registros puesto hay que poder pedir "el mismo codigo con N bytes mas de .bss" y volver a correr.
#
# QUE HACE. Copia las fuentes del POC al directorio de salida, le pone N a `relleno.go`, compila y enlaza
# contra el arbol de enlace YA ARMADO (`<base>/gox-enlace`, no recompila los 49 paquetes del motor), arma la
# imagen con los dos MPQ y corre Flycast. Imprime solo las lineas que importan: el relleno con el que se
# corrio, el cuadro, el volcado de registros y las excepciones.
#
# Uso:  scripts/console/repro-layout-287.sh <bytes_de_relleno> [dir_base] [segundos] [recolecciones]
#       <dir_base>   directorio de un build del cooperativo ya hecho (por defecto /tmp/dc287/A); de ahi se
#                    toman `gox-enlace` y el arbol de enlace. La salida va a <dir_base>/relleno-<bytes>.
#       <segundos>   0 = solo compilar y armar la imagen, sin correr el emulador (por defecto 70).
#       <recolecciones> DC-002.87b: recolecciones FORZADAS al arrancar (0 = ninguna). Es el otro eje del
#                    acomodo: cambia que semiespacio esta activo en el primer cuadro sin tocar la logica.
#
# OJO: los .gox del motor NO se rehacen aca. Si lo que se quiere medir es un cambio en el MOTOR, esto no
# alcanza: hay que rehacer el cierre (scripts/console/armar-gox-coop-283.sh).
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
BYTES="${1:?uso: repro-layout-287.sh <bytes_de_relleno> [dir_base] [segundos] [recolecciones]}"
BASE="${2:-/tmp/dc287/A}"
SECS="${3:-70}"
RECS="${4:-${RECOLECCIONES:-0}}"
RESERVA="${RESERVA:-0}"
FALLA="${FALLA:-0}"
OUT="$BASE/relleno-$BYTES-r$RESERVA-f$FALLA-g$RECS"
SRC="$OUT/src"
GEN="$BASE/gox-enlace"

[ -f "$GEN/orden.txt" ] || { echo "falta el arbol de enlace en $GEN (corre build-coop-276.sh antes)" >&2; exit 1; }

echo "== 1/5 las fuentes del POC con relleno=$BYTES reserva=$RESERVA"
rm -rf "$SRC"
mkdir -p "$SRC"
cp "$REPO/console/dc/poc/coop"/*.go "$SRC"/
cp -r "$REPO/console/dc/poc/coop/csrc" "$SRC"/
cp "$REPO/console/dc/poc/personaje-mapa/nivel.go" "$SRC"/

"$DC_PYTHON" - "$SRC/relleno.go" "$BYTES" "$RESERVA" "$FALLA" "$RECS" <<'PY'
import re
import sys

ruta, bytes_pedidos, reserva, falla, recs = sys.argv[1], int(sys.argv[2]), int(sys.argv[3]), int(sys.argv[4]), int(sys.argv[5])
if bytes_pedidos < 1:
    sys.exit("el relleno tiene que ser >= 1 byte (el init toca el arreglo)")

t = open(ruta).read()
t, uno = re.subn(r"var rellenoDeLayout = \[\d+\]byte\{\}", "var rellenoDeLayout = [%d]byte{}" % bytes_pedidos, t)
t, dos = re.subn(r"const reservaDeMonton = \d+", "const reservaDeMonton = %d" % reserva, t)
t, tres = re.subn(r"const fallaDePrueba = \w+", "const fallaDePrueba = %s" % ("true" if falla else "false"), t)
t, cuatro = re.subn(r"var recoleccionesDePrueba = \d+", "var recoleccionesDePrueba = %d" % recs, t)
if uno != 1 or dos != 1 or tres != 1 or cuatro != 1:
    sys.exit("no encontre los mandos en %s (relleno=%d reserva=%d falla=%d recolecciones=%d)"
             % (ruta, uno, dos, tres, cuatro))
open(ruta, "w").write(t)
PY
grep -n "var rellenoDeLayout\|const reservaDeMonton\|const fallaDePrueba\|var recoleccionesDePrueba" "$SRC/relleno.go"

echo "== 2/5 compilando y enlazando (sin rehacer los .gox del motor)"
GODC_GOX="$GEN" bash "$HERE/build-dc.sh" "$SRC" "$OUT" > "$OUT/build.log" 2>&1 || {
    echo "   FALLO el build (ver $OUT/build.log)"; tail -20 "$OUT/build.log"; exit 1; }
for f in prog.elf 1ST_READ.BIN; do
    [ -f "$OUT/$f" ] && printf '   %-14s %10s B\n' "$f" "$(stat -c%s "$OUT/$f")"
done

echo "== 3/5 la imagen con los DOS MPQ en /JUEGO"
D2DIR="${D2_DIR:-$HOME/.wine-d2/drive_c/Games/Diablo II}"
MPQ_DATA="$(ls "$D2DIR" | grep -ix 'd2data.mpq' | head -1)"
MPQ_CHARS="$(ls "$D2DIR" | grep -ix 'd2char.mpq' | head -1)"
[ -n "$MPQ_DATA" ] || { echo "   no encuentro d2data.mpq en $D2DIR" >&2; exit 1; }
[ -n "$MPQ_CHARS" ] || { echo "   no encuentro d2char.mpq en $D2DIR" >&2; exit 1; }
LISTA="$OUT/iso-lista.txt"
printf '%s:/JUEGO/D2DATA.MPQ;1\n%s:/JUEGO/D2CHAR.MPQ;1\n' \
    "$D2DIR/$MPQ_DATA" "$D2DIR/$MPQ_CHARS" > "$LISTA"
MKISO_JOLIET=0 "$DC_PYTHON" "$HERE/mkiso.py" "$OUT/disc.iso" "$OUT/IP.BIN" "$OUT/1ST_READ.BIN" "@$LISTA" | tail -1
"$DC_PYTHON" "$HERE/mkgdi.py" "$OUT" | tail -1

if [ "$SECS" = "0" ]; then
    echo "== imagen lista en $OUT (no se corre el emulador: segundos=0)"
    exit 0
fi

echo "== 4/5 la corrida (${SECS}s, sin capturas)"
CAPTURE=none bash "$HERE/run-flycast.sh" "$OUT/disc.gdi" "$SECS" "$OUT/run" > "$OUT/run-script.log" 2>&1 || true
LOG="$OUT/run/flycast.log"
[ -f "$LOG" ] || { echo "   no hay $LOG"; exit 1; }

echo "== 5/5 lo que dijo la corrida"
tr -d '\r' < "$LOG" | grep -aE 'COOP287|COOP276: (CUADRO|JUGADOR|LIENZO|FILA)|SONDA-PC FUERA|SONDA-REG|Unhandled|Fatal' | head -60 || true
echo "--"
printf '   lineas del log: %s   excepciones (SONDA-REG FALLO): %s\n' \
    "$(wc -l < "$LOG")" "$(tr -d '\r' < "$LOG" | grep -ac 'SONDA-REG FALLO' || true)"
