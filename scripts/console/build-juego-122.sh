#!/bin/bash
# DC-002.122 -- EL GOLPE: EL JUGADOR LE PEGA AL MONSTRUO, LE BAJA LA VIDA Y LO MATA.
#
# QUE SALE DE ACA. El POC de DC-002.121 (el bicho compuesto del MPQ, patrullando y frenando al jugador) MAS EL GOLPE
# (`combate_122.go`): el boton Y del maple de CADA puerto, el A1 del heroe (13 cuadros a 40 ms), el alcance del exe
# (`alcance_122.go`, la tabla de distancias del motor y el 7 VERIFICADO de meleeInRange), la vida del bicho por el
# camino de siembra del motor (monstats.txt + MonLvl.txt, `monstruo.go`, el archivo COMPARTIDO con el verificador
# del host) y la muerte con su animacion (GH -> DT -> DD).
#
# LO QUE SE MIDE
#   1. El A1 del heroe: sus 13 cuadros compuestos (caja, pixeles, crc32 por cuadro) y el cuadro en el que cae el
#      golpe (la mitad: 7 de 13, la regla de checkEvents del motor).
#   2. La vida del bicho (2..3 HP para el Returned) bajando golpe a golpe, con el dano del dado del motor (1..2 a
#      mano limpia), y la muerte: GH al recibir, DT al morir, DD despues.
#   3. El ALCANCE: los golpes de la celda de al lado bajan vida; los de lejos se cuentan y NO bajan nada.
#   4. Los CUATRO pegandole al mismo bicho: ataques y dano POR JUGADOR y cuantos jugadores le bajaron vida.
#
# LAS PALANCAS DE ESTA TARJETA: COMBATE=si|no (no = la corrida de DC-002.121), GOLPE_AUTO=si|no (el instrumento que
# simula el flanco del boton Y, porque el emulador corre sin teclado), ALCANCE=<n> (la distancia de borde, 7 = el
# VERIFICADO del exe), DIRS_ATAQUE_LISTA="<n>,<n>" (QUE direcciones del A1 se componen, medidas con
# `sonda-dir-122`), MODOS_DIRS=<n> (en cuantas direcciones de la patrulla se compone GH/DT/DD), VIDA_X=<n> (el
# instrumento de la corrida de los cuatro: multiplica la vida del bicho y lo dice en el log) y CELDA_P1..CELDA_P4
# (donde arranca cada jugador: es lo que lo pone al lado del bicho o lejos, para el A/B del alcance).
#
# LA TABLA DEL ALCANCE, MEDIDA DESDE EL HOST (`go run ./console/dc/sonda-alcance-122`, misma funcion que la POC):
#   dx= 5 dy= 0  (la celda de al lado)      distancia= 5  alcance 7 -> LLEGA
#   dx= 0 dy= 5  (la celda de al lado)      distancia= 5  alcance 7 -> LLEGA
#   dx= 5 dy= 5  (la celda de al lado, en DIAGONAL) distancia= 8 -> NO llega
#   dx=10 dy= 0  (a dos celdas)             distancia=18  alcance 7 -> NO llega
# Ojo con la diagonal: es la trampa de esta tarjeta. El POC nace en (28,20) y el bicho en (29,21), o sea en
# DIAGONAL: con el jugador en su celda de siempre NINGUN golpe llega, y eso es correcto (la tabla del exe da 8).
# Por eso la corrida del golpe usa CELDA_P1="0 1" (la celda de al lado, en la misma fila).
#
# LA RAM DE ESTA TARJETA (el limite de verdad, medido): el A1 del heroe son 74.971 B de indices por direccion y el
# DT del bicho 171.304 B. El semiespacio del recolector es 1.835.008 B fijos (libgodc) y antes de esta tarjeta la
# imagen viva quedaba en 1.166.320 B (63 %). Con las 8 direcciones del A1 (499.525 B) queda al 96 % y el paso
# siguiente muere con `GC to-space overflow`; con 1 direccion del A1 y GH/DT/DD en 1 direccion queda al 91 % y
# entra (`SEMIESPACIO bicho-despues pct= 91`). Ese es el por que de `DIRS_ATAQUE_LISTA="14"` y `MODOS_DIRS=1`.
#
# Uso:  scripts/console/build-juego-122.sh [dir_salida]
#   1. El bicho existe y se dibuja: sus cuadros compuestos (crc32 por cuadro), su ancla y su caja en pantalla.
#   2. La patrulla de DC-002.121 sigue igual cuando no lo estan golpeando (celdas, celdas por segundo, extremos).
#   3. La colision de CELDA de DC-002.121 se mantiene: el jugador no entra a la celda del bicho y el bicho no entra
#      a la del jugador (bloqueos contados, solapamientos 0). El CADAVER deja de frenar (flag 0x8000 del motor).
#   4. Memoria: los cuadros del A1 y de GH/DT/DD son del TIPO (se comparten), y lo que cuesta por jugador es su
#      estado de ataque, medido con `unsafe.Sizeof`.
#   5. ms por cuadro y lo que cuesta el golpe: con COMBATE=si|no, la corrida sin combate es la de DC-002.121 y su
#      crc32 del mapa tiene que seguir siendo 1437022778.
#
# LO QUE SE TOCA DEL POC: `combate_122.go` y `alcance_122.go` (nuevos), `monstruo.go` (el archivo COMPARTIDO con el
# verificador del host, copiado tal cual), `monstruo_121.go` (el bicho gana vida y modos del golpe, y el cadaver
# deja de ocupar celda), cinco palancas nuevas en `knobs.go`, el golpe en `actualizar`, el resumen en `dibujar`, el
# A1 en el dibujo de los jugadores (`coop.go`) y la carga del A1 en `jugador_carga.go`. Con `COMBATE=no` la corrida
# es la de DC-002.121.
#
# LO QUE SE TOCA DEL MOTOR: NADA. El bicho se compone con los mismos paquetes que el heroe (`d2cof`, `d2dcc`,
# `d2dc6`), se dibuja con el mismo pincel (`PintarJugador` -> `RenderIndicesTintado`) y el alcance usa la tabla de
# distancias del motor tal cual (`d2common/d2skill`, que ya viene compilado en el arbol de `.gox`). Por eso este
# build puede reusar los `.gox` con `SALTAR_MOTOR=1`.
#
# LOS ARCHIVOS COMPARTIDOS. `personaje.go` y `jugador.go` del POC son BYTE A BYTE los de los medidores del host
# (`console/dc/medidas/render-294/`), y `monstruo.go` es BYTE A BYTE el del verificador del host
# (`console/dc/verif-monstruo-250/`). El script lo verifica con `cmp`: si se edita uno solo de un lado, la
# comparacion entre consola y host no vale nada. Por eso el compositor del monstruo (`componerCicloConRuta`) vive
# en `monstruo_121.go` y no en `jugador.go`.
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
# Uso:  scripts/console/build-juego-121.sh [dir_salida]
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
#       LAS DEL GRUPO (DC-002.117): GRUPO=si|no (el leash de grupo de 3 y 4 | el de a pares, para el A/B).
#       LAS DE ESTA TARJETA (DC-002.121): FASE=ciclo|apagado, CUADROS_POR_FASE=<n>, ORDEN_FASES=a|b,
#       TRANSICION="<preset> <archivo>" y SOLTAR_EN_EL_CICLO=si|no.
#       Toda palanca se pisa con AVISO EN GRANDE: el knobs.go del POC es la unica fuente de verdad (DC-002.90).
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"
# shellcheck source=palancas-del-build.sh
. "$HERE/palancas-del-build.sh"   # DC-002.107: las exclusiones del build, por el camino unico

REPO="$(cd "$HERE/../.." && pwd)"
SRC="$REPO/console/dc/poc/juego-122"
MEDIDOR="$REPO/console/dc/medidas/render-294"
BASE_GOX="${BASE_GOX:-/tmp/gox-296}"
OUT="${1:-/tmp/dcbuild/juego120}"
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

echo "== 0/6 los archivos compartidos con el medidor y el verificador del host"
for f in personaje.go jugador.go; do
    if ! cmp -s "$SRC/$f" "$MEDIDOR/$f"; then
        echo "   FALLO: $SRC/$f y $MEDIDOR/$f NO son el mismo archivo: la comparacion consola/host no vale" >&2
        exit 1
    fi
    echo "   $f $(sha256sum "$SRC/$f" | cut -c1-16)"
done
# DC-002.122: `monstruo.go` es el archivo de la VIDA y el GOLPE, y es el mismo que compila el verificador del host
# (los numeros de la vida y del dano salen de ahi, no de una copia).
if ! cmp -s "$SRC/monstruo.go" "$REPO/console/dc/verif-monstruo-250/monstruo.go"; then
    echo "   FALLO: $SRC/monstruo.go y verif-monstruo-250/monstruo.go NO son el mismo archivo" >&2
    exit 1
fi
echo "   monstruo.go $(sha256sum "$SRC/monstruo.go" | cut -c1-16) (el mismo del verificador del host)"

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
PAQUETES="$(cd "$REPO" && go list -deps -e -tags dreamcast ./console/dc/poc/juego-122 2>/dev/null |
    grep "$PREFIJO" | sed "s|.*$PREFIJO/||" | grep -vx 'console/dc/poc/juego-122' |
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

# LAS DEL COOPERATIVO (DC-002.114): CUANTOS JUGADORES Y DONDE ARRANCA CADA UNO. DC-002.121 lleva JUGADORES hasta 4.
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
# DC-002.121: el leash de GRUPO de 3 y 4 jugadores (GRUPO=no deja el de a pares de DC-002.115, para el A/B).
[ -n "${GRUPO:-}" ] && pisa_bool leashDeGrupo "$GRUPO"

# LAS DEL CICLO CON LOS CUATRO (DC-002.121): las tres fases del cambio de nivel con los cuatro adentro.
[ -n "${FASE:-}" ] && pisa_knob faseKnob "$FASE" 1
[ -n "${CUADROS_POR_FASE:-}" ] && pisa_knob cuadrosPorFase "$CUADROS_POR_FASE"
[ -n "${ORDEN_FASES:-}" ] && pisa_knob ordenDeLasFases "$ORDEN_FASES" 1
[ -n "${TRANSICION:-}" ] && { set -- $TRANSICION; pisa_knob transicionPreset "$1"; pisa_knob transicionArchivo "$2"; }
[ -n "${SOLTAR_EN_EL_CICLO:-}" ] && pisa_bool soltarEnElCiclo "$SOLTAR_EN_EL_CICLO"
[ -n "${SOLTAR_CAMINANDO:-}" ] && pisa_bool soltarCaminandoEnElCiclo "$SOLTAR_CAMINANDO"
[ -n "${CASILLAS:-}" ] && pisa_bool reusarLasCasillas "$CASILLAS"

echo "   FASE=$(leer_knob faseKnob) cuadrosPorFase=$(leer_knob cuadrosPorFase) ordenFases=$(leer_knob ordenDeLasFases) transicion=$(leer_knob transicionPreset),$(leer_knob transicionArchivo) soltarEnElCiclo=$(leer_knob soltarEnElCiclo)"

echo "   LEASH celdas=$(leer_knob leashCeldas) guionPoc=$(leer_knob guionPoc) leashDeGrupo=$(leer_knob leashDeGrupo) cuadros=$(leer_knob cuadros)"

# LAS DEL SPLIT (DC-002.116): dos lienzos de 320x480 o el cuadro unico. DC-002.121 lo deja APAGADO por omision y solo
# corre con JUGADORES=2 (el split de 3 y 4 es otra tarjeta).
[ -n "${SPLIT:-}" ] && pisa_knob splitKnob "$SPLIT" 1

echo "   SPLIT=$(leer_knob splitKnob) celdaP2=$(leer_knob celdaP2X),$(leer_knob celdaP2Y) tramoDeLaCache=$(leer_knob tramoDeLaCache)"

# LAS DE ESTA TARJETA (DC-002.121): EL MONSTRUO. MONSTRUOS=0 apaga el bicho y la corrida es la de DC-002.120 (es
# el 0 de la tabla "0 / 1 / N monstruos" de los ms por cuadro). PATRULLA="<dx> <dy> <largo>" mueve el eje del paseo.
[ -n "${MONSTRUOS:-}" ] && pisa_knob monstruosKnob "$MONSTRUOS"
[ -n "${PATRULLA:-}" ] && { set -- $PATRULLA; pisa_knob patrullaDx "$1"; pisa_knob patrullaDy "$2"; pisa_knob patrullaLargo "$3"; }

echo "   BICHO monstruos=$(leer_knob monstruosKnob) patrulla=$(leer_knob patrullaDx),$(leer_knob patrullaDy) largo=$(leer_knob patrullaLargo) separacion=$(leer_knob separacionEntreBichos)"

# LAS DE ESTA TARJETA (DC-002.122): EL GOLPE. COMBATE=no apaga la vida y el golpe (la corrida es la de DC-002.121).
[ -n "${COMBATE:-}" ] && pisa_knob combateKnob "$COMBATE" 1
[ -n "${GOLPE_AUTO:-}" ] && pisa_knob golpeAuto "$GOLPE_AUTO" 1
[ -n "${ALCANCE:-}" ] && pisa_knob alcanceGolpe "$ALCANCE"
[ -n "${DIRS_ATAQUE:-}" ] && pisa_knob dirsAtaque "$DIRS_ATAQUE"
[ -n "${DIRS_ATAQUE_LISTA:-}" ] && pisa_knob dirsAtaqueLista "$DIRS_ATAQUE_LISTA" 1
[ -n "${MODOS_DIRS:-}" ] && pisa_knob dirsModoBicho "$MODOS_DIRS"
[ -n "${VIDA_X:-}" ] && pisa_knob vidaMultiplicada "$VIDA_X"
[ -n "${CELDA_P1:-}" ] && { set -- $CELDA_P1; pisa_knob celdaP1X "$1"; pisa_knob celdaP1Y "$2"; }

echo "   COMBATE combate=$(leer_knob combateKnob) golpeAuto=$(leer_knob golpeAuto) alcance=$(leer_knob alcanceGolpe) dirsAtaque=$(leer_knob dirsAtaque) dirs=$(leer_knob dirsAtaqueLista) celdaP1=$(leer_knob celdaP1X),$(leer_knob celdaP1Y)"

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
echo "     JUEGO122: GOLPE-EMPIEZA jugador= P1 ataque= <n> dir= <d> vida= <hp>"
echo "     JUEGO122: GOLPE jugador= P1 ataque= 1 cuadroDelHeroe= 7 de 13 dano= <1|2> vidaAntes= <hp> vidaDespues= <hp-dano>"
echo "     JUEGO122: GOLPE-AFUERA jugador= P1 ... distancia= <n> alcance= 7        (con la vida sin cambiar)"
echo "     JUEGO122: MUERTE vida= 0 golpes= <n> dano= <n> modo= DT"
echo "     JUEGO122: MODO-BICHO modo= DT cuadros= 19 / modo= DD cuadros= 1"
echo "     JUEGO122: COMBATE RESUMEN ... golpesDados= <n> golpesAfuera= <n> muerto= true modoFinal= DD"
echo "     JUEGO122: COMBATE CRITERIO elGolpeBajaVida= true fueraDeAlcanceNoBaja= true murioConSuAnimacion= true losCuatroPegaron= <true con JUGADORES=4>"
[ -f "$OUT/disc.gdi" ] && echo "   imagen:         $OUT/disc.gdi"
