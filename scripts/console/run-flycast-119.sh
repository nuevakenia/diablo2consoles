#!/bin/bash
# DC-002.119 -- correr el POC DEL CENSO Y LOS TRAMOS en Flycast y volcar los numeros del semiespacio.
#
# QUE HACE Y POR QUE ASI
# ----------------------
#  1. Arranca Flycast con el GDI del build, con la ventana IDENTIFICABLE ("Flycast - dc119") y keepAbove (lo
#     mismo de run-flycast-249b.sh, por los mismos motivos medidos: `spectacle -f` captura la pantalla entera y
#     sin keepAbove cualquier ventana tapa al emulador). La ventana se pone en 640x480 para que la captura sea
#     1:1 con el cuadro del POC, que es lo que hace comparables las capturas entre las dos corridas del A/B.
#  2. Espera la marca `JUEGO119: PRIMER-CUADRO` (antes de eso la pantalla no tiene el cuadro del POC).
#  3. NO manda nada por el teclado virtual: los jugadores van por el GUION del shim, cada uno por SU puerto del
#     bus maple. Si el que se mueve lo hiciera el teclado del host, la prueba dependeria del foco de la ventana.
#  4. Captura cada INTERVALO (2 s por omision) durante toda la corrida: los cuadros del guion y los tres del
#     cierre, y cada captura va con lo que el LOG dice en ese momento.
#  5. Al final vuelca los numeros del criterio que el log ya midio: las etapas del CENSO, los TRAMOS (pedidos,
#     repartidos, devueltos), el COOP RESUMEN y el cuadro cuadro por cuadro (crc32, ms).
#
# Uso:  run-flycast-119.sh <disc.gdi> <dir_salida> [x y]
#       CAPTURAS=2 INTERVALO=2 -> la corrida del criterio (9 cuadros del guion, ~40 s de emulador)
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMG="${1:?uso: run-flycast-119.sh <disc.gdi> <dir_salida> [x y]}"
OUT="${2:-/tmp/dc119-run}"
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
TOKEN="dc119"
BARRA_TITULO="${BARRA_TITULO:-28}"
PX="${POS%% *}"; PY="${POS##* }"
PYTHON="${DC_PYTHON:-python3}"
ESPERA_ARRANQUE="${ESPERA_ARRANQUE:-300}"
CAPTURAS="${CAPTURAS:-80}"
INTERVALO="${INTERVALO:-2}"
PAUSA="${PAUSA:-0.4}"
# EL GUION DEL TECLADO VIRTUAL: en esta tarjeta el que mueve es el POC (guionPoc=leash) y el teclado NO se usa por
# defecto. Si se pide TECLAS=1, este guion le llega al puerto A (jugador 1) y "nada" es la cruceta suelta.
GUION="${GUION:-nada 8 nada 8 nada 8 nada 8}"
# DC-002.119: la corrida del criterio es SIN teclado (los dos por el guion del POC, cada uno por su puerto).
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
    if (cap.toLowerCase().indexOf("dc119") >= 0) {
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
    grep -qa -e 'JUEGO119: PRIMER-CUADRO' -e 'JUEGO119: SPLIT PRIMER-CUADRO' "$OUT/flycast.log" 2>/dev/null && { echo "  primer cuadro a los ${T}s"; break; }
    if vivo; then MUERTO=0; else
        MUERTO=$(( MUERTO + 1 ))
        if [ "$MUERTO" -ge 3 ]; then
            echo "  el emulador se murio antes de dibujar"; tail -12 "$OUT/flycast.log"; exit 1
        fi
    fi
    sleep 1
    T=$(( T + 1 ))
done

if ! grep -qa -e 'JUEGO119: PRIMER-CUADRO' -e 'JUEGO119: SPLIT PRIMER-CUADRO' "$OUT/flycast.log" 2>/dev/null; then
    echo "  NO llego al primer cuadro en ${ESPERA_ARRANQUE}s"
    grep -aE 'JUEGO119|Fatal|EXCEPTION|fatal error' "$OUT/flycast.log" | tail -25
    pkill -f "$PATRON" 2>/dev/null
    exit 1
fi

echo "-- las marcas del POC que ya salieron:"
grep -a 'JUEGO119: DIRECCION\|JUEGO119: CONTROL hay\|JUEGO119: MOTOR velocidad\|JUEGO119: JUGADOR CICLOS' \
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
    anim="$(grep -a 'JUEGO119: CUADRO [0-9]* CON-JUGADOR' "$OUT/flycast.log" 2>/dev/null | tr -d '\r' | tail -1 |
        sed -n 's/.*crc32= \([0-9-]*\) pixelesDelJugador= \([0-9]*\) ancla= \([0-9-]*\) \([0-9-]*\) dir= \([0-9-]*\) cuadroDelCiclo= \([0-9-]*\) modo= \([A-Z0-9]*\).*/crc32=\1 pixeles=\2 ancla=\3x\4 dir=\5 cuadro=\6 modo=\7/p')"
    pos="$(grep -a 'JUEGO119: CONTROL cuadro' "$OUT/flycast.log" 2>/dev/null | tr -d '\r' | tail -1 |
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

# DC-002.119: EL CIERRE DEL POC TARDA MAS QUE LAS CAPTURAS. El POC dibuja `cuadros+3` = 19 cuadros (~1,2-1,4 s cada
# uno en Flycast) y recien en el ultimo imprime el LEASH RESUMEN y el COOP RESUMEN, que son los numeros del criterio.
# Si el script matara el emulador al terminar las capturas, esos renglones no existirian. Se espera la marca, con
# tope.
ESPERA_CIERRE="${ESPERA_CIERRE:-600}"
T=0
while [ "$T" -lt "$ESPERA_CIERRE" ]; do
    grep -qa -e 'JUEGO119: SPLIT RESUMEN' -e 'JUEGO119: LEASH RESUMEN' "$OUT/flycast.log" 2>/dev/null && { echo "  el POC cerro a los ${T}s de esperar"; break; }
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
qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.unregisterScript "regla-$TOKEN" >/dev/null 2>&1
qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.unloadScript "regla-$TOKEN" >/dev/null 2>&1
rm -f "$REGLA_JS"

echo "== LOS PUERTOS DEL BUS: quien tiene control y quien va por el guion (uno por jugador)"
grep -a 'JUEGO119: PUERTO' "$OUT/flycast.log" | tr -d '\r'
echo "== LA MEMORIA DE LOS JUGADORES: estado por jugador, cuadros compartidos, estructura"
grep -a 'JUEGO119: COOP MEMORIA' "$OUT/flycast.log" | tr -d '\r'
echo "== LA MEDIDA DEL LEASH y el grupo"
grep -a 'JUEGO119: LEASH CAJA-PEOR\|JUEGO119: LEASH AUTOCHEQUEO\|JUEGO119: LEASH MEDIDA\|JUEGO119: LEASH GRUPO\|JUEGO119: LEASH RESUMEN\|JUEGO119: LEASH no se aplica\|JUEGO119: LEASH AVISO' "$OUT/flycast.log" | tr -d '\r'
echo "== LA VIGILANCIA: cada cuadro, cada jugador (dentroDelLienzo y recortes) y la separacion del grupo"
grep -a 'JUEGO119: LEASH cuadro=\|JUEGO119: LEASH SEPARACION\|JUEGO119: LEASH FUERA\|JUEGO119: LEASH SIMETRICO\|JUEGO119: LEASH GRUPO SIMETRICO' "$OUT/flycast.log" | tr -d '\r'
echo "== LOS TRAMOS: los ticks y las celdas de CADA jugador"
grep -a 'JUEGO119: COOP TRAMO' "$OUT/flycast.log" | tr -d '\r'
echo "== EL CUADRO, cuadro por cuadro (ms del mapa, ms de los jugadores, crc)"
grep -a 'JUEGO119: CUADRO [0-9]* CON-JUGADOR' "$OUT/flycast.log" | tr -d '\r'
echo "== CADA JUGADOR EN EL CUADRO (ancla, caja, pixeles pintados, celda)"
grep -a 'JUEGO119: CUADRO [0-9]* JUGADOR nombre=' "$OUT/flycast.log" | tr -d '\r' | tail -24
echo "== EL CICLO CON LOS CUATRO: las tres fases (crc32 contra el del host, pico y ms por fase)"
grep -a 'JUEGO119: CICLO\|JUEGO119: FASE .* CUADRO\|JUEGO119: FASE .* FIN\|JUEGO119: FASE .* ORDER\|JUEGO119: FASE .* ORDEN\|JUEGO119: FASE .* FORMACION\|JUEGO119: FASE .* DS1' "$OUT/flycast.log" | tr -d '\r'
echo "== el cierre: origen, repetibilidad y los numeros del criterio"
grep -a 'JUEGO119: REPETIBILIDAD\|JUEGO119: VUELTA AL ORIGEN\|JUEGO119: COOP JUGADOR\|JUEGO119: COOP RESUMEN\|JUEGO119: RESUMEN\|JUEGO119: FIN' "$OUT/flycast.log" | tr -d '\r'
echo "== censo del semiespacio (DC-002.119)"; grep -a 'JUEGO119: CENSO' "$OUT/flycast.log" | tr -d '\r' | tail -30
echo "== tramos con dueno (DC-002.119)"; grep -a 'JUEGO119: TRAMO\|D2C119: TRAMO' "$OUT/flycast.log" | tr -d '\r' | tail -20
echo "== memoria"; grep -a 'JUEGO119: MEMORIA\|JUEGO119: SEMIESPACIO\|JUEGO119: TRAMO despues\|JUEGO119: VRAM' "$OUT/flycast.log" | tr -d '\r' | tail -10
echo "== EL SPLIT: el reparto, los lienzos, cada recuadro y los numeros del criterio"
grep -a 'JUEGO119: SPLIT' "$OUT/flycast.log" | tr -d '\r'
echo "== capturas"; ls -la "$OUT"/c*.png 2>/dev/null | awk '{print "   "$5" bytes  "$9}'
echo "== teclas"; tail -6 "$OUT/teclas.log" 2>/dev/null
exit 0
