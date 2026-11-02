#!/bin/bash
# DC-002.128 -- correr LA DEN OF EVIL en Flycast: aceptar la quest hablando con Akara, matar los bichos de la cueva
# con el contador avanzando, completarla, cobrar el punto de habilidad y ver que sobrevive al cambio de nivel.
#
# Es el mismo arranque de DC-002.127 (ventana identificable "Flycast - dc128", keepAbove, espera del primer cuadro,
# capturas cada INTERVALO), con tres cambios:
#
#   1. la marca con la que se da por cerrada la corrida es `JUEGO128: QUEST CRITERIO` (ademas de las de antes);
#   2. al final se vuelcan las lineas NUEVAS de esta tarjeta: la carga de la quest (los 34 nodos del motor y los
#      textos del juego), el habla con Akara y su marcador, el panel con la pagina del log, el cambio de area, los
#      bichos y el contador cuadro a cuadro, los bits del slot, el cobro y la supervivencia (registro + fases);
#   3. la espera del primer cuadro es mas larga (`ESPERA_ARRANQUE`): este POC carga el pueblo, el bicho, el drop, el
#      inventario, los NPCs, la quest y el HUD antes de dibujar, y en Flycast eso son minutos.
#
# QUE HACE Y POR QUE ASI
# ----------------------
#  1. Arranca Flycast con el GDI del build, con la ventana IDENTIFICABLE y keepAbove (lo mismo de run-flycast-249b.sh,
#     por los mismos motivos medidos: `spectacle -f` captura la pantalla entera y sin keepAbove cualquier ventana tapa
#     al emulador). La ventana se pone en 640x480 para que la captura sea 1:1 con el cuadro del POC.
#  2. Espera la marca `JUEGO121: PRIMER-CUADRO` (antes de eso la pantalla no tiene el cuadro del POC).
#  3. NO manda nada por el teclado virtual: el guion de la quest lo corre el POC (el emulador de estas corridas no
#     tiene teclado, y el A1 del heroe tarda 13 cuadros por golpe). El log dice en cada cuadro que paso.
#  4. Captura cada INTERVALO (4 s por omision) durante toda la corrida: el habla con Akara, el panel, el cambio de
#     area, los bichos cayendo y el cobro.
#  5. Espera la marca del cierre (las tres fases del ciclo de niveles de DC-002.120 son ~25 s cada una) y despues
#     mata el emulador.
#
# Uso:  run-flycast-128.sh <disc.gdi> <dir_salida> [x y]
#       CAPTURAS=60 INTERVALO=4 ESPERA_ARRANQUE=900 -> la ventana de captura de la corrida del criterio
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMG="${1:?uso: run-flycast-128.sh <disc.gdi> <dir_salida> [x y]}"
OUT="${2:-/tmp/dc128-run}"
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
TOKEN="dc128"
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
# DC-002.121: la corrida del criterio es SIN teclado (los dos por el guion del POC, cada uno por su puerto).
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
    if (cap.toLowerCase().indexOf("dc128") >= 0) {
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
    grep -qa -e 'JUEGO121: PRIMER-CUADRO' -e 'JUEGO121: SPLIT PRIMER-CUADRO' "$OUT/flycast.log" 2>/dev/null && { echo "  primer cuadro a los ${T}s"; break; }
    if vivo; then MUERTO=0; else
        MUERTO=$(( MUERTO + 1 ))
        if [ "$MUERTO" -ge 3 ]; then
            echo "  el emulador se murio antes de dibujar"; tail -12 "$OUT/flycast.log"; exit 1
        fi
    fi
    sleep 1
    T=$(( T + 1 ))
done

if ! grep -qa -e 'JUEGO121: PRIMER-CUADRO' -e 'JUEGO121: SPLIT PRIMER-CUADRO' "$OUT/flycast.log" 2>/dev/null; then
    echo "  NO llego al primer cuadro en ${ESPERA_ARRANQUE}s"
    grep -aE 'JUEGO121|Fatal|EXCEPTION|fatal error' "$OUT/flycast.log" | tail -25
    pkill -f "$PATRON" 2>/dev/null
    exit 1
fi

echo "-- las marcas del POC que ya salieron:"
grep -a 'JUEGO121: DIRECCION\|JUEGO121: CONTROL hay\|JUEGO121: MOTOR velocidad\|JUEGO121: JUGADOR CICLOS' \
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
    anim="$(grep -a 'JUEGO121: CUADRO [0-9]* CON-JUGADOR' "$OUT/flycast.log" 2>/dev/null | tr -d '\r' | tail -1 |
        sed -n 's/.*crc32= \([0-9-]*\) pixelesDelJugador= \([0-9]*\) ancla= \([0-9-]*\) \([0-9-]*\) dir= \([0-9-]*\) cuadroDelCiclo= \([0-9-]*\) modo= \([A-Z0-9]*\).*/crc32=\1 pixeles=\2 ancla=\3x\4 dir=\5 cuadro=\6 modo=\7/p')"
    pos="$(grep -a 'JUEGO121: CONTROL cuadro' "$OUT/flycast.log" 2>/dev/null | tr -d '\r' | tail -1 |
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

# DC-002.121: EL CIERRE DEL POC TARDA MAS QUE LAS CAPTURAS. El POC dibuja `cuadros+3` = 19 cuadros (~1,2-1,4 s cada
# uno en Flycast) y recien en el ultimo imprime el LEASH RESUMEN y el COOP RESUMEN, que son los numeros del criterio.
# Si el script matara el emulador al terminar las capturas, esos renglones no existirian. Se espera la marca, con
# tope.
ESPERA_CIERRE="${ESPERA_CIERRE:-600}"
T=0
while [ "$T" -lt "$ESPERA_CIERRE" ]; do
    grep -qa -e 'JUEGO128: QUEST CRITERIO' -e 'JUEGO125: HUD CRITERIO' -e 'JUEGO121: CICLO CRITERIO' -e 'JUEGO121: BICHO CRITERIO' -e 'JUEGO121: LEASH RESUMEN' "$OUT/flycast.log" 2>/dev/null && { echo "  el POC cerro a los ${T}s de esperar"; break; }
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

echo "== DC-002.128: LA CARGA DE LA QUEST (el nodo del motor, el registro y los textos del juego)"
grep -a 'JUEGO128: QUEST ARRANCA\|JUEGO128: QUEST ESTADO-INICIAL\|JUEGO128: QUEST TEXTO' "$OUT/flycast.log" | tr -d '\r'
echo "== DC-002.128: EL DIALOGO CON AKARA (el marcador, la lista del motor y el mensaje 64)"
grep -a 'JUEGO128: QUEST HABLA\|JUEGO128: QUEST marcad\|JUEGO128: QUEST MARCADOR\|JUEGO128: QUEST ACTIVATE\|JUEGO128: QUEST PASO paso= ACEPTA' "$OUT/flycast.log" | tr -d '\r'
echo "== DC-002.128: EL PANEL DE LA QUEST (nombre, pagina del log y su texto)"
grep -a 'JUEGO128: QUEST PANEL\|JUEGO128: QUEST PANEL-CIERRA' "$OUT/flycast.log" | tr -d '\r'
echo "== DC-002.128: LA CUEVA -- EL CAMBIO DE AREA Y LOS BICHOS (contador del motor cuadro a cuadro)"
grep -a 'JUEGO128: QUEST AREA\|JUEGO128: QUEST CUEVA BICHOS\|JUEGO128: QUEST GOLPE\|JUEGO128: QUEST GOLPE-UNO\|JUEGO128: QUEST FUERA\|JUEGO128: QUEST MATA\|JUEGO128: QUEST SONIDO' "$OUT/flycast.log" | tr -d '\r'
echo "== DC-002.128: LOS BITS DEL SLOT (una linea por cambio, el camino del motor)"
grep -a 'JUEGO128: QUEST BIT\|JUEGO128: QUEST EFECTOS\|JUEGO128: QUEST PASO' "$OUT/flycast.log" | tr -d '\r'
echo "== DC-002.128: LA CUEVA LIMPIA Y EL COBRO"
grep -a 'JUEGO128: QUEST LIMPIA\|JUEGO128: QUEST COBRA\|JUEGO128: QUEST NO-COBRA\|JUEGO128: QUEST COBRO' "$OUT/flycast.log" | tr -d '\r'
echo "== DC-002.128: LA SUPERVIVENCIA (el registro del guardado y el cambio de nivel de las fases)"
grep -a 'JUEGO128: QUEST SUPERVIVE\|JUEGO128: QUEST ESTADO-TRAS-CARGAR\|JUEGO128: QUEST FASE' "$OUT/flycast.log" | tr -d '\r'
echo "== DC-002.128: EL CIERRE (resumen y criterio)"
grep -a 'JUEGO128: QUEST RESUMEN\|JUEGO128: QUEST CRITERIO' "$OUT/flycast.log" | tr -d '\r'
echo "== DC-002.128: LA MEMORIA DE LA QUEST y el bicho de la cueva"
grep -a 'JUEGO121: MEMORIA quest\|JUEGO121: SEMIESPACIO quest\|JUEGO121: BICHO ARRANCA\|JUEGO121: BICHO MEMORIA' "$OUT/flycast.log" | tr -d '\r'
echo "== el ciclo de niveles (DC-002.120): las tres fases"
grep -a 'JUEGO121: CICLO SOSTIENE\|JUEGO121: FASE .* CUADRO\|JUEGO121: CICLO CRITERIO' "$OUT/flycast.log" | tr -d '\r' | head -12
echo "== capturas"; ls -la "$OUT"/c*.png 2>/dev/null | awk '{print "   "$5" bytes  "$9}'
exit 0
