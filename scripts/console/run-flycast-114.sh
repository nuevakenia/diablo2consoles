#!/bin/bash
# DC-002.114 -- correr el POC DE LOS DOS JUGADORES en Flycast y sacar las capturas.
#
# QUE HACE Y POR QUE ASI
# ----------------------
#  1. Arranca Flycast con el GDI del build, con la ventana IDENTIFICABLE ("Flycast - dc114") y keepAbove (lo
#     mismo de run-flycast-249b.sh, por los mismos motivos medidos: `spectacle -f` captura la pantalla entera y
#     sin keepAbove cualquier ventana tapa al emulador). La ventana se pone en 640x480 para que la captura sea
#     1:1 con la pantalla logica del POC.
#  2. Espera la marca `JUEGO114: PRIMER-CUADRO` (antes de eso la pantalla no tiene el cuadro del POC y las teclas
#     se perderian en el arranque).
#  3. Manda la CRUCETA con el teclado virtual (/dev/uinput -> Flycast -> bus maple) con UN SOLO aparato para toda
#     la secuencia (medido en DC-002.38: dos aparatos seguidos, el emulador solo ve el primero). Ese aparato llega
#     al puerto A, o sea al JUGADOR 1. El jugador 2 no tiene aparato en el puerto B: su estado lo pone el guion del
#     POC (`botonesCoop`, coop.go) y el log lo dice en cada linea (`inyecta= si`).
#  4. Captura cada INTERVALO mientras dura el guion y anota, al lado de cada captura, lo que dice el LOG en ese
#     momento (la celda y el cuadro de cada jugador): la captura no se etiqueta por el reloj.
#  5. Al final vuelca los numeros del criterio que el log ya midio: los COOP TRAMO (la independencia del input), los
#     CUADRO JUGADOR (los dos en pantalla), el COOP RESUMEN y la memoria.
#
# LAS DOS CORRIDAS (el A/B de la tarjeta):
#   JUGADORES=1 (build aparte)  -> un jugador, con el teclado: la referencia de ms por cuadro y memoria
#   JUGADORES=2 (este build)    -> los dos: `TECLAS=0 INYECTAR_P1=si` deja que el guion del POC mueva a los DOS (los
#                                  tramos SOLO-P1 / SOLO-P2 / LOS-DOS alineados al reloj de la consola)
#
# Uso:  run-flycast-114.sh <disc.gdi> <dir_salida> [x y]
#       CAPTURAS=20 INTERVALO=2.5 PAUSA=1 TECLAS=0 -> sin cruceta (el jugador 1 quieto: solo se mueve el 2)
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMG="${1:?uso: run-flycast-114.sh <disc.gdi> <dir_salida> [x y]}"
OUT="${2:-/tmp/dc114-run}"
POS="${3:-1400 120}"
[ -n "${4:-}" ] && POS="$3 $4"
# DC-002.64: el tamano de la ventana del emulador NO es un detalle: la pantalla logica del POC es 640x480 y con la
# ventana en 320x240 el personaje se reduce a la mitad. Con VENTANA=640x480 la captura es 1:1 con la pantalla
# logica del POC, que es lo que hace comparables las capturas con el cuadro de 640x480.
VENTANA="${VENTANA:-640x480}"
VW="${VENTANA%x*}"
VH="${VENTANA#*x}"
FLY="${FLYCAST:-$HOME/dreamcast/dl/flycast.AppImage}"
DISP="${DISPLAY:-:0}"
TOKEN="dc114"
BARRA_TITULO="${BARRA_TITULO:-28}"
PX="${POS%% *}"; PY="${POS##* }"
PYTHON="${DC_PYTHON:-python3}"
ESPERA_ARRANQUE="${ESPERA_ARRANQUE:-300}"
CAPTURAS="${CAPTURAS:-18}"
INTERVALO="${INTERVALO:-2}"
PAUSA="${PAUSA:-0.4}"
# EL GUION: los TRES tramos del cooperativo, alineados al reloj del POC (`guionCoop` en coop.go: 4 s, pausa 1 s,
# 4 s, pausa 1 s, 6 s). El teclado virtual llega al puerto A (jugador 1); el jugador 2 lo mueve el guion del POC.
# "nada" = la cruceta suelta: es el tramo en el que el jugador 1 NO tiene que moverse.
GUION="${GUION:-derecha 4 nada 1 nada 4 nada 1 arriba+izquierda 4}"

mkdir -p "$OUT"
rm -f "$OUT"/c*.png 2>/dev/null

DISC_DIR="$(cd "$(dirname "$IMG")" && pwd)"
GDI="$DISC_DIR/$TOKEN.gdi"
cp -f "$IMG" "$GDI"

REGLA_JS="${TMPDIR:-/tmp}/kwin-regla-$TOKEN-$$.js"
cat > "$REGLA_JS" <<'JSFIN'
var PX = __PX__, PY = __PY__, objetivo = null;
function ubicar(w) {
    var cap = "", cls = "";
    try { cap = (w.caption || "").toString(); } catch (e) { cap = ""; }
    try { cls = (w.resourceClass || "").toString(); } catch (e) { cls = ""; }
    if (cls.toLowerCase().indexOf("flycast") < 0) { return; }
    if (cap.toLowerCase().indexOf("dc114") >= 0) {
        var g = w.frameGeometry;
        if (g.x !== PX || g.y !== PY - 28) {
            w.frameGeometry = { x: PX, y: PY - 28, width: g.width, height: g.height };
        }
        w.keepAbove = true;
        objetivo = w;
    }
    print("DC113-CAP|" + cap + "|" + cls + "|pid=" + w.pid + "|geo=" + w.frameGeometry.x + "," +
          w.frameGeometry.y + "," + w.frameGeometry.width + "," + w.frameGeometry.height + "|cli=" +
          (function () {
              try {
                  var c = w.clientGeometry;
                  if (c && c.width > 0) { return c.x + "," + c.y + "," + c.width + "," + c.height; }
              } catch (e) { }
              return "?";
          })());
}
var pila = workspace.stackingOrder;
for (var i = 0; i < pila.length; i++) { ubicar(pila[i]); }
if (objetivo) {
    try { workspace.activeWindow = objetivo; print("DC113-CAP|foco=" + objetivo.caption); } catch (e) { print("DC113-CAP|foco=no-pude " + e); }
}
if (objetivo) {
    var gancho = setInterval(function () {
        try { objetivo.keepAbove = true; workspace.activeWindow = objetivo; } catch (e) { }
    }, 300);
}
print("DC113-CAP|fin");
JSFIN
sed -i "s/__PX__/$PX/; s/__PY__/$PY/" "$REGLA_JS"

regla() {
    qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.unloadScript "regla-$TOKEN" >/dev/null 2>&1
    qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.loadScript "$REGLA_JS" "regla-$TOKEN" >/dev/null 2>&1
    qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.start >/dev/null 2>&1
    sleep 1
    journalctl --user --since "-6 s" 2>/dev/null | grep -a 'DC113-CAP|' | tr -d '\r' |
        grep -i "flycast - $TOKEN" | grep 'geo=' | sed 's/.*DC113-CAP|/ventana: /' | tail -1
}

echo "== Flycast: $GDI (ventana $VENTANA en $PX,$PY + keepAbove; consola serie si)"
setsid env DISPLAY="$DISP" "$FLY" -config "window:fullscreen=no" \
    -config "window:width=$VW" -config "window:height=$VH" \
    -config "window:left=$PX" -config "window:top=$PY" \
    -config "Debug:SerialConsoleEnabled=true" "$GDI" >"$OUT/flycast.log" 2>&1 &
PID=$!
echo "pid=$PID"

PATRON="$TOKEN[.]gdi"
vivo() { pgrep -f "$PATRON" >/dev/null 2>&1; }

sleep 5
T=5
MUERTO=0
while [ "$T" -lt "$ESPERA_ARRANQUE" ]; do
    grep -qa 'JUEGO114: PRIMER-CUADRO' "$OUT/flycast.log" 2>/dev/null && { echo "  primer cuadro a los ${T}s"; break; }
    if vivo; then MUERTO=0; else
        MUERTO=$(( MUERTO + 1 ))
        if [ "$MUERTO" -ge 3 ]; then
            echo "  el emulador se murio antes de dibujar"; tail -12 "$OUT/flycast.log"; exit 1
        fi
    fi
    sleep 1
    T=$(( T + 1 ))
done

if ! grep -qa 'JUEGO114: PRIMER-CUADRO' "$OUT/flycast.log" 2>/dev/null; then
    echo "  NO llego al primer cuadro en ${ESPERA_ARRANQUE}s"
    grep -aE 'JUEGO114|Fatal|EXCEPTION|fatal error' "$OUT/flycast.log" | tail -25
    pkill -f "$PATRON" 2>/dev/null
    exit 1
fi

echo "-- las marcas del POC que ya salieron:"
grep -a 'JUEGO114: DIRECCION\|JUEGO114: CONTROL hay\|JUEGO114: MOTOR velocidad\|JUEGO114: JUGADOR CICLOS' \
    "$OUT/flycast.log" | tr -d '\r' | head -14

LIN="$(regla)"
echo "  ventana del emulador: ${LIN:-NO la encuentro}"
if ! echo "$LIN" | grep -qE 'cli=|geo='; then
    echo "  NO encuentro la ventana: ni las capturas ni las teclas valdrian"
    pkill -f "$PATRON" 2>/dev/null
    exit 1
fi
sleep 1

ultimo_md5=""
esperar_quieto() {
    local f="$1" a=-1 b=-1 i=0
    while [ "$i" -lt 15 ]; do
        b="$(stat -c %s "$f" 2>/dev/null || echo -1)"
        if [ "$b" = "$a" ] && [ "$b" != "-1" ] && [ "$b" != "0" ]; then return 0; fi
        a="$b"; sleep 0.2; i=$(( i + 1 ))
    done
    return 1
}

capturar() {
    local nombre="$1" lin cli geo anim pos md5
    lin="$(regla)"
    cli="$(echo "$lin" | sed -n 's/.*cli=\([0-9-]*,[0-9-]*,[0-9]*,[0-9-]*\).*/\1/p')"
    if [ -z "$cli" ]; then
        geo="$(echo "$lin" | sed -n 's/.*geo=\([0-9-]*\),\([0-9-]*\),\([0-9]*\),\([0-9]*\).*/\1,\2,\3,\4/p')"
        if [ -n "$geo" ]; then
            local gx gy gw gh
            IFS=, read -r gx gy gw gh <<<"$geo"
            cli="$gx,$(( gy + BARRA_TITULO )),$gw,$(( gh - BARRA_TITULO ))"
        fi
    fi
    for intento in 1 2 3; do
        spectacle -b -n -f -o "$OUT/$nombre.png" >/dev/null 2>&1
        esperar_quieto "$OUT/$nombre.png"
        md5="$(md5sum "$OUT/$nombre.png" 2>/dev/null | cut -d' ' -f1)"
        [ -z "$ultimo_md5" ] && break
        [ "$md5" != "$ultimo_md5" ] && break
        sleep 0.7
    done
    ultimo_md5="$md5"
    anim="$(grep -a 'JUEGO114: CUADRO [0-9]* CON-JUGADOR' "$OUT/flycast.log" 2>/dev/null | tr -d '\r' | tail -1 |
        sed -n 's/.*crc32= \([0-9-]*\) pixelesDelJugador= \([0-9]*\) ancla= \([0-9-]*\) \([0-9-]*\) dir= \([0-9-]*\) cuadroDelCiclo= \([0-9-]*\) modo= \([A-Z0-9]*\).*/crc32=\1 pixeles=\2 ancla=\3x\4 dir=\5 cuadro=\6 modo=\7/p')"
    pos="$(grep -a 'JUEGO114: CONTROL cuadro' "$OUT/flycast.log" 2>/dev/null | tr -d '\r' | tail -1 |
        sed -n 's/.*celda= \([0-9.-]*\) \([0-9.-]*\) ticks= \([0-9]*\) subtiles= \([0-9.-]*\).*/celda=\1,\2 ticks=\3 subtiles=\4/p')"
    echo "  $nombre.png md5=${md5:0:8} ${anim:-anim=?} ${pos:-pos=?}"
    [ -n "$cli" ] && echo "GEO captura=$nombre cliente=$cli ${anim:-} ${pos:-} ms=$(date +%s%3N)" | tee -a "$OUT/geo.txt"
}

echo "== capturas: una QUIETO y despues una cada ${INTERVALO}s mientras corre el guion"
vivo && capturar "c00-quieto"

if [ "${TECLAS:-1}" = "1" ]; then
    ( PAUSA="$PAUSA" "$PYTHON" "$HERE/teclas-virtuales.py" guion $GUION ) > "$OUT/teclas.log" 2>&1 &
    TECLAS_PID=$!
    echo "  guion de la cruceta: $GUION"
fi

for n in $(seq -w 1 "$CAPTURAS"); do
    vivo || { echo "  el emulador se murio antes de la captura $n"; break; }
    capturar "c$n"
    sleep "$INTERVALO"
done

if [ "${TECLAS:-1}" = "1" ]; then wait "${TECLAS_PID:-0}" 2>/dev/null; fi
vivo && capturar "c99-final"
sleep 1

kill -TERM -"$PID" 2>/dev/null || kill -TERM "$PID" 2>/dev/null
pkill -f "$PATRON" 2>/dev/null
sleep 1
kill -KILL -"$PID" 2>/dev/null
pkill -9 -f "$PATRON" 2>/dev/null
qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.unloadScript "regla-$TOKEN" >/dev/null 2>&1
rm -f "$REGLA_JS"

echo
echo "== los puertos del bus (quien tiene control y quien va por guion)"
grep -a 'JUEGO114: PUERTO' "$OUT/flycast.log" | tr -d '\r'
echo "== los tramos del COOPERATIVO (la independencia del input: los ticks y las celdas de CADA uno)"
grep -a 'JUEGO114: COOP TRAMO' "$OUT/flycast.log" | tr -d '\r'
echo "== los dos en el mismo cuadro (los ultimos 2 cuadros, un renglon por jugador)"
grep -a 'JUEGO114: CUADRO [0-9]* JUGADOR' "$OUT/flycast.log" | tr -d '\r' | tail -4
echo "== el cuadro 1 (el mapa solo y el compuesto) y la repetibilidad"
grep -a 'JUEGO114: CUADRO 1 SOLO-MAPA\|JUEGO114: CUADRO 1 REFERENCIA\|JUEGO114: REPETIBILIDAD\|JUEGO114: MAPA-RESTAURADO' "$OUT/flycast.log" |
    tr -d '\r'
echo "== el resumen del cooperativo"
grep -a 'JUEGO114: RESUMEN\|JUEGO114: COOP RESUMEN\|JUEGO114: COOP MEMORIA' "$OUT/flycast.log" | tr -d '\r'
echo "== memoria"
grep -a 'JUEGO114: MEMORIA despues-de-componer\|JUEGO114: SEMIESPACIO despues-de-componer\|JUEGO114: MEMORIA coop-fin\|JUEGO114: SEMIESPACIO coop-fin' "$OUT/flycast.log" |
    tr -d '\r' | tail -6
echo "== capturas"; ls -la "$OUT"/c*.png 2>/dev/null | awk '{print "   "$5" bytes  "$9}'
echo "== teclas"; tail -6 "$OUT/teclas.log" 2>/dev/null
exit 0
