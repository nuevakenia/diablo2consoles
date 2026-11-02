#!/bin/bash
# DC-002.117 -- TRES Y CUATRO JUGADORES: LA TABLA DE MEMORIA Y MS POR CUADRO.
#
# QUE SALE DE ACA. El MISMO POC de DC-002.116 con la palanca `JUGADORES` llevada hasta CUATRO, que es lo que la
# tarjeta pide: los cuatro puertos del bus maple, cada jugador con su control, su estado y su reloj de animacion, y
# los cuatro dibujados en el MISMO cuadro de 640x480.
#   1. CUATRO PARTICIPANTES, uno por PUERTO del bus maple (0..3). El que no tiene aparato en su puerto va por el
#      GUION del shim (el log dice, puerto por puerto, cual de los dos caminos corrio).
#   2. LOS CUADROS COMPARTIDOS: los `.dcc` decodificados y pegados se componen UNA vez y los cuatro jugadores leen
#      de ahi. Lo que es por jugador es el ESTADO (104 B) y la estructura del participante (su paleta de 1.024 B).
#      Las dos cosas las imprime el POC en la linea COOP MEMORIA.
#   3. EL LEASH DE GRUPO: con tres y cuatro jugadores el leash de a pares de DC-002.115 no alcanza (el grupo tiene
#      diametro y la camara va en el centro), asi que el limite pasa a ser la distancia de CADA jugador al centro
#      del grupo: `leashCeldas/2`. Es la misma medida de DC-002.115 leida al reves.
#   4. LA TABLA: la MISMA compilacion corrida con `JUGADORES=1`, `2`, `3` y `4` da la memoria y los ms por cuadro de
#      los cuatro casos. Nada mas cambia entre una corrida y otra.
#   5. NO HAY SPLIT PARA TRES NI CUATRO (es otra tarjeta): `prepararElSplit` rechaza cualquier numero que no sea 2 y
#      lo dice. El camino del split sigue entero y CORRE (medido): `SPLIT=si JUGADORES=2` da los dos lienzos de
#      320x480, las dos camaras y el ancla 160,283 en las dos mitades. Para que sea el MISMO cuadro de DC-002.116
#      falta ademas el arranque de aquella tarjeta, que aca cambio a proposito (el grupo de cuatro): con
#      `CELDA_P2="24 -8"` los dos vuelven a estar a 25 celdas (ver la evidencia, el A/B del split).
#
# LOS DOS ARCHIVOS COMPARTIDOS. `personaje.go` y `jugador.go` del POC son BYTE A BYTE los de los medidores del
# host (`console/dc/medidas/render-294/`), y `personaje.go` es el que trae la velocidad del motor (d2path) y la
# direccion del piso (`DireccionDelPiso`, que es lo que mide el leash). El script lo verifica con `cmp`: si se
# edita uno solo de un lado, la comparacion entre consola y host no vale nada.
#
# EL MAPLE, POR PUERTO (DC-002.114). `d2core/d2input/maple` se RECOMPILA aca porque esa tarjeta le agrego el lector
# POR PUERTO (`ActualizarDesdeElControlEn`, con `maple_enum_dev`) y la inyeccion POR PUERTO. El lector de C
# (`d2c_control_leer_puerto`, `d2c_control_inyectar_puerto`) vive en `console/dc/std/csrc/control_dc.c`, que
# `build-dc.sh` compila SIEMPRE: no hay que tocar nada para tenerlo.
#
# EL BACKEND (DC-002.116). `d2core/d2render/pvr` se recompila aca porque esa tarjeta le agrego tres cosas
# ADITIVAS: `UsarSuperficieDelCuadro` y `UsarPresentador` (el ciclo deja de presentar el lienzo de la pantalla),
# `PresentarEnColumna`/`SuperficieEnBloque`, y del lado de C la conversion a una COLUMNA de la textura
# (`d2c_pvr_pantalla_convertir_en`, en `console/dc/std/csrc/pvr_pantalla.c`, que `build-dc.sh` compila siempre).
# Sin las palancas puestas, el camino de todos los POC anteriores es exactamente el de antes.
#
# Uso:  scripts/console/build-juego-117.sh [dir_salida]
#       BASE_GOX=...    arbol del motor compilado (por defecto /tmp/gox-296, el de DC-002.96/.99)
#       SALTAR_GEN=1    reusar el arbol de enlace ya armado
#       SALTAR_MOTOR=1  NO recompilar los paquetes del motor (solo cambio el POC: los .gox son los mismos)
#
#       LAS PALANCAS DEL PUEBLO (las mismas de DC-002.105...112): ORDEN=, TRAMO_CACHE=, ENVRAM=, PEDAZO=,
#       SONDA=, TRAMO_ARRANQUE=, SOLTAR=, ARENA_DS1=, SCRATCH_POR_ARCHIVO=, TRAMO_ARENA=, TRAMOS_AL_CAMBIAR=.
#       LAS DEL JUGADOR: JUGADOR=encendido|apagado, CAMARA=siguiendo|fija, DIRS_CAMINANDO=<n>, CUADROS=<n>.
#       LAS DEL COOPERATIVO (DC-002.114): JUGADORES=1|2|3|4, COMPARTIR=si|no, GUION_P2=si|no, INYECTAR_P1=si|no,
#       CELDA_P2 / CELDA_P3 / CELDA_P4 = "<x> <y>" (donde arranca cada uno, en celdas desde la celda del POC).
#       LAS DEL LEASH (DC-002.115): LEASH=<celdas> (0 = la MEDIDA al arranque) y GUION_POC=leash|coop|cuatro.
#       LAS DE ESTA TARJETA (DC-002.117): GRUPO=si|no (el leash de grupo de 3 y 4 | el de a pares, para el A/B) y
#       GUION_POC=cuatro (los tres tramos de 2 s: los cuatro juntos, cada uno en su direccion, y los cuatro de
#       vuelta). El SPLIT queda apagado por omision y solo corre con JUGADORES=2.
#       Toda palanca se pisa con AVISO EN GRANDE: el knobs.go del POC es la unica fuente de verdad (DC-002.90).
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"
# shellcheck source=palancas-del-build.sh
. "$HERE/palancas-del-build.sh"   # DC-002.107: las exclusiones del build, por el camino unico

REPO="$(cd "$HERE/../.." && pwd)"
SRC="$REPO/console/dc/poc/juego-117"
MEDIDOR="$REPO/console/dc/medidas/render-294"
BASE_GOX="${BASE_GOX:-/tmp/gox-296}"
OUT="${1:-/tmp/dcbuild/juego117}"
GEN="$OUT/gox-enlace"
PREFIJO="github.com/OpenDiablo2/OpenDiablo2"
SRCB="$OUT/src"

# los paquetes que este build recompila en el arbol, en orden (cada uno contra los .gox de los anteriores).
# Es la lista de DC-002.103 (.106...111) MAS la cadena del personaje: `d2cof`/`d2dcc`/`d2dc6` (el COF y las
# capas), `d2path` (las rutas de las capas del COF) y `d2math/d2vector` (el subtile y el RenderOffset).
PAQUETES_MOTOR="d2common/d2datautils
 d2common/d2data/d2compression
 d2common/d2math/d2vector
 d2common/d2fileformats/d2dt1
 d2common/d2fileformats/d2ds1
 d2common/d2fileformats/d2mpq
 d2common/d2fileformats/d2cof
 d2common/d2fileformats/d2dc6
 d2common/d2fileformats/d2dcc
 d2common/d2cache
 d2common/d2fileformats/d2txt
 d2common/d2loader/asset/types
 d2common/d2loader/asset
 d2common/d2loader/mpq
 d2common/d2loader/filesystem
 d2common/d2loader
 d2common/d2path
 d2core/d2records
 d2core/d2asset
 d2core/d2term
 d2core/d2render/pvr
 d2core/d2input/maple
 d2core/d2map/d2lightmap
 d2core/d2map/d2mapentity
 d2core/d2map/d2mapstamp
 d2core/d2map/d2mapengine
 d2core/d2map/d2maprenderer
 console/dc/fuenteconsola"

[ -f "$BASE_GOX/orden.txt" ] || { echo "falta el arbol del motor en $BASE_GOX (corre build-motor-all.sh)" >&2; exit 1; }
mkdir -p "$OUT"

echo "== 0/6 los dos archivos compartidos con el medidor del host"
for f in personaje.go jugador.go; do
    if ! cmp -s "$SRC/$f" "$MEDIDOR/$f"; then
        echo "   FALLO: $SRC/$f y $MEDIDOR/$f NO son el mismo archivo: la comparacion consola/host no vale" >&2
        exit 1
    fi
    echo "   $f $(sha256sum "$SRC/$f" | cut -c1-16)"
done

# ============================== EL ARBOL DE .gox (DC-002.91) ==============================
BASE_GOX="$BASE_GOX" RECOMPILA="$PAQUETES_MOTOR" "$HERE/revisar-arbol-gox.sh"

echo "== 1/6 los paquetes que se compilan aca (el motor que se dibuja)"
aviso_mutante_lookups
if [ "${SALTAR_MOTOR:-0}" = "1" ]; then
    echo "   SALTAR_MOTOR=1: se reusan los .gox del arbol $BASE_GOX"
    PAQUETES_MOTOR=""
fi
for p in $PAQUETES_MOTOR; do
    # DC-002.107 -- la exclusion de la tabla de OBJECTLOOKUPS por el camino unico (`palancas-del-build.sh`).
    mapfile -t SRC_P < <(fuentes_del_paquete "$REPO" "$p")
    mkdir -p "$(dirname "$BASE_GOX/$PREFIJO/$p.gox")"
    sh-elf-gccgo $GODC_GOFLAGS -I "$BASE_GOX" \
        -fgo-pkgpath="$PREFIJO/$p" \
        -c "${SRC_P[@]}" \
        -o "$BASE_GOX/$PREFIJO/$p.gox"
    printf '   %-34s %9s B\n' "$(basename "$p").gox" "$(stat -c%s "$BASE_GOX/$PREFIJO/$p.gox")"
done

# el cierre del POC, con `go list -deps` sobre el paquete del POC
PAQUETES="$(cd "$REPO" && go list -deps -e -tags dreamcast ./console/dc/poc/juego-117 2>/dev/null |
    grep "$PREFIJO" | sed "s|.*$PREFIJO/||" | grep -vx 'console/dc/poc/juego-117' |
    sort | tr '\n' ' ')"
[ -n "${PAQUETES// /}" ] || { echo "la lista de paquetes quedo vacia (go list -deps fallo?)" >&2; exit 1; }

echo "== 2/6 el arbol de enlace ($GEN)"
if [ "${SALTAR_GEN:-0}" != "1" ]; then
    rm -rf "$GEN"
    mkdir -p "$GEN"
    # enlaces simbolicos, no copias: los .gox del motor son decenas de MB y no se tocan desde aca
    cp -rs "$BASE_GOX/." "$GEN"/
fi
rm -f "$GEN/orden.txt"
# DC-002.101/.108: las rutas de `orden.txt` vienen con el prefijo del arbol BASE (o con el de la carpeta donde se
# armo, si el arbol se copio): se DEDUCE del archivo y se reescribe ese, para que el enlazador no vea dos veces el
# mismo .gox ("multiple definition of ...").
RAIZ_ORDEN="$(sed -n '1{s|/[^/]*$||;p}' "$BASE_GOX/orden.txt")"
sed -e "s|^$BASE_GOX/|$GEN/|" -e "s|^$RAIZ_ORDEN/|$GEN/|" "$BASE_GOX/orden.txt" > "$GEN/orden.txt"
for p in $PAQUETES; do
    f="$GEN/$PREFIJO/$p.gox"
    [ -f "$f" ] || { echo "   FALTA $p.gox en el arbol del motor: no se puede enlazar lo que no existe" >&2; exit 1; }
    grep -qxF "$f" "$GEN/orden.txt" || echo "$f" >> "$GEN/orden.txt"
done
echo "   paquetes del motor en el enlace: $(printf '%s\n' $PAQUETES | wc -l)"
echo "   objetos en orden.txt: $(wc -l < "$GEN/orden.txt")"

echo "== 3/6 el directorio de fuentes del POC"
rm -rf "$SRCB"
mkdir -p "$SRCB"
cp "$SRC"/*.go "$SRCB"/
cp -r "$SRC/csrc" "$SRCB"/

# ============================== LAS PALANCAS (DC-002.90) ==============================
# UNA SOLA FUENTE DE VERDAD: el knobs.go del POC. El entorno solo puede PISARLO y, cuando lo pisa, se dice aca y
# el valor queda escrito en el knobs.go del directorio de build (el binario dice con cual se compilo).
leer_knob() {  # $1 = const de knobs.go -> imprime su valor tal cual
    awk -v n="$1" '
        {
            if ($1 == "const") { nom = $2; igual = $3; val = $4 } else { nom = $1; igual = $2; val = $3 }
            if (nom == n && igual == "=" && !visto) { print val; visto = 1; exit }
        }' "$SRCB/knobs.go" | tr -d '"'
}

ORDEN_KNOB="$(leer_knob orden)"
[ -n "$ORDEN_KNOB" ] || { echo "   knobs.go no declara 'orden'" >&2; exit 1; }
if [ -n "${ORDEN:-}" ] && [ "$ORDEN" != "$ORDEN_KNOB" ]; then
    echo "   ****************************************************************"
    echo "   * PISADA DE PALANCA: orden=$ORDEN (knobs.go dice $ORDEN_KNOB)  *"
    echo "   * el binario NO es el de la referencia del POC                  *"
    echo "   ****************************************************************"
    sed -i "s/^\torden = \".*\"$/\torden = \"$ORDEN\"/" "$SRCB/knobs.go"
    ORDEN_KNOB="$ORDEN"
fi
echo "   orden=$ORDEN_KNOB cuadros=$(leer_knob cuadros) modoTablas=$(leer_knob modoTablas)"

pisa_knob() {  # $1 = nombre en knobs.go, $2 = valor nuevo (sin comillas), $3 = comillas (1 = cadena)
    local actual
    actual="$(leer_knob "$1")"
    [ -n "$actual" ] || { echo "   knobs.go no declara '$1'" >&2; exit 1; }
    [ "$actual" = "$2" ] && return 0
    echo "   ****************************************************************"
    printf '   * PISADA DE PALANCA: %-24s %s -> %s\n' "$1" "$actual" "$2"
    echo "   * el binario NO es el de la referencia del POC"
    echo "   ****************************************************************"
    if [ "${3:-0}" = "1" ]; then
        sed -i "s/^\t$1 = \".*\"$/\t$1 = \"$2\"/" "$SRCB/knobs.go"
    else
        sed -i "s/^\t$1 = .*/\t$1 = $2/" "$SRCB/knobs.go"
    fi
}

pisa_bool() {  # $1 = nombre, $2 = lo que vino del entorno (0/1/si/no/true/false) -> pisa la palanca booleana
    case "$2" in
        0|no|false) pisa_knob "$1" false ;;
        1|si|true) pisa_knob "$1" true ;;
        *) pisa_knob "$1" "$2" ;;
    esac
}

[ -n "${TRAMO_CACHE:-}" ] && pisa_knob tramoDeLaCache "$TRAMO_CACHE"
[ -n "${ENVRAM:-}" ] && pisa_bool cacheEnVram "$ENVRAM"
[ -n "${PEDAZO:-}" ] && pisa_knob tramoEnPedazos "$PEDAZO"
[ -n "${SONDA:-}" ] && pisa_bool pedazoLoDiceElMonton "$SONDA"
[ -n "${TRAMO_ARRANQUE:-}" ] && pisa_bool tramoAlArranque "$TRAMO_ARRANQUE"
[ -n "${SOLTAR:-}" ] && pisa_knob soltarDespuesDeComponer "$SOLTAR" 1
[ -n "${ARENA_DS1:-}" ] && pisa_knob arenaDelDS1 "$ARENA_DS1" 1
[ -n "${SCRATCH_POR_ARCHIVO:-}" ] && pisa_bool scratchPorArchivo "$SCRATCH_POR_ARCHIVO"
[ -n "${TRAMO_ARENA:-}" ] && pisa_bool tramoDelArenaPorElMonton "$TRAMO_ARENA"
[ -n "${TRAMOS_AL_CAMBIAR:-}" ] && pisa_bool soltarTramosAlCambiarDeNivel "$TRAMOS_AL_CAMBIAR"

# LAS DE ESTA TARJETA
[ -n "${JUGADOR:-}" ] && pisa_knob jugadorKnob "$JUGADOR" 1
[ -n "${MODO_JUGADOR:-}" ] && pisa_knob modoCaminando "$MODO_JUGADOR" 1
[ -n "${CAMARA:-}" ] && pisa_knob camara "$CAMARA" 1
[ -n "${DIRS_CAMINANDO:-}" ] && pisa_knob dirsCaminando "$DIRS_CAMINANDO"
[ -n "${CUADROS:-}" ] && pisa_knob cuadros "$CUADROS"
[ -n "${INYECTAR:-}" ] && pisa_bool inyectarGuion "$INYECTAR"
[ -n "${UMBRAL_GC:-}" ] && pisa_knob umbralDelRecolector "$UMBRAL_GC"

echo "   soltarDespuesDeComponer=$(leer_knob soltarDespuesDeComponer) cacheEnVram=$(leer_knob cacheEnVram) tramoDeLaCache=$(leer_knob tramoDeLaCache) tramoEnPedazos=$(leer_knob tramoEnPedazos) pedazoLoDiceElMonton=$(leer_knob pedazoLoDiceElMonton) tramoAlArranque=$(leer_knob tramoAlArranque)"
echo "   JUGADOR=$(leer_knob jugadorKnob) modoQuieto=$(leer_knob modoQuieto) modoCaminando=$(leer_knob modoCaminando) dirsCaminando=$(leer_knob dirsCaminando) camara=$(leer_knob camara) inyectarGuion=$(leer_knob inyectarGuion) cuadros=$(leer_knob cuadros) arenaDelDS1=$(leer_knob arenaDelDS1)"

# LAS DEL COOPERATIVO (DC-002.114): CUANTOS JUGADORES Y DONDE ARRANCA CADA UNO. DC-002.117 lleva JUGADORES hasta 4.
[ -n "${JUGADORES:-}" ] && pisa_knob jugadoresKnob "$JUGADORES"
[ -n "${COMPARTIR:-}" ] && pisa_knob compartirJugador2 "$COMPARTIR" 1
[ -n "${GUION_P2:-}" ] && pisa_knob guionJugador2 "$GUION_P2" 1
[ -n "${INYECTAR_P1:-}" ] && pisa_knob inyectarJugador1 "$INYECTAR_P1" 1
[ -n "${CELDA_P2:-}" ] && { set -- $CELDA_P2; pisa_knob celdaP2X "$1"; pisa_knob celdaP2Y "$2"; }
[ -n "${CELDA_P3:-}" ] && { set -- $CELDA_P3; pisa_knob celdaP3X "$1"; pisa_knob celdaP3Y "$2"; }
[ -n "${CELDA_P4:-}" ] && { set -- $CELDA_P4; pisa_knob celdaP4X "$1"; pisa_knob celdaP4Y "$2"; }

echo "   COOP jugadores=$(leer_knob jugadoresKnob) compartir=$(leer_knob compartirJugador2) guionP2=$(leer_knob guionJugador2) inyectarP1=$(leer_knob inyectarJugador1) celdaP2=$(leer_knob celdaP2X),$(leer_knob celdaP2Y) celdaP3=$(leer_knob celdaP3X),$(leer_knob celdaP3Y) celdaP4=$(leer_knob celdaP4X),$(leer_knob celdaP4Y)"

# LAS DEL LEASH (DC-002.115). LEASH=0 (o sin variable) = la distancia que el POC MIDE al arranque.
[ -n "${LEASH:-}" ] && pisa_knob leashCeldas "$LEASH"
[ -n "${GUION_POC:-}" ] && pisa_knob guionPoc "$GUION_POC" 1
# DC-002.117: el leash de GRUPO de 3 y 4 jugadores (GRUPO=no deja el de a pares de DC-002.115, para el A/B).
[ -n "${GRUPO:-}" ] && pisa_bool leashDeGrupo "$GRUPO"

echo "   LEASH celdas=$(leer_knob leashCeldas) guionPoc=$(leer_knob guionPoc) leashDeGrupo=$(leer_knob leashDeGrupo) cuadros=$(leer_knob cuadros)"

# LAS DEL SPLIT (DC-002.116): dos lienzos de 320x480 o el cuadro unico. DC-002.117 lo deja APAGADO por omision y solo
# corre con JUGADORES=2 (el split de 3 y 4 es otra tarjeta).
[ -n "${SPLIT:-}" ] && pisa_knob splitKnob "$SPLIT" 1

echo "   SPLIT=$(leer_knob splitKnob) celdaP2=$(leer_knob celdaP2X),$(leer_knob celdaP2Y) tramoDeLaCache=$(leer_knob tramoDeLaCache)"

echo "== 4/6 compilando y enlazando contra los .gox del motor"
GODC_GOX="$GEN" bash "$HERE/build-dc.sh" "$SRCB" "$OUT" > "$OUT/build.log" 2>&1 || {
    echo "   FALLO el build (ver $OUT/build.log)"; tail -25 "$OUT/build.log"; exit 1; }

# DC-002.107 -- LA GUARDIA: el build NO SALE con la tabla de objectLookups adentro (se mide el BINARIO, no la
# bandera: un simbolo de datos de mas de 1 MB es, hoy, el volcado de la tabla de 1.672.892 B).
if lleva_la_tabla_de_lookups "$OUT/prog.elf"; then
    echo "   FALLA: $OUT/prog.elf lleva la tabla de objectLookups:" >&2
    lleva_la_tabla_de_lookups "$OUT/prog.elf" | sed 's/^/      /' >&2
    echo "   la lista de fuentes del build es scripts/console/palancas-del-build.sh (DC-002.107)" >&2
    [ -f "$OUT/1ST_READ.BIN" ] && mv -f "$OUT/1ST_READ.BIN" "$OUT/1ST_READ.BIN.CON-LA-TABLA"
    exit 1
fi

echo "== 5/6 el disco con los TRES MPQ en /JUEGO"
D2DIR="${D2_DIR:-$HOME/.wine-d2/drive_c/Games/Diablo II}"
MPQ_PATCH="$(ls "$D2DIR" | grep -ix 'patch_d2.mpq' | head -1)"
MPQ_DATA="$(ls "$D2DIR" | grep -ix 'd2data.mpq' | head -1)"
MPQ_CHAR="$(ls "$D2DIR" | grep -ix 'd2char.mpq' | head -1)"
[ -n "$MPQ_PATCH" ] || { echo "   no encuentro patch_d2.mpq en $D2DIR" >&2; exit 1; }
[ -n "$MPQ_DATA" ] || { echo "   no encuentro d2data.mpq en $D2DIR" >&2; exit 1; }
[ -n "$MPQ_CHAR" ] || { echo "   no encuentro d2char.mpq en $D2DIR" >&2; exit 1; }
LISTA="$OUT/iso-lista.txt"
{
    printf '%s:/JUEGO/PATCH_D2.MPQ;1\n' "$D2DIR/$MPQ_PATCH"
    printf '%s:/JUEGO/D2DATA.MPQ;1\n' "$D2DIR/$MPQ_DATA"
    printf '%s:/JUEGO/D2CHAR.MPQ;1\n' "$D2DIR/$MPQ_CHAR"
} > "$LISTA"
MKISO_JOLIET=0 "$DC_PYTHON" "$HERE/mkiso.py" "$OUT/disc.iso" "$OUT/IP.BIN" "$OUT/1ST_READ.BIN" "@$LISTA" | tail -2
"$DC_PYTHON" "$HERE/mkgdi.py" "$OUT" | tail -2

echo "== 6/6 el binario"
for f in prog.elf 1ST_READ.BIN disc.iso; do
    [ -f "$OUT/$f" ] && printf '   %-14s %10s B\n' "$f" "$(stat -c%s "$OUT/$f")"
done
printf '   codigo (.text)  %10s B\n' "$(sh-elf-size -A "$OUT/prog.elf" | awk '$1==".text"{print $2}')"
printf '   data+bss        %10s B\n' "$(sh-elf-size -A "$OUT/prog.elf" | awk '$1==".data"||$1==".bss"{s+=$2} END{print s}')"
echo "   LOS NUMEROS QUE TIENE QUE DECIR EL LOG DE LA CORRIDA:"
echo "     JUEGO117: LEASH CAJA-PEOR izq= <n> arriba= <n> ancho= 26 alto= 79 de= quieto|caminando"
echo "     JUEGO117: LEASH AUTOCHEQUEO camara= 28 20 ancla= 320 283 esperado= 320 283 igual= true"
echo "     JUEGO117: LEASH MEDIDA peorCeldas= 5.25 leashCeldas= 5.25"
echo "     JUEGO117: LEASH GRUPO jugadores= 4 deGrupo= true radioDelLeash= 2.625 separacionAlArrancar= 4.2426"
echo "     JUEGO117: PUERTO jugador= P1..P4 puerto= 0..3 inyecta= si (no hay cuatro controles en el bus)"
echo "     JUEGO117: COOP MEMORIA jugadores= 4 cuadrosCompartidosBytes= 165152 estadoPorJugadorBytes= 104"
echo "     JUEGO117: COOP TRAMO LOS-CUATRO-DERECHA ... jugador= P1..P4 ticks= <n> celdas= <n>"
echo "     JUEGO117: LEASH RESUMEN jugadores= 4 leashCeldas= 5.25 deGrupo= true cuadrosFuera= 0"
echo "     JUEGO117: COOP RESUMEN jugadores= 4 cuadrosPintados= 17 msMapa= <n> msJugadores= <n> msCuadro= <n>"
echo "     (con JUGADORES=1 no hay leash ni grupo: el cuadro es el de DC-002.113)"
echo "     (con SPLIT=si y JUGADORES=2, los de DC-002.116: SPLIT LIENZOS, SPLIT CAMARAS, SPLIT RESUMEN)"
[ -f "$OUT/disc.gdi" ] && echo "   imagen:         $OUT/disc.gdi"
