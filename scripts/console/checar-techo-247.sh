#!/bin/bash
# DC-002.47: EL TECHO DEL ARRANQUE -- la palanca que lo mueve, su MUTANTE, y la particion de memoria con sus
# numeros. Es el corpus de esta tarjeta; el de DC-002.45 (`checar-trozos-245.sh`, 12 controles) sigue aparte
# y sin tocar.
#
# QUE MIDE (el numero, no el relato):
#
#   SANO      el arbol de HOY: el lector por trozos de `d2std/bufio` no pierde el ultimo trozo cuando el
#             stream del MPQ entrega BYTES Y `io.EOF` en la misma llamada, y la particion nueva (semiespacio
#             del vendor + `.eh_frame` fuera del mapa). Tiene que PASAR de las 3 tablas de DC-002.45
#             (LvlTypes, LvlPrest, LvlWarp) y pedir TABLAS DESPUES de LvlWarp.
#   MUTANTE   la MISMA copia con el lector de antes (`mutar-bufio-247.py --mutante`) y la MISMA particion.
#             Tiene que QUEDARSE en las 3 tablas: LvlWarp.txt son 3.935 B, entra en un pedido de 4.096 B del
#             buffer y el lector viejo tiraba esos bytes, asi que la tabla se leia vacia y el arranque no
#             pedia la siguiente. Ese es el mutante que caza la regresion (si el mutante avanzara igual, el
#             arreglo no estaria haciendo nada).
#   VIEJA     la particion de antes (semiespacio 512 KB y el `.eh_frame` adentro del mapa) CON el lector
#             arreglado. Es el mutante de la SEGUNDA palanca: mide que la particion no es la que mueve el
#             arranque (llega a las mismas tablas) pero si la que cambia los numeros del semiespacio: el dato
#             vivo al 98 % contra el 79 %, y las recolecciones de la corrida.
#
# CONTROLES:
#   0   libgodc.a al dia y el techo del semiespacio leido de `godc_config.h` (el techo es una voluntad)
#   0b  el arbol esta SANO segun `mutar-bufio-247.py --estado`
#   1   procedencia: el `bufio.gox` de las dos capas NO es el mismo archivo (sha), y el de la capa del mutante
#       es el del lector viejo
#   2   [sano] LvlWarp.txt se LEYO (`lee-volvio` despues de su `lee-entra`)
#   3   [sano] el motor SIGUE: pide una tabla POSTERIOR a LvlWarp
#   4   [sano] assets de MPQ pedidos > 3 (el numero de la tarjeta: 3 tablas era la pared)
#   5   [mutante] el arranque se queda en las 3 tablas y NO pide ninguna tabla posterior a LvlWarp
#   6   [criterio 3] el mutante devuelve el arranque a 3 tablas: sano > mutante
#   7   [host] la difftest de la stdlib caza la propiedad: con el lector sano PASA, con el mutante FALLA
#   8   [numeros] el dato vivo maximo y las recolecciones de las tres corridas (informativo)
#
# Uso:  bash scripts/console/checar-techo-247.sh [--refrescar] [segundos_por_corrida]
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
SAL="${SAL:-/home/dgalli/.hermes/cache/scratch/techo-247}"
BASE="${BASE:-/home/dgalli/.hermes/cache/scratch/trozos-245}"
GOX_BASE="$BASE/gox-base"
JUEGO="$BASE/juego"
SRC="$SAL/src-247"
LIBGODC="${LIBGODC:-$HOME/dreamcast/libgodc}"
MPQ="${MPQ:-$HOME/.wine-d2/drive_c/Games/Diablo II}"
LISTA="patch_d2.mpq,d2data.mpq,d2char.mpq"
PACKS="d2common/d2fileformats/d2txt d2common/d2loader d2core/d2asset"
STD_PACKS="bufio"
SEGS="${2:-50}"
[ "${1:-}" = "--refrescar" ] && SEGS="${2:-50}"

OK=0
FALLOS=0
nota() { echo "   $*"; }
verde() { OK=$((OK + 1)); echo "   OK: $*"; }
rojo() { FALLOS=$((FALLOS + 1)); echo "   FALLO: $*"; }
paso() { printf '\n== %s\n' "$*"; }
log_de() { tr -d '\r' < "$1/flycast.log"; }
assets_de() { log_de "$1" | grep -ac 'Loaded /cd/juego'; }
tablas_leidas_de() { log_de "$1" | grep -ao 'lee-volvio.*excel/[A-Za-z]*\.txt' | sed 's|.*excel/||' | sort -u | tr '\n' ' '; }
vivo_de() { log_de "$1" | grep -ao 'gc-vivo v=[0-9]*' | sed 's/.*=//' | sort -n | tail -1; }
gc_de() { log_de "$1" | grep -ao 'gc_collect v=[0-9]*' | sed 's/.*=//' | sort -n | tail -1; }
techo_de() { log_de "$1" | grep -ao '/[0-9]* gc=' | sed 's|/||;s| gc=||' | sort -n | tail -1; }

mkdir -p "$SAL"
TECHO="$("$DC_PYTHON" "$HERE/fix-libgodc-semiespacio-245.py" "$LIBGODC" --estado 2>/dev/null |
    grep -m1 'cada semiespacio' | sed 's/.*cada semiespacio: *\([0-9]*\) B.*/\1/')"
KB_ORIGINAL=$(( ${TECHO:-524288} / 1024 ))

restaura() {
    "$DC_PYTHON" "$HERE/fix-libgodc-semiespacio-245.py" "$LIBGODC" --kb "$KB_ORIGINAL" > /dev/null 2>&1
    [ -d "$SRC" ] && "$DC_PYTHON" "$SRC/scripts/console/mutar-bufio-247.py" --sano > /dev/null 2>&1
}
trap restaura EXIT

# ================================================================================================
# 0. LOS CONTROLES DE PROCEDENCIA
# ================================================================================================
paso "0. los controles de procedencia (el runtime, el techo y el estado del arbol)"
VIEJO="$(find "$LIBGODC/runtime" -name '*.c' -newer "$LIBGODC/libgodc.a" -print -quit 2>/dev/null)"
if [ -n "$VIEJO" ]; then
    rojo "control 0: libgodc.a esta VIEJO ($(basename "$VIEJO") es mas nuevo): corre 'make -C $LIBGODC'"
else
    verde "control 0: libgodc.a al dia; el techo del semiespacio es $TECHO B (de godc_config.h, y el script rehace el .a porque el Makefile no mira las cabeceras)"
fi

if [ "$("$DC_PYTHON" "$HERE/mutar-bufio-247.py" --estado | grep -c '^estado: sano')" = "1" ]; then
    verde "control 0b: el arbol trae el arreglo del ultimo trozo (mutar-bufio-247.py --estado = sano)"
else
    rojo "control 0b: el arbol NO trae el arreglo ($("$DC_PYTHON" "$HERE/mutar-bufio-247.py" --estado)): corre --sano"
fi

if [ -d "$GOX_BASE" ] && [ -f "$GOX_BASE/orden-motor.txt" ]; then
    nota "capa base reusada: $GOX_BASE ($(wc -l < "$GOX_BASE/orden-motor.txt") paquetes del motor)"
else
    echo "no encuentro la capa base $GOX_BASE: corre antes bash scripts/console/checar-trozos-245.sh 25 (es la que arma los 142 paquetes del motor)"; exit 1
fi

# ================================================================================================
# 1. LA COPIA DE FUENTES Y LAS DOS CAPAS
# ================================================================================================
paso "1. la copia de fuentes ($REPO -> $SRC)"
mkdir -p "$SRC"
rsync -a --exclude '.git' "$REPO/" "$SRC/" > /dev/null || exit 1
nota "copia al dia: $SRC"

capa() { # $1 = destino, $2 = estado del lector: sano|mutante
    if [ -d "$1" ]; then mv "$1" "$1.vieja.$(date +%s)"; fi
    cp -a "$GOX_BASE" "$1"
    python3 "$HERE/reescribe-rutas-gox.py" "$GOX_BASE" "$1" "$1" > /dev/null || exit 1
    case "$2" in
        sano) "$DC_PYTHON" "$SRC/scripts/console/mutar-bufio-247.py" --sano > /dev/null || exit 1 ;;
        mutante) "$DC_PYTHON" "$SRC/scripts/console/mutar-bufio-247.py" --mutante > /dev/null || exit 1 ;;
    esac
    for p in $STD_PACKS; do
        bash "$SRC/scripts/console/compilar-paquete-std.sh" "$p" "$1" | sed 's/^/      /'
    done
    nota "capa $1 (lector $2)"
}

sube() { # $1 = capa, $2 = paquete del MOTOR (en la copia)
    ( cd "$SRC" && bash scripts/console/recompilar-paquete-motor.sh "$2" "$1" ) > /dev/null || exit 1
}

arma() { # $1 = nombre, $2 = capa gox, $3 = KB del semiespacio, $4 = .eh_frame fuera del mapa 0|1
    local dir="$SAL/$1"
    if [ -d "$dir" ] && [ -f "$dir/disc.gdi" ]; then mv "$dir" "$dir.vieja.$(date +%s)"; fi
    "$DC_PYTHON" "$HERE/fix-libgodc-semiespacio-245.py" "$LIBGODC" --kb "$3" | sed 's/^/   /' || exit 1
    mkdir -p "$dir"
    ( cd "$SRC" && MKISO_JOLIET=0 DC_RUTA_DATOS=/cd/juego DC_MPQ_JUEGO="$LISTA" DC_SIN_SELLOS=1 \
        DC_DISCO_JUEGO="$JUEGO" POC=motor-arranque DC_EH_FINAL="$4" \
        bash scripts/console/build-motor-arranque.sh "$dir" "$2" ) > "$dir.build.log" 2>&1 ||
        { echo "   NO COMPILA/ARMA $1 (ver $1.build.log)"; exit 1; }
    nota "$1: 1ST_READ.BIN $(stat -c%s "$dir/1ST_READ.BIN") bytes, disc.gdi $(stat -c%s "$dir/disc.iso") bytes (semiespacio ${3} KB, eh_frame=$4)"
}

paso "1a. la capa y la imagen del ARREGLO (lector sano)"
capa "$SAL/gox247-sano" sano
for p in $PACKS; do sube "$SAL/gox247-sano" "$p"; done
arma "sano247" "$SAL/gox247-sano" "$KB_ORIGINAL" 1

paso "1b. la capa y la imagen del MUTANTE (el lector de antes)"
capa "$SAL/gox247-mut" mutante
for p in $PACKS; do sube "$SAL/gox247-mut" "$p"; done
arma "mut247" "$SAL/gox247-mut" "$KB_ORIGINAL" 1

paso "1c. la imagen de la PARTICAON VIEJA (lector sano, semiespacio 512 KB, .eh_frame adentro del mapa)"
arma "sano247-viejo" "$SAL/gox247-sano" 512 0
"$DC_PYTHON" "$HERE/fix-libgodc-semiespacio-245.py" "$LIBGODC" --kb "$KB_ORIGINAL" > /dev/null || exit 1

# ================================================================================================
# 2. EL CONTROL DE PROCEDENCIA DE LAS DOS CAPAS
# ================================================================================================
paso "2. control 1: las dos capas NO traen el mismo lector"
SHA_SANO="$(sha256sum "$SAL/gox247-sano/bufio.gox" | cut -c1-16)"
SHA_MUT="$(sha256sum "$SAL/gox247-mut/bufio.gox" | cut -c1-16)"
if [ -n "$SHA_SANO" ] && [ "$SHA_SANO" != "$SHA_MUT" ]; then
    verde "control 1: bufio.gox difiere entre las capas (arreglo $SHA_SANO $(stat -c%s "$SAL/gox247-sano/bufio.gox") B / mutante $SHA_MUT $(stat -c%s "$SAL/gox247-mut/bufio.gox") B)"
else
    rojo "control 1: bufio.gox es el MISMO en las dos capas ('$SHA_SANO'): el mutante no se recompilo y la corrida no mide nada"
fi

# el mutante y la vuelta, sobre la COPIA (el checkout no se toca)
"$DC_PYTHON" "$SRC/scripts/console/mutar-bufio-247.py" --mutante > /dev/null
paso "3. control 7 (host): la difftest de d2std con el lector de cada lado"
DIFT_MUT="$(cd "$SRC/console/dc/std" && go test ./difftest/ -run DC00247 2>&1 | tail -1)"
"$DC_PYTHON" "$SRC/scripts/console/mutar-bufio-247.py" --sano > /dev/null
DIFT_SANO="$(cd "$SRC/console/dc/std" && go test ./difftest/ -run DC00247 2>&1 | tail -1)"
case "$DIFT_SANO" in
    ok*) verde "control 7a: con el lector arreglado la difftest PASA ($DIFT_SANO)" ;;
    *) rojo "control 7a: la difftest NO pasa con el lector arreglado ($DIFT_SANO)" ;;
esac
case "$DIFT_MUT" in
    ok*) rojo "control 7b: la difftest PASA con el lector viejo ($DIFT_MUT): la prueba no caza la regresion" ;;
    *) verde "control 7b: con el lector viejo la difftest FALLA ($DIFT_MUT): la prueba caza la regresion" ;;
esac

# ================================================================================================
# 4. LAS TRES CORRIDAS
# ================================================================================================
paso "4. las tres corridas ($SEGS s cada una)"
for caso in "sano247:run247-sano" "mut247:run247-mut" "sano247-viejo:run247-viejo"; do
    img="${caso%%:*}"; sal="${caso##*:}"
    echo "   -- $img"
    bash "$HERE/captura-ventana-234.sh" "$SAL/$img/disc.gdi" "$SEGS" "$SAL/$sal" | sed -n 's/^/      /p' | head -6
done

# ================================================================================================
# 5. LOS CONTROLES DEL ARRANQUE
# ================================================================================================
S="$SAL/run247-sano"; M="$SAL/run247-mut"; V="$SAL/run247-viejo"

paso "5. control 2 (sano): LvlWarp.txt -- la tabla que el lector viejo leia VACIA -- se leyo"
ENTRA=$(log_de "$S" | grep -an 'ARQ-CARGA lee-entra .*LvlWarp.txt' | head -1 | cut -d: -f1)
VOLVIO=$(log_de "$S" | grep -an 'ARQ-CARGA lee-volvio .*LvlWarp.txt' | head -1 | cut -d: -f1)
if [ -n "${ENTRA:-}" ] && [ -n "${VOLVIO:-}" ] && [ "$VOLVIO" -gt "$ENTRA" ]; then
    verde "control 2: LvlWarp.txt tiene lee-volvio (linea $VOLVIO) despues de su lee-entra (linea $ENTRA)"
else
    rojo "control 2: LvlWarp.txt NO se leyo (lee-entra=${ENTRA:-ninguno} lee-volvio=${VOLVIO:-ninguno})"
fi

paso "6. control 3 (sano): el motor SIGUE -- pide una tabla POSTERIOR a LvlWarp"
POSTERIORES=("objtype" "Objects" "weapons" "armor" "misc" "MonStats")
SIGUE=""
for t in "${POSTERIORES[@]}"; do
    if log_de "$S" | grep -qa "ARQ-CARGA .*excel/$t.txt"; then SIGUE="$t"; break; fi
done
if [ -n "$SIGUE" ]; then
    verde "control 3: el sano pidio $SIGUE.txt DESPUES de LvlWarp.txt"
else
    rojo "control 3: el sano NO pidio ninguna tabla posterior a LvlWarp.txt"
fi

paso "7. control 4 (sano) y control 5 (mutante): el numero del arranque"
A_SANO=$(assets_de "$S")
A_MUT=$(assets_de "$M")
A_VIEJO=$(assets_de "$V")
if [ "$A_SANO" -gt 3 ]; then
    verde "control 4: el sano pidio $A_SANO assets del MPQ (la pared de DC-002.45 eran 3): tablas leidas [$([ "$A_SANO" -gt 0 ] && tablas_leidas_de "$S")]"
else
    rojo "control 4: el sano se quedo en $A_SANO assets del MPQ (el arranque NO paso de las 3 tablas de DC-002.45)"
fi
MUT_POST=""
for t in "${POSTERIORES[@]}"; do
    if log_de "$M" | grep -qa "ARQ-CARGA .*excel/$t.txt"; then MUT_POST="$t"; break; fi
done
if [ "$A_MUT" = "3" ] && [ -z "$MUT_POST" ]; then
    verde "control 5: el mutante se quedo en las 3 tablas y NO pidio ninguna posterior a LvlWarp.txt (tablas leidas [$(tablas_leidas_de "$M")])"
else
    rojo "control 5: el mutante pidio $A_MUT assets y una posterior es '${MUT_POST:-ninguna}': el mutante NO reproduce la pared (el arreglo no seria el que la corre)"
fi

paso "8. control 6 (criterio 3): el mutante devuelve el arranque a 3 tablas"
if [ "$A_SANO" -gt "$A_MUT" ]; then
    verde "control 6: sano $A_SANO assets > mutante $A_MUT assets (mismo binario, lo unico que cambia es el lector)"
else
    rojo "control 6: sano $A_SANO assets contra mutante $A_MUT: el mutante NO vuelve a las 3 tablas"
fi

# ================================================================================================
# 9. LOS NUMEROS (el dato vivo, las recolecciones y la particion)
# ================================================================================================
paso "9. control 8 (numeros): el dato vivo y las recolecciones de las tres corridas"
for caso in "sano:$S" "mutante:$M" "particion-vieja:$V"; do
    nombre="${caso%%:*}"; d="${caso##*:}"
    VV="$(vivo_de "$d")"; GG="$(gc_de "$d")"; TT="$(techo_de "$d")"
    if [ -n "${VV:-}" ] && [ -n "${TT:-}" ]; then
        P=$((VV * 1000 / TT))
        nota "$nombre: dato vivo max $VV B de $TT ($((P / 10)).$((P % 10)) %), gc_collect max $GG, assets $([ "$nombre" = sano ] && echo "$A_SANO" || { [ "$nombre" = mutante ] && echo "$A_MUT" || echo "$A_VIEJO"; })"
    else
        nota "$nombre: sin marcas gc-vivo ni techo (el runtime no trae instrumentar-vivo-245.py)"
    fi
done
# LA COMPARACION TIENE QUE SER AL MISMO TRAMO. Comparar los MAXIMOS del recorrido no dice nada de la
# particion: el que llega mas lejos llena mas su semiespacio. Medido (corrida de hoy): el sano llega a 6
# assets y el suyo da 98,9 % de 1.048.576, mientras la particion vieja se queda en 2 assets y da 97,9 % de
# 524.288 -- al reves, y sin que la particion haya empeorado: el que se muere antes no alcanza a llenarse.
# Lo que SI compara particiones es el MISMO tramo: el `gsemi` que deja la PRIMERA tabla del MPQ.
TRAMO="/data/global/excel/LvlTypes.txt"
gsemi_en() { log_de "$1" | grep -a "lee-volvio $TRAMO" | head -1 | grep -ao 'gsemi=[0-9]*/[0-9]*' | head -1; }
GS_S="$(gsemi_en "$S")"; GS_V="$(gsemi_en "$V")"
if [ -n "$GS_S" ] && [ -n "$GS_V" ]; then
    US="${GS_S#gsemi=}"; US="${US%%/*}"; PTS="${GS_S##*/}"
    UV="${GS_V#gsemi=}"; UV="${UV%%/*}"; PTV="${GS_V##*/}"
    PPS=$((US * 1000 / PTS)); PPV=$((UV * 1000 / PTV))
    if [ "$PPS" -lt "$PPV" ]; then
        verde "control 8: al MISMO tramo (lee-volvio de $(basename "$TRAMO")) la particion nueva usa $US de $PTS ($((PPS / 10)).$((PPS % 10)) %) contra $UV de $PTV ($((PPV / 10)).$((PPV % 10)) %) de la vieja; en el maximo del recorrido son 98,9 % contra 97,9 % porque el sano LLEGA MAS LEJOS ($A_SANO assets contra $A_VIEJO)"
    else
        rojo "control 8: al MISMO tramo la particion nueva NO baja la ocupacion ($((PPS / 10)).$((PPS % 10)) % contra $((PPV / 10)).$((PPV % 10)) %)"
    fi
else
    nota "control 8: sin el tramo comun ($TRAMO) en las dos corridas (sano='$GS_S' vieja='$GS_V') no se puede comparar la particion"
fi

echo
echo "== RESULTADO: $OK controles OK, $FALLOS con fallo =="
[ "$FALLOS" -eq 0 ]
