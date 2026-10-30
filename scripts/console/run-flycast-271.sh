#!/bin/bash
# DC-002.71: correr la PRUEBA DE LA ENTRADA en Flycast, apretando el control de la Dreamcast DE VERDAD (por
# /dev/uinput, que es lo unico que ve el emulador en Wayland), y capturar la pantalla EN EL MOMENTO en que la
# tecla esta apretada.
#
# POR QUE ASI (lo medido en DC-002.38, no repetido a ciegas):
#   1. la ventana del emulador tiene que estar ARRIBA y CON EL FOCO: las teclas del teclado virtual las recibe
#      la ventana con el foco (medido: sin foco las pulsaciones llegan a medias o no llegan).
#   2. las teclas se mandan con UN SOLO aparato virtual para toda la secuencia (teclas-virtuales.py guion):
#      llamar al script una vez por tecla NO funciona, el emulador solo ve la primera.
#   3. la captura NO va por reloj: se espera la marca que imprime el PROPIO POC en la consola serie
#      ("HITO271: FLANCO bajada=...") y se fotografia mientras la tecla sigue apretada. Con el reloj, la
#      captura se cruza con el tramo siguiente.
#
# Uso:  scripts/console/run-flycast-271.sh <disc.gdi> <dir_salida>
#       SIN_TECLAS=1    corre sin mandar nada (linea de base: el POC no tiene que ver ningun boton)
#       AGUANTE=6       segundos que se sostiene cada tecla
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMG="${1:?uso: run-flycast-271.sh <disc.gdi> <dir_salida>}"
OUT="${2:-$HOME/.hermes/cache/scratch/dc271-run}"
FLY="${FLYCAST:-$HOME/dreamcast/dl/flycast.AppImage}"
DISP="${DISPLAY:-:0}"
TOKEN="dc271"
POS="${POS:-1400 120}"
PX="${POS%% *}"
PY="${POS##* }"
BARRA_TITULO="${BARRA_TITULO:-28}"
ESPERA_ARRANQUE="${ESPERA_ARRANQUE:-90}"
AGUANTE="${AGUANTE:-6}"
SIN_TECLAS="${SIN_TECLAS:-0}"

mkdir -p "$OUT"
rm -f "$OUT"/f*.png "$OUT"/guion.log

[ -f "$IMG" ] || { echo "no existe $IMG"; exit 1; }
[ -x "$FLY" ] || { echo "no existe/falta permiso: $FLY"; exit 1; }

DISC_DIR="$(cd "$(dirname "$IMG")" && pwd)"
GDI="$DISC_DIR/$TOKEN.gdi"
cp -f "$IMG" "$GDI"

# ------------------------------------------------------------------ la regla de KWin (posicion + foco)
REGLA_JS="${TMPDIR:-/tmp}/kwin-regla-$TOKEN.js"
cat > "$REGLA_JS" <<'JSFIN'
var PX = __PX__, PY = __PY__, objetivo = null;
function ubicar(w) {
    var cap = "", cls = "";
    try { cap = (w.caption || "").toString(); } catch (e) { cap = ""; }
    try { cls = (w.resourceClass || "").toString(); } catch (e) { cls = ""; }
    if (cls.toLowerCase().indexOf("flycast") < 0) { return; }
    if (cap.toLowerCase().indexOf("dc271") >= 0) {
        var g = w.frameGeometry;
        if (g.x !== PX || g.y !== PY - 28) {
            w.frameGeometry = { x: PX, y: PY - 28, width: g.width, height: g.height };
        }
        w.keepAbove = true;
        objetivo = w;
    }
    print("DC271-CAP|" + cap + "|" + cls + "|pid=" + w.pid + "|geo=" + w.frameGeometry.x + "," +
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
    try {
        workspace.activeWindow = objetivo;
        print("DC271-CAP|foco=" + objetivo.caption);
    } catch (e) { print("DC271-CAP|foco=no-pude " + e); }
    var gancho = setInterval(function () {
        try { objetivo.keepAbove = true; workspace.activeWindow = objetivo; } catch (e) { }
    }, 300);
}
print("DC271-CAP|fin");
JSFIN
sed -i "s/__PX__/$PX/; s/__PY__/$PY/" "$REGLA_JS"

regla() {
    qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.unloadScript "regla-$TOKEN" >/dev/null 2>&1
    qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.loadScript "$REGLA_JS" "regla-$TOKEN" >/dev/null 2>&1
    qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.start >/dev/null 2>&1
    sleep 1
    journalctl --user --since "-6 s" 2>/dev/null | grep -a 'DC271-CAP|' | tr -d '\r' |
        grep -i "flycast - $TOKEN" | grep 'geo=' | sed 's/.*DC271-CAP|/ventana: /' | tail -1
}

echo "== Flycast: $GDI (ventana $PX,$PY + keepAbove + foco; consola serie si)"
# stdbuf -o0: LA CONSOLA SERIE TIENE QUE SALIR EN EL MOMENTO. Medido en esta tarjeta: con la salida
# redirigida a un archivo, el emulador la bufferiza (bloques de ~4 KB), asi que las marcas del POC aparecian
# en el log recien cuando el buffer se llenaba -- y las esperas por marca se venciaan con las teclas YA
# apretadas (el POC las tenia: botones=128 en el cuadro 45, pero en el archivo todavia no estaba). Con
# stdbuf -o0 el log va en vivo (medido: 18 marcas y 4168 B a los 14 s) y la espera por marca sincroniza de
# verdad.
setsid stdbuf -o0 -e0 env DISPLAY="$DISP" "$FLY" -config "window:fullscreen=no" \
    -config "window:left=$PX" -config "window:top=$PY" \
    -config "Debug:SerialConsoleEnabled=true" "$GDI" >"$OUT/flycast.log" 2>&1 &
PID=$!
echo "pid=$PID"

PATRON='dc271[.]gdi'
vivo() { pgrep -f "$PATRON" >/dev/null 2>&1; }
cuenta() { local n; n="$(grep -ac "$1" "$OUT/flycast.log" 2>/dev/null || true)"; echo "${n:-0}"; }

# esperar_cuenta <patron> <cuenta_previa> <cuartos_de_segundo>
esperar_cuenta() {
    local patron="$1" previa="$2" limite="${3:-240}" i=0 n
    while [ "$i" -lt "$limite" ]; do
        n="$(cuenta "$patron")"
        [ "$n" -gt "$previa" ] && return 0
        sleep 0.25
        i=$(( i + 1 ))
    done
    return 1
}

capturar() {
    local nombre="$1"
    spectacle -b -n -f -o "$OUT/f$nombre.png" >/dev/null 2>&1
    echo "  captura $nombre ($(stat -c%s "$OUT/f$nombre.png" 2>/dev/null || echo 0) B)"
}

# ---- 1. esperar el primer cuadro del POC (antes no hay nada que capturar ni que apretar)
sleep 5
T=5
MUERTO=0
while [ "$T" -lt "$ESPERA_ARRANQUE" ]; do
    if grep -qa 'HITO271: PRIMER-CUADRO' "$OUT/flycast.log" 2>/dev/null; then
        echo "  el POC esta dibujando (a los ${T}s)"
        break
    fi
    if vivo; then MUERTO=0; else
        MUERTO=$(( MUERTO + 1 ))
        if [ "$MUERTO" -ge 3 ]; then
            echo "  el emulador se murio antes de dibujar"; tail -8 "$OUT/flycast.log"; exit 1
        fi
    fi
    sleep 1
    T=$(( T + 1 ))
done
if ! grep -qa 'HITO271: PRIMER-CUADRO' "$OUT/flycast.log" 2>/dev/null; then
    echo "  el POC NO llego a dibujar en ${ESPERA_ARRANQUE}s"
    grep -aE 'HITO271|Fatal|EXCEPTION|arena' "$OUT/flycast.log" | tail -20
    kill -TERM -"$PID" 2>/dev/null; kill -KILL -"$PID" 2>/dev/null
    exit 1
fi

# ---- 2. la ventana: arriba y con el foco (sin foco las teclas no llegan)
LIN="$(regla)"
echo "  ventana del emulador: ${LIN:-NO la encuentro}"
if ! echo "$LIN" | grep -qE 'cli=|geo='; then
    echo "  NO encuentro la ventana del emulador: ni las capturas ni las teclas valdrian"; exit 1
fi
sleep 1

# ---- 3. la captura del reposo (el cursor en el centro, ninguna tecla apretada): es la referencia
capturar 00-reposo

if [ "$SIN_TECLAS" = "1" ]; then
    echo "== SIN_TECLAS: linea de base, el POC no tiene que ver ningun boton"
    sleep 4
    capturar 99-final
elif [ -n "${GUION_TECLAS:-}" ]; then
    # ---- SONDA: mandar una lista de teclas y NO esperar marcas. Sirve para descubrir que tecla del
    #      emulador produce cada boton del control (el mapeo del emulador no esta en su emu.cfg: es el de
    #      fabrica). El log del POC dice, por cada cambio, que bits llegaron: se lee de ahi.
    echo "== sonda de teclas: $GUION_TECLAS"
    PAUSA="${PAUSA:-0.6}" python3 "$HERE/teclas-virtuales.py" guion $GUION_TECLAS >"$OUT/guion.log" 2>&1
    sleep 1
    capturar 99-final
else
    # ---- 4. el guion de teclas: UN solo teclado virtual para toda la secuencia.
    #
    # EL MAPEO DEL EMULADOR, MEDIDO EN ESTA TARJETA (una corrida por tecla, scripts/console/sonda-teclas-271.sh,
    # evidencia dc-002.71-maple-input.txt seccion 3): el emu.cfg de este host no tiene seccion de entrada, asi
    # que son los valores de FABRICA de Flycast 2.7:
    #
    #	flechas -> la CRUCETA          x -> A          c -> B
    #	s -> X      d -> Y             f -> L (LT)     v -> R (RT)
    #	enter -> START                 z, y, a, q, w, e, r, espacio -> NINGUN boton
    #
    # OJO: NO es el mapeo que decia la nota de DC-002.50 (Z->A, X->B, C->Start, V->X). Con esas teclas esta
    # corrida produce OTROS botones (z no produce nada, x produce A, v produce el gatillo R): se midio tecla
    # por tecla y se corrigio aca. El aguante es largo a proposito: la captura se saca cuando aparece la marca
    # del POC, y con un aguante corto la tecla ya se solto.
    GUION="guion derecha $AGUANTE abajo $AGUANTE x $AGUANTE c $AGUANTE s $AGUANTE d $AGUANTE f $AGUANTE"
    GUION="$GUION v $AGUANTE enter $AGUANTE"
    echo "== guion de teclas: $GUION"
    PAUSA=1.0 setsid python3 "$HERE/teclas-virtuales.py" $GUION >"$OUT/guion.log" 2>&1 &
    GUION_PID=$!

    # ---- 5. una captura por tramo, esperando LA MARCA DEL POC (no el reloj)
    #      Cada tramo: la marca que tiene que aparecer en el log y el nombre del archivo.
    tramo() {
        local marca="$1" nombre="$2"
        local previa; previa="$(cuenta "$marca")"
        if esperar_cuenta "$marca" "$previa" 120; then
            sleep 0.8            # que el cuadro con la tecla apretada llegue a la pantalla
            capturar "$nombre"
        else
            echo "  el tramo $nombre NO llego al POC (no aparecio la marca: $marca)"
        fi
    }

    tramo "HITO271: FLANCO bajada= DERECHA"   01-cruceta-derecha
    tramo "HITO271: FLANCO bajada= ABAJO"     02-cruceta-abajo
    tramo "HITO271: MOUSE bajada= IZQUIERDO"  03-boton-A
    tramo "HITO271: MOUSE bajada= DERECHO"    04-boton-B
    tramo "HITO271: MOUSE bajada= MEDIO"      05-boton-X
    tramo "HITO271: FLANCO bajada= Y"         06-boton-Y
    tramo "HITO271: FLANCO bajada= Q"         07-gatillo-L
    tramo "HITO271: FLANCO bajada= E"         08-gatillo-R
    # el Start tambien da ENTER (es el SEGUNDO Enter del guion: el primero fue el del A). Se espera a que la
    # cuenta de bajadas de ENTER llegue a 2, no a la primera: si se esperara la primera, la captura del Start
    # seria la foto del A.
    n=0
    while [ "$(cuenta 'HITO271: FLANCO bajada= ENTER')" -lt 2 ] && [ "$n" -lt 120 ]; do
        sleep 0.25
        n=$(( n + 1 ))
    done
    sleep 0.8
    capturar 07-start

    wait "$GUION_PID" 2>/dev/null
fi

sleep 2
capturar 99-final
kill -TERM -"$PID" 2>/dev/null || kill -TERM "$PID" 2>/dev/null
sleep 1
kill -KILL -"$PID" 2>/dev/null
qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.unloadScript "regla-$TOKEN" >/dev/null 2>&1

echo "== pantalla"
ls -la "$OUT"/f*.png 2>/dev/null | awk '{print "   "$5" B  "$9}'
echo "== las marcas del POC (la consola serie dentro del log del emulador)"
grep -a 'HITO271:' "$OUT/flycast.log" | tr -d '\r' | sed 's/^/   /'
echo "== lo que dijo el guion"
sed 's/^/   /' "$OUT/guion.log" 2>/dev/null
echo "== avisos del emulador"
grep -aE 'Fatal|EXCEPTION|Unknown|not found' "$OUT/flycast.log" | tail -5
