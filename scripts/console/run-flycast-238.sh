#!/bin/bash
# DC-002.38: correr el POC del PERSONAJE EN EL MAPA en Flycast, MOVIENDO al personaje con el teclado virtual,
# y sacar las capturas en los momentos que importan (antes, durante y despues de cada tecla sostenida).
#
# QUE HACE Y POR QUE
# ------------------
#  1. Arranca Flycast con el disco del POC, con la ventana IDENTIFICABLE (titulo "Flycast - dc238"), puesta en
#     una posicion conocida y keepAbove (lo mismo que run-flycast-237.sh, y por los mismos motivos medidos:
#     `spectacle -f` captura la pantalla entera y sin keepAbove cualquier ventana tapa al emulador).
#  2. ESPERA a que el POC avise que esta dibujando ("HITO238: dibujando"). Antes de eso la pantalla todavia no
#     tiene el mapa y la captura no mide nada: la linea de tiempo arranca de ahi, no del `setsid`.
#  3. Manda las teclas por /dev/uinput (teclas-virtuales.py sostener: la tecla queda APRETADA N segundos, que
#     es lo que hace falta para que el movimiento del motor acumule).
#  4. Saca la captura de cada tramo CUANDO LA TECLA YA SE SOLTO, esperando la marca del PROPIO POC (la
#     pulsacion y despues su suelta), no el reloj: con el reloj las capturas se corrian respecto de las
#     pulsaciones (medido: la captura del tramo "abajo" salio despues de la diagonal y el tramo daba 0 px).
#  5. Deja en $OUT el log del emulador (con la consola serie del POC) y una linea "GEO t=Ns cliente=..." por
#     captura, que es la geometria que el medidor necesita.
#
# Uso:  run-flycast-238.sh <disc.gdi> <dir_salida> [x y]
#       SEGS_EXTRA=20 para dejar la corrida viva unos segundos mas al final.
#       SIN_TECLAS=1 para correr sin mandar teclas (linea de base: el personaje no se tiene que mover).
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMG="${1:?uso: run-flycast-238.sh <disc.gdi> <dir_salida> [x y]}"
OUT="${2:-$HOME/.hermes/cache/scratch/dc238-run}"
POS="${3:-1400 120}"
[ -n "${4:-}" ] && POS="$3 $4"
FLY="${FLYCAST:-$HOME/dreamcast/dl/flycast.AppImage}"
DISP="${DISPLAY:-:0}"
TOKEN="dc238"
BARRA_TITULO="${BARRA_TITULO:-28}"
PX="${POS%% *}"; PY="${POS##* }"
PYTHON="${DC_PYTHON:-python3}"
ESPERA_ARRANQUE="${ESPERA_ARRANQUE:-90}"

mkdir -p "$OUT"
rm -f "$OUT"/f*.png

DISC_DIR="$(cd "$(dirname "$IMG")" && pwd)"
GDI="$DISC_DIR/$TOKEN.gdi"
cp -f "$IMG" "$GDI"

# la regla de KWin (identica a la de DC-002.37, con el token de esta tarjeta)
REGLA_JS="${TMPDIR:-/tmp}/kwin-regla-$TOKEN.js"
cat > "$REGLA_JS" <<'JSFIN'
var PX = __PX__, PY = __PY__, objetivo = null;
function ubicar(w) {
    var cap = "", cls = "";
    try { cap = (w.caption || "").toString(); } catch (e) { cap = ""; }
    try { cls = (w.resourceClass || "").toString(); } catch (e) { cls = ""; }
    if (cls.toLowerCase().indexOf("flycast") < 0) { return; }
    if (cap.toLowerCase().indexOf("dc238") >= 0) {
        var g = w.frameGeometry;
        if (g.x !== PX || g.y !== PY - 28) {
            w.frameGeometry = { x: PX, y: PY - 28, width: g.width, height: g.height };
        }
        w.keepAbove = true;
        objetivo = w;
    }
    print("DC238-CAP|" + cap + "|" + cls + "|pid=" + w.pid + "|geo=" + w.frameGeometry.x + "," +
          w.frameGeometry.y + "," + w.frameGeometry.width + "," + w.frameGeometry.height + "|cli=" +
          (function () {
              try {
                  var c = w.clientGeometry;
                  if (c && c.width > 0) {
                      return c.x + "," + c.y + "," + c.width + "," + c.height;
                  }
              } catch (e) { }
              return "?";
          })());
}
var pila = workspace.stackingOrder;
for (var i = 0; i < pila.length; i++) { ubicar(pila[i]); }

// EL FOCO NO ES UN DETALLE: las teclas del teclado virtual las recibe la VENTANA CON EL FOCO. Medido en esta
// tarjeta: en la corrida en que la ventana del emulador no tenia el foco, las flechas llegaron a medias (el
// POC vio botones sueltos 64/68/1024 y el personaje no se movio en la direccion pedida); en la corrida en que
// si lo tenia, llegaron enteras (128 = derecha, 32 = abajo, 144 = arriba+derecha). Asi que la regla tambien
// pide el foco.
if (objetivo) {
    try {
        workspace.activeWindow = objetivo;
        print("DC238-CAP|foco=" + objetivo.caption);
    } catch (e) {
        print("DC238-CAP|foco=no-pude " + e);
    }
}
// EL GANCHO DEL FOCO: cada 300 ms vuelve a pedir la ventana ARRIBA y CON EL FOCO mientras la regla este
// cargada. POR QUE (medido en esta tarjeta): en este host hay otra corrida del mismo emulador; cuando su
// ventana se pone encima, `spectacle -f` devuelve SU cuadro y la captura deja de valer (salieron capturas
// identicas byte a byte con el personaje en otro lado segun el motor). Pedir el foco una sola vez no alcanza.
if (objetivo) {
    var gancho = setInterval(function () {
        try {
            objetivo.keepAbove = true;
            workspace.activeWindow = objetivo;
        } catch (e) { }
    }, 300);
}
print("DC238-CAP|fin");
JSFIN
sed -i "s/__PX__/$PX/; s/__PY__/$PY/" "$REGLA_JS"

regla() {
    qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.unloadScript "regla-$TOKEN" >/dev/null 2>&1
    qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.loadScript "$REGLA_JS" "regla-$TOKEN" >/dev/null 2>&1
    qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.start >/dev/null 2>&1
    sleep 1
    journalctl --user --since "-6 s" 2>/dev/null | grep -a 'DC238-CAP|' | tr -d '\r' |
        grep -i "flycast - $TOKEN" | grep 'geo=' | sed 's/.*DC238-CAP|/ventana: /' | tail -1
}

echo "== Flycast: $GDI (ventana pedida en $PX,$PY + keepAbove; consola serie si)"
setsid env DISPLAY="$DISP" "$FLY" -config "window:fullscreen=no" \
    -config "window:left=$PX" -config "window:top=$PY" \
    -config "Debug:SerialConsoleEnabled=true" "$GDI" >"$OUT/flycast.log" 2>&1 &
PID=$!
echo "pid=$PID"

# EL EMULADOR VIVO SE BUSCA POR EL NOMBRE DEL DISCO, NO POR EL PID DEL `setsid`
# ------------------------------------------------------------------------------------------------
# Medido en esta tarjeta: el AppImage se desdobla (queda un "flycast.AppImag" y un "Flycast-rend") y el
# `$!` del `setsid` deja de existir enseguida: la comprobacion `kill -0 $PID` contestaba "se murio" con el
# emulador andando y dibujando en pantalla, y la corrida se abortaba sin capturar nada. Se busca el proceso
# por el nombre del .gdi, que es unico de esta corrida.
#
# El patron va con [.] para que no se matchee a si mismo: en la linea de comandos de ESTE script la cadena
# aparece como "dc238[.]gdi" y el patron no la reconoce (es la trampa de siempre con pkill -f).
PATRON='dc238[.]gdi'

vivo() { pgrep -f "$PATRON" >/dev/null 2>&1; }

# ---- 1. esperar a que el POC diga que esta dibujando (antes no hay nada que medir)
#
# OJO CON LA PRIMERA COMPROBACION: el AppImage tarda en montarse y en aparecer el proceso del emulador, asi
# que preguntar "esta vivo?" en el mismo instante del lanzamiento da "se murio" con el emulador arrancando
# (medido: paso dos veces y abortaba la corrida antes de capturar nada). Se espera un poco y, ademas, hacen
# falta TRES comprobaciones seguidas sin proceso para declararlo muerto.
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
            pgrep -af "$PATRON" || echo "  (pgrep no lo encuentra: $PATRON)"
            tail -5 "$OUT/flycast.log"
            exit 1
        fi
    fi
    sleep 1
    T=$(( T + 1 ))
done

if ! grep -qa 'HITO238: dibujando' "$OUT/flycast.log" 2>/dev/null; then
    echo "  el POC NO llego a dibujar en ${ESPERA_ARRANQUE}s"
    grep -aE 'HITO238|Fatal|EXCEPTION|arena' "$OUT/flycast.log" | tail -20
    kill -TERM -"$PID" 2>/dev/null; kill -KILL -"$PID" 2>/dev/null
    exit 1
fi

# ---- 1b. la ventana del emulador: la regla la ubica, la pone arriba y le pide el FOCO
#          (sin foco las teclas del teclado virtual no llegan: medido, ver el comentario de la regla de KWin)
LIN="$(regla)"
echo "  ventana del emulador: ${LIN:-NO la encuentro}"
if ! echo "$LIN" | grep -qE 'cli=|geo='; then
    echo "  NO encuentro la ventana del emulador: ni las capturas ni las teclas valdrian"
    echo "  (la regla de KWin no la vio; ventanas con 'flycast' en la clase:)"
    journalctl --user --since "-6 s" 2>/dev/null | grep -a 'DC238-CAP|' | tr -d '\r' | tail -5
    pkill -f "$PATRON" 2>/dev/null
    exit 1
fi
echo "  foco: $(journalctl --user --since '-6 s' 2>/dev/null | grep -a 'DC238-CAP|foco=' | tr -d '\r' |
    tail -1 | sed 's/.*DC238-CAP|//')"
sleep 1

# ---- 1c. la PRIMERA captura: el personaje parado, ANTES de tocar ninguna tecla (es la referencia del
#          tramo 1). Va ANTES de lanzar las teclas: si se saca despues, la primera pulsacion ya movio al
#          personaje y la foto "parado" no es la del arranque (medido: salio con pos=382,240 en vez de 320,240).
cuenta() {
    local n
    n="$(grep -ac "$1" "$OUT/flycast.log" 2>/dev/null || true)"
    echo "${n:-0}"
}

esperar_cuenta() {
    # esperar_cuenta <patron> <cuenta_previa> <cuartos_de_segundo>
    local patron="$1" previa="$2" limite="${3:-200}" i=0 n
    while [ "$i" -lt "$limite" ]; do
        n="$(cuenta "$patron")"
        [ "$n" -gt "$previa" ] && return 0
        sleep 0.25
        i=$(( i + 1 ))
    done
    return 1
}

tras_tecla() {
    # tras_tecla <botones> <nombre de la captura>
    local bot="$1" nombre="$2" p0 z0
    p0="$(cuenta "HITO238: CONTROL botones $bot ejes")"
    z0="$(cuenta "HITO238: CONTROL botones 0 ejes")"
    if ! esperar_cuenta "HITO238: CONTROL botones $bot ejes" "$p0" 200; then
        echo "  la pulsacion botones=$bot NO llego al POC en 50 s"
        return 1
    fi
    if ! esperar_cuenta "HITO238: CONTROL botones 0 ejes" "$z0" 200; then
        echo "  la pulsacion botones=$bot NO se solto en 50 s"
        return 1
    fi
    sleep 1.8
    capturar "$bot" "$nombre"
    return 0
}

# ---- 2. las teclas y la PRIMERA captura: van las dos DESPUES de `capturar()` (mas abajo), porque la foto del
#        personaje PARADO tiene que ser ANTERIOR a la primera pulsacion. Ver el bloque "ARRANQUE" en la
#        seccion 3.

# ---- 3. las capturas: la linea de tiempo esta atada a las teclas de arriba
#
# TRES COSAS QUE SE MIDIERON AQUI Y NO SON ADORNO:
#  a) LA VENTANA SE PIDE ARRIBA Y CON EL FOCO antes de cada captura (regla). Sin eso la captura puede salir
#     con la ventana de OTRA corrida encima: en esta tarjeta salieron capturas IDENTICAS byte a byte con el
#     personaje en otro lado segun el motor, o sea la captura era de otra ventana (hay otro agente del tablero
#     que usa el mismo emulador en este host).
#  b) SE CAPTURA LA PANTALLA ENTERA (-f) Y SE RECORTA CON LA GEOMETRIA QUE DIO EL GESTOR DE VENTANAS (la
#     linea GEO). Se probo `-a` (ventana activa) y NO sirve para esto: depende de que la ventana siga siendo
#     la activa en el instante de la captura, y con dos corridas del emulador en el host la captura salio de
#     ventanas de tamaños distintos (1154x852 y 812x611 en la misma corrida). Con -f + geometria el recorte es
#     siempre el mismo rectangulo.
#  c) SE COMPRUEBA QUE LA CAPTURA CAMBIO: si el md5 es el mismo que el de la captura anterior, no es que el
#     personaje no se movio: es que el compositor devolvio el mismo cuadro. Se espera y se vuelve a pedir
#     (hasta 3 veces), y si igual no cambia se informa "SIN-CAMBIO" para que el corpus lo vea y lo declare.
ultimo_md5=""
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
    local t="$1" nombre="$2"
    local lin geo cli md5 pos cuadro intento
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
        # `spectacle` devuelve antes de que el PNG este escrito del todo (medido: md5sum salio VACIO en dos
        # capturas y esas dos quedaron como si fueran del mismo cuadro). Se espera a que el tamaño se quede
        # quieto antes de hashear.
        esperar_quieto "$OUT/$nombre.png"
        md5="$(md5sum "$OUT/$nombre.png" 2>/dev/null | cut -d' ' -f1)"
        if [ -z "$ultimo_md5" ] || [ "$md5" != "$ultimo_md5" ]; then
            break
        fi
        echo "  captura ${t}s  $nombre.png SIN-CAMBIO (md5 igual al anterior), reintento $intento"
        sleep 0.7
    done
    ultimo_md5="$md5"

    # la posicion que el POC dice tener EN ESE MOMENTO (para poder comparar la pantalla con el motor).
    # La linea del POC es: "HITO238: POS cuadro N ms N botones N subtiles x y celda cx cy tile tx ty ancla ax ay
    # pantalla px py caminado C tiempo T pasos N dtCero N dtGrande N".
    pos="$(grep -a 'HITO238: POS cuadro' "$OUT/flycast.log" 2>/dev/null | tr -d '\r' | tail -1 |
        sed -n 's/.* pantalla \([0-9-]*\) \([0-9-]*\) caminado.*/\1,\2/p')"
    cuadro="$(grep -a 'HITO238: POS cuadro' "$OUT/flycast.log" 2>/dev/null | tr -d '\r' | tail -1 |
        sed -n 's/.*POS cuadro \([0-9]*\) ms.*/\1/p')"

    echo "  captura ${t}s  $nombre.png  md5=${md5:0:8} ${lin:-ventana: NO la encuentro}  pos=${pos:-?} cuadro=${cuadro:-?}"
    [ -n "$cli" ] && echo "GEO t=${t}s captura=$nombre cliente=$cli pos=${pos:-?} cuadro=${cuadro:-?} ms=$(date +%s%3N)"
}

# ---- 2b. ARRANQUE: la foto del personaje PARADO y despues las teclas
#
# EL ORDEN ES LA MITAD DEL INSTRUMENTO (medido en esta tarjeta): si la primera captura se saca DESPUES de
# lanzar las teclas, la primera pulsacion ya movio al personaje y la foto que deberia ser la del arranque sale
# con el personaje en otro lado (salio con pos=382,240 en vez de 320,240 y el corpus la comparaba como si
# fuera el cuadro quieto).
capturar 0 f00-parado

# las teclas, en segundo plano, con UN SOLO teclado virtual (medido: tres aparatos distintos y el emulador
# solo ve el primero).
#
# OJO CON LA DURACION (medido): el personaje camina 1,875 celdas del mapa por segundo, o sea ~85 px de pantalla
# por segundo hacia la derecha: con las flechas sostenidas 4 s se iba FUERA de la ventana y las capturas
# quedaban sin personaje. Con 1,5 s la corrida entera entra en el cuadro. Y la PAUSA entre tramos (5 s) es
# para que la captura de un tramo, que se saca cuando la tecla YA se solto, no se cruce con la pulsacion del
# tramo siguiente.
if [ "${SIN_TECLAS:-0}" != "1" ]; then
    (
        PAUSA="${PAUSA:-5.0}" "$PYTHON" "$HERE/teclas-virtuales.py" guion derecha 1.0 abajo 1.0 arriba+derecha 1.0
    ) > "$OUT/teclas.log" 2>&1 &
    TECLAS_PID=$!
    echo "  teclas en segundo plano, con UN solo teclado virtual (1,0 s por flecha, 5 s de pausa)"
else
    echo "  SIN_TECLAS=1: no se manda ninguna tecla (linea de base)"
fi

# ---- 3. LAS CAPTURAS, ATADAS A LA MARCA DEL POC Y NO AL RELOJ
# POR QUE (medido en esta tarjeta): con la linea de tiempo por reloj (capturar a los 1, 3, 5, 8 y 11 s del
# arranque de las teclas) las capturas SE CORREN respecto de las pulsaciones: cada `spectacle` + la espera a
# que el PNG se escriba tarda uno o dos segundos, y el resultado medido fue que la captura del tramo "abajo"
# salio DESPUES de que la diagonal ya habia movido al personaje, asi que la pantalla no se movio entre dos
# capturas consecutivas (0 px contra 0 px del motor) y el criterio 2 se cayo por el INSTRUMENTO, no por el POC.
#
# Ahora la captura de cada tramo espera la marca del PROPIO POC: primero la pulsacion (una linea
# "HITO238: CONTROL botones <bits>") y despues su suelta ("botones 0"). Cuando la tecla se suelta el personaje
# ya no camina, y en 1,8 s el POC deja al menos una marca "HITO238: POS" nueva (las deja cada 30 cuadros,
# ~1,5 s): la linea POS que el script lee para la captura es entonces la posicion FINAL de ese tramo.
# (Las funciones `cuenta`/`esperar_cuenta`/`tras_tecla` y la captura del personaje PARADO estan mas arriba,
# antes de lanzar las teclas: la foto "parado" tiene que ser anterior a la primera pulsacion.)
ok=1
if [ "${SIN_TECLAS:-0}" = "1" ]; then
    # linea de base: mismas capturas, mismo orden y mismos nombres, pero sin esperar pulsaciones
    for paso in "3 f03-derecha" "5 f05-abajo" "8 f08-diagonal"; do
        t="${paso%% *}"; nombre="${paso##* }"
        sleep 2
        if vivo; then capturar "$t" "$nombre"; else ok=0; break; fi
    done
else
    tras_tecla 128 f03-derecha || ok=0
    [ "$ok" = 1 ] && { tras_tecla 32 f05-abajo || ok=0; }
    [ "$ok" = 1 ] && { tras_tecla 144 f08-diagonal || ok=0; }
    if [ "$ok" != 1 ]; then
        echo "  la corrida NO sirve: no llegaron las tres pulsaciones enteras del control"
    fi
fi

# el personaje se queda quieto: una captura final para ver que NO se mueve sin teclas
sleep 2
if vivo; then
    capturar 11 f11-quieto
fi

[ "${SIN_TECLAS:-0}" != "1" ] && wait "${TECLAS_PID:-0}" 2>/dev/null

sleep 1
kill -TERM -"$PID" 2>/dev/null || kill -TERM "$PID" 2>/dev/null
pkill -f "$PATRON" 2>/dev/null
sleep 1
kill -KILL -"$PID" 2>/dev/null
pkill -9 -f "$PATRON" 2>/dev/null
qdbus6 org.kde.KWin /Scripting org.kde.kwin.Scripting.unloadScript "regla-$TOKEN" >/dev/null 2>&1

echo
echo "== consola serie: la posicion del personaje (HITO238: POS) y el control"
grep -a 'HITO238: POS\|HITO238: CONTROL\|HITO238: MUTANTE' "$OUT/flycast.log" | tr -d '\r' | tail -40
echo
echo "== consola serie: lo demas del POC"
grep -aE 'HITO238' "$OUT/flycast.log" | tr -d '\r' | grep -av 'HITO238: POS\|HITO238: CONTROL' | tail -40
echo
echo "== pantalla"; ls -la "$OUT"/f*.png 2>/dev/null | awk '{print "   "$5" bytes  "$9}'
echo "== teclas"; cat "$OUT/teclas.log" 2>/dev/null | tail -8

# si no llegaron las tres pulsaciones enteras, la corrida NO sirve: se sale con error para que el corpus
# la reintente (medido en esta tarjeta: en una de cada tres corridas el control llega a medias y medir eso
# seria medir el instrumento, no el POC)
if [ "$ok" != 1 ]; then
    echo "== CORRIDA-INCOMPLETA (el control no llego entero: ver las lineas de arriba)"
    exit 1
fi
exit 0
