#!/bin/bash
# Corre un ELF o una imagen (.cue/.gdi/.cdi) en Flycast y deja evidencia: capturas de pantalla
# y el texto de la consola serie (println de Go / printf de KOS) dentro del log.
#
# Uso:  scripts/console/run-flycast.sh <archivo> [segundos] [dir_salida]
#   - Se le puede pasar un .elf directo: Flycast lo arranca con su propio loader (reios_loadElf),
#     sin IP.BIN ni ISO. Es el ciclo rapido de desarrollo.
#   - En KDE/Wayland las capturas se hacen con spectacle (x11grab sale negro: es XWayland).
#     OJO: spectacle -f fotografía la PANTALLA ENTERA, asi que la ventana de Flycast tiene que estar
#     ADELANTE cuando se toman las capturas (a los 3, 6, 10 y 15 s). Si queda atras sale el escritorio.
#   - No hace falta "source env.sh" a mano: este script carga el entorno solo. (env.sh es de bash: en
#     fish no se puede sourcear, tira "case builtin usado fuera de un bloque switch" y nada queda en el
#     entorno. De ahi que el source sea interno.)
set -uo pipefail

# entorno del toolchain (idempotente y silencioso: se puede volver a cargar sin problema)
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -z "${DC_ROOT:-}" ] && [ -f "$HERE/env.sh" ]; then
  # shellcheck disable=SC1091
  . "$HERE/env.sh" >/dev/null 2>&1
fi
IMG="${1:?uso: run-flycast.sh <archivo> [segundos] [dir_salida]}"
SECS="${2:-30}"
OUT="${3:-/tmp/dcframes}"
FLY="${FLYCAST:-$HOME/dreamcast/dl/flycast.AppImage}"
DISP="${DISPLAY:-:0}"
# la consola serie del SH-4 sale dentro del log del emulador: asi se lee el println de Go
CFG="${FLYCFG:-Debug:SerialConsoleEnabled=true}"
CAPTURE="${CAPTURE:-auto}"   # auto | spectacle | x11grab | none

[ -f "$IMG" ] || { echo "no existe $IMG"; exit 1; }
[ -x "$FLY" ] || { echo "no existe/falta permiso de ejecucion: $FLY"; exit 1; }
mkdir -p "$OUT"; rm -f "$OUT"/f*.png

if [ "$CAPTURE" = auto ]; then
  if command -v spectacle >/dev/null 2>&1; then CAPTURE=spectacle; else CAPTURE=x11grab; fi
fi
shot() {
  case "$CAPTURE" in
    spectacle) spectacle -b -n -f -o "$1" >/dev/null 2>&1 ;;
    x11grab)   ffmpeg -v error -f x11grab -i "$DISP" -frames:v 1 -y "$1" >/dev/null 2>&1 ;;
    *)         return 1 ;;
  esac
}

echo "== Flycast: $IMG (capturas: $CAPTURE, consola serie: si) =="
# la captura de pantalla entera solo ve el emulador si su ventana esta arriba: se la fuerza por KWin
# (DC-002.9: Diablo II en fullscreen dentro de gamescope tapaba al emulador y las capturas salian con
# el juego, no con la consola). Si no hay KWin disponible, esto no hace nada.
[ "$CAPTURE" != "none" ] && bash "$HERE/flycast-adelante.sh" arriba
setsid env DISPLAY="$DISP" "$FLY" -config "window:fullscreen=no" -config "$CFG" "$IMG" >"$OUT/flycast.log" 2>&1 &
PID=$!
echo "pid=$PID"
ult=0
# EL TOPE DE LA CORRIDA ES ESTA LISTA, NO `$SECS` (DC-002.24b, medido): con `run-flycast.sh <elf> 150` la
# corrida se terminaba igual a los ~41 s, porque el bucle de capturas se acababa en t=40 y de ahi saltaba
# al SIGTERM. Eso ya confundio una medicion ("la corrida dura los 30 s completos"): con SECS=30 el tope real
# es ~26 s (el bucle rompe en t=40 > 30 DESPUES de dormir hasta t=25). La lista va hasta 600 s para que un
# SECS largo sea de verdad largo (el motor completa un cuadro cada ~0,3 s: el heartbeat del loop recien
# imprime a los 300 cuadros, o sea ~100 s de corrida).
for t in 3 6 10 15 25 40 60 90 120 150 180 240 300 360 420 480 540 600; do
  [ "$t" -gt "$SECS" ] && break
  sleep $(( t - ult )); ult=$t
  if kill -0 "$PID" 2>/dev/null; then
    shot "$OUT/f${t}s.png"; echo "  captura ${t}s"
  else
    echo "  el emulador se detuvo antes de los ${t}s"; break
  fi
done
sleep 1
kill -TERM -"$PID" 2>/dev/null || kill -TERM "$PID" 2>/dev/null
sleep 1; kill -KILL -"$PID" 2>/dev/null
bash "$HERE/flycast-adelante.sh" sacar

echo "== pantalla =="; ls -la "$OUT"/f*.png 2>/dev/null | awk '{print $5" bytes  "$9}'
echo "== consola serie / avisos del emulador =="
grep -aE "SH4|GDROM|REIOS|EXCEPTION|Fatal|Unknown|not found|Error" "$OUT/flycast.log" | tail -30
