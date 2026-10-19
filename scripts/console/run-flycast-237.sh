#!/bin/bash
# DC-002.37: correr el POC en Flycast con la ventana IDENTIFICABLE, posicionada y VERIFICADA.
#
# POR QUE EXISTE (todo medido en esta tarjeta)
# 1. `spectacle -f` (KDE/Wayland) captura la PANTALLA COMPLETA: la ventana del emulador hay que ubicarla
#    adentro de la captura. Y la geometria no es fija: Flycast la guarda en ~/.config/flycast/emu.cfg.
# 2. Cuando hay OTRO run con un Flycast vivo en la misma pantalla, la captura puede traer SU ventana.
#    Medido: a los 40 s de una corrida el recorte mostraba otra cosa (99,11% de pixeles no negros y 0,98%
#    de coincidencia con la referencia, contra 90,57% en las capturas limpias). `flycast-adelante.sh` mueve
#    TODA ventana de clase "flycast": con dos emuladores vivos no sirve para distinguirlas.
# 3. La ventana de Flycast lleva en el titulo el nombre del .gdi. Copiando el disco a "dc237.gdi" el titulo
#    es "Flycast - dc237" y se puede filtrar por ESE titulo, sin tocar la ventana del otro run.
# 4. La posicion se pide con `-config window:left/top` (medido: queda en el cliente pedido, con la barra de
#    titulo 28 px mas arriba) y ADEMAS se pide keepAbove: sin eso, cualquier ventana grande (el editor)
#    tapa al emulador y la captura mide el editor -- medido: a los 15 s la captura mostraba VS Code y la
#    pantalla del emulador no aparecia.
# 5. La geometria NO se supone: se le pregunta a KWin (sonda por DBus -> journal) y se contrasta con el
#    instrumento, que ubica la pantalla por la banda negra.
#
# Uso:  run-flycast-235.sh <disc.gdi> <segundos> <dir_salida> [x y]
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMG="${1:?uso: run-flycast-235.sh <disc.gdi> <segundos> <dir_salida> [x y]}"
SEGS="${2:-40}"
OUT="${3:-$HOME/.hermes/cache/scratch/dc237-run}"
POS="${4:-1400 120}"
# la posicion se puede pasar como UN argumento ("1400 120") o como DOS (1400 120). Medido: leyendo solo
# $4, la llamada con dos argumentos dejaba PX=PY=1400 y la ventana terminaba en y=1372, FUERA de la
# pantalla (3584x1080): las capturas salian sin emulador y el instrumento contestaba SIN-BANDA.
[ -n "${5:-}" ] && POS="$4 $5"
FLY="${FLYCAST:-$HOME/dreamcast/dl/flycast.AppImage}"
DISP="${DISPLAY:-:0}"
TOKEN="dc237"
PX="${POS%% *}"; PY="${POS##* }"

mkdir -p "$OUT"
rm -f "$OUT"/f*.png

DISC_DIR="$(cd "$(dirname "$IMG")" && pwd)"
GDI="$DISC_DIR/$TOKEN.gdi"
cp -f "$IMG" "$GDI"

REGLA_JS="${TMPDIR:-/tmp}/kwin-regla-$TOKEN.js"
cat > "$REGLA_JS" <<'JSFIN'
// Pone arriba y EN SU LUGAR la ventana del disco dc237. Las demas ventanas de Flycast NO se tocan.
// Y se reafirma en cada captura: un run vecino que carga su propio script de KWin mueve TODA ventana de
// clase "flycast" (medido: mi ventana paso de 1400,92 a 210,210 a los 15 s de una corrida).
var PX = __PX__, PY = __PY__;
function ubicar(w) {
    var cap = "", cls = "";
    try { cap = (w.caption || "").toString(); } catch (e) { cap = ""; }
    try { cls = (w.resourceClass || "").toString(); } catch (e) { cls = ""; }
    if (cls.toLowerCase().indexOf("flycast") < 0) { return; }
    if (cap.toLowerCase().indexOf("dc237") >= 0) {
        var g = w.frameGeometry;
        if (g.x !== PX || g.y !== PY - 28) {
            w.frameGeometry = { x: PX, y: PY - 28, width: g.width, height: g.height };
        }
        w.keepAbove = true;
    }
    print("DC237-CAP|" + cap + "|" + cls + "|pid=" + w.pid + "|geo=" + w.frameGeometry.x + "," +
          w.frameGeometry.y + "," + w.frameGeometry.width + "," + w.frameGeometry.height);
}
var pila = workspace.stackingOrder;
for (var i = 0; i < pila.length; i++) { ubicar(pila[i]); }
print("DC237-CAP|fin");
JSFIN
sed -i "s/__PX__/$PX/; s/__PY__/$PY/" "$REGLA_JS"

# Cada llamada vuelve a aplicar la regla (la ventana aparece recien al segundo, con el titulo ya puesto).
regla() {
    qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.unloadScript "regla-$TOKEN" >/dev/null 2>&1
    qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.loadScript "$REGLA_JS" "regla-$TOKEN" >/dev/null 2>&1
    qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.start >/dev/null 2>&1
    sleep 1
    journalctl --user --since "-6 s" 2>/dev/null | grep -a 'DC237-CAP|' | tr -d '\r' |
        grep -i "flycast - $TOKEN" | sed 's/.*DC237-CAP|/ventana: /' | tail -1
}

echo "== Flycast: $GDI (ventana pedida en $PX,$PY + keepAbove; consola serie si)"
setsid env DISPLAY="$DISP" "$FLY" -config "window:fullscreen=no" \
    -config "window:left=$PX" -config "window:top=$PY" \
    -config "Debug:SerialConsoleEnabled=true" "$GDI" >"$OUT/flycast.log" 2>&1 &
PID=$!
echo "pid=$PID"

ult=0
for t in 3 6 10 15 25 40 60 90; do
    [ "$t" -gt "$SEGS" ] && break
    sleep $(( t - ult )); ult=$t
    if kill -0 "$PID" 2>/dev/null; then
        LIN="$(regla)"
        spectacle -b -n -f -o "$OUT/f${t}s.png" >/dev/null 2>&1
        echo "  captura ${t}s   ${LIN:-ventana: NO la encuentro}"
    else
        echo "  el emulador se detuvo antes de los ${t}s"; break
    fi
done

sleep 1
kill -TERM -"$PID" 2>/dev/null || kill -TERM "$PID" 2>/dev/null
sleep 1
kill -KILL -"$PID" 2>/dev/null
qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.unloadScript "regla-$TOKEN" >/dev/null 2>&1

echo "== pantalla"; ls -la "$OUT"/f*.png 2>/dev/null | awk '{print "   "$5" bytes  "$9}'
echo "== consola serie (marcas del POC)"
grep -aE 'HITO235|SH4|EXCEPTION|Fatal' "$OUT/flycast.log" | tr -d '\r' | tail -12
