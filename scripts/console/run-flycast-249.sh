#!/bin/bash
# DC-002.49 -- correr el POC del personaje ANIMADO en Flycast y sacar las capturas que miden la animacion.
#
# QUE HACE, Y POR QUE ASI
# -----------------------
#  1. Arranca Flycast con el disco del POC y la ventana IDENTIFICABLE ("Flycast - dc249"), en una posicion
#     conocida y keepAbove (lo mismo que run-flycast-238.sh y por los mismos motivos medidos: `spectacle -f`
#     captura la pantalla entera y sin keepAbove cualquier ventana tapa al emulador).
#  2. ESPERA a que el POC avise que esta dibujando Y que el CICLO este corriendo ("HITO249: MODO TN ...").
#     Antes de eso la pantalla no tiene el personaje animado y la captura no mide nada.
#  3. Saca N capturas separadas en el tiempo, CON EL PERSONAJE QUIETO (no se manda ninguna tecla). El ciclo de
#     TN dura 2048 ms y cada captura tarda ~1 s, asi que las capturas caen en fases distintas del ciclo sin
#     necesidad de sincronizar: lo que las separa es el DIBUJO, no el reloj.
#  4. Deja en $OUT, por cada captura: el PNG, la geometria del area cliente del emulador (la linea GEO) y las
#     ultimas marcas del POC (ANIM: modo y cuadro; POS: posicion). El medidor (medir-anim-249.py) recorta la
#     caja del personaje con esa geometria y compara contra los cuadros del ciclo del host.
#  5. Con TECLAS=1 ademas manda la flecha derecha sostenida y captura EN MEDIO del movimiento: es la
#     evidencia del estado CAMINANDO (modo TW, 8 cuadros).
#
# Uso:  run-flycast-249.sh <disc.gdi> <dir_salida> [x y]
#       CAPTURAS=8 INTERVALO=1.6 SEGS_EXTRA=6
#       TECLAS=1 -> manda la flecha derecha sostenida 3 s y captura durante el movimiento
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMG="${1:?uso: run-flycast-249.sh <disc.gdi> <dir_salida> [x y]}"
OUT="${2:-$HOME/.hermes/cache/scratch/dc249-run}"
POS="${3:-1400 120}"
[ -n "${4:-}" ] && POS="$3 $4"
FLY="${FLYCAST:-$HOME/dreamcast/dl/flycast.AppImage}"
DISP="${DISPLAY:-:0}"
TOKEN="dc249"
BARRA_TITULO="${BARRA_TITULO:-28}"
PX="${POS%% *}"; PY="${POS##* }"
PYTHON="${DC_PYTHON:-python3}"
ESPERA_ARRANQUE="${ESPERA_ARRANQUE:-240}"
CAPTURAS="${CAPTURAS:-8}"
INTERVALO="${INTERVALO:-1.6}"

mkdir -p "$OUT"
rm -f "$OUT"/f*.png

DISC_DIR="$(cd "$(dirname "$IMG")" && pwd)"
GDI="$DISC_DIR/$TOKEN.gdi"
cp -f "$IMG" "$GDI"

REGLA_JS="${TMPDIR:-/tmp}/kwin-regla-$TOKEN.js"
cat > "$REGLA_JS" <<'JSFIN'
var PX = __PX__, PY = __PY__, objetivo = null;
function ubicar(w) {
    var cap = "", cls = "";
    try { cap = (w.caption || "").toString(); } catch (e) { cap = ""; }
    try { cls = (w.resourceClass || "").toString(); } catch (e) { cls = ""; }
    if (cls.toLowerCase().indexOf("flycast") < 0) { return; }
    if (cap.toLowerCase().indexOf("dc249") >= 0) {
        var g = w.frameGeometry;
        if (g.x !== PX || g.y !== PY - 28) {
            w.frameGeometry = { x: PX, y: PY - 28, width: g.width, height: g.height };
        }
        w.keepAbove = true;
        objetivo = w;
    }
    print("DC249-CAP|" + cap + "|" + cls + "|pid=" + w.pid + "|geo=" + w.frameGeometry.x + "," +
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

// EL FOCO: las teclas del teclado virtual las recibe la VENTANA CON EL FOCO (medido en DC-002.38: sin foco
// las flechas llegan a medias). Y el gancho cada 300 ms la vuelve a pedir: en este host puede haber otra
// corrida del mismo emulador y, si su ventana se pone encima, `spectacle -f` devuelve SU cuadro.
if (objetivo) {
    try {
        workspace.activeWindow = objetivo;
        print("DC249-CAP|foco=" + objetivo.caption);
    } catch (e) {
        print("DC249-CAP|foco=no-pude " + e);
    }
}
if (objetivo) {
    var gancho = setInterval(function () {
        try {
            objetivo.keepAbove = true;
            workspace.activeWindow = objetivo;
        } catch (e) { }
    }, 300);
}
print("DC249-CAP|fin");
JSFIN
sed -i "s/__PX__/$PX/; s/__PY__/$PY/" "$REGLA_JS"

regla() {
    qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.unloadScript "regla-$TOKEN" >/dev/null 2>&1
    qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.loadScript "$REGLA_JS" "regla-$TOKEN" >/dev/null 2>&1
    qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.start >/dev/null 2>&1
    sleep 1
    journalctl --user --since "-6 s" 2>/dev/null | grep -a 'DC249-CAP|' | tr -d '\r' |
        grep -i "flycast - $TOKEN" | grep 'geo=' | sed 's/.*DC249-CAP|/ventana: /' | tail -1
}

echo "== Flycast: $GDI (ventana en $PX,$PY + keepAbove; consola serie si)"
setsid env DISPLAY="$DISP" "$FLY" -config "window:fullscreen=no" \
    -config "window:left=$PX" -config "window:top=$PY" \
    -config "Debug:SerialConsoleEnabled=true" "$GDI" >"$OUT/flycast.log" 2>&1 &
PID=$!
echo "pid=$PID"

PATRON='dc249[.]gdi'
vivo() { pgrep -f "$PATRON" >/dev/null 2>&1; }

# ---- 1. esperar a que el POC dibuje Y a que el ciclo este corriendo
sleep 5
T=5
MUERTO=0
while [ "$T" -lt "$ESPERA_ARRANQUE" ]; do
    if grep -qa 'HITO238: dibujando' "$OUT/flycast.log" 2>/dev/null; then
        echo "  el POC esta dibujando (a los ${T}s)"
        break
    fi
    if vivo; then
        MUERTO=0
    else
        MUERTO=$(( MUERTO + 1 ))
        if [ "$MUERTO" -ge 3 ]; then
            echo "  el emulador se murio antes de dibujar (3 comprobaciones sin proceso)"
            tail -8 "$OUT/flycast.log"
            exit 1
        fi
    fi
    sleep 1
    T=$(( T + 1 ))
done

if ! grep -qa 'HITO238: dibujando' "$OUT/flycast.log" 2>/dev/null; then
    echo "  el POC NO llego a dibujar en ${ESPERA_ARRANQUE}s"
    grep -aE 'HITO249|HITO238|Fatal|EXCEPTION|arena' "$OUT/flycast.log" | tail -20
    kill -TERM -"$PID" 2>/dev/null; kill -KILL -"$PID" 2>/dev/null
    exit 1
fi

# el ciclo tiene que estar corriendo: la primera marca del modo (y el cuadro 0 dibujado)
CUADROS_CICLO="${CUADROS_CICLO:-60}"
i=0
while [ "$i" -lt "$CUADROS_CICLO" ]; do
    grep -qa 'HITO249: ANIM' "$OUT/flycast.log" 2>/dev/null && break
    sleep 0.5
    i=$(( i + 1 ))
done
if ! grep -qa 'HITO249: ANIM' "$OUT/flycast.log" 2>/dev/null; then
    echo "  el POC NO imprimio ninguna marca de ANIM en ${CUADROS_CICLO} medios segundos"
    grep -aE 'HITO249|HITO238' "$OUT/flycast.log" | tail -20
    pkill -f "$PATRON" 2>/dev/null
    exit 1
fi

# ---- 1b. la ventana del emulador
LIN="$(regla)"
echo "  ventana del emulador: ${LIN:-NO la encuentro}"
if ! echo "$LIN" | grep -qE 'cli=|geo='; then
    echo "  NO encuentro la ventana del emulador: ni las capturas ni las teclas valdrian"
    journalctl --user --since "-6 s" 2>/dev/null | grep -a 'DC249-CAP|' | tr -d '\r' | tail -5
    pkill -f "$PATRON" 2>/dev/null
    exit 1
fi
sleep 1

ultimo_md5=""

# DC-002.49b: los md5 de TODAS las capturas ya tomadas en esta corrida. La captura se acepta solo si NO
# repite ninguna anterior: la animacion del personaje quieto tiene 16 cuadros y el plan de captura puede caer
# dos veces en el mismo (medido en este host: con INTERVALO=1.6 los pares f1/f6 y f2/f7 salian IDENTICOS y el
# control C6 del corpus --que exige que TODOS los pares cambien-- se caia por el plan, no por la animacion).
# Con el mutante de ciclo quieto el reintento se agota y la captura se acepta igual: ahi repetir es el
# resultado que se mide.
md5s_vistos=" "

# DC-002.49b: la huella se calcula sobre el AREA CLIENTE del emulador (scripts/console/huella-zona.py), no
# sobre la pantalla entera: el md5 del PNG completo cambia por el reloj de la barra aunque el cuadro del juego
# sea el mismo (medido: f5 y f8 con md5 distinto y caja identica pixel a pixel). Sin esto el corpus de
# DC-002.49 se cae en C6 --que exige que TODOS los pares de capturas cambien-- por el plan de captura.
huellas_vistas=" "
esperar_quieto() {
    local f="$1" a=-1 b=-1 i=0
    while [ "$i" -lt 15 ]; do
        b="$(stat -c %s "$f" 2>/dev/null || echo -1)"
        if [ "$b" = "$a" ] && [ "$b" != "-1" ] && [ "$b" != "0" ]; then
            return 0
        fi
        a="$b"
        sleep 0.2
        i=$(( i + 1 ))
    done
    return 1
}

capturar() {
    local nombre="$1"
    local lin geo cli md5 anim pos
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

    for intento in 1 2 3 4 5 6; do
        spectacle -b -n -f -o "$OUT/$nombre.png" >/dev/null 2>&1
        esperar_quieto "$OUT/$nombre.png"
        md5="$(md5sum "$OUT/$nombre.png" 2>/dev/null | cut -d' ' -f1)"
        if [ -n "$cli" ] && [ -f "$HERE/huella-zona.py" ]; then
            huella="$("$PYTHON" "$HERE/huella-zona.py" "$OUT/$nombre.png" "$cli" 2>/dev/null)"
        fi
        [ -n "${huella:-}" ] || huella="$md5"
        case "$huellas_vistas" in
            *" $huella "*) ;;   # la zona del emulador repite una captura anterior: se vuelve a sacar
            *) break ;;
        esac
        echo "  captura $nombre SIN-CAMBIO (la zona del emulador repite una anterior), reintento $intento"
        sleep 0.7
    done
    huellas_vistas="$huellas_vistas$huella "
    md5s_vistos="$md5s_vistos$md5 "
    ultimo_md5="$md5"

    # las dos marcas del POC que enmarcan la captura: la ultima de cada tipo
    anim="$(grep -a 'HITO249: ANIM' "$OUT/flycast.log" 2>/dev/null | tr -d '\r' | tail -1 |
        sed -n 's/.*ANIM modo \([A-Z0-9]*\) cuadro \([0-9]*\) de \([0-9]*\).*/modo=\1 cuadro=\2 de=\3/p')"
    pos="$(grep -a 'HITO238: POS cuadro' "$OUT/flycast.log" 2>/dev/null | tr -d '\r' | tail -1 |
        sed -n 's/.* pantalla \([0-9-]*\) \([0-9-]*\) caminado.*/ancla=\1,\2/p')"

    echo "  captura $nombre.png md5=${md5:0:8} huella=${huella:0:8} ${anim:-anim=?} ${pos:-pos=?}"
    # DC-002.49b: la linea GEO se guarda TAMBIEN en geo.txt. El area cliente del emulador no la puede adivinar
    # el corpus (medido: la pantalla de este host es 3584x1080 y la ventana mide 320x240), asi que el dato
    # tiene que viajar con la corrida en vez de quedar solo en la salida del script.
    [ -n "$cli" ] && echo "GEO captura=$nombre cliente=$cli ${anim:-} ${pos:-} ms=$(date +%s%3N)" | tee -a "$OUT/geo.txt"
}

# ---- 2. las capturas CON EL PERSONAJE QUIETO (el criterio 1 y el 3 se miden asi)
echo "== capturas con el personaje QUIETO ($CAPTURAS cada ${INTERVALO}s)"
for n in $(seq -w 1 "$CAPTURAS"); do
    if vivo; then
        capturar "f$n-quieto"
    else
        echo "  el emulador se murio antes de la captura $n"
        break
    fi
    sleep "$INTERVALO"
done

# ---- 3. con TECLAS=1: caminar y capturar EN MEDIO del movimiento (el estado TW)
if [ "${TECLAS:-0}" = "1" ]; then
    echo "== la flecha derecha sostenida: capturas EN MEDIO del movimiento"
    (
        PAUSA=1.0 "$PYTHON" "$HERE/teclas-virtuales.py" guion derecha 4.0
    ) > "$OUT/teclas.log" 2>&1 &
    TECLAS_PID=$!

    # esperar la marca de la pulsacion y capturar mientras la tecla sigue apretada
    p0="$(grep -ac 'HITO238: CONTROL botones 128 ejes' "$OUT/flycast.log" 2>/dev/null || true)"
    i=0
    while [ "$i" -lt 120 ]; do
        n="$(grep -ac 'HITO238: CONTROL botones 128 ejes' "$OUT/flycast.log" 2>/dev/null || true)"
        [ "${n:-0}" -gt "${p0:-0}" ] && break
        sleep 0.25
        i=$(( i + 1 ))
    done
    echo "  la pulsacion llego (o se esperaron 30 s): capturando con la tecla apretada"
    vivo && capturar "f90-caminando"
    vivo && capturar "f91-caminando"
    wait "$TECLAS_PID" 2>/dev/null
    sleep 2
    vivo && capturar "f95-quieto-otra-vez"
fi

sleep "${ESPERA_FINAL:-1}"
kill -TERM -"$PID" 2>/dev/null || kill -TERM "$PID" 2>/dev/null
pkill -f "$PATRON" 2>/dev/null
sleep 1
kill -KILL -"$PID" 2>/dev/null
pkill -9 -f "$PATRON" 2>/dev/null
qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.unloadScript "regla-$TOKEN" >/dev/null 2>&1

echo
echo "== consola serie: los modos y los cuadros que se dibujaron (HITO249: MODO / ANIM)"
grep -a 'HITO249: MODO\|HITO249: CICLO\|HITO249: CICLOS\|HITO249: COF DEL MODO' "$OUT/flycast.log" | tr -d '\r' | tail -20
echo "-- cuadros distintos que se dibujaron, por modo:"
grep -a 'HITO249: ANIM' "$OUT/flycast.log" | tr -d '\r' |
    sed -n 's/.*ANIM modo \([A-Z0-9]*\) cuadro \([0-9]*\) de.*/\1 \2/p' | sort -u | awk '{a[$1]=a[$1]" "$2; c[$1]++} END {for (m in a) print "   modo "m": "c[m]" cuadros distintos:"a[m]}'
echo "-- cambios de cuadro dibujados: $(grep -ac 'HITO249: ANIM' "$OUT/flycast.log")"
echo
echo "== pantalla"; ls -la "$OUT"/f*.png 2>/dev/null | awk '{print "   "$5" bytes  "$9}'
echo "== teclas"; cat "$OUT/teclas.log" 2>/dev/null | tail -5
exit 0
