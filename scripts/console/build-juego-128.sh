#!/bin/bash
# DC-002.128 -- LA DEN OF EVIL: ACEPTAR, AVANZAR Y COMPLETAR LA PRIMERA QUEST (etapa E7, docs/PLAN-JUEGO.md).
#
# QUE SALE DE ACA. El POC de DC-002.127 completo, MAS LA QUEST (`quest_128.go`), con el motor de quests DEL JUEGO
# (`d2common/d2quest`, 11.272 lineas) portado ENTERO: no se reescribe la quest, se le dan los eventos.
#
#   1. ACEPTAR HABLANDO CON AKARA. El jugador arranca en la celda de Akara (`VISTA_X/Y` = su celda del townN1, medida
#      en la sonda: 38.4, 3.2), el panel de DC-002.127 se abre con el NOMBRE de la quest y la PAGINA DE SU LOG sacada
#      de `string.tbl` (`qstsa1q1<page>`, el texto del juego) y el panel escucha el mensaje 64, que es el que el nodo
#      A1Q1 del exe atiende para entregarla (`d2quest/a1_basic.go`). El motor pasa la quest a "aceptada" y le pone
#      sus bits en el REGISTRO DEL GUARDADO (slot 1).
#   2. AVANZAR MATANDO. El cambio de area (`EvAreaChanged` pueblo -> area 8, la Den of Evil) la lleva a "en la cueva";
#      cada bicho que muere va con su evento (`SyncDenMonsters` + `EvMonsterKilled`) y el CONTADOR DEL MOTOR avanza
#      (la pagina del log dice "Monsters remaining:"). Los bichos mueren con el MISMO dado y el MISMO camino que el
#      golpe del boton Y de DC-002.122 (`Combate.GolpeDelHeroe` + DT + cadaver); el instrumento que reemplaza al
#      teclado pone al jugador en la celda de al lado (dx=5, dy=0: la tabla del exe da 5 y el alcance es 7).
#   3. COMPLETAR Y COBRAR. La ultima baja deja la cueva limpia, la recompensa PENDIENTE y el sonido 35 de la quest;
#      volver al pueblo y escuchar el mensaje 76 de Akara marca la quest como hecha y emite `EffectSkillPoint(1)`, el
#      punto de habilidad, que se GUARDA en el resumen (gastarlo es E8).
#   4. SOBREVIVIR AL CAMBIO DE NIVEL. El estado vive en el registro del guardado (96 B por dificultad, 2 B el slot de
#      la quest): las tres fases de DC-002.120 recomponen niveles y la quest se imprime IDENTICA en cada una
#      (`JUEGO128: QUEST FASE ... igualAlCierre= true`), y un `d2quest.New` sobre el mismo registro + `Start` (el
#      camino de cargar la partida) la deja INERTE y el log COMPLETADA.
#
# LO QUE SE MIDE: el presupuesto ANTES de portar (`console/dc/sonda-quest-128`: lineas, cierre de imports, 34 nodos
# que arma `d2quest.New`, la secuencia de eventos corrida en el host y la celda de Akara), los bytes vivos del
# registro, los golpes y el dano por bicho, el contador del motor cuadro a cuadro, el panel (px y ms), y el A/B con
# `QUEST=no` (la corrida de DC-002.127 bit a bit).
#
# LAS PALANCAS DE ESTA TARJETA: QUEST=si|no, QUEST_BICHOS=<n> (los bichos de la cueva, por omision 3),
# QUEST_CUADRO_ACEPTA/CUEVA/LIMPIA/VUELVE/COBRA/VERIFICA (el guion), VISTA_X/VISTA_Y (la celda de Akara) y las de
# siempre: MONSTRUOS=, JUGADORES=, FASE=ciclo|apagado, CUADROS_POR_FASE=.
#
# Uso:
#   bash scripts/console/build-juego-128.sh /tmp/dcbuild/juego128
#   CAPTURE=spectacle bash scripts/console/run-flycast-128.sh /tmp/dcbuild/juego128/disc.gdi 200 /tmp/capturas-128
#
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"
# shellcheck source=palancas-del-build.sh
. "$HERE/palancas-del-build.sh"   # DC-002.107: las exclusiones del build, por el camino unico

REPO="$(cd "$HERE/../.." && pwd)"
SRC="$REPO/console/dc/poc/juego-128"
MEDIDOR="$REPO/console/dc/medidas/render-294"
BASE_GOX="${BASE_GOX:-/tmp/gox-296}"
OUT="${1:-/tmp/dcbuild/juego128}"
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
PAQUETES="$(cd "$REPO" && go list -deps -e -tags dreamcast ./console/dc/poc/juego-127 2>/dev/null |
    grep "$PREFIJO" | sed "s|.*$PREFIJO/||" | grep -vx 'console/dc/poc/juego-127' |
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
echo "     JUEGO128: QUEST ARRANCA quest= Den of Evil id= 1 slot= 1 act= 0 logIndex= 1 questsDelMotor= 34 registroBytes= 96"
echo "     JUEGO128: QUEST TEXTO nombre= Den of Evil paginasConTexto= <n> de 15 paginaAceptada= Look for the Den in the wilderness..."
echo "     JUEGO128: QUEST HABLA npc= AKARA claseExe= 148 distancia= 0 radio= 2 cuadro= 2"
echo "     JUEGO128: QUEST MARCADOR npc= AKARA activa= true   (el \"!\" del motor sobre la cabeza)"
echo "     JUEGO128: QUEST PASO paso= ACEPTA bits= 4 estado= aceptada log= sin-empezar"
echo "     JUEGO128: QUEST PANEL cuadro= 2 npc= AKARA pagina= 1 renglones= <n> panel= <w> x <h> bytes= <n> texto= Look for the Den in the wilderness outside the Rogues' Camp."
echo "     JUEGO128: QUEST AREA de= 1 a= 8 (Den of Evil) cuadro= 4"
echo "     JUEGO128: QUEST PASO paso= CUEVA bits= 20 estado= en-la-cueva log= en-curso"
echo "     JUEGO128: QUEST GOLPE bicho= B1 celda= <x> <y> jugador= <x-1> <y> dx= 5 dy= 0 distancia= 5 alcance= 7 alcanza= true vida= <hp>"
echo "     JUEGO128: QUEST GOLPE-UNO bicho= B1 golpe= <n> dano= <1|2> vidaAntes= <hp> vidaDespues= <hp-dano> muerto= <bool>"
echo "     JUEGO128: QUEST MATA bicho= B1 golpes= <n> dano= <n> vida= 0 vivosEnLaCueva= 2 quedaSegunElMotor= 2 estado= en-la-cueva pagina= 4"
echo "     JUEGO128: QUEST BIT paso= MATA slot= 1 antes= <n> despues= <n> motivo= Den of Evil cleared estado= 4"
echo "     JUEGO128: QUEST SONIDO valor= 35 (el de la Den of Evil terminada)"
echo "     JUEGO128: QUEST LIMPIA cuadro= 8 vivos= 0 estado= cueva-limpia pagina= 5 recompensaPendiente= true marcador= true"
echo "     JUEGO128: QUEST PASO paso= COBRA bits= 8193 estado= cobrada log= COMPLETADA"
echo "     JUEGO128: QUEST EFECTOS paso= COBRA efectos= 3 SkillPoint(1) Respec(0) LogUpdate(13)"
echo "     JUEGO128: QUEST COBRO puntoDeHabilidad= 1 estado= cobrada log= COMPLETADA marcador= false"
echo "     JUEGO128: QUEST SUPERVIVE registroBytes= 96 bitsQueSeGuardan= 8193 bitsTrasCargar= 1 inerte= true recompensaCobrada= true log= COMPLETADA marcador= false"
echo "     JUEGO128: QUEST FASE nombre= TRANSICION bits= 8193 estado= cobrada pagina= 13 log= COMPLETADA igualAlCierre= true"
echo "     JUEGO128: QUEST RESUMEN quest= Den of Evil slot= 1 bits= 8193 estado= cobrada pagina= 13 log= COMPLETADA aceptada= true contadorAvanzo= true completada= true cobrada= true puntoDeHabilidad= 1"
echo "     JUEGO128: QUEST RESUMEN MEDIDO golpes= <n> dano= <n> muertes= 3 paneles= 3 bytesDelPanel= <n> pxDelPanel= <n> msDelPanel= <n> fasesMedidas= 3 fasesIguales= true sobreviveAlNivel= true sobreviveAlCargar= true"
echo "     JUEGO128: QUEST CRITERIO seAcepto= true elContadorAvanzo= true seCompleto= true seCobro= true cobroPuntoDeHabilidad= 1 sobreviveAlCambioDeNivel= true sobreviveAlCargarLaPartida= true"
[ -f "$OUT/disc.gdi" ] && echo "   imagen:         $OUT/disc.gdi"
