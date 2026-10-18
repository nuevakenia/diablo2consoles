#!/bin/bash
# DC-002.35: prueba corta -- lanzar el emulador y PREGUNTARLE a KWin (por titulo, clase y pid) donde
# quedo la ventana de Flycast. Sirve para saber si `-config window:left/top` posiciona y con que titulo
# aparece mi ventana (para no medir la de otro run).
set -uo pipefail
GDI="${1:?uso: probar-ventana-235.sh <disc.gdi> [x y]}"
POS="${2:-1400 120}"
PX="${POS%% *}"; PY="${POS##* }"
FLY="${FLYCAST:-$HOME/dreamcast/dl/flycast.AppImage}"
TOKEN="dc235"
DISC_DIR="$(cd "$(dirname "$GDI")" && pwd)"
GDI2="$DISC_DIR/$TOKEN.gdi"
cp -f "$GDI" "$GDI2"

SONDA="${TMPDIR:-/tmp}/kwin-lista-$TOKEN.js"
cat > "$SONDA" <<'JSFIN'
function revisar(w) {
    var cap = "", cls = "";
    try { cap = (w.caption || "").toString(); } catch (e) { cap = "?"; }
    try { cls = (w.resourceClass || "").toString(); } catch (e) { cls = "?"; }
    if (cls.toLowerCase().indexOf("flycast") < 0) { return; }
    print("LISTA-CAP|" + cap + "|" + cls + "|pid=" + w.pid + "|geo=" + w.frameGeometry.x + "," +
          w.frameGeometry.y + "," + w.frameGeometry.width + "," + w.frameGeometry.height +
          "|keepAbove=" + w.keepAbove);
}
var pila = workspace.stackingOrder;
for (var i = 0; i < pila.length; i++) { revisar(pila[i]); }
print("LISTA-CAP|fin");
JSFIN

ver() {
    qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.unloadScript "lista-$TOKEN" >/dev/null 2>&1
    qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.loadScript "$SONDA" "lista-$TOKEN" >/dev/null 2>&1
    qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.start >/dev/null 2>&1
    sleep 1
    journalctl --user --since "-10 s" 2>/dev/null | grep -a 'LISTA-CAP|' | sed 's/.*LISTA-CAP|/   /' | tail -6
}

echo "== ventanas Flycast ANTES"
ver
echo "== lanzo $GDI2 pidiendo posicion $PX,$PY"
setsid env DISPLAY="${DISPLAY:-:0}" "$FLY" -config "window:fullscreen=no" \
    -config "window:left=$PX" -config "window:top=$PY" \
    -config "Debug:SerialConsoleEnabled=true" "$GDI2" >"${TMPDIR:-/tmp}/probe-$TOKEN.log" 2>&1 &
PID=$!
sleep 4
echo "== ventanas Flycast CON el mio corriendo (pid del lanzador=$PID)"
ver
echo "== arbol de procesos del lanzador"
ps -eo pid,ppid,args | awk -v p="$PID" '$2==p || $1==p' | head -5
sleep 3
kill -TERM -"$PID" 2>/dev/null || kill -TERM "$PID" 2>/dev/null
sleep 1; kill -KILL -"$PID" 2>/dev/null
echo "== fin"
