#!/bin/bash
# DC-002.140 -- CORRER LA DEMO EN FLYCAST COMO LA CORRE UNA PERSONA: con el TECLADO.
#
# QUE HACE Y POR QUE ASI
# ----------------------
#  1. Arranca Flycast con el GDI del build, la ventana IDENTIFICABLE ("Flycast - dc140"), keepAbove y 640x480 (la
#     pantalla logica del POC), igual que run-mvp-138.sh: `spectacle -f` captura la pantalla entera (2560x1080) y
#     sin keepAbove cualquier ventana tapa al emulador.
#  2. Espera `JUEGO121: PRIMER-CUADRO`: antes de esa marca la pantalla no tiene el cuadro del POC.
#  3. MANDA LAS TECLAS POR EL TECLADO VIRTUAL del kernel (/dev/uinput, `teclas-virtuales.py`): es el mismo camino
#     que el dedo de una persona sobre el teclado del emulador (tecla -> Flycast -> bus maple -> servicio -> POC).
#     El POC de esta tarjeta NO aprieta solo: si las teclas no llegan, no pasa nada en la pantalla.
#  4. Captura una foto por paso, con la linea del POC de ese instante, y las guarda con el nombre del paso.
#  5. Al final vuelca las lineas de la demo (etapas, FPS, quest, salida) y la lista de capturas.
#
# Uso:
#   run-demo-140.sh <disc.gdi> [dir_salida] [x y]
#   GUION='nada 8 derecha 2 nada 1 x 0.3 ...' CAPTURAS=40 bash scripts/console/run-demo-140.sh ...
#
# EL GUION DE TECLAS (por omision, el recorrido del criterio):
#   nada 8          dejar que cargue el mundo (el POC tarda ~60 s desde el disco hasta el primer cuadro)
#   derecha 2.2     caminar hacia Akara (la proyeccion iso: derecha = +x -y en celdas)
#   x 0.3           HABLAR con Akara: abre el panel y toma la Den of Evil
#   nada 1.2
#   abajo 1   ...   alejarse de Akara y buscar el bicho (aparece a +1 +1 de la celda de arranque)
#   y 0.25 (x5)     GOLPEAR: el boton Y, un pulso corto por golpe
#   x 0.3           volver a Akara y COBRAR
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

IMG="${1:?uso: run-demo-140.sh <disc.gdi> [dir_salida] [x y]}"
OUT="${2:-/tmp/capturas-demo140}"
POS="${3:-900 120}"
[ -n "${4:-}" ] && POS="$3 $4"

VENTANA="${VENTANA:-640x480}"
VW="${VENTANA%x*}"
VH="${VENTANA#*x}"
FLY="${FLYCAST:-$HOME/dreamcast/dl/flycast.AppImage}"
DISP="${DISPLAY:-:0}"
TOKEN="dc140"
BARRA_TITULO="${BARRA_TITULO:-28}"
PX="${POS%% *}"
PY="${POS##* }"
PYTHON="${DC_PYTHON:-python3}"
ESPERA_ARRANQUE="${ESPERA_ARRANQUE:-420}"
INTERVALO="${INTERVALO:-2}"
PAUSA="${PAUSA:-0.35}"
SEGUNDOS="${SEGUNDOS:-120}"
GUION="${GUION:-nada 8 derecha 2.2 nada 0.6 x 0.3 nada 1.2 abajo 1.6 nada 0.6 y 0.25 nada 0.5 y 0.25 nada 0.5 y 0.25 nada 0.5 y 0.25 nada 0.5 y 0.25 nada 0.5 y 0.25 nada 1.0 arriba 2.0 x 0.3 nada 1.0 y 0.25 nada 0.5 y 0.25}"

mkdir -p "$OUT"
rm -f "$OUT"/c*.png 2>/dev/null

DISC_DIR="$(cd "$(dirname "$IMG")" && pwd)"
GDI="$DISC_DIR/$TOKEN.gdi"
cp -f "$IMG" "$GDI"

# ---- la regla de KWin: ventana donde el script dice, keepAbove, y devuelve su geometria ----
REGLA_JS="${TMPDIR:-/tmp}/kwin-regla-$TOKEN-$$.js"
cat > "$REGLA_JS" <<'JSFIN'
var PX = __PX__, PY = __PY__, objetivo = null;
function ubicar(w) {
    var cap = "", cls = "";
    try { cap = (w.caption || "").toString(); } catch (e) { cap = ""; }
    try { cls = (w.resourceClass || "").toString(); } catch (e) { cls = ""; }
    if (cls.toLowerCase().indexOf("flycast") < 0) { return; }
    if (cap.toLowerCase().indexOf("dc140") >= 0) {
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
    try { workspace.activeWindow = objetivo; } catch (e) { }
    setInterval(function () {
        try { objetivo.keepAbove = true; } catch (e) { }
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

echo "== DEMO DC-002.140 en Flycast: $GDI (ventana $VENTANA en $PX,$PY, keepAbove, consola serie si)"
setsid env DISPLAY="$DISP" "$FLY" -config "window:fullscreen=no" \
    -config "window:width=$VW" -config "window:height=$VH" \
    -config "window:left=$PX" -config "window:top=$PY" \
    -config "Debug:SerialConsoleEnabled=true" "$GDI" >"$OUT/flycast.log" 2>&1 &
PID=$!
echo "  pid=$PID"

PATRON="$TOKEN[.]gdi"
vivo() { pgrep -f "$PATRON" >/dev/null 2>&1; }

sleep 5
T=5
MUERTO=0
while [ "$T" -lt "$ESPERA_ARRANQUE" ]; do
    grep -qa 'JUEGO121: PRIMER-CUADRO' "$OUT/flycast.log" 2>/dev/null && { echo "  primer cuadro a los ${T}s"; break; }
    if vivo; then MUERTO=0; else
        MUERTO=$(( MUERTO + 1 ))
        if [ "$MUERTO" -ge 3 ]; then
            echo "  el emulador se murio antes de dibujar"; tail -20 "$OUT/flycast.log"; exit 1
        fi
    fi
    sleep 1
    T=$(( T + 1 ))
done

if ! grep -qa 'JUEGO121: PRIMER-CUADRO' "$OUT/flycast.log" 2>/dev/null; then
    echo "  NO llego al primer cuadro en ${ESPERA_ARRANQUE}s"
    grep -aE 'DEMO140|Fatal|EXCEPTION|fatal error|OOM' "$OUT/flycast.log" | tail -25
    pkill -f "$PATRON" 2>/dev/null
    exit 1
fi

LIN="$(regla)"
echo "  ventana del emulador: ${LIN:-NO la encuentro}"
if ! echo "$LIN" | grep -qE 'cli=|geo='; then
    echo "  NO encuentro la ventana: las capturas no valdrian"
    pkill -f "$PATRON" 2>/dev/null
    exit 1
fi
sleep 1

cli() {
    local lin
    lin="$(regla)"
    echo "$lin" | sed -n 's/.*cli=\([0-9-]*,[0-9-]*,[0-9]*,[0-9-]*\).*/\1/p'
}

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

# capturar: la foto de la pantalla entera y, al lado, la ultima linea de la demo (cuadro, celda, nivel, quest)
capturar() {
    local nombre="$1" md5 lin
    for intento in 1 2 3; do
        spectacle -b -n -f -o "$OUT/$nombre.png" >/dev/null 2>&1
        esperar_quieto "$OUT/$nombre.png"
        md5="$(md5sum "$OUT/$nombre.png" 2>/dev/null | cut -d' ' -f1)"
        [ -z "$ultimo_md5" ] && break
        [ "$md5" != "$ultimo_md5" ] && break
        sleep 0.7
    done
    ultimo_md5="$md5"
    lin="$(grep -a 'DEMO140: FPS' "$OUT/flycast.log" 2>/dev/null | tr -d '\r' | tail -1)"
    echo "  $nombre.png md5=${md5:0:8} ${lin:-sin-linea-aun}"
}

echo "== teclas: $GUION"
( PAUSA="$PAUSA" "$PYTHON" "$HERE/teclas-virtuales.py" guion $GUION ) > "$OUT/teclas.log" 2>&1 &
TECLAS_PID=$!

echo "== capturas cada ${INTERVALO}s durante ${SEGUNDOS}s"
N=$(( SEGUNDOS / INTERVALO ))
for n in $(seq -w 1 "$N"); do
    vivo || { echo "  el emulador se murio antes de la captura $n"; break; }
    capturar "c$n"
    sleep "$INTERVALO"
done

wait "$TECLAS_PID" 2>/dev/null
vivo && capturar "c99-final"

kill -TERM -"$PID" 2>/dev/null || kill -TERM "$PID" 2>/dev/null
pkill -f "$PATRON" 2>/dev/null
sleep 1
kill -KILL -"$PID" 2>/dev/null
pkill -9 -f "$PATRON" 2>/dev/null
qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.unregisterScript "regla-$TOKEN" >/dev/null 2>&1
rm -f "$REGLA_JS"

echo "== DC-002.140: EL ESCENARIO Y LAS TECLAS"
grep -a 'DEMO140: ARRANCA\|DEMO140: TECLAS\|DEMO140: ESCENARIO\|DEMO140: AKARA\|DEMO140: BICHO' "$OUT/flycast.log" | tr -d '\r'
echo "== DC-002.140: EL RITMO (los cuadros por segundo de la corrida jugable)"
grep -a 'DEMO140: FPS' "$OUT/flycast.log" | tr -d '\r'
echo "== DC-002.140: LA QUEST Y LAS BAJAS"
grep -a 'DEMO140: HABLA\|DEMO140: QUEST\|DEMO140: MATA\|DEMO140: COBRA\|DEMO140: AREA' "$OUT/flycast.log" | tr -d '\r'
echo "== DC-002.140: LA SALIDA DEL PUEBLO"
grep -a 'DEMO140: CRUZA\|DEMO140: SALIDA' "$OUT/flycast.log" | tr -d '\r'
echo "== el panel de los NPCs (DC-002.127) y el combate (DC-002.122)"
grep -a 'JUEGO127: DIALOGO\|JUEGO122: GOLPE-EMPIEZA\|JUEGO122: GOLPE-UNO' "$OUT/flycast.log" | tr -d '\r' | head -20
echo "== los tramos caminados (la cruceta del jugador)"
grep -a 'JUEGO121: TRAMO ' "$OUT/flycast.log" | tr -d '\r' | head -10
echo "== el cierre de la demo"
grep -a 'DEMO140: CIERRE' "$OUT/flycast.log" | tr -d '\r' | tail -3
echo "== lo que se rompio, si se rompio"
grep -aE 'fatal error|Fatal:|OOM|exception|Out of memory' "$OUT/flycast.log" | tr -d '\r' | tail -6
echo "== capturas"
ls -la "$OUT"/c*.png 2>/dev/null | awk '{print "   "$5" bytes  "$9}'
echo "== la ventana del cliente (para recortar la captura)"
echo "   ${LIN}"
exit 0
