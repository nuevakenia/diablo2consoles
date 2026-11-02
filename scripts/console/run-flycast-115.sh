#!/bin/bash
# DC-002.115 -- correr el POC DEL LEASH en Flycast y sacar las capturas.
#
# QUE HACE Y POR QUE ASI
# ----------------------
#  1. Arranca Flycast con el GDI del build, con la ventana IDENTIFICABLE ("Flycast - dc115") y keepAbove (lo
#     mismo de run-flycast-249b.sh, por los mismos motivos medidos: `spectacle -f` captura la pantalla entera y
#     sin keepAbove cualquier ventana tapa al emulador). La ventana se pone en 640x480 para que la captura sea
#     1:1 con la pantalla logica del POC.
#  2. Espera la marca `JUEGO115: PRIMER-CUADRO` (antes de eso la pantalla no tiene el cuadro del POC y las teclas
#     se perderian en el arranque).
#  3. NO manda nada por el teclado virtual: en esta tarjeta los DOS jugadores van por el guion del POC (`guionPoc=
#     leash`: 8 tramos de 7 s en los que se intentan separar en las 8 direcciones), cada uno por SU puerto del bus
#     maple. Es la corrida del criterio: si el que se mueve lo hiciera el teclado del host, la mitad de la prueba
#     dependeria de que la ventana tenga el foco.
#  4. Captura cada INTERVALO durante los 63 s del guion, y anota al lado de cada captura lo que dice el LOG en ese
#     momento (la separacion en celdas y px y si los dos estan adentro del lienzo): la captura no se etiqueta por el
#     reloj.
#  5. Al final vuelca los numeros del criterio que el log ya midio: la MEDIDA del leash (las 8 direcciones), la
#     SEPARACION de cada cuadro con `cuadrosFuera`, los COOP TRAMO (los ticks de cada uno) y el LEASH RESUMEN.
#
# Uso:  run-flycast-115.sh <disc.gdi> <dir_salida> [x y]
#       CAPTURAS=10 INTERVALO=8 TECLAS=1 -> ademas de los dos guionados, la cruceta del teclado virtual al puerto A
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMG="${1:?uso: run-flycast-115.sh <disc.gdi> <dir_salida> [x y]}"
OUT="${2:-/tmp/dc115-run}"
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
TOKEN="dc115"
BARRA_TITULO="${BARRA_TITULO:-28}"
PX="${POS%% *}"; PY="${POS##* }"
PYTHON="${DC_PYTHON:-python3}"
ESPERA_ARRANQUE="${ESPERA_ARRANQUE:-300}"
CAPTURAS="${CAPTURAS:-10}"
INTERVALO="${INTERVALO:-8}"
PAUSA="${PAUSA:-0.4}"
# EL GUION DEL TECLADO VIRTUAL: en esta tarjeta el que mueve es el POC (guionPoc=leash) y el teclado NO se usa por
# defecto. Si se pide TECLAS=1, este guion le llega al puerto A (jugador 1) y "nada" es la cruceta suelta.
GUION="${GUION:-nada 8 nada 8 nada 8 nada 8}"
# DC-002.115: la corrida del criterio es SIN teclado (los dos por el guion del POC, cada uno por su puerto).
TECLAS="${TECLAS:-0}"

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
    if (cap.toLowerCase().indexOf("dc115") >= 0) {
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
    grep -qa 'JUEGO115: PRIMER-CUADRO' "$OUT/flycast.log" 2>/dev/null && { echo "  primer cuadro a los ${T}s"; break; }
    if vivo; then MUERTO=0; else
        MUERTO=$(( MUERTO + 1 ))
        if [ "$MUERTO" -ge 3 ]; then
            echo "  el emulador se murio antes de dibujar"; tail -12 "$OUT/flycast.log"; exit 1
        fi
    fi
    sleep 1
    T=$(( T + 1 ))
done

if ! grep -qa 'JUEGO115: PRIMER-CUADRO' "$OUT/flycast.log" 2>/dev/null; then
    echo "  NO llego al primer cuadro en ${ESPERA_ARRANQUE}s"
    grep -aE 'JUEGO115|Fatal|EXCEPTION|fatal error' "$OUT/flycast.log" | tail -25
    pkill -f "$PATRON" 2>/dev/null
    exit 1
fi

echo "-- las marcas del POC que ya salieron:"
grep -a 'JUEGO115: DIRECCION\|JUEGO115: CONTROL hay\|JUEGO115: MOTOR velocidad\|JUEGO115: JUGADOR CICLOS' \
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
    anim="$(grep -a 'JUEGO115: CUADRO [0-9]* CON-JUGADOR' "$OUT/flycast.log" 2>/dev/null | tr -d '\r' | tail -1 |
        sed -n 's/.*crc32= \([0-9-]*\) pixelesDelJugador= \([0-9]*\) ancla= \([0-9-]*\) \([0-9-]*\) dir= \([0-9-]*\) cuadroDelCiclo= \([0-9-]*\) modo= \([A-Z0-9]*\).*/crc32=\1 pixeles=\2 ancla=\3x\4 dir=\5 cuadro=\6 modo=\7/p')"
    pos="$(grep -a 'JUEGO115: CONTROL cuadro' "$OUT/flycast.log" 2>/dev/null | tr -d '\r' | tail -1 |
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

# DC-002.115: EL CIERRE DEL POC TARDA MAS QUE LAS CAPTURAS. El POC dibuja `cuadros+3` = 87 cuadros (~1,2 s cada
# uno en Flycast) y recien en el ultimo imprime el LEASH RESUMEN, que es el numero del criterio. Si el script
# matara el emulador al terminar las capturas, ese renglon no existiria. Se espera la marca, con tope.
ESPERA_CIERRE="${ESPERA_CIERRE:-180}"
T=0
while [ "$T" -lt "$ESPERA_CIERRE" ]; do
    grep -qa 'JUEGO115: LEASH RESUMEN' "$OUT/flycast.log" 2>/dev/null && { echo "  el POC cerro a los ${T}s de esperar"; break; }
    vivo || { echo "  el emulador se murio esperando el cierre"; break; }
    sleep 3
    T=$(( T + 3 ))
done

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
echo "== la MEDIDA del leash (la caja peor, el autochequeo y las 8 direcciones de separacion)"
grep -a 'JUEGO115: LEASH CAJA-PEOR\|JUEGO115: LEASH AUTOCHEQUEO\|JUEGO115: LEASH DIRECCION\|JUEGO115: LEASH MEDIDA' "$OUT/flycast.log" | tr -d '\r'
echo "== la SEPARACION de cada cuadro (celdas y px) y los cuadros FUERA del lienzo"
grep -a 'JUEGO115: LEASH SEPARACION\|JUEGO115: LEASH FUERA' "$OUT/flycast.log" | tr -d '\r'
echo "== los jugadores, cuadro por cuadro (la caja SIN recortar y dentroDelLienzo)"
grep -a 'JUEGO115: LEASH cuadro=' "$OUT/flycast.log" | tr -d '\r' | tail -8
echo "== el cierre del leash (el numero del criterio)"
grep -a 'JUEGO115: LEASH RESUMEN' "$OUT/flycast.log" | tr -d '\r'
echo "== los puertos del bus (quien tiene control y quien va por guion)"
grep -a 'JUEGO115: PUERTO' "$OUT/flycast.log" | tr -d '\r'
echo "== los tramos del COOPERATIVO (la independencia del input: los ticks y las celdas de CADA uno)"
grep -a 'JUEGO115: COOP TRAMO' "$OUT/flycast.log" | tr -d '\r'
echo "== los dos en el mismo cuadro (los ultimos 2 cuadros, un renglon por jugador)"
grep -a 'JUEGO115: CUADRO [0-9]* JUGADOR' "$OUT/flycast.log" | tr -d '\r' | tail -4
echo "== el cuadro 1 (el mapa solo y el compuesto) y la repetibilidad"
grep -a 'JUEGO115: CUADRO 1 SOLO-MAPA\|JUEGO115: CUADRO 1 REFERENCIA\|JUEGO115: REPETIBILIDAD\|JUEGO115: MAPA-RESTAURADO' "$OUT/flycast.log" |
    tr -d '\r'
echo "== el resumen del cooperativo"
grep -a 'JUEGO115: RESUMEN\|JUEGO115: COOP RESUMEN\|JUEGO115: COOP MEMORIA' "$OUT/flycast.log" | tr -d '\r'
echo "== memoria"
grep -a 'JUEGO115: MEMORIA despues-de-componer\|JUEGO115: SEMIESPACIO despues-de-componer\|JUEGO115: MEMORIA coop-fin\|JUEGO115: SEMIESPACIO coop-fin' "$OUT/flycast.log" |
    tr -d '\r' | tail -6
echo "== capturas"; ls -la "$OUT"/c*.png 2>/dev/null | awk '{print "   "$5" bytes  "$9}'
echo "== teclas"; tail -6 "$OUT/teclas.log" 2>/dev/null
exit 0
