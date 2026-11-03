#!/bin/bash
# DC-002.138 -- EL MVP JUGABLE en Flycast: una corrida, las nueve acciones, y las capturas de los momentos clave.
#
# QUE HACE Y POR QUE ASI
# ----------------------
#  1. Arranca Flycast con el GDI del build, la ventana IDENTIFICABLE ("Flycast - dc138") y keepAbove, en 640x480
#     para que la captura sea 1:1 con el cuadro logico del POC. Es el mismo arranque de run-flycast-131.sh y por
#     los mismos motivos medidos: `spectacle -f` captura la pantalla entera (2560x1080) y sin keepAbove cualquier
#     ventana tapa al emulador.
#  2. Espera `JUEGO121: PRIMER-CUADRO`: antes de esa marca la pantalla no tiene el cuadro del POC.
#  3. NO manda un solo byte por el teclado virtual: el guion lo corre el POC (los bits de la cruceta los empuja su
#     shim, que es el mismo camino del maple). Lo que el script hace es CAPTURAR cada INTERVALO.
#  4. Espera el fin del guion (`JUEGO138: MVP GUION-TERMINADO`), despues los tres cuadros del cierre del POC
#     guion del MVP son ~45 cuadros de guion, tres del cierre y cada cuadro de este POC es ~1,2-1,5 s.
#  5. Vuelca las lineas de la tarjeta: el escenario, las etapas, las nueve marcas, la tabla y el mundo al final.
#
# Uso:  run-mvp-138.sh <disc.gdi> [segundos] [dir_salida] [x y]
#       CAPTURAS=200 INTERVALO=1 ESPERA_ARRANQUE=900 bash scripts/console/run-mvp-138.sh ...
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

IMG="${1:?uso: run-mvp-138.sh <disc.gdi> [segundos] [dir_salida] [x y]}"
SEGUNDOS="${2:-90}"
OUT="${3:-/tmp/capturas-mvp138}"
POS="${4:-900 120}"
[ -n "${5:-}" ] && POS="$4 $5"

VENTANA="${VENTANA:-640x480}"
VW="${VENTANA%x*}"
VH="${VENTANA#*x}"
FLY="${FLYCAST:-$HOME/dreamcast/dl/flycast.AppImage}"
DISP="${DISPLAY:-:0}"
TOKEN="dc138"
BARRA_TITULO="${BARRA_TITULO:-28}"
PX="${POS%% *}"
PY="${POS##* }"
ESPERA_ARRANQUE="${ESPERA_ARRANQUE:-600}"
INTERVALO="${INTERVALO:-1}"
PAUSA_CAPTURA="${PAUSA_CAPTURA:-0.4}"

mkdir -p "$OUT"
rm -f "$OUT"/c*.png 2>/dev/null

DISC_DIR="$(cd "$(dirname "$IMG")" && pwd)"
GDI="$DISC_DIR/$TOKEN.gdi"
cp -f "$IMG" "$GDI"

# ---- la regla de KWin: pone la ventana donde el script dice, con keepAbove, y devuelve su geometria ----
REGLA_JS="${TMPDIR:-/tmp}/kwin-regla-$TOKEN-$$.js"
cat > "$REGLA_JS" <<'JSFIN'
var PX = __PX__, PY = __PY__, objetivo = null;
function ubicar(w) {
    var cap = "", cls = "";
    try { cap = (w.caption || "").toString(); } catch (e) { cap = ""; }
    try { cls = (w.resourceClass || "").toString(); } catch (e) { cls = ""; }
    if (cls.toLowerCase().indexOf("flycast") < 0) { return; }
    if (cap.toLowerCase().indexOf("dc138") >= 0) {
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

echo "== Flycast: $GDI (ventana $VENTANA en $PX,$PY + keepAbove; consola serie si)"
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
    grep -aE 'JUEGO121|JUEGO138|Fatal|EXCEPTION|fatal error' "$OUT/flycast.log" | tail -25
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

# capturar: una foto de la pantalla entera (spectacle) y, al lado, la linea del POC de ese instante (el cuadro,
# la celda del heroe, el crc32 y la etapa del MVP): asi cada captura queda atada al cuadro que la produjo.
capturar() {
    local nombre="$1" lin cli geo md5 etapa celda crc
    lin="$(regla)"
    cli="$(echo "$lin" | sed -n 's/.*cli=\([0-9-]*,[0-9-]*,[0-9]*,[0-9-]*\).*/\1/p')"
    if [ -z "$cli" ]; then
        geo="$(echo "$lin" | sed -n 's/.*geo=\([0-9-]*\),\([0-9-]*\),\([0-9]*\),\([0-9-]*\).*/\1,\2,\3,\4/p')"
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
    etapa="$(grep -a 'JUEGO138: MVP ETAPA' "$OUT/flycast.log" 2>/dev/null | tr -d '\r' | tail -1 |
        sed -n 's/.*a= \([a-z-]*\) cuadro= \([0-9]*\).*/etapa=\1 cuadro=\2/p')"
    celda="$(grep -a 'JUEGO121: CONTROL cuadro' "$OUT/flycast.log" 2>/dev/null | tr -d '\r' | tail -1 |
        sed -n 's/.*celda= \([0-9.-]*\) \([0-9.-]*\).*/celda=\1,\2/p')"
    crc="$(grep -a 'JUEGO121: CUADRO [0-9]* CON-JUGADOR' "$OUT/flycast.log" 2>/dev/null | tr -d '\r' | tail -1 |
        sed -n 's/.*crc32= \([0-9-]*\).*/crc=\1/p')"
    echo "  $nombre.png md5=${md5:0:8} ${etapa:-etapa=?} ${celda:-} ${crc:-}"
    [ -n "$cli" ] && echo "GEO captura=$nombre cliente=$cli ${etapa:-} ${celda:-} ${crc:-} ms=$(date +%s%3N)" |
        tee -a "$OUT/geo.txt"
}

echo "== capturas: una QUIETO y despues una cada ${INTERVALO}s mientras corre el guion"
vivo && capturar "c000-quieto"

LIMITE=$(( SEGUNDOS / INTERVALO ))
[ "$LIMITE" -lt 1 ] && LIMITE=1

for n in $(seq -w 1 "$LIMITE"); do
    vivo || { echo "  el emulador se murio antes de la captura $n"; break; }
    grep -qa 'JUEGO138: MVP GUION-TERMINADO' "$OUT/flycast.log" 2>/dev/null && { echo "  el guion del MVP ya termino (la tabla sale en el cierre)"; break; }
    capturar "c$n"
    sleep "$INTERVALO"
done

# EL CIERRE DEL MVP. La tabla la imprime el guion en cuanto termina la novena accion, pero la marca 9 (el mundo y
# el HUD DESPUES de todo) se mide en los tres cuadros del cierre del POC (`cuadros+1..+3`: el origen, la
# repetibilidad y la vuelta). El guion adelanta el reloj (`cuadro = cuadros-1`) para que esos tres cuadros corran
# enseguida: hay que ESPERAR `JUEGO121: VUELTA AL FINAL`, que es el ultimo, y recien despues matar el emulador.
ESPERA_CIERRE="${ESPERA_CIERRE:-420}"
T=0
CIERRE_VISTO=0
while [ "$T" -lt "$ESPERA_CIERRE" ]; do
    grep -qa 'JUEGO121: VUELTA AL FINAL' "$OUT/flycast.log" 2>/dev/null && { CIERRE_VISTO=1; echo "  el cierre del POC salio a los ${T}s de esperar"; break; }
    vivo || { echo "  el emulador se murio esperando el cierre"; break; }
    if [ $(( T % 6 )) -eq 0 ] && [ "$T" -gt 0 ]; then capturar "c9$(printf '%02d' $(( T / 6 )))-cierre"; fi
    sleep 3
    T=$(( T + 3 ))
done
[ "$CIERRE_VISTO" = "0" ] && echo "  AVISO: no vi 'JUEGO121: VUELTA AL FINAL' en ${ESPERA_CIERRE}s"

vivo && capturar "c999-final"
sleep 1

kill -TERM -"$PID" 2>/dev/null || kill -TERM "$PID" 2>/dev/null
pkill -f "$PATRON" 2>/dev/null
sleep 1
kill -KILL -"$PID" 2>/dev/null
pkill -9 -f "$PATRON" 2>/dev/null
qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.unregisterScript "regla-$TOKEN" >/dev/null 2>&1
qdbus6 org.kde.KWin /Scripting/Scripting org.kde.kwin.Scripting.unloadScript "regla-$TOKEN" >/dev/null 2>&1
rm -f "$REGLA_JS"

echo "== DC-002.138: EL ESCENARIO (donde corrio el MVP y por que ahi)"
grep -a 'JUEGO138: MVP ARRANCA\|JUEGO138: MVP ESCENARIO' "$OUT/flycast.log" | tr -d '\r'
echo "== DC-002.138: LAS ETAPAS (lo que el guion hizo, cuadro por cuadro)"
grep -a 'JUEGO138: MVP ETAPA\|JUEGO138: MVP ARMA\|JUEGO138: MVP AVISO\|JUEGO138: MVP CIERRA\|JUEGO138: MVP GUION-TERMINADO' "$OUT/flycast.log" | tr -d '\r'
echo "== DC-002.138: LAS NUEVE MARCAS"
grep -a 'JUEGO138: MVP [0-9] ' "$OUT/flycast.log" | tr -d '\r'
echo "== DC-002.138: LA TABLA DEL MVP"
grep -a 'JUEGO138: MVP TABLA\|JUEGO138: MVP CRITERIO' "$OUT/flycast.log" | tr -d '\r'
echo "== el camino: los tramos de la cruceta (DC-002.113) y el control"
grep -a 'JUEGO121: TRAMO ' "$OUT/flycast.log" | tr -d '\r' | head -12
echo "== el combate y la muerte (DC-002.122/.126)"
grep -a 'JUEGO122: GOLPE-EMPIEZA\|JUEGO122: GOLPE-UNO\|JUEGO126: MUERTE EMPIEZA\|JUEGO126: MUERTE CUADRO' "$OUT/flycast.log" | tr -d '\r' | head -14
echo "== el drop y la recogida (DC-002.123)"
grep -a 'JUEGO123: DROP SUELTA\|JUEGO123: ITEM RECOGIDO' "$OUT/flycast.log" | tr -d '\r'
echo "== el inventario y el alijo (DC-002.124)"
grep -a 'JUEGO124: INV ABRE\|JUEGO124: INV ENTRA\|JUEGO124: INV CIERRA\|JUEGO124: INV CUADRO\|JUEGO124: ALIJO ABRE\|JUEGO124: ALIJO GUARDA\|JUEGO124: ALIJO DEVUELVE\|JUEGO124: ALIJO CIERRA' "$OUT/flycast.log" | tr -d '\r'
echo "== los NPCs y la quest (DC-002.127/.128)"
grep -a 'JUEGO127: DIALOGO ABRE\|JUEGO128: QUEST HABLA\|JUEGO128: QUEST ACEPTA\|JUEGO128: QUEST AREA\|JUEGO128: QUEST MATA\|JUEGO128: QUEST LIMPIA\|JUEGO128: QUEST CUEVA BICHOS' "$OUT/flycast.log" | tr -d '\r'
echo "== el mundo al final (la repetibilidad del cierre y el mundo solo)"
grep -a 'JUEGO121: REPETIBILIDAD\|JUEGO121: CUADRO [0-9]* SOLO-MAPA\|JUEGO121: FIN pico' "$OUT/flycast.log" | tr -d '\r' | tail -6
echo "== lo que se rompio, si se rompio"
grep -aE 'fatal error|Fatal:|OOM|exception' "$OUT/flycast.log" | tr -d '\r' | tail -5
echo "== capturas"
ls -la "$OUT"/c*.png 2>/dev/null | awk '{print "   "$5" bytes  "$9}'
exit 0
