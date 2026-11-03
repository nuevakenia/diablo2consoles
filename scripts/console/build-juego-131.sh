#!/bin/bash
# DC-002.130 -- EL MINIMAPA: `d2automap` PORTADO Y MEDIDO EN LA CONSOLA (etapa E4, docs/PLAN-JUEGO.md).
#
# QUE SALE DE ACA. El POC de DC-002.128 completo, MAS EL MINIMAPA (`automap_130.go`), con el MODELO del motor
# (`d2common/d2automap`, 11 archivos / 1.490 lineas / cierre de 65 paquetes SIN ebiten y SIN d2asset: portable,
# medido en DC-002.125 y re-medido aca) portado TAL CUAL: no se reescribe el automapa, se le dan las casillas.
#
#   1. LA TABLA Y EL ARTE DEL MPQ. `AutoMap.txt` (122.022 B, 2.603 filas) parseada con el lector del motor y
#      FILTRADA al pueblo (`ParseTxtLevels`: 83 filas, 6.176 B, porque la tabla entera son 271.144 B del semiespacio
#      y con el HUD queda al 96 %). El arte es `maximap.dc6`: el `maximaps.dc6` de 8x16 que pide el motor NO EXISTE
#      en esta instalacion, asi que los cuadros de 16x32 se DECIMAN (1 de cada 2 px) a la caja de 8x16 que el motor
#      dibuja (59 cuadros, 7.552 B) y van al MONTON DE C, como el HUD.
#   2. EL REVEAL SIGUE AL JUGADOR. Cada cuadro, si el heroe se movio lo suficiente (`d2automap.MovedFar`, el
#      disparador del motor), `RevealTiles` revela las casillas de alrededor con `ScreenArea(640,480)`. El log dice
#      cuantas celdas agrego cada revelado y cuantas hay. Con `MINIMAPA_TODO=si` ademas se revela el pueblo ENTERO
#      en el cuadro 1: es el caso peor (610 de las 680 celdas caen en la caja de 279x225 en (359,57)-(638,282)) y el
#      numero que pide la tarjeta.
#   3. EL DIBUJO Y EL MARCADOR. Las celdas del modelo, en el orden del modelo (capa, y, x, cuadro), pegadas con su
#      esquina de abajo a la izquierda en la proyeccion del motor (`ComputeLayoutOffsets(SizeMini, ...)`), y la cruz
#      del heroe con la polilinea del motor (`automapCross`, 12 puntos duplicados) y su color.
#   4. LA DECISION, CON EL NUMERO. El minimapa se mide DENTRO de la corrida (crc32 del lienzo antes y despues, px y
#      ms por cuadro) y el cierre decide `siempre` o `por-tecla` con el % del cuadro: si cuesta menos del 10 % del
#      cuadro del mapa, se dibuja SIEMPRE (que es lo que hace el juego). La palanca `MINIMAPA_CUANDO=cambio` es el
#      A/B (se salta los cuadros en los que ni el heroe ni el modelo cambiaron).
#
# LO QUE SE MIDE: el presupuesto re-verificado (`console/dc/sonda-automap-130`: lineas, cierre, tabla, 680 celdas,
# 46 cuadros distintos, el recorte en el lienzo del POC, la memoria del modelo y el A/B de la tabla filtrada), y en
# Flycast los ms por cuadro, el maximo, el promedio, el % del cuadro, la memoria del arte y del modelo, y el A/B con
# `MINIMAPA=no` (la corrida de DC-002.128 bit a bit: sin un pixel de minimapa).
#
# LAS PALANCAS DE ESTA TARJETA: MINIMAPA=si|no, MINIMAPA_CUANDO=siempre|cambio, MINIMAPA_TODO=si|no, y las de
# siempre: JUGADORES=, CUADROS=, FASE=, MONSTRUOS=, HUD=, NPC=, QUEST=.
#
# Uso:
#   bash scripts/console/build-juego-131.sh /tmp/dcbuild/juego131
#   CAPTURE=spectacle bash scripts/console/run-flycast-131.sh /tmp/dcbuild/juego131/disc.gdi 200 /tmp/capturas-131
#
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"
# shellcheck source=palancas-del-build.sh
. "$HERE/palancas-del-build.sh"   # DC-002.107: las exclusiones del build, por el camino unico

REPO="$(cd "$HERE/../.." && pwd)"
SRC="$REPO/console/dc/poc/juego-131"
# DC-002.132: POC_SRC permite compilar una COPIA del POC (una foto del arbol: `git archive HEAD` + los archivos
# de la tarjeta) sin tocar el directorio del repo, que puede estar siendo editado por otra corrida. Es el mismo
# patron que POC_DIR en build-monstruo-250.sh, por el mismo motivo. Sin POC_SRC todo queda como estaba.
SRC="${POC_SRC:-$SRC}"
MEDIDOR="$REPO/console/dc/medidas/render-294"
BASE_GOX="${BASE_GOX:-/tmp/gox-296}"
OUT="${1:-/tmp/dcbuild/juego131}"
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
 d2common/d2fileformats/d2s
 d2common/d2automap
 d2common/d2quest
 d2common/d2npc
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
PAQUETES="$(cd "$REPO" && go list -deps -e -tags dreamcast ./console/dc/poc/juego-131 2>/dev/null |
    grep "$PREFIJO" | sed "s|.*$PREFIJO/||" | grep -vx 'console/dc/poc/juego-131' |
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

# LAS DE ESTA TARJETA (DC-002.123): EL DROP Y LA RECOGIDA. DROP=no deja la corrida de DC-002.122 bit a bit.
[ -n "${DROP:-}" ] && pisa_knob dropKnob "$DROP" 1
[ -n "${SEMILLA_DROP:-}" ] && pisa_knob semillaDrop "$SEMILLA_DROP"
[ -n "${RECOGER_AUTO:-}" ] && pisa_knob recogerAuto "$RECOGER_AUTO" 1

echo "   DROP drop=$(leer_knob dropKnob) semilla=$(leer_knob semillaDrop) recogerAuto=$(leer_knob recogerAuto)"

# LAS DE ESTA TARJETA (DC-002.124): EL INVENTARIO Y EL ALIJO. INVENTARIO=no deja la corrida de DC-002.123 bit a bit.
[ -n "${INVENTARIO:-}" ] && pisa_knob inventarioKnob "$INVENTARIO" 1
[ -n "${ALIJO:-}" ] && pisa_knob alijoKnob "$ALIJO" 1
[ -n "${INVENTARIO_AUTO:-}" ] && pisa_knob inventarioAuto "$INVENTARIO_AUTO" 1
[ -n "${CUADRO_INVENTARIO:-}" ] && pisa_knob cuadroAbreElInventario "$CUADRO_INVENTARIO"
[ -n "${ALIJO_AUTO:-}" ] && pisa_knob alijoAuto "$ALIJO_AUTO" 1
[ -n "${CUADRO_ALIJO:-}" ] && pisa_knob cuadroAbreElAlijo "$CUADRO_ALIJO"
[ -n "${RADIO_ALIJO:-}" ] && pisa_knob radioDelAlijo "$RADIO_ALIJO"
[ -n "${CERRAR_PANELES:-}" ] && pisa_knob cerrarLosPaneles "$CERRAR_PANELES" 1

echo "   INVENTARIO inventario=$(leer_knob inventarioKnob) alijo=$(leer_knob alijoKnob) inventarioAuto=$(leer_knob inventarioAuto) cuadroAbre=$(leer_knob cuadroAbreElInventario) alijoAuto=$(leer_knob alijoAuto) cuadroAlijo=$(leer_knob cuadroAbreElAlijo) radioAlijo=$(leer_knob radioDelAlijo)"

# LAS DE ESTA TARJETA (DC-002.126): EL RELOJ DE LA MUERTE Y EL CADAVER. MUERTE_POR_CUADRO=si es el instrumento que
# deja VER la caida (un cuadro de animacion por cuadro del POC); CADAVER=no deja el cadaver sin expirar (el A/B).
[ -n "${MUERTE_POR_CUADRO:-}" ] && pisa_knob muertePorCuadro "$MUERTE_POR_CUADRO" 1
[ -n "${CADAVER:-}" ] && pisa_knob cadaverKnob "$CADAVER" 1

echo "   MUERTE muertePorCuadro=$(leer_knob muertePorCuadro) cadaver=$(leer_knob cadaverKnob)"

# LAS DE ESTA TARJETA (DC-002.125): EL HUD. HUD=no deja la corrida de DC-002.126 bit a bit (es el A/B del costo:
# los mismos cuadros sin un pixel de HUD encima). HUD_INSTRUMENTO=si hace bajar la vida y el mana un punto por
# cuadro hasta 1, para poder ver que el relleno del globo sigue al VALOR (el POC no tiene dano al heroe todavia).
# NIVELES=si da a cada jugador su nivel (1, 2, 3, 4) y con eso su vida y su mana, que es lo que hace que en el
# split cada mitad muestre SU HUD y no una copia del primero.
[ -n "${HUD:-}" ] && pisa_knob hudKnob "$HUD" 1
[ -n "${HUD_PIEZAS:-}" ] && pisa_knob hudPiezas "$HUD_PIEZAS" 1
[ -n "${HUD_INSTRUMENTO:-}" ] && pisa_knob hudInstrumento "$HUD_INSTRUMENTO" 1
[ -n "${NIVELES:-}" ] && pisa_knob nivelesDistintos "$NIVELES" 1
[ -n "${CLASE:-}" ] && pisa_knob claseDelHeroe "$CLASE" 1
[ -n "${NIVEL:-}" ] && pisa_knob nivelDelHeroe "$NIVEL"

echo "   HUD hud=$(leer_knob hudKnob) instrumento=$(leer_knob hudInstrumento) nivelesDistintos=$(leer_knob nivelesDistintos) clase=$(leer_knob claseDelHeroe) nivel=$(leer_knob nivelDelHeroe)"

# LAS DE ESTA TARJETA (DC-002.127): LOS NPCs Y EL DIALOGO. NPC=no deja la corrida de DC-002.125 bit a bit (el A/B
# del costo). HABLAR_AUTO=si es el instrumento que aprieta el boton (el emulador corre sin teclado).
[ -n "${NPC:-}" ] && pisa_knob npcKnob "$NPC" 1
[ -n "${NPC_LISTA:-}" ] && pisa_knob npcLista "$NPC_LISTA" 1
[ -n "${HABLAR_AUTO:-}" ] && pisa_knob hablarAuto "$HABLAR_AUTO" 1
[ -n "${CUADRO_DIALOGO:-}" ] && pisa_knob cuadroAbreElDialogo "$CUADRO_DIALOGO"
[ -n "${CERRAR_DIALOGO:-}" ] && pisa_knob cerrarElDialogo "$CERRAR_DIALOGO" 1
[ -n "${VISTA_X:-}" ] && pisa_knob vistaX "$VISTA_X"
[ -n "${VISTA_Y:-}" ] && pisa_knob vistaY "$VISTA_Y"

echo "   VISTA vista=$(leer_knob vistaX),$(leer_knob vistaY)"
echo "   NPC npc=$(leer_knob npcKnob) lista=$(leer_knob npcLista) hablarAuto=$(leer_knob hablarAuto) cuadro=$(leer_knob cuadroAbreElDialogo) cerrar=$(leer_knob cerrarElDialogo)"


# LAS DE ESTA TARJETA (DC-002.128): LA DEN OF EVIL. QUEST=no deja la corrida de DC-002.127 bit a bit (el A/B del
# costo). QUEST_BICHOS y QUEST_CUADRO_* son el guion: cuantos bichos y en que cuadro del POC pasa cada cosa (el
# emulador de estas corridas no tiene teclado, asi que el boton y los cambios de area los pone el guion).
[ -n "${QUEST:-}" ] && pisa_knob questKnob "$QUEST" 1
[ -n "${QUEST_BICHOS:-}" ] && pisa_knob monstruosKnob "$QUEST_BICHOS"
[ -n "${QUEST_CUADRO_ACEPTA:-}" ] && pisa_knob questCuadroAcepta "$QUEST_CUADRO_ACEPTA"
[ -n "${QUEST_CUADRO_CUEVA:-}" ] && pisa_knob questCuadroCueva "$QUEST_CUADRO_CUEVA"
[ -n "${QUEST_CUADRO_LIMPIA:-}" ] && pisa_knob questCuadroLimpia "$QUEST_CUADRO_LIMPIA"
[ -n "${QUEST_CUADRO_VUELVE:-}" ] && pisa_knob questCuadroVuelve "$QUEST_CUADRO_VUELVE"
[ -n "${QUEST_CUADRO_COBRA:-}" ] && pisa_knob questCuadroCobra "$QUEST_CUADRO_COBRA"
[ -n "${QUEST_CUADRO_VERIFICA:-}" ] && pisa_knob questCuadroVerifica "$QUEST_CUADRO_VERIFICA"

echo "   QUEST quest=$(leer_knob questKnob) bichos=$(leer_knob monstruosKnob) acepta=$(leer_knob questCuadroAcepta) cueva=$(leer_knob questCuadroCueva) limpia=$(leer_knob questCuadroLimpia) vuelve=$(leer_knob questCuadroVuelve) cobra=$(leer_knob questCuadroCobra) verifica=$(leer_knob questCuadroVerifica)"
# LAS DE ESTA TARJETA (DC-002.130): EL MINIMAPA. MINIMAPA=no deja la corrida de DC-002.128 bit a bit (el A/B del
# costo: los mismos cuadros sin un pixel de minimapa). MINIMAPA_CUANDO=siempre|cambio es el A/B de la decision y
# MINIMAPA_TODO=si revela el pueblo entero en el cuadro 1 (el caso peor, las 610 celdas de la caja).
[ -n "${MINIMAPA:-}" ] && pisa_knob minimapaKnob "$MINIMAPA" 1
[ -n "${MINIMAPA_CUANDO:-}" ] && pisa_knob minimapaCuando "$MINIMAPA_CUANDO" 1
[ -n "${MINIMAPA_TODO:-}" ] && pisa_knob minimapaTodo "$MINIMAPA_TODO" 1

echo "   MINIMAPA minimapa=$(leer_knob minimapaKnob) cuando=$(leer_knob minimapaCuando) todo=$(leer_knob minimapaTodo)"

# LAS DE ESTA TARJETA (DC-002.131): EL INSTRUMENTO DE LO VIVO. MEDIR_VIVO=no lo apaga entero (recolecciones
# incluidas), que es el A/B que separa el instrumento del arreglo: con el instrumento prendido, sus propias
# recolecciones bajan el pico y la corrida pasa aunque el codigo sea el mismo.
[ -n "${MEDIR_VIVO:-}" ] && pisa_knob medirVivo "$MEDIR_VIVO" 1

echo "   VIVO medirVivo=$(leer_knob medirVivo)"

# ARTE_C=no deja los cuadros compuestos del jugador y del bicho en el semiespacio del recolector (el A/B del arreglo).
[ -n "${ARTE_C:-}" ] && pisa_knob arteEnMontonDeC "$ARTE_C" 1

echo "   VIVO arteEnMontonDeC=$(leer_knob arteEnMontonDeC)"

# CUADRO_BORDE_DESDE / CUADRO_BORDE_HASTA: la ventana del instrumento fino por paso (vivoDeUnPaso). Con
# CUADRO_BORDE_DESDE vacio se deja la ventana por omision; con DESDE mayor que HASTA el instrumento no corre.
[ -n "${CUADRO_BORDE_DESDE:-}" ] && pisa_knob cuadroDesdeElBorde "$CUADRO_BORDE_DESDE"
[ -n "${CUADRO_BORDE_HASTA:-}" ] && pisa_knob cuadroHastaElBorde "$CUADRO_BORDE_HASTA"

echo "   VIVO ventanaDelBorde=$(leer_knob cuadroDesdeElBorde)..$(leer_knob cuadroHastaElBorde)"

# GC_POR_CUADRO=si es la recoleccion en el borde del cuadro (medida y RECHAZADA en esta tarjeta: cuesta 86-92 ms
# por cuadro y corrompe el ancho del texto del HUD; queda para poder reproducir la medida). UMBRAL_GC_CUADRO es el
# % de semiespacio a partir del cual corre.
[ -n "${GC_POR_CUADRO:-}" ] && pisa_knob gcPorCuadro "$GC_POR_CUADRO" 1
[ -n "${UMBRAL_GC_CUADRO:-}" ] && pisa_knob umbralDelGcDeCuadro "$UMBRAL_GC_CUADRO"

echo "   VIVO gcPorCuadro=$(leer_knob gcPorCuadro) umbral=$(leer_knob umbralDelGcDeCuadro)"

# LAS DE ESTA TARJETA (DC-002.132): LOS CUADROS POR SEGUNDO.
#   MEDIR_CUADRO=no  apaga el instrumento POR CUADRO del POC (16 crc32 del lienzo de 1.228.800 B y el conteo de
#                    pixeles). Los tres primeros cuadros se miden igual: ahi vive la comparacion contra el host.
#   BLIT_RAPIDO=no   el bucle viejo del tile (el A/B del recorte y del tinte, en el mismo binario).
[ -n "${MEDIR_CUADRO:-}" ] && pisa_knob medirCuadro "$MEDIR_CUADRO" 1
[ -n "${BLIT_RAPIDO:-}" ] && pisa_knob blitRapido "$BLIT_RAPIDO" 1

echo "   ANIM132 medirCuadro=$(leer_knob medirCuadro) blitRapido=$(leer_knob blitRapido)"

# LAS DE ESTA TARJETA (DC-002.134): LAS ETAPAS DEL CUADRO.
#   ETAPAS=no   apaga el cronometro por etapa (el binario es el de DC-002.132 otra vez).
#   VERBOSO=no  apaga las lineas de evidencia POR CUADRO (solo quedan los tres primeros cuadros y los resumenes
#               del cierre): es el A/B de cuanto del cuadro es el puerto serie.
[ -n "${ETAPAS:-}" ] && pisa_knob etapasKnob "$ETAPAS" 1
[ -n "${VERBOSO:-}" ] && pisa_knob verbosoEtapas "$VERBOSO" 1
[ -n "${BLIT_PALABRAS:-}" ] && pisa_knob blitPalabras134 "$BLIT_PALABRAS" 1
[ -n "${PINTAR_UNA_VEZ:-}" ] && pisa_knob pintarUnaVez "$PINTAR_UNA_VEZ" 1

[ -n "${PRES_CONV_RAPIDA:-}" ] && pisa_knob presentarConvRapida "$PRES_CONV_RAPIDA" 1
[ -n "${PRES_VRAM_AGRUPADA:-}" ] && pisa_knob presentarVramAgrupada "$PRES_VRAM_AGRUPADA" 1

echo "   ETAPAS134 etapas=$(leer_knob etapasKnob) verboso=$(leer_knob verbosoEtapas) blitPalabras=$(leer_knob blitPalabras134) pintarUnaVez=$(leer_knob pintarUnaVez)"

# LAS DE ESTA TARJETA (DC-002.137): EL MINIMAPA.
#   MINIMAPA_RAPIDO=si|no   una tabla de 256 palabras armada UNA vez por cuadro y sin Push/Pop por celda (si), o
#                           el camino de siempre, una tabla por celda (no). El A/B es una corrida y los pixeles
#                           son los mismos: lo dice el `crcDeLaCaja` de la linea del minimapa.
#   MINIMAPA_DIAG=no|caja|celdas|tabla610
#                           las corridas de medir por dentro. `caja` y `celdas` rompen el dibujo a proposito;
#                           `tabla610` deja el cuadro IDENTICO (control del instrumento).
[ -n "${MINIMAPA_RAPIDO:-}" ] && pisa_knob minimapaRapido "$MINIMAPA_RAPIDO" 1
[ -n "${MINIMAPA_DIAG:-}" ] && pisa_knob minimapaDiag "$MINIMAPA_DIAG" 1

echo "   MINIMAPA137 rapido=$(leer_knob minimapaRapido) diag=$(leer_knob minimapaDiag)"
echo "   PRESENTACION135 convRapida=$(leer_knob presentarConvRapida) vramAgrupada=$(leer_knob presentarVramAgrupada)"

# LAS DE ESTA TARJETA (DC-002.135, EL MAPA): EL TILE POR DENTRO.
#   TILE_CRON=si    el cronometro por bloque DENTRO del renderer del mapa (cache, vista, estado, pintado) y las
#                   pasadas del renderer: la linea `JUEGO135M: MAPA-POR-TILE` cada 30 cuadros.
#   BUCLE_135=no|lectura|escritura|tabla2
#                   la corrida de DIAGNOSTICO del bucle de pixeles del mapa (parte el costo por pixel en leer,
#                   escribir y armar la tabla). "lectura" y "escritura" rompen el dibujo a proposito: son de medir.
[ -n "${TILE_CRON:-}" ] && pisa_knob tileCron "$TILE_CRON" 1
[ -n "${BUCLE_135:-}" ] && pisa_knob bucle135 "$BUCLE_135" 1
[ -n "${CACHE_RAPIDA:-}" ] && pisa_knob cacheRapida135 "$CACHE_RAPIDA" 1
[ -n "${PLANO_135:-}" ] && pisa_knob plano135 "$PLANO_135" 1

echo "   MAPA135 tileCron=$(leer_knob tileCron) bucle=$(leer_knob bucle135) cacheRapida=$(leer_knob cacheRapida135) plano=$(leer_knob plano135)"

# LAS DE ESTA TARJETA (DC-002.136): EL ARRANQUE EN LOS 16 MB.
#   MEDIR_CARGA=no   apaga la tabla de consumo del arranque (instrumento_136.go): el A/B del costo del instrumento.
#   BLOQUES=si|no    el `malloc` interpuesto (`-Wl,--wrap=malloc`, csrc/shim.c seccion 12): imprime cada pedido de
#                    64 KB o mas con el monton que habia antes. Es el que dice QUIEN pide los bloques grandes. Viene
#                    PRENDIDO porque sin el la tabla de etapas dice cuando y no quien (y el costo es una linea por
#                    pedido grande en todo el arranque).
[ -n "${MEDIR_CARGA:-}" ] && pisa_knob medirCarga "$MEDIR_CARGA" 1
if [ -n "${BLOQUES:-}" ]; then
    case "$BLOQUES" in
        si|1|true) export DC_WRAP_MALLOC=1 ;;
        *)         export DC_WRAP_MALLOC=0 ;;
    esac
fi
export DC_WRAP_MALLOC="${DC_WRAP_MALLOC:-1}"
echo "   ARRANQUE136 medirCarga=$(leer_knob medirCarga) wrapMalloc=$DC_WRAP_MALLOC"

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
echo "     JUEGO131: MINIMAPA TABLA archivo= data\\global\\excel\\AutoMap.txt bytes= 122022 filasDelPueblo= 83 nivel= 1 nombre= 1 Town"
echo "     JUEGO131: MINIMAPA ARTE pide 59 cuadros de 1260 del archivo de 234458 B caja= 8 x 16 (decimado de 16 x 32 )"
echo "     JUEGO131: MINIMAPA ARTE decodificados= 59 faltan= 0 bytes= 7552 nombreQuePideElMotor= ...\\maximaps.dc6 existe=false (medido)"
echo "     JUEGO131: MINIMAPA REVEAL-TODO casillas= 57 x 41 agregadas= 680 celdas= 680 piso= 514 pared= 166"
echo "     JUEGO131: MINIMAPA LISTO celdas= 680 piso= 514 pared= 166 palanca= si cuando= siempre todo= si ms= <n>"
echo "     JUEGO131: MINIMAPA cuadro= 1 jugador= 1 celdas= <n> de 680 pixelesPintados= <n> ms= <n> heroe= <x> <y> enLaCaja= <x> <y> caja= 359 57 638 282 sinArte= 0"
echo "     JUEGO131: MINIMAPA CUADRO 1 elCuadroCambio= true crcAntes= <n> crcDespues= <n> pixeles= <n> ms= <n> lienzoLleno= <n> de 307200"
echo "     JUEGO131: MINIMAPA REVEAL cuadro= <n> jugador= 1 heroe= <x> <y> agregadas= <n> celdas= <n> piso= <n> pared= <n>"
echo "     JUEGO131: MINIMAPA RESUMEN celdasEnElModelo= 680 piso= 514 pared= 166 celdasDibujadasMax= <n> celdasDibujadasUltimo= <n> pxPorCuadro= <n> pxTotal= <n> sinArte= 0"
echo "     JUEGO131: MINIMAPA COSTO cuadrosDibujados= <n> saltados= <n> msPromedio= <n> msMax= <n> msTotal= <n> msDelCuadroDelMapa= <n> pctDelCuadro= <n>"
echo "     JUEGO131: MINIMAPA ARTE-RESUMEN cuadros= 59 bytesEnMontonDeC= 7552 bytesDeLaTabla= 122022 bytesDeArteSiSeDecodificaraEntero= 645120 bytesDecimadoAlPueblo= 7552 montonC_libre= <n>"
echo "     JUEGO131: MINIMAPA REVEAL-RESUMEN revelados= <n> agregadasMax= <n> agregadasUltimas= <n> sigueAlJugador= true"
echo "     JUEGO131: MINIMAPA DECISION escogido= siempre msPromedio= <n> pctDelCuadro= <n> umbralPct= 10 cuando= siempre todo= si"
echo "     JUEGO131: MINIMAPA CRITERIO seDibujo= true pintoPixeles= true elRevealSigueAlJugador= true celdasEnLaCaja= <n> msPorCuadro= <n> arteEnElMontonDeC= true decision= siempre"
[ -f "$OUT/disc.gdi" ] && echo "   imagen:         $OUT/disc.gdi"
