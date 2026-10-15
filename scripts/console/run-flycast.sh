#!/bin/bash
# Corre un ELF o una imagen (.cue/.gdi/.cdi) en Flycast y deja evidencia: capturas de pantalla
# y el texto de la consola serie (println de Go / printf de KOS) dentro del log.
#
# Uso:  scripts/console/run-flycast.sh <archivo> [segundos] [dir_salida]
#   - Se le puede pasar un .elf directo: Flycast lo arranca con su propio loader (reios_loadElf),
#     sin IP.BIN ni ISO. Es el ciclo rapido de desarrollo.
#   - En KDE/Wayland las capturas se hacen con spectacle (x11grab sale negro: es XWayland).
set -uo pipefail
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
setsid env DISPLAY="$DISP" "$FLY" -config "window:fullscreen=no" -config "$CFG" "$IMG" >"$OUT/flycast.log" 2>&1 &
PID=$!
echo "pid=$PID"
ult=0
for t in 3 6 10 15 25 40; do
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

echo "== pantalla =="; ls -la "$OUT"/f*.png 2>/dev/null | awk '{print $5" bytes  "$9}'
echo "== consola serie / avisos del emulador =="
grep -aE "SH4|GDROM|REIOS|EXCEPTION|Fatal|Unknown|not found|Error" "$OUT/flycast.log" | tail -30
