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
# 3. La ventana de Flycast lleva en el titulo el nombre del .gdi. Copiando el disco a "dc236.gdi" el titulo
#    es "Flycast - dc236" y se puede filtrar por ESE titulo, sin tocar la ventana del otro run.
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
OUT="${3:-$HOME/.hermes/cache/scratch/dc236-run}"
POS="${4:-1400 120}"
FLY="${FLYCAST:-$HOME/dreamcast/dl/flycast.AppImage}"
DISP="${DISPLAY:-:0}"
TOKEN="dc236"
PX="${POS%% *}"; PY="${POS##* }"

mkdir -p "$OUT"
rm -f "$OUT"/f*.png

case "$IMG" in
*.elf)
    # EL ARRANQUE PUEDE SER EL ELF PELADO: los POC que llevan sus assets adentro (el romdisk que libgodc
    # monta en /rd) no necesitan disco. Se copia con el MISMO nombre base que el disco ($TOKEN) para que
    # Flycast le ponga "dc236" en el titulo y la regla de KWin siga filtrando ESA ventana y no la de un run
    # vecino. Medido en DC-002.36: el disco del menu no arranca de forma confiable y el ELF si.
    GDI="$OUT/$TOKEN.elf"
    cp -f "$IMG" "$GDI"
    ;;
*)
    DISC_DIR="$(cd "$(dirname "$IMG")" && pwd)"
    GDI="$DISC_DIR/$TOKEN.gdi"
    cp -f "$IMG" "$GDI"
    ;;
esac

REGLA_JS="${TMPDIR:-/tmp}/kwin-regla-$TOKEN.js"
cat > "$REGLA_JS" <<'JSFIN'
// Pone arriba y EN SU LUGAR la ventana del disco dc236. Las demas ventanas de Flycast NO se tocan.
// Y se reafirma en cada captura: un run vecino que carga su propio script de KWin mueve TODA ventana de
// clase "flycast" (medido: mi ventana paso de 1400,92 a 210,210 a los 15 s de una corrida).
var PX = __PX__, PY = __PY__;
function ubicar(w) {
    var cap = "", cls = "";
    try { cap = (w.caption || "").toString(); } catch (e) { cap = ""; }
    try { cls = (w.resourceClass || "").toString(); } catch (e) { cls = ""; }
    if (cls.toLowerCase().indexOf("flycast") < 0) { return; }
    if (cap.toLowerCase().indexOf("dc236") >= 0) {
        var g = w.frameGeometry;
        if (g.x !== PX || g.y !== PY - 28) {
            w.frameGeometry = { x: PX, y: PY - 28, width: g.width, height: g.height };
        }
        w.keepAbove = true;
        // Y SE LE DA EL FOCO: el teclado virtual del host (/dev/uinput) manda las teclas a la ventana
        // ACTIVA, no a la de arriba. Medido en DC-002.36: sin esto la tecla 'abajo' no llegaba al
        // emulador (la consola serie se quedaba con 'FOCO inicial= -1' y las capturas salian iguales:
        // 0 pixeles de diferencia entre las dos).
        try { workspace.activeWindow = w; } catch (e) { print("DC236-CAP|no pude activar: " + e); }
    }
    print("DC236-CAP|" + cap + "|" + cls + "|pid=" + w.pid + "|geo=" + w.frameGeometry.x + "," +
          w.frameGeometry.y + "," + w.frameGeometry.width + "," + w.frameGeometry.height);
}
var pila = workspace.stackingOrder;
for (var i = 0; i < pila.length; i++) { ubicar(pila[i]); }
// Y se informa QUIEN QUEDO ACTIVA: el teclado virtual del host manda las teclas a la ventana activa, asi
// que el runner necesita saber si de verdad la ventana del emulador quedo adelante (medido en DC-002.36:
// cuando no queda, las dos capturas de la navegacion salen identicas).
var act = "ninguna";
try { if (workspace.activeWindow) { act = workspace.activeWindow.caption.toString(); } } catch (e) { act = "?"; }
print("DC236-CAP|activo|" + act);
print("DC236-CAP|fin");
JSFIN
sed -i "s/__PX__/$PX/; s/__PY__/$PY/" "$REGLA_JS"

# Cada llamada vuelve a aplicar la regla (la ventana aparece recien al segundo, con el titulo ya puesto).
# Deja en ACTIVO el titulo de la ventana que quedo activa (para saber si la tecla va a llegar al emulador).
ACTIVO=""
regla() {
    qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.unloadScript "regla-$TOKEN" >/dev/null 2>&1
    qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.loadScript "$REGLA_JS" "regla-$TOKEN" >/dev/null 2>&1
    qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.start >/dev/null 2>&1
    sleep 1
    local J
    J="$(journalctl --user --since "-6 s" 2>/dev/null | grep -a 'DC236-CAP|' | tr -d '\r')"
    ACTIVO="$(printf '%s\n' "$J" | grep -a 'DC236-CAP|activo|' | sed 's/.*DC236-CAP|activo|//' | tail -1)"
    printf '%s\n' "$ACTIVO" > "$OUT/activo.txt"
    # la linea de la VENTANA es la que trae pid= (la de ACTIVO tambien dice "Flycast - dc236", asi que hay
    # que separarlas: sin esto la corrida se quedaba sin geometria y el instrumento no media nada).
    printf '%s\n' "$J" | grep -a '|pid=' | sed 's/.*DC236-CAP|/ventana: /' | tail -1
}

# activar(): deja la ventana del emulador ADELANTE antes de mandar una tecla. Se insiste (medido: la
# activacion no siempre prende en el primer intento) y se avisa si no se logra, en vez de medir una
# navegacion que no paso. El estado se lee del archivo porque `regla` se llama dentro de un subshell.
activar() {
    local i
    for i in 1 2 3 4 5; do
        regla >/dev/null
        case "$(cat "$OUT/activo.txt" 2>/dev/null)" in
        *"$TOKEN"*) return 0 ;;
        esac
        sleep 1
    done
    echo "  AVISO: la ventana activa es '$(cat "$OUT/activo.txt" 2>/dev/null)' y no la del emulador: la tecla puede caer en otra"
    return 1
}

echo "== Flycast: $GDI (ventana pedida en $PX,$PY + keepAbove; consola serie si)"
setsid env DISPLAY="$DISP" "$FLY" -config "window:fullscreen=no" \
    -config "window:left=$PX" -config "window:top=$PY" \
    -config "Debug:SerialConsoleEnabled=true" "$GDI" >"$OUT/flycast.log" 2>&1 &
PID=$!
echo "pid=$PID"

# TECLAS: "<segundo>:<tecla>" separado por espacios (p.ej. TECLAS="15:abajo 25:abajo"). Antes de la captura
# de ese segundo se manda la tecla por el TECLADO VIRTUAL de /dev/uinput (scripts/console/teclas-virtuales.py:
# en KDE/Wayland no hay xdotool y `wtype` no anda, pero /dev/uinput es del kernel y el emulador lo lee) y se
# espera 1 s: la captura de f<seg>s.png sale CON la tecla ya aplicada. Es la unica forma de probar la
# navegacion sin que nadie apriete nada.
teclas_de() { # $1 = segundo: las teclas que tocan en ese segundo, una por linea
    for par in ${TECLAS:-}; do
        [ "${par%%:*}" = "$1" ] && echo "${par##*:}"
    done
}

tecla() { # $1 = nombre de la tecla de teclas-virtuales.py
    python3 "$HERE/teclas-virtuales.py" "$1" >/dev/null 2>&1 &&
        echo "  tecla enviada: $1" || echo "  NO pude mandar la tecla $1"
}

ult=0
for t in 3 6 10 15 25 40 60 90; do
    [ "$t" -gt "$SEGS" ] && break
    sleep $(( t - ult )); ult=$t
    if kill -0 "$PID" 2>/dev/null; then
        # PRIMERO LA REGLA (que ubica, pone arriba y ACTIVA la ventana), DESPUES la tecla: el teclado
        # virtual del host manda a la ventana ACTIVA, asi que si la tecla sale antes de activarla, la tecla
        # se va a otra ventana. Medido en DC-002.36: con el orden al reves, las dos capturas salian
        # identicas (0 pixeles de diferencia) y la consola serie quedaba en `FOCO inicial= -1`.
        LIN="$(regla)"

        for k in $(teclas_de "$t"); do
            # La tecla va a la ventana ACTIVA y no siempre prende al primer intento (medido: con otro
            # Flycast vivo, la activacion se pelea). Se comprueba en la consola serie que la tecla ENTRO
            # (el POC imprime 'MENU236: FOCO' cada vez que la seleccion se mueve) y si no entro se
            # reintenta: asi la captura no mide una navegacion que no paso.
            antes="$(grep -ac 'MENU236: FOCO' "$OUT/flycast.log" 2>/dev/null || echo 0)"

            for intento in 1 2 3; do
                activar >/dev/null
                tecla "$k"
                sleep 1
                ahora="$(grep -ac 'MENU236: FOCO' "$OUT/flycast.log" 2>/dev/null || echo 0)"
                [ "${ahora:-0}" -gt "${antes:-0}" ] && break
                echo "  (la tecla $k no entro en el intento $intento: la ventana activa es '$(cat "$OUT/activo.txt" 2>/dev/null)')"
            done
        done

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
