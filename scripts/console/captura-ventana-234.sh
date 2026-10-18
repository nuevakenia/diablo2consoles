#!/bin/bash
# DC-002.34: corre el MOTOR en Flycast y captura LA VENTANA DEL EMULADOR (no el escritorio), con numeros.
#
# POR QUE ASI (medido, no supuesto):
#  * `spectacle -f` (lo que usa run-flycast.sh) fotografia el ESCRITORIO COMPLETO: en este host 3584x1080
#    (dos monitores: uno de 2560x1080 y otro de 1024x768 en +2560+312). El recorte FIJO de DC-002.35
#    (210,210 640x480) no caia sobre el emulador: sus numeros median el tema oscuro del escritorio.
#    Aca NO se recorta: se captura la VENTANA por su id de X11.
#  * La ventana de Flycast es X11 (XWayland) y aparece en `_NET_CLIENT_LIST` del root: se identifica por
#    `WM_CLASS=flycast` y se captura con `import -window 0x<id>`. NO por nombre: `import -window Flycast`
#    contesta "no window with specified ID exists 'Flycast'" y se queda esperando un clic (medido: cuelga
#    la corrida 300 s).
#  * Se elige la ventana NUEVA (la que no estaba antes de lanzar): si otro agente tiene su Flycast abierto
#    -- paso en este host, dos corridas del tablero a la vez -- no se le captura la ventana al otro.
#
# Uso:  captura-ventana-234.sh <imagen .gdi/.elf> [segundos] [dir_salida]
#       (por defecto: 25 s, y el dir sale de $SAL)
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

IMG="${1:?uso: captura-ventana-234.sh <imagen> [segundos] [dir_salida]}"
SECS="${2:-25}"
OUT="${3:-/home/dgalli/.hermes/cache/scratch/corpus-234/capturas}"
FLY="${FLYCAST:-$HOME/dreamcast/dl/flycast.AppImage}"
DISP="${DISPLAY:-:0}"
CLASIF="${CLASIF:-$HERE/clasifica-captura-234.py}"

mkdir -p "$OUT"
rm -f "$OUT"/win-*.png "$OUT"/full-*.png

[ -x "$FLY" ] || { echo "no existe $FLY"; exit 1; }

ventanas_flycast() {
    DISPLAY="$DISP" xprop -root _NET_CLIENT_LIST 2>/dev/null |
        tr ',' '\n' | grep -o '0x[0-9a-f]*' |
        while read -r id; do
            clase=$(DISPLAY="$DISP" xprop -id "$id" WM_CLASS 2>/dev/null)
            case "$clase" in *flycast*) echo "$id" ;; esac
        done
}
pids_flycast() { pgrep -f 'flycast\.AppImage' 2>/dev/null | sort; }

ANTES="$(ventanas_flycast | sort | tr '\n' ' ')"
PIDS_ANTES="$(pids_flycast | tr '\n' ' ')"
echo "ventanas flycast ANTES: ${ANTES:-ninguna}"

bash "$HERE/flycast-adelante.sh" arriba 2>/dev/null

setsid env DISPLAY="$DISP" "$FLY" -config "window:fullscreen=no" \
    -config "Debug:SerialConsoleEnabled=true" "$IMG" > "$OUT/flycast.log" 2>&1 &
PID=$!
echo "lanzado: pid=$PID imagen=$IMG"

ID=""
for _ in $(seq 1 40); do
    sleep 0.5
    for v in $(ventanas_flycast | sort | tr '\n' ' '); do
        case " $ANTES " in *" $v "*) ;; *) ID="$v" ;; esac
    done
    [ -n "$ID" ] && break
done

if [ -z "$ID" ]; then
    echo "NO APARECIO la ventana del emulador (¿el binario murio antes de abrirla?)"
else
    echo "ventana NUEVA: $ID ($(DISPLAY="$DISP" xprop -id "$ID" WM_CLASS _NET_WM_NAME 2>/dev/null | tr '\n' ' '))"
fi

t=0
while [ "$t" -lt "$SECS" ]; do
    sleep 1
    t=$((t + 1))
    kill -0 "$PID" 2>/dev/null || { echo "  el emulador se detuvo en t=${t}s"; break; }
    [ -n "$ID" ] || continue
    n=$(printf '%03d' "$t")
    DISPLAY="$DISP" timeout 15 import -window "$ID" "$OUT/win-${n}s.png" 2>"$OUT/win-${n}s.err"
done

echo "== capturas de la ventana: $(ls "$OUT"/win-*s.png 2>/dev/null | wc -l) =="
python3 "$CLASIF" "$OUT"/win-*s.png 2>/dev/null | sed 's/.*-> //' | sort | uniq -c | sort -rn

# unas capturas del escritorio entero, para el registro de donde estaba la ventana
for t in 3 10; do
    [ "$t" -le "$SECS" ] || continue
    timeout 25 spectacle -b -n -f -o "$OUT/full-${t}s.png" >/dev/null 2>&1
done

sleep 1
kill -TERM -"$PID" 2>/dev/null || kill -TERM "$PID" 2>/dev/null
sleep 2
for p in $(pids_flycast); do
    case " $PIDS_ANTES " in *" $p "*) ;; *) kill -TERM "$p" 2>/dev/null ;; esac
done
sleep 1
for p in $(pids_flycast); do
    case " $PIDS_ANTES " in *" $p "*) ;; *) kill -KILL "$p" 2>/dev/null ;; esac
done
bash "$HERE/flycast-adelante.sh" sacar 2>/dev/null

echo "== vueltas del arranque en la consola serie (cada muerte del motor reinicia la consola) =="
tr -d '\r' < "$OUT/flycast.log" | grep -ac 'MOTOR-ARRANQUE arranca' || true
echo "== el punto donde muere (ultima vuelta) =="
tr -d '\r' < "$OUT/flycast.log" |
    grep -a 'Adding source\|Out of memory\|fatal error\|Memory:\|EBITEN-PERFIL\|cuadros del loop\|gc-por-umbral\|game files not found' |
    tail -14
