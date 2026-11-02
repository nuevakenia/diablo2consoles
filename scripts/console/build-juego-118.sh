#!/bin/bash
# DC-002.118 -- EL SPLIT DE TRES Y CUATRO: UNA REJILLA DE RECUADROS CON SU CAMARA CADA UNO.
#
# QUE SALE DE ACA. El POC de DC-002.120 (los cuatro jugadores en el mismo cuadro de 640x480, uno por PUERTO del
# bus maple, con el leash de grupo y el ciclo de niveles) con el SPLIT llevado de DOS a CUATRO: cada jugador tiene
# SU recuadro de la pantalla y SU camara.
#   1. El reparto de la pantalla para 1, 2, 3 y 4 jugadores (`repartirElSplit`): 640x480, dos columnas de 320x480,
#      tres recuadros (1 grande + 2 chicas, o tres tiras) y cuatro cuadrantes de 320x240.
#   2. Lo MEDIDO de cada caso: la VRAM de los lienzos, los ms por cuadro (mapa + jugadores + presentar) y la cache
#      de tiles con N zonas del mapa (imagenes y derrames), que es el recurso que se acaba antes que la VRAM.
#   3. El criterio: cuatro jugadores en cuatro cuadrantes, cada uno con su camara y su jugador adentro.
#
# LO QUE SE TOCA DEL POC (y nada mas): `split.go` entero (los N recuadros, N camaras y N lienzos; el reparto por
# tabla) y las palancas de esta tarjeta en `knobs.go`. El camino del mapa, el del jugador, el del leash y el del
# ciclo son los de DC-002.120 sin una linea cambiada: con `SPLIT=no` la corrida es la de aquella tarjeta.
#
# LO QUE SE TOCA DEL MOTOR: DOS PIEZAS NUEVAS Y ADITIVAS en el backend del PVR, y ninguna cambia el camino de los
# POC anteriores:
#   - `d2c_pvr_pantalla_convertir_en_bloque` (C, `console/dc/std/csrc/pvr_pantalla.c`): la conversion de DC-002.116
#     con la FILA del destino -- los cuatro cuadrantes son DOS filas y sin y_dest los de abajo se escribian encima
#     de los de arriba. Exige destino multiplo de 32 (los bloques de la textura son 32x32).
#   - `d2c_pvr_pantalla_pintar_bloque` (C) y `pvr.PresentarEnBloque`/`pvr.PintarEnBloque` (Go): un lienzo a una
#     region de la textura y un quad de esa region a un recuadro de la pantalla. La fila de abajo vive en la 256 de
#     la textura (multiplo de 32) y se pinta en la 240 de la pantalla, asi que textura y pantalla ya no coinciden
#     y hace falta decir las dos posiciones.
# Por eso este build NO puede usar `SALTAR_MOTOR=1` la primera vez: recompila `d2core/d2render/pvr`, y
# `build-dc.sh` compila siempre el C del std.
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
# Uso:  scripts/console/build-juego-118.sh [dir_salida]
#       BASE_GOX=...    arbol del motor compilado (por defecto /tmp/gox-296, el de DC-002.96/.99)
#       SALTAR_GEN=1    reusar el arbol de enlace ya armado
#       SALTAR_MOTOR=1  NO recompilar los paquetes del motor (solo cambio el POC: los .gox son los mismos).
#                       OJO: vale SOLO despues de un build de esta tarjeta (el de `d2render/pvr` tiene que estar).
#
#       LAS PALANCAS DEL PUEBLO (las mismas de DC-002.105...112): ORDEN=, TRAMO_CACHE=, ENVRAM=, PEDAZO=,
#       SONDA=, TRAMO_ARRANQUE=, SOLTAR=, ARENA_DS1=, SCRATCH_POR_ARCHIVO=, TRAMO_ARENA=, TRAMOS_AL_CAMBIAR=.
#       LAS DEL JUGADOR: JUGADOR=encendido|apagado, CAMARA=siguiendo|fija, DIRS_CAMINANDO=<n>, CUADROS=<n>.
#       LAS DEL COOPERATIVO (DC-002.114): JUGADORES=1|2|3|4, COMPARTIR=si|no, GUION_P2=si|no, INYECTAR_P1=si|no,
#       CELDA_P2 / CELDA_P3 / CELDA_P4 = "<x> <y>" (donde arranca cada uno, en celdas desde la celda del POC).
#       LAS DEL LEASH (DC-002.115): LEASH=<celdas> (0 = la MEDIDA al arranque) y GUION_POC=leash|coop|cuatro.
#       LAS DEL GRUPO (DC-002.117): GRUPO=si|no (el leash de grupo de 3 y 4 | el de a pares, para el A/B).
#       LAS DEL CICLO (DC-002.120): FASE=ciclo|apagado (ACA POR OMISION apagado), CUADROS_POR_FASE=<n>,
#       ORDEN_FASES=a|b, TRANSICION="<preset> <archivo>" y SOLTAR_EN_EL_CICLO=si|no.
#       LAS DE ESTA TARJETA (DC-002.118): SPLIT=si|no (la rejilla | la pantalla entera con el leash) y
#       REPARTO3=grande|tiras (la forma del split de TRES).
#       Toda palanca se pisa con AVISO EN GRANDE: el knobs.go del POC es la unica fuente de verdad (DC-002.90).
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"
# shellcheck source=palancas-del-build.sh
. "$HERE/palancas-del-build.sh"   # DC-002.107: las exclusiones del build, por el camino unico

REPO="$(cd "$HERE/../.." && pwd)"
SRC="$REPO/console/dc/poc/juego-118"
MEDIDOR="$REPO/console/dc/medidas/render-294"
BASE_GOX="${BASE_GOX:-/tmp/gox-296}"
OUT="${1:-/tmp/dcbuild/juego118}"
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
PAQUETES="$(cd "$REPO" && go list -deps -e -tags dreamcast ./console/dc/poc/juego-118 2>/dev/null |
    grep "$PREFIJO" | sed "s|.*$PREFIJO/||" | grep -vx 'console/dc/poc/juego-118' |
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

# LAS DEL COOPERATIVO (DC-002.114): CUANTOS JUGADORES Y DONDE ARRANCA CADA UNO. DC-002.118 lleva JUGADORES hasta 4.
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
# DC-002.118: el leash de GRUPO de 3 y 4 jugadores (GRUPO=no deja el de a pares de DC-002.115, para el A/B).
[ -n "${GRUPO:-}" ] && pisa_bool leashDeGrupo "$GRUPO"

# LAS DEL CICLO CON LOS CUATRO (DC-002.118): las tres fases del cambio de nivel con los cuatro adentro.
[ -n "${FASE:-}" ] && pisa_knob faseKnob "$FASE" 1
[ -n "${CUADROS_POR_FASE:-}" ] && pisa_knob cuadrosPorFase "$CUADROS_POR_FASE"
[ -n "${ORDEN_FASES:-}" ] && pisa_knob ordenDeLasFases "$ORDEN_FASES" 1
[ -n "${TRANSICION:-}" ] && { set -- $TRANSICION; pisa_knob transicionPreset "$1"; pisa_knob transicionArchivo "$2"; }
[ -n "${SOLTAR_EN_EL_CICLO:-}" ] && pisa_bool soltarEnElCiclo "$SOLTAR_EN_EL_CICLO"
[ -n "${SOLTAR_CAMINANDO:-}" ] && pisa_bool soltarCaminandoEnElCiclo "$SOLTAR_CAMINANDO"
[ -n "${CASILLAS:-}" ] && pisa_bool reusarLasCasillas "$CASILLAS"

echo "   FASE=$(leer_knob faseKnob) cuadrosPorFase=$(leer_knob cuadrosPorFase) ordenFases=$(leer_knob ordenDeLasFases) transicion=$(leer_knob transicionPreset),$(leer_knob transicionArchivo) soltarEnElCiclo=$(leer_knob soltarEnElCiclo)"

echo "   LEASH celdas=$(leer_knob leashCeldas) guionPoc=$(leer_knob guionPoc) leashDeGrupo=$(leer_knob leashDeGrupo) cuadros=$(leer_knob cuadros)"

# LAS DEL SPLIT (DC-002.116 y DC-002.118): la rejilla de recuadros (si) o el cuadro unico (no). DC-002.118 la deja
# ENCENDIDA por omision y el split llega a UNO, DOS, TRES y CUATRO.
[ -n "${SPLIT:-}" ] && pisa_knob splitKnob "$SPLIT" 1
# DC-002.118: la forma del split de TRES (grande = 1 grande + 2 chicas | tiras = tres tiras de 640x160).
[ -n "${REPARTO3:-}" ] && pisa_knob repartoDeTresKnob "$REPARTO3" 1

echo "   SPLIT=$(leer_knob splitKnob) repartoDeTres=$(leer_knob repartoDeTresKnob) celdaP2=$(leer_knob celdaP2X),$(leer_knob celdaP2Y) tramoDeLaCache=$(leer_knob tramoDeLaCache)"

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
echo "     JUEGO118: SPLIT REPARTO jugadores= 4 pantalla= 640 480 textura= 1024 512"
echo "     JUEGO118: SPLIT LIENZO i= 3 pantalla= 320 240 320 240 textura= 320 256 bytes= 307200"
echo "     JUEGO118: SPLIT LIENZOS n= 4 bytesTotales= 1228800 mismosBytesQueUnLienzoUnico= true"
echo "     JUEGO118: SPLIT CUADRO <n> lienzos= 4 crc32= <n> pixelesNoVacios= <n> de 307200 msMapa= <n>"
echo "     JUEGO118: SPLIT RECUADRO cuadro= <n> i= 3 jugador= P4 pantalla= 320 240 320 240 textura= 320 256"
echo "     JUEGO118: SPLIT CRITERIO cuadrosFueraDeSuRecuadro= 0 lienzos= 4"
echo "     JUEGO118: SPLIT MEMORIA vramTotal= <n> vramDisponible= <n> cacheImagenes= <n> cacheDerrames= <n>"
echo "     (con SPLIT=no la corrida es la de DC-002.120: un lienzo de 640x480 y el leash)"
echo "     (con JUGADORES=3 REPARTO3=tiras el reparto son tres tiras de 640x160)"
[ -f "$OUT/disc.gdi" ] && echo "   imagen:         $OUT/disc.gdi"
