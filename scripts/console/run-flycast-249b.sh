#!/bin/bash
# DC-002.49b -- correr el POC CON EL GIRO en Flycast y sacar las capturas que miden la DIRECCION.
#
# QUE HACE Y POR QUE ASI
# ----------------------
#  1. Arranca Flycast con el GDI del build, con la ventana IDENTIFICABLE ("Flycast - dc249b") y keepAbove
#     (lo mismo que run-flycast-249.sh, por los mismos motivos medidos: `spectacle -f` captura la pantalla
#     entera y sin keepAbove cualquier ventana tapa al emulador).
#  2. Espera la marca `HITO238: dibujando` (antes de eso la pantalla no tiene el personaje) y las marcas del
#     giro (`HITO249B: DIRECCIONES`, `HITO249: ANIM`).
#  3. Manda la CRUCETA con el teclado virtual (un solo aparato para toda la secuencia: medido, dos aparatos
#     seguidos NO llegan) y captura CADA INTERVALO mientras dura el guion.
#  4. NO decide que direccion muestra cada captura: eso lo dice el LOG (`HITO249B: GIRO` + el crc32 de
#     `HITO249: ANIM` cruzado con la tabla del host). La captura se clasifica DESPUES, contra las referencias
#     del host compuestas en el MISMO ancla, y los dos canales tienen que coincidir. Si la captura se
#     etiquetara por el reloj, un retraso del emulador pondria la etiqueta equivocada.
#
# Uso:  run-flycast-249b.sh <disc.gdi> <dir_salida> [x y]
#       CAPTURAS=10 INTERVALO=1.5 SEGS_GUION=12 TECLAS=0 -> sin cruceta (el control del mutante quieto)
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMG="${1:?uso: run-flycast-249b.sh <disc.gdi> <dir_salida> [x y]}"
OUT="${2:-/tmp/dc249b-run}"
POS="${3:-1400 120}"
[ -n "${4:-}" ] && POS="$3 $4"
# DC-002.64: el tamano de la ventana del emulador NO es un detalle: la pantalla logica del POC es 640x480 y
# con la ventana en 320x240 (el default de esta maquina, leido de ~/.config/flycast/emu.cfg) el sprite de 26x77
# se reduce a 13x38 y las direcciones que solo se diferencian en el detalle fino (00 / 08 / 09) dejan de
# distinguirse: medido, el margen del canal de pixeles se cae a 0,5-1,5 pts. Con VENTANA=640x480 la captura es
# 1:1 con la pantalla logica y el margen pasa de 4 pts en TODAS las capturas.
VENTANA="${VENTANA:-320x240}"
VW="${VENTANA%x*}"
VH="${VENTANA#*x}"
FLY="${FLYCAST:-$HOME/dreamcast/dl/flycast.AppImage}"
DISP="${DISPLAY:-:0}"
TOKEN="dc249b"
BARRA_TITULO="${BARRA_TITULO:-28}"
PX="${POS%% *}"; PY="${POS##* }"
PYTHON="${DC_PYTHON:-python3}"
ESPERA_ARRANQUE="${ESPERA_ARRANQUE:-300}"
CAPTURAS="${CAPTURAS:-10}"
INTERVALO="${INTERVALO:-1.5}"
GUION="${GUION:-derecha 0.8 arriba 0.8 izquierda 0.8 abajo 0.8}"

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
    if (cap.toLowerCase().indexOf("dc249b") >= 0) {
        var g = w.frameGeometry;
        if (g.x !== PX || g.y !== PY - 28) {
            w.frameGeometry = { x: PX, y: PY - 28, width: g.width, height: g.height };
        }
        w.keepAbove = true;
        objetivo = w;
    }
    print("DC249B-CAP|" + cap + "|" + cls + "|pid=" + w.pid + "|geo=" + w.frameGeometry.x + "," +
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
    try { workspace.activeWindow = objetivo; print("DC249B-CAP|foco=" + objetivo.caption); } catch (e) { print("DC249B-CAP|foco=no-pude " + e); }
}
if (objetivo) {
    var gancho = setInterval(function () {
        try { objetivo.keepAbove = true; workspace.activeWindow = objetivo; } catch (e) { }
    }, 300);
}
print("DC249B-CAP|fin");
JSFIN
sed -i "s/__PX__/$PX/; s/__PY__/$PY/" "$REGLA_JS"

regla() {
    qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.unloadScript "regla-$TOKEN" >/dev/null 2>&1
    qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.loadScript "$REGLA_JS" "regla-$TOKEN" >/dev/null 2>&1
    qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.start >/dev/null 2>&1
    sleep 1
    journalctl --user --since "-6 s" 2>/dev/null | grep -a 'DC249B-CAP|' | tr -d '\r' |
        grep -i "flycast - $TOKEN" | grep 'geo=' | sed 's/.*DC249B-CAP|/ventana: /' | tail -1
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
    grep -qa 'HITO238: dibujando' "$OUT/flycast.log" 2>/dev/null && { echo "  dibujando a los ${T}s"; break; }
    if vivo; then MUERTO=0; else
        MUERTO=$(( MUERTO + 1 ))
        if [ "$MUERTO" -ge 3 ]; then
            echo "  el emulador se murio antes de dibujar"; tail -12 "$OUT/flycast.log"; exit 1
        fi
    fi
    sleep 1
    T=$(( T + 1 ))
done

if ! grep -qa 'HITO238: dibujando' "$OUT/flycast.log" 2>/dev/null; then
    echo "  NO llego a dibujar en ${ESPERA_ARRANQUE}s"
    grep -aE 'HITO249B|HITO249|HITO250|Fatal|EXCEPTION|fatal error' "$OUT/flycast.log" | tail -25
    pkill -f "$PATRON" 2>/dev/null
    exit 1
fi

echo "-- las marcas del giro que ya salieron:"
grep -a 'HITO249B' "$OUT/flycast.log" | tr -d '\r' | head -12

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
    anim="$(grep -a 'HITO249: ANIM' "$OUT/flycast.log" 2>/dev/null | tr -d '\r' | tail -1 |
        sed -n 's/.*ANIM modo \([A-Z0-9]*\) cuadro \([0-9]*\) de \([0-9]*\) caja \([0-9]*\) \([0-9]*\).*crc32 \([0-9]*\).*/modo=\1 cuadro=\2 de=\3 caja=\4x\5 crc32=\6/p')"
    pos="$(grep -a 'HITO238: POS cuadro' "$OUT/flycast.log" 2>/dev/null | tr -d '\r' | tail -1 |
        sed -n 's/.* celda \([0-9-]*\) \([0-9-]*\).*/celda=\1,\2/p')"
    echo "  $nombre.png md5=${md5:0:8} ${anim:-anim=?} ${pos:-pos=?}"
    [ -n "$cli" ] && echo "GEO captura=$nombre cliente=$cli ${anim:-} ${pos:-} ms=$(date +%s%3N)" | tee -a "$OUT/geo.txt"
}

echo "== capturas: una QUIETO y despues una cada ${INTERVALO}s mientras corre el guion"
vivo && capturar "c00-quieto"

if [ "${TECLAS:-1}" = "1" ]; then
    ( PAUSA="${PAUSA:-1.2}" "$PYTHON" "$HERE/teclas-virtuales.py" guion $GUION ) > "$OUT/teclas.log" 2>&1 &
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
echo "== las direcciones que el POC dice que compuso"
grep -a 'HITO249B: DIRECCIONES\|HITO249B: DIR ' "$OUT/flycast.log" | tr -d '\r' | head -20
echo "== los giros (una linea por CAMBIO de direccion)"
grep -a 'HITO249B: GIRO' "$OUT/flycast.log" | tr -d '\r' | head -20
echo "== memoria despues de cada ciclo (la marca de siempre)"
grep -a 'HITO250: MEMORIA' "$OUT/flycast.log" | tr -d '\r' | tail -6
echo "== capturas"; ls -la "$OUT"/c*.png 2>/dev/null | awk '{print "   "$5" bytes  "$9}'
echo "== teclas"; tail -6 "$OUT/teclas.log" 2>/dev/null
exit 0
