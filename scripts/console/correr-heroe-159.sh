#!/bin/bash
# DC-002.159: CORRER EL GATE 0 CON EL HEROE Y MOVERLO CON LA CRUCETA, dejando capturas y log.
#
# QUE HACE, EN ORDEN:
#   1. arranca Flycast con la imagen de la tarjeta (consola serie al log, ventana a 210,210);
#   2. espera en el log la linea `HEROE159 LISTO` (esa linea la escribe la pantalla cuando el heroe del
#      motor ya esta creado: ente, compuesto del MPQ y estadisticas de charstats);
#   3. deja pasar los primeros 20 cuadros, que son el tramo A/B de la tarjeta (el mapa repintandose cada
#      cuadro: ~18 s a 818 ms), y recien ahi captura;
#   4. aprieta la cruceta con ydotool (el teclado del host entra al control emulado de Flycast) y captura
#      despues de cada direccion: arriba, derecha, la diagonal abajo+derecha y la izquierda;
#   5. corta el emulador y deja el log de la consola serie en el directorio de salida.
#
# Uso:  scripts/console/correr-heroe-159.sh [dir_salida]
#
# OJO: `spectacle -f` fotografia la PANTALLA ENTERA (2560x1080): el recorte a la ventana lo hace el paso
# final con ffmpeg (`crop=760:560:210:210`, la geometria que deja flycast-adelante.sh). Las capturas crudas
# quedan en $OUT/crudo y las recortadas en $OUT (que es lo que va a la evidencia).
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
. "$HERE/env.sh" >/dev/null 2>&1

OUT="${1:-/tmp/capturas159}"
IMG="${IMG:-/tmp/dcbuild/gate0-heroe-159/disc.gdi}"
FLY="${FLYCAST:-$HOME/dreamcast/dl/flycast.AppImage}"
LOG="$OUT/flycast.log"
ESPERA_MAXIMA="${ESPERA_MAXIMA:-240}"     # segundos maximos esperando HEROE159 LISTO

# codigos evdev de la cruceta (los mismos que manda un teclado: KEY_UP/DOWN/LEFT/RIGHT)
K_UP=103; K_DOWN=108; K_LEFT=105; K_RIGHT=106

[ -f "$IMG" ] || { echo "no existe $IMG: corre build-gate0-159.sh" >&2; exit 1; }
mkdir -p "$OUT/crudo"
rm -f "$OUT"/*.png "$OUT/crudo"/*.png "$LOG"

shot() {  # <nombre>
    bash "$HERE/flycast-adelante.sh" arriba >/dev/null 2>&1
    sleep 0.6
    spectacle -b -n -f -o "$OUT/crudo/$1.png" >/dev/null 2>&1
    echo "  captura $1"
}

cruceta() {  # <tecla> <segundos>
    local k="$1" s="$2"
    bash "$HERE/flycast-adelante.sh" arriba >/dev/null 2>&1
    ydotool key "$k":1
    sleep "$s"
    ydotool key "$k":0
    echo "  cruceta $k durante ${s}s"
}

echo "== Flycast: $IMG (capturas en $OUT, log en $LOG)"
setsid env DISPLAY="${DISPLAY:-:0}" "$FLY" -config "window:fullscreen=no" \
    -config "Debug:SerialConsoleEnabled=true" "$IMG" > "$LOG" 2>&1 &
PID=$!
echo "pid=$PID"

# ---- 1. esperar a que el heroe exista ----
t=0
until grep -aq "HEROE159 LISTO" "$LOG" 2>/dev/null; do
    sleep 0.5; t=$((t + 1))
    kill -0 "$PID" 2>/dev/null || { echo "el emulador se cayo a los $((t / 2))s" >&2; break; }
    if [ "$t" -gt $((ESPERA_MAXIMA * 2)) ]; then echo "no aparecio HEROE159 LISTO en ${ESPERA_MAXIMA}s" >&2; break; fi
done
if ! grep -aq "HEROE159 LISTO" "$LOG" 2>/dev/null; then
    echo "== SIN HEROE: lo que diga el log ==" >&2
    grep -aE "HEROE159|HITO238|SIN-HEROE|Fatal|EXCEPTION" "$LOG" | tail -20 >&2
    kill -TERM -"$PID" 2>/dev/null; exit 1
fi
echo "== HEROE159 LISTO a los $((t / 2))s: $(
    grep -a "HEROE159 LISTO" "$LOG" | tail -1 | sed 's/.*HEROE159 LISTO //')"

# ---- 2. el tramo A/B del mapa (cuadros 1..20, 818 ms cada uno) ----
echo "== esperando los 20 cuadros del tramo A/B del mapa (26 s)"
sleep 26
shot "0-parado"

# ---- 3. la cruceta ----
# cada apretada deja al heroe de cara a esa direccion y lo hace caminar; dos flechas dan la diagonal
cruceta $K_UP 2.0;        sleep 0.4; shot "1-arriba"
cruceta $K_RIGHT 2.0;     sleep 0.4; shot "2-derecha"
ydotool key $K_DOWN:1; ydotool key $K_RIGHT:1; sleep 2.0
ydotool key $K_DOWN:0; ydotool key $K_RIGHT:0; sleep 0.4; shot "3-diagonal-abajo-derecha"
cruceta $K_LEFT 2.0;      sleep 0.4; shot "4-izquierda"

# ---- 4. cerrar y recortar ----
sleep 1
kill -TERM -"$PID" 2>/dev/null || kill -TERM "$PID" 2>/dev/null
sleep 1; kill -KILL -"$PID" 2>/dev/null
bash "$HERE/flycast-adelante.sh" sacar >/dev/null 2>&1

for f in "$OUT"/crudo/*.png; do
    [ -f "$f" ] || continue
    ffmpeg -v error -i "$f" -vf "crop=760:560:210:210" -y "$OUT/$(basename "$f")" 2>/dev/null
done

echo "== capturas =="
ls -la "$OUT"/*.png 2>/dev/null | awk '{print "  "$5" B  "$9}'
echo "== el heroe en el log =="
grep -a "HEROE159" "$LOG" | tail -25
echo "== los cuadros medidos =="
grep -a "CUADRO " "$LOG" | tail -12
echo "== las fases del PVR (ms por cuadro) =="
grep -a "FASES" "$LOG" | tail -8
