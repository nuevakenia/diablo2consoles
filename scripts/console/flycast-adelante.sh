#!/bin/bash
# Saca la ventana de Flycast del camino de la captura: la MUDA al monitor principal y la deja arriba.
#
# POR QUE EXISTE (medido 2026-10-16, DC-002.9): la captura se hace con `spectacle -f`, que fotografia el
# escritorio COMPUESTO. Si algo tapa al emulador, sale eso y no el emulador: no alcanza con que el
# emulador este corriendo bien. En la maquina de Daniel habia Diablo II en fullscreen dentro de gamescope
# (`gamescope | Diablo II | fullScreen=true`) y el emulador abre su ventana en ESA pantalla (la de
# 1024x768, en 2830,385), debajo. Las cuatro capturas salieron con el juego adelante y CERO pixeles del
# POC.
#
# Lo que se probo y lo que quedo (medido, no supuesto):
#   - `keepAbove = true`: se aplica, pero NO alcanza. Con gamescope en fullscreen y ACTIVO, el apilado de
#     KWin lo deja igual arriba de la ventana con keepAbove.
#   - `workspace.activeWindow = w`: funciona, pero le roba el foco al juego de Daniel por los segundos de
#     la corrida. Se descarto.
#   - `w.move(x, y)`: NO existe (TypeError). `w.output`: es de solo lectura. `Qt.rect`: `Qt` no existe en
#     el motor de scripts de KWin 6.
#   - `w.frameGeometry = {x, y, width, height}`: SI. Movida a 210,210 con su MISMO tamano (no se
#     redimensiona: el emulador sigue pintando 1:1, que es lo que necesita la comparacion pixel a pixel).
#   - Las tres son X11 (la ventana se ve como `KWin::X11Window`): el emulador corre por XWayland.
#
# Se hace por KWin y no por configuracion del emulador porque Flycast no tiene opcion de "always on top"
# ni de elegir monitor, y `SDL_VIDEO_WINDOW_POS` no tiene efecto (medido: la ventana quedo igual en
# 2830,385).
#
# Uso:  scripts/console/flycast-adelante.sh arriba     (antes de lanzar el emulador)
#       scripts/console/flycast-adelante.sh sacar      (al terminar)
#
# Si no hay sesion de KWin (o no hay qdbus6), no hace nada y sale 0: la captura queda como estaba.
set -uo pipefail

ACCION="${1:-arriba}"
NOMBRE="flycast-adelante-d2"     # nombre del plugin de KWin (para poder sacarlo despues)
POS="${POS:-210 210}"            # donde se deja la ventana (monitor principal, en el escritorio)
JS="${TMPDIR:-/tmp}/kwin-flycast-adelante.js"

escribir_js() {
    cat > "$JS" <<JSFIN
// Deja la ventana de Flycast en el monitor principal, con su mismo tamano, y arriba. Las ventanas que ya
// existen y las que aparezcan despues (por eso queda escuchando windowAdded).
var D2_POS = "$POS".split(" ");

function d2Mueve(w) {
    var rc = "";
    try { rc = (w.resourceClass || "").toString().toLowerCase(); } catch (e) { rc = ""; }
    if (rc.indexOf("flycast") < 0) { return; }

    var g = w.frameGeometry;      // su MISMO tamano: mover no tiene que redimensionar (1:1)
    w.frameGeometry = { x: parseInt(D2_POS[0], 10), y: parseInt(D2_POS[1], 10), width: g.width, height: g.height };
    w.keepAbove = true;
}
var pila = workspace.stackingOrder;
for (var i = 0; i < pila.length; i++) { d2Mueve(pila[i]); }
workspace.windowAdded.connect(d2Mueve);
JSFIN
}

sacar() {
    qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.unloadScript "$NOMBRE" >/dev/null 2>&1
}

# geo (DC-002.58): donde quedo la ventana del emulador, AHORA. La geometria la sabe el gestor de ventanas y no
# hay que adivinarla mirando la captura: la imprime un script de KWin por la consola del journal, igual que el
# gancho de run-flycast-249.sh (medido: el `print()` de un script de KWin cae en `journalctl --user`).
#
# Sale:  FLYCAST-GEO|cli=x,y,w,h|geo=x,y,w,h     (cli = el area de video; geo = la ventana con su barra)
#        nada, si no encuentra la ventana.
geo() {
    local js="${TMPDIR:-/tmp}/kwin-flycast-geo.js"
    local nombre="flycast-geo-d2"
    cat > "$js" <<'JSFIN'
var pila = workspace.stackingOrder;
for (var i = 0; i < pila.length; i++) {
    var w = pila[i], cls = "";
    try { cls = (w.resourceClass || "").toString().toLowerCase(); } catch (e) { continue; }
    if (cls.indexOf("flycast") < 0) { continue; }
    var g = w.frameGeometry, cli = "?";
    try {
        var c = w.clientGeometry;
        if (c && c.width > 0) { cli = c.x + "," + c.y + "," + c.width + "," + c.height; }
    } catch (e) { cli = "?"; }
    print("FLYCAST-GEO|cli=" + cli + "|geo=" + g.x + "," + g.y + "," + g.width + "," + g.height);
}
JSFIN
    qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.unloadScript "$nombre" >/dev/null 2>&1
    qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.loadScript "$js" "$nombre" >/dev/null 2>&1
    qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.start >/dev/null 2>&1
    sleep 1
    journalctl --user --since "-8 s" 2>/dev/null | grep -a 'FLYCAST-GEO|' | tr -d '\r' | tail -1 |
        sed 's/.*FLYCAST-GEO|/FLYCAST-GEO|/'
    qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.unloadScript "$nombre" >/dev/null 2>&1
}

if [ "$ACCION" = "geo" ]; then
    if ! command -v qdbus6 >/dev/null 2>&1; then
        exit 0
    fi
    geo
    exit 0
fi

if ! command -v qdbus6 >/dev/null 2>&1; then
    exit 0
fi
if ! qdbus6 org.kde.KWin /Scripting >/dev/null 2>&1; then
    echo "   (sin sesion de KWin por DBus: la captura queda como este)" >&2
    exit 0
fi

if [ "$ACCION" = "sacar" ]; then
    sacar
    exit 0
fi

escribir_js
sacar    # por si habia quedado cargado de una corrida anterior
qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.loadScript "$JS" "$NOMBRE" >/dev/null 2>&1
qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.start >/dev/null 2>&1
echo "   ventana del emulador: la muevo a $POS y la dejo arriba (KWin)"
