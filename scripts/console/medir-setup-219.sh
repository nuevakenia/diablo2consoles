#!/bin/bash
# DC-002.19: LA SELECCION DE LOS ARCHIVOS DEL JUEGO, medida en la consola.
#
# Corre el POC console/dc/poc/setup-archivos (build-motor-arranque.sh con POC=setup-archivos) y saca los
# numeros de la frontera. Lo que mide, y por que:
#
#  A) El camino de los dialogos nativos. En la consola `d2setup.DefaultUI()` devuelve `OsaUI`
#     (ui_native_other.go), que pregunta por osascript. El motor se traga ese error y devuelve
#     `ErrNoGameFiles`, asi que el texto NO se ve en el arranque: el POC lo pide a mano.
#     Tambien mide `os.Getenv("OD2_SETUP_AUTOPICK")`: el camino sin dialogos que el motor ya tiene
#     (d2setup/dialog.go:111) depende de os.Getenv, y en la consola os.Getenv es un stub.
#  B) `Validate(dir)` contra `Complete(dir, orden)` sobre la MISMA carpeta con los siete MPQ creados: los dos
#     caminos de la seleccion (uno mira el contenido del directorio, el otro mira cada nombre).
#  C) Hasta donde llega `Run()` con el config apuntando a los siete MPQ VACIOS: esa es la frontera nueva.
#
# Uso:  bash scripts/console/medir-setup-219.sh [dir_gox] [dir_build] [segundos]
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

GEN="${1:-/tmp/gox-00218}"
BUILD="${2:-/tmp/dcbuild/setup-219}"
SEGS="${3:-25}"
REPO="${REPO:-$(cd "$HERE/../.." && pwd)}"
RUN=/tmp/dc-run-setup219

[ -f "$GEN/orden.txt" ] || { echo "falta $GEN/orden.txt (la stdlib minima)" >&2; exit 1; }
[ -f "$GEN/orden-motor.txt" ] || { echo "falta $GEN/orden-motor.txt (el motor: build-motor-all.sh)" >&2; exit 1; }

export GODC_GOX="$GEN"

echo "== 1/3 enlazando el POC de la seleccion de archivos (los 142 paquetes del motor)"
POC=setup-archivos bash "$HERE/build-motor-arranque.sh" "$BUILD" "$GEN" > "$BUILD.build.log" 2>&1 || {
    echo "   NO ENLAZA (ver $BUILD.build.log)" >&2; tail -5 "$BUILD.build.log" >&2; exit 1; }

echo "== 2/3 corriendo la consola ($SEGS s)"
rm -rf "$RUN"
(cd "$REPO" && CAPTURE=none bash "$HERE/run-flycast.sh" "$BUILD/prog-emu.elf" "$SEGS" "$RUN") > "$RUN.salida" 2>&1

LOG="$RUN/flycast.log"
[ -f "$LOG" ] || { echo "no hay log de Flycast en $LOG" >&2; exit 1; }

echo "== 3/3 las mediciones (log: $LOG)"
grep -a 'SETUP-ARCHIVOS\|MOTOR-ARRANQUE\|JSON-CONSOLA\|^panic\|loaded configuration\|File Loader' "$LOG" |
    tr -d '\r' | sed 's/^/   /'

echo
echo "== el resumen de la frontera"
printf '   seleccion: os.Getenv(OD2_SETUP_AUTOPICK) = "%s" (stub)  /  OsaUI.ChooseFolder = %s\n' \
    "$(grep -a 'os.Getenv' "$LOG" | head -1 | sed 's/.*= //' | tr -d '\r')" \
    "$(grep -a 'OsaUI.ChooseFolder devolvio error' "$LOG" | head -1 | sed 's/.*error: //' | tr -d '\r')"
grep -a 'Validate(dir).OK()\|Complete(dir, orden)\|archivos del juego creados' "$LOG" | tr -d '\r' | sed 's/^/   /'
grep -a 'Run() termino con error\|^panic' "$LOG" | tr -d '\r' | sed 's/^/   /'
grep -a 'Memory:' "$LOG" | tail -1 | tr -d '\r' | sed 's/^/   /'
