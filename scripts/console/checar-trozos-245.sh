#!/bin/bash
# DC-002.45: LA TABLA DEL RECORD MANAGER SE CONSUME POR TROZOS -- el arreglo medido en la consola y su MUTANTE.
#
# QUE MIDE (criterios 1 y 3 de la tarjeta): el arranque del MOTOR ENTERO (POC `motor-arranque`) con los TRES
# MPQ del disco, corrido en Flycast con la consola serie capturada, DOS veces sobre la MISMA copia de fuentes:
#
#   SANO      el arreglo de esta tarjeta: `LoadDataDictionary` le pasa a la tabla el STREAM del asset y el
#             `csv.Reader` lo consume POR TROZOS (el MPQ ya entrega por SECTOR). Tiene que PASAR la lectura de
#             `LvlPrest.txt` (134.937 B contra 132.104 libres en el semiespacio al entrar, DC-002.43) y seguir
#             leyendo tablas DESPUES de ella.
#   MUTANTE   la MISMA copia con `fix-tablas-trozos-245.py --pristino`: `LoadFile` (el `ioutil.ReadAll` del
#             asset entero) + la tabla armada con esos bytes. Tiene que VOLVER a morir ADENTRO de la lectura
#             de `LvlPrest.txt` -- la ultima marca `lee-entra` de ese asset y el `fatal error` detras -- o sea
#             que la prueba caza su regresion (si el mutante pasara, el arreglo no estaria haciendo nada).
#
# Los DOS numeros que se piden (el criterio 1 dice "el numero, no el relato"): el `gsemi` del semiespacio del
# GC en las marcas de `LvlPrest.txt`, y el maximo del semiespacio de la corrida.
#
# CONTROLES:
#   0   libgodc.a al dia (si no, el binario que se mide no es el de las fuentes: DC-002.43)
#   0b  el arbol del checkout en el estado de DC-002.33 (7/7) -- es el estado del motor que se mide
#   1   la copia de fuentes lleva el arreglo (`fix-tablas-trozos-245.py --estado` = sano)
#   2   PROCEDENCIA DEL BINARIO: el .gox de d2txt de la capa del arreglo TRAE `LoadDataDictionaryReader`
#       (el simbolo nuevo) y el de la capa del mutante NO. Sin esto, un .gox viejo se lee como "el arreglo no
#       cambio nada"
#   3   [sano] la lectura de `LvlPrest.txt` TERMINO: hay `lee-volvio .../LvlPrest.txt` despues de su
#       `lee-entra`, y el `gsemi` de esa marca esta por DEBAJO del techo (524288)
#   4   [sano] el motor SIGUE: pide una tabla POSTERIOR a LvlPrest en `d2resource.DataDictionaries()`
#   5   [mutante] la muerte es ADENTRO de la lectura: ultima marca `lee-entra .../LvlPrest.txt` + `fatal error`
#   6   [mutante] la lectura NO termino (no hay `lee-volvio .../LvlPrest.txt`)
#
# Uso:  bash scripts/console/checar-trozos-245.sh [--refrescar] [segundos_por_corrida]
#   --refrescar   rehace la copia, las capas y las DOS imagenes desde cero (si no, lo que ya esta en disco se
#                 reusa: los enlaces son de minutos y las imagenes de 553 MB)
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
SRC="${SRC:-/home/dgalli/.hermes/cache/scratch/trozos-245-src}"
SAL="${SAL:-/home/dgalli/.hermes/cache/scratch/trozos-245}"
MPQ="${MPQ:-$HOME/.wine-d2/drive_c/Games/Diablo II}"
# La capa BASE: los 142 paquetes del MOTOR recompilados DESDE la copia de fuentes de este arbol. Se parte de
# una capa del arbol (solo por su stdlib minima: 63 .gox de `d2std`/terceros, que no cambian con esta
# tarjeta) y se recompila el motor ENTERO adentro para que no quede ni un paquete viejo. PARTIDA es una capa
# del arbol de HOY (si el arranque pidiera un simbolo que esa capa no tiene, el POC no enlaza: es el
# sintoma de una capa de otro dia -- medido con /tmp/gox-233, que es de antes de DC-002.42 y no trae
# `d2records.RutaDeLosLookups`).
PARTIDA="${PARTIDA:-/home/dgalli/.hermes/cache/scratch/dc242-rev/gox-fix}"
GEN_BASE="$SAL/gox-base"
GOX_SANO="$SAL/gox-sano"
GOX_MUT="$SAL/gox-mut"
LISTA="patch_d2.mpq,d2data.mpq,d2char.mpq"
SEGS="${2:-25}"
REFRESCAR=0
case "${1:-}" in
    --refrescar) REFRESCAR=1 ;;
    # DC-002.47: el uso documentado es `checar-trozos-245.sh 50` (los segundos primero, como en los demas
    # corpus). Antes eso dejaba SEGS=25 -- el valor por defecto del segundo argumento -- y las corridas se
    # cortaban a los 25 s sin que nada lo dijera (medido: los `win-025s.png` de todas las corridas). Un numero
    # suelto en $1 son los segundos.
    ""|*[!0-9]*) ;;
    *) SEGS="$1" ;;
esac

LIBGODC="${LIBGODC:-$HOME/dreamcast/libgodc}"
PACKS="d2common/d2fileformats/d2txt d2common/d2loader d2core/d2asset"
# DC-002.47: EL TECHO NO ES UNA CONSTANTE, ES UNA VOLUNTAD. El tamano del semiespacio del GC lo fija
# `godc_config.h` del runtime vendorizado y esta tarjeta lo movio (la voluntad se deja con
# `fix-libgodc-semiespacio-245.py --kb N`, que ademas rehace libgodc.a). Si el techo queda escrito a mano en
# los controles, una corrida con otro semiespacio mide contra un numero que no es el suyo: los controles
# tienen que leer el techo de donde vive. Se lee de la misma voluntad.
#
# Y LA VOLUNTAD LA FIJA EL CORPUS, NO EL ENTORNO DE TURNO (medido, 2026-10-19): el runtime vendorizado es UNO
# SOLO y lo comparten todas las tarjetas del tablero, asi que entre dos corridas de este corpus alguien puede
# dejarlo en el semiespacio viejo: la corrida entonces mide OTRA particion y los controles de la lectura se
# ponen rojos sin que el arreglo haya cambiado (medido: el semiespacio en 512 KB mientras el corpus leia el
# techo al empezar, marcas `gsemi=373472/524288` contra `TECHO=1048576`). El corpus DECLARA la particion que
# mide y la deja puesta antes de armar nada. Es la misma regla que con la capa de partida: el binario que se
# mide tiene que salir del arbol y de la particion que dice el corpus.
SEMI_KB="${SEMI_KB:-1024}"
"$DC_PYTHON" "$HERE/fix-libgodc-semiespacio-245.py" "$LIBGODC" --kb "$SEMI_KB" | sed 's/^/   /'
TECHO="$("$DC_PYTHON" "$HERE/fix-libgodc-semiespacio-245.py" "$LIBGODC" --estado 2>/dev/null |
    grep -m1 'cada semiespacio' | sed 's/.*cada semiespacio: *\([0-9]*\) B.*/\1/')"
[ -n "${TECHO:-}" ] || TECHO=$((SEMI_KB * 1024))
LECTURA="data/global/excel/LvlPrest.txt"
# DC-002.47: los paquetes de la stdlib minima que se recompilan del arbol en cada capa (ver `capa()`).
STD_PACKS="${STD_PACKS:-bufio}"

OK=0
FALLOS=0
nota() { echo "   $*"; }
verde() { OK=$((OK + 1)); echo "   OK: $*"; }
rojo() { FALLOS=$((FALLOS + 1)); echo "   FALLO: $*"; }
paso() { printf '\n== %s\n' "$*"; }
log_de() { tr -d '\r' < "$1/flycast.log"; }

mkdir -p "$SAL/juego"

# ================================================================================================
# 0. LOS CONTROLES DE PROCEDENCIA
# ================================================================================================
paso "0. los controles de procedencia (el runtime, el arbol y el estado del arreglo)"
VIEJO="$(find "$LIBGODC/runtime" -name '*.c' -newer "$LIBGODC/libgodc.a" -print -quit 2>/dev/null)"
if [ -n "$VIEJO" ]; then
    rojo "control 0: libgodc.a esta VIEJO ($(basename "$VIEJO") es mas nuevo): corre 'make -C $LIBGODC' y volve a medir"
else
    verde "control 0: libgodc.a ($(stat -c%s "$LIBGODC/libgodc.a") bytes) es mas nuevo que sus fuentes; el corpus deja el semiespacio en $SEMI_KB KB y lo lee de godc_config.h ($TECHO B)"
fi

if [ "$(cd "$REPO" && "$DC_PYTHON" scripts/console/fix-mpq-tablas-233.py --estado | tail -1 | grep -c '7/7 como el arreglo')" = "1" ]; then
    verde "control 0b: el arbol del checkout esta como el arreglo de DC-002.33 (7/7)"
else
    rojo "control 0b: el arbol NO esta como el arreglo (correr fix-mpq-tablas-233.py --sano)"
fi

# control 0e (DC-002.45): el runtime trae LOS ITABS FUERA DEL MONTON MOVIBLE. Sin esto la corrida del arreglo
# no mide la lectura: el camino por trozos hace una recoleccion cada 4 KB de tabla, el cache de itabs de
# `.bss` queda apuntando a la mitad vieja del semiespacio y la CPU salta a basura (PC 6c000000) DENTRO de un
# gc_collect. Medido en esta tarjeta; el porque esta en scripts/console/fix-libgodc-itab-245.py.
#
# Y DC-002.47: el control tambien exige EL INSTRUMENTO DEL DATO VIVO. La cadena del runtime
# (`fix-libgodc-cadena.sh`) RESETEA `gc_copy.c` y se lleva la marca `gc-vivo` (que es el numero que decide
# esta tarjeta): sin ella la corrida sigue, pero los numeros no existen y el corpus solo puede dejar una
# nota -- un corpus que no puede medir su numero no puede decir que esta verde. Medido el 2026-10-19: el
# instrumento se habia caido con un rearme del vendor y el sano corrio sin una sola marca `gc-vivo`.
ESTADO_ITAB="$("$DC_PYTHON" "$HERE/fix-libgodc-itab-245.py" "$LIBGODC" --estado 2>/dev/null | tail -1)"
INSTRUMENTO="$(grep -cF 'godc_marca("gc-vivo"' "$LIBGODC/runtime/gc_copy.c" 2>/dev/null || true)"
case "$ESTADO_ITAB" in
    *"arreglado"*)
        if [ "${INSTRUMENTO:-0}" -ge 1 ]; then
            verde "control 0e: el runtime pide los itabs al monton de C (fuera del monton movible) y trae el instrumento del dato vivo (gc_copy.c)"
        else
            rojo "control 0e: el runtime trae el itab arreglado pero le FALTA la marca 'gc-vivo' en gc_copy.c: los numeros de la corrida no existen. Correr: python3 $HERE/instrumentar-vivo-245.py $LIBGODC && make -C $LIBGODC"
        fi ;;
    *) rojo "control 0e: el runtime NO trae el arreglo de itabs ($ESTADO_ITAB): correr fix-libgodc-itab-245.py $LIBGODC y make" ;;
esac

# la COPIA DE FUENTES: la mutacion del mutante NO se hace en el checkout (lo puede estar usando otro agente
# del tablero y el arreglo tiene que quedar puesto en el repo)
paso "1. la copia de fuentes ($REPO -> $SRC)"
if [ "$REFRESCAR" = 1 ] || [ ! -d "$SRC/d2core" ]; then
    mkdir -p "$SRC"
    rsync -a --exclude '.git' "$REPO/" "$SRC/" || exit 1
fi
# la copia tiene que llevar el ARBOL de AHORA (si quedo vieja, se mide otro motor): se sincroniza siempre
rsync -a --exclude '.git' "$REPO/" "$SRC/" > /dev/null || exit 1
nota "copia al dia: $SRC"

ESTADO="$(cd "$SRC" && python3 scripts/console/fix-tablas-trozos-245.py --estado 2>/dev/null | grep -m1 '^estado: ' | sed 's/estado: //')"
if [ "$ESTADO" = "sano" ]; then
    verde "control 1: la copia lleva el arreglo (la tabla por trozos)"
else
    rojo "control 1: la copia quedo en estado '$ESTADO' (correr fix-tablas-trozos-245.py --sano en $SRC)"
    (cd "$SRC" && python3 scripts/console/fix-tablas-trozos-245.py --sano)
fi

# ================================================================================================
# 2. LA CARPETA DEL JUEGO (enlaces duros) Y LA TABLA DE OBJECTLOOKUPS
# ================================================================================================
paso "2. la carpeta del juego y la tabla de objectLookups (el arranque no pasa de leer su config sin ella)"
for n in patch_d2.mpq d2data.mpq d2char.mpq; do
    real="$(ls -1 "$MPQ" | grep -ix "$n" | head -1)"
    [ -n "$real" ] || { echo "no encuentro $n en $MPQ"; exit 1; }
    ln -f "$MPQ/$real" "$SAL/juego/$n" 2>/dev/null || cp "$MPQ/$real" "$SAL/juego/$n"
done
nota "MPQ horneados: $(ls "$SAL/juego" | tr '\n' ' ') ($(du -sb "$SAL/juego" | cut -f1) bytes)"

if [ ! -f "$SAL/juego/objectlookups.pack" ]; then
    if (cd "$SRC" && go run ./console/dc/gen-lookups-pack -dir "$SAL/juego") > "$SAL/gen-lookups-pack.log" 2>&1; then
        nota "tabla de objectLookups horneada: $(stat -c%s "$SAL/juego/objectlookups.pack") bytes"
    else
        echo "no pude armar objectlookups.pack (ver $SAL/gen-lookups-pack.log)"; exit 1
    fi
else
    nota "tabla de objectLookups horneada: $(stat -c%s "$SAL/juego/objectlookups.pack") bytes (ya estaba)"
fi

# ================================================================================================
# 3. LAS DOS CAPAS Y LAS DOS IMAGENES
# ================================================================================================
revisa_rutas() { # $1 = capa de .gox: no puede quedar ni una ruta absoluta vieja /tmp/gox-* adentro
    local n
    n=$(grep -c '/tmp/gox' "$1/orden-motor.txt" 2>/dev/null || true)
    if [ "${n:-0}" != "0" ]; then
        echo "   FALLO: $1 conserva $n rutas /tmp/gox en orden-motor.txt (correr reescribe-rutas-gox.py)"
        return 1
    fi
    return 0
}

capa() { # $1 = destino, $2 = dir de donde copiar (vacio = $GEN_BASE)
    # SIEMPRE se copia de nuevo: una capa a medio armar (o de otro dia) se lee como "el arreglo no cambio
    # nada" y es el peor error de esta medicion. La copia son 71 MB: no se reusa por ahorrar eso.
    if [ -d "$1" ]; then mv "$1" "$1.vieja.$(date +%s)"; fi
    cp -a "${2:-$GEN_BASE}" "$1"
    python3 "$HERE/reescribe-rutas-gox.py" "${2:-$GEN_BASE}" "$1" "$1" > /dev/null || exit 1
    revisa_rutas "$1" || exit 1
    nota "capa copiada de ${2:-$GEN_BASE}: $1"
    # DC-002.47: LA STDLIB MINIMA TAMBIEN ES DEL ARBOL DE HOY. La capa de partida trae los .gox de la stdlib
    # compilados en un arbol de otro dia, y esta tarjeta CAMBIO un paquete de la stdlib (el lector por trozos
    # de `d2std/bufio`). Sin este paso la capa lleva el lector VIEJO y la corrida mide el arbol de ayer sin
    # que nada lo diga -- el mismo error que el comentario de arriba. Se recompilan de la COPIA de fuentes
    # (el snapshot que este corpus acaba de sacar), NO del checkout vivo: otro agente del tablero puede estar
    # editando el checkout a mitad de la corrida y las dos capas tienen que salir del MISMO arbol.
    for p in $STD_PACKS; do
        sha="$(bash "$SRC/scripts/console/compilar-paquete-std.sh" "$p" "$1" | tail -1)"
        nota "   stdlib del arbol: $sha"
    done
}

compila() { # $1 = capa, $2 = paquete (en la COPIA de fuentes)
    ( cd "$SRC" && bash scripts/console/recompilar-paquete-motor.sh "$2" "$1" ) || exit 1
}

armar() { # $1 = nombre (dir), $2 = dir de .gox
    local dir="$SAL/$1"
    if [ "$REFRESCAR" = 0 ] && [ -f "$dir/1ST_READ.BIN" ] && [ "$dir/prog.elf" -nt "$SRC/d2core/d2asset/asset_manager.go" ]; then
        nota "ya construido: $1"; return 0
    fi
    revisa_rutas "$2" || exit 1
    # DC-002.47: la PARTICION de memoria es parte del entorno del corpus, no un detalle de cada corrida: el
    # semiespacio lo fija `godc_config.h` del vendor (`fix-libgodc-semiespacio-245.py --kb`) y el `.eh_frame`
    # fuera del mapa lo agrega esta palanca del enlace (`DC_EH_FINAL=1`, medido: el fin del `.bss` baja
    # 1.076.448 B y el monton de C gana ese techo). Se declara aca para que el corpus sea reproducible: el
    # binario que se mide es el de la particion que la tarjeta dejo, no el que salga del entorno de turno.
    ( cd "$SRC" && MKISO_JOLIET=0 DC_RUTA_DATOS=/cd/juego DC_MPQ_JUEGO="$LISTA" DC_SIN_SELLOS=1 \
        DC_DISCO_JUEGO="$SAL/juego" POC=motor-arranque DC_EH_FINAL="${DC_EH_FINAL:-1}" \
        bash scripts/console/build-motor-arranque.sh "$dir" "$2" ) > "$dir.build.log" 2>&1 ||
        { echo "   NO COMPILA/ARMA $1 (ver $1.build.log)"; exit 1; }
    nota "$1: 1ST_READ.BIN $(stat -c%s "$dir/1ST_READ.BIN") bytes, ISO $(stat -c%s "$dir/disc.iso") bytes"
}

paso "3. la capa BASE: los 142 paquetes del MOTOR recompilados desde la COPIA de fuentes"
# control 0f (DC-002.45): orden-motor.txt SIN repetidos. Dos lineas iguales en la lista de enlace matan al
# linker con `multiple definition of ...` (medido en esta tarjeta: 171 lineas, 29 repetidas, 142 unicas, y el
# arranque no se arma). Se deduplica arriba (se queda la PRIMERA aparicion, que es el orden de la capa 1) y el
# control dice cuantas habia.
ordensito() {
    local f="$GEN_BASE/orden-motor.txt" dup
    [ -f "$f" ] || return 0
    dup="$(sort "$f" | uniq -d | wc -l)"
    if [ "$dup" != "0" ]; then
        awk '!visto[$0]++' "$f" > "$f.sin-dup" && mv "$f.sin-dup" "$f"
        nota "orden-motor.txt tenia $dup lineas repetidas: deduplicado a $(wc -l < "$f") paquetes"
    fi
    dup="$(sort "$f" | uniq -d | wc -l)"
    if [ "$dup" = "0" ]; then
        verde "control 0f: orden-motor.txt sin repetidos ($(wc -l < "$f") paquetes, un .gox cada uno)"
    else
        rojo "control 0f: orden-motor.txt conserva $dup repetidas: el enlace va a morir por multiple definition"
    fi
}
if [ ! -d "$PARTIDA" ]; then
    rojo "no existe la capa de partida $PARTIDA (una capa del arbol de HOY: solo se usa por su stdlib)"
    echo; echo "== RESULTADO: $OK controles OK, $FALLOS con fallo =="; exit 1
fi
# DC-002.48: LA CAPA BASE SE REHACE TAMBIEN CUANDO EL LOG NO DICE CUANTOS COMPILO. Antes bastaba con que
# `motor-all.log` EXISTIERA, y una corrida matada a mitad deja el archivo en 0 bytes: la corrida siguiente
# se salta la recompilacion, hereda el `orden-motor.txt` de la capa de PARTIDA (111 de 142 paquetes, sin
# d2records) y el enlace muere con TODOS los simbolos de d2records indefinidos -- que se lee como "el arbol
# no compila" cuando el que no compilo nunca fue invocado (medido el 2026-10-19: el log tenia 0 bytes).
if [ "$REFRESCAR" = 1 ] || [ ! -f "$GEN_BASE/orden-motor.txt" ] || \
   ! grep -q '^compilados:' "$SAL/motor-all.log" 2>/dev/null; then
    if [ -d "$GEN_BASE" ]; then mv "$GEN_BASE" "$GEN_BASE.vieja.$(date +%s)"; fi
    cp -a "$PARTIDA" "$GEN_BASE"
    python3 "$HERE/reescribe-rutas-gox.py" "$PARTIDA" "$GEN_BASE" "$GEN_BASE" > /dev/null || exit 1
    ( cd "$SRC" && bash scripts/console/build-motor-all.sh "$GEN_BASE" "$SAL/motor-all-log" ) > "$SAL/motor-all.log" 2>&1
fi
COMPILADOS="$(grep -a -m1 '^compilados:' "$SAL/motor-all.log" | sed 's/compilados: //')"
nota "capa base: $COMPILADOS paquetes compilados (log: $SAL/motor-all.log)"
if [ "$(echo "$COMPILADOS" | awk -F' de ' '{print ($1 == $2) ? "si" : "no"}')" = "si" ]; then
    verde "control 0c: el motor entero compilo desde la copia ($COMPILADOS): la capa base no tiene paquetes viejos"
else
    rojo "control 0c: el motor NO compilo entero ($COMPILADOS): ver $SAL/motor-all.log"
fi

# la capa base tiene que traer los paquetes del arbol de HOY: el POC pide `d2records.RutaDeLosLookups`, que
# existe desde DC-002.42. Con una capa de antes de ese dia el POC no enlaza (medido con /tmp/gox-233)
LOOKUPS_GOX="$GEN_BASE/github.com/OpenDiablo2/OpenDiablo2/d2core/d2records.gox"
N_LOOKUPS=$(grep -ac 'AbrirLookupsDelArchivo' "$LOOKUPS_GOX" 2>/dev/null || true)
if [ "${N_LOOKUPS:-0}" -ge 1 ]; then
    verde "control 0d: la capa base trae la tabla de lookups del disco ($N_LOOKUPS): es una capa del arbol de hoy"
else
    rojo "control 0d: la capa base NO trae la tabla de lookups del disco: es una capa vieja (el POC no enlaza)"
fi

paso "3a. la capa y la imagen del ARREGLO (la tabla por trozos)"
ordensito
capa "$GOX_SANO"
for p in $PACKS; do compila "$GOX_SANO" "$p"; done | sed 's/^/   /'
armar sano-con-juego "$GOX_SANO"

paso "3b. la capa y la imagen del MUTANTE (el asset entero en memoria)"
(cd "$SRC" && python3 scripts/console/fix-tablas-trozos-245.py --pristino) | sed 's/^/   /'
ESTADO_MUT="$(cd "$SRC" && python3 scripts/console/fix-tablas-trozos-245.py --estado 2>/dev/null | grep -m1 '^estado: ' | sed 's/estado: //')"
if [ "$ESTADO_MUT" = "pristino" ]; then
    nota "la copia quedo en pristino (el mutante)"
else
    echo "no pude poner la copia en pristino"; exit 1
fi
capa "$GOX_MUT" "$GOX_SANO"
for p in $PACKS; do compila "$GOX_MUT" "$p"; done | sed 's/^/   /'
armar mutante-con-juego "$GOX_MUT"
(cd "$SRC" && python3 scripts/console/fix-tablas-trozos-245.py --sano) | sed 's/^/   /'
ESTADO_SRC="$(cd "$SRC" && python3 scripts/console/fix-tablas-trozos-245.py --estado 2>/dev/null | grep -m1 '^estado: ' | sed 's/estado: //')"
nota "la copia quedo de vuelta en el arreglo (el checkout no se toco)"

# ================================================================================================
# 4. EL CONTROL DE PROCEDENCIA DE LAS DOS CAPAS (que el binario sea el que se cree que es)
# ================================================================================================
paso "4. control 2: la capa del arreglo y la del mutante NO son el mismo binario"
GOX_TXT="$GOX_SANO/github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2txt.gox"
GOX_TXT_MUT="$GOX_MUT/github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2txt.gox"
# OJO: el control NO puede mirar `LoadDataDictionaryReader` en d2txt.gox, como decia la primera version: esa
# funcion EXISTE en los dos arboles (el que se muta es el que la LLAMA, `asset_manager.go`, o sea d2asset.gox)
# y el simbolo aparece igual en las dos capas -- medido: 4 y 4, que se lee como "el mutante no se recompilo"
# cuando en realidad si. Lo que se mira es el paquete que decide: d2asset.gox tiene que DIFERIR entre las dos
# capas (si es el mismo archivo, el mutante es una copia del arreglo y la corrida no mide nada).
GOX_AM="$GOX_SANO/github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset.gox"
GOX_AM_MUT="$GOX_MUT/github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset.gox"
SHA_AM="$(sha256sum "$GOX_AM" 2>/dev/null | cut -c1-16)"
SHA_AM_MUT="$(sha256sum "$GOX_AM_MUT" 2>/dev/null | cut -c1-16)"
if [ -n "$SHA_AM" ] && [ -n "$SHA_AM_MUT" ] && [ "$SHA_AM" != "$SHA_AM_MUT" ]; then
    verde "control 2: d2asset.gox difiere entre las dos capas (arreglo $SHA_AM $(stat -c%s "$GOX_AM") B / mutante $SHA_AM_MUT $(stat -c%s "$GOX_AM_MUT") B)"
else
    rojo "control 2: d2asset.gox es el MISMO en las dos capas (sha '$SHA_AM' vs '$SHA_AM_MUT'): el mutante no se recompilo"
fi

# control 2b: la copia estaba en PRISTINO cuando se compilo el mutante. Se guarda el estado del paso 3b (el
# script no sigue si la mutacion no quedo puesta) y se comprueba que la copia volvio al arreglo despues.
if [ "${ESTADO_MUT:-}" = "pristino" ] && [ "${ESTADO_SRC:-}" = "sano" ]; then
    verde "control 2b: la copia estaba en 'pristino' al compilar el mutante y quedo en 'sano' despues"
else
    rojo "control 2b: la copia estaba en '${ESTADO_MUT:-?}' al compilar el mutante y en '${ESTADO_SRC:-?}' al terminar"
fi

# ================================================================================================
# 5. LAS DOS CORRIDAS
# ================================================================================================
paso "5a. la corrida del ARREGLO"
if [ "$REFRESCAR" = 1 ] || [ ! -f "$SAL/run-sano/flycast.log" ]; then
    bash "$HERE/captura-ventana-234.sh" "$SAL/sano-con-juego/disc.gdi" "$SEGS" "$SAL/run-sano"
else
    nota "corrida ya hecha: run-sano ($(log_de "$SAL/run-sano" | grep -ac 'ARQ-CARGA') marcas de carga)"
fi

paso "5b. la corrida del MUTANTE"
if [ "$REFRESCAR" = 1 ] || [ ! -f "$SAL/run-mutante/flycast.log" ]; then
    bash "$HERE/captura-ventana-234.sh" "$SAL/mutante-con-juego/disc.gdi" "$SEGS" "$SAL/run-mutante"
else
    nota "corrida ya hecha: run-mutante ($(log_de "$SAL/run-mutante" | grep -ac 'ARQ-CARGA') marcas de carga)"
fi

# ================================================================================================
# 6. LA LECTURA DEL ARREGLO (criterios 1 y 2) Y LA DEL MUTANTE (criterio 3)
# ================================================================================================
paso "6. la lectura de $LECTURA en las dos corridas (la consola serie, literal)"
for e in sano mutante; do
    echo "   -- $e:"
    log_de "$SAL/run-$e" | grep -a "ARQ-CARGA.*$LECTURA" | sed 's/^/      /' | tail -6
done

paso "7. control 3 (sano): la lectura de LvlPrest.txt TERMINO y con el semiespacio debajo del techo"
LSANO="$SAL/run-sano/flycast.log"
LEE_ENTRA=$(log_de "$SAL/run-sano" | grep -an "ARQ-CARGA lee-entra .*$LECTURA" | head -1 | cut -d: -f1)
LEE_VOLVIO_LINEA=$(log_de "$SAL/run-sano" | grep -an "ARQ-CARGA lee-volvio .*$LECTURA" | head -1 | cut -d: -f1)
GSEMI_VOLVIO=""
[ -n "${LEE_VOLVIO_LINEA:-}" ] && GSEMI_VOLVIO=$(log_de "$SAL/run-sano" | sed -n "${LEE_VOLVIO_LINEA}p" | sed 's/.*gsemi=\([0-9]*\)\/.*/\1/')
if [ -n "${LEE_VOLVIO_LINEA:-}" ] && [ -n "${LEE_ENTRA:-}" ] && [ "$LEE_VOLVIO_LINEA" -gt "$LEE_ENTRA" ]; then
    if [ -n "${GSEMI_VOLVIO:-}" ] && [ "$GSEMI_VOLVIO" -lt "$TECHO" ]; then
        verde "control 3: lee-volvio de LvlPrest.txt esta (linea $LEE_VOLVIO_LINEA, despues del lee-entra de la $LEE_ENTRA) con gsemi=$GSEMI_VOLVIO < $TECHO ($((TECHO - GSEMI_VOLVIO)) B debajo del techo)"
    else
        rojo "control 3: el lee-volvio de LvlPrest.txt esta pero con gsemi=${GSEMI_VOLVIO:-?} (queria < $TECHO; el techo lo fija godc_config.h)"
    fi
else
    rojo "control 3: NO hay lee-volvio de LvlPrest.txt en la corrida del arreglo (lee-entra=$LEE_ENTRA lee-volvio=${LEE_VOLVIO_LINEA:-ninguno})"
fi

paso "8. control 4 (sano): el motor SIGUE -- pide una tabla POSTERIOR a LvlPrest"
# d2resource.DataDictionaries(): LevelType, LevelPreset, LevelWarp, ObjectType, ObjectDetails, Weapons, ...
# OJO con los NOMBRES: el motor pide los archivos como los llama el juego (`LvlWarp.txt`, `objtype.txt`,
# `Objects.txt`, `Weapons.txt`), no con el nombre del TIPO de la tabla. La primera version de este control
# buscaba `LevelWarp`/`ObjectType`/`ObjectDetails` y se ponia ROJO con la tabla ya pedida en el log (medido:
# LvlWarp.txt esta en la marca siguiente y el control decia que no).
SIGUIENTES=("LvlWarp" "objtype" "Objects" "Weapons" "armor" "misc" "MonStats")
ENCONTRADA=""
for t in "${SIGUIENTES[@]}"; do
    if log_de "$SAL/run-sano" | grep -qa "ARQ-CARGA .*excel/$t.txt"; then ENCONTRADA="$t"; break; fi
done
if [ -n "$ENCONTRADA" ]; then
    verde "control 4: el motor pidio $ENCONTRADA.txt DESPUES de LvlPrest.txt (la lectura no es la pared)"
else
    rojo "control 4: despues de LvlPrest.txt no hay ninguna tabla posterior pedida"
fi

paso "9. criterio 3: la marca de la lectura en las dos corridas (y el margen del ReadAll)"
# DC-002.45 -- EL CRITERIO 3 DE LA TARJETA CAMBIO DE FORMA, Y EL QUE LO CAMBIA ES UN NUMERO.
# La tarjeta pedia "un control con el ReadAll puesto (mutante) tiene que volver a morir en la lectura". Medido
# hoy con el runtime al dia, el mutante NO muere: entra a la lectura de LvlPrest.txt con mas semiespacio libre
# del que pesa el archivo. Por eso este paso NO exige la muerte (seria un rojo que no habla del arreglo) y en
# su lugar hace lo que el criterio queria: mide el margen y lo imprime.
#   control 7 (sano)      la marca de la lectura NO crece el semiespacio en el tamano del archivo: la tabla se
#                         consume por trozos. Es la propiedad del arreglo, y se mide en la consola.
#   nota  (mutante)       free al entrar a la lectura contra los 134937 B del archivo: si sobra, el ReadAll ya
#                         entra y el mutante dejo de ser letal (que es lo que paso); si falta, muere.
LECTURA_BYTES=134937
marca_de() { # $1 = corrida, $2 = punto -> "gsemi free" de la primera marca de ese punto con ese asset
    log_de "$SAL/run-$1" | grep -a "ARQ-CARGA $2 .*$LECTURA" | head -1
}
S_ENTRA="$(marca_de sano lee-entra)"; S_VOLVIO="$(marca_de sano lee-volvio)"
M_ENTRA="$(marca_de mutante lee-entra)"; M_VOLVIO="$(marca_de mutante lee-volvio)"
S_GS_ENTRA="$(printf '%s' "$S_ENTRA" | sed 's/.*gsemi=\([0-9]*\).*/\1/')"
S_GS_VOLVIO="$(printf '%s' "$S_VOLVIO" | sed 's/.*gsemi=\([0-9]*\).*/\1/')"
M_FREE_ENTRA=""
if [ -n "$M_ENTRA" ]; then
    M_GS_ENTRA="$(printf '%s' "$M_ENTRA" | sed 's/.*gsemi=\([0-9]*\).*/\1/')"
    [ -n "$M_GS_ENTRA" ] && M_FREE_ENTRA=$((TECHO - M_GS_ENTRA))
fi
# DC-002.47: LA PROPIEDAD SE MIDE DONDE HAY CONTADOR, Y EL PROXY DE LA CONSOLA DEJO DE VALER.
# El control de DC-002.45 era un proxy: "la marca `gsemi` de la lectura no crece el semiespacio en el tamano
# del archivo". Ese proxy pasaba POR CASUALIDAD: en la ventana de UNA tabla caia una recoleccion entera (la
# marca de gc pasaba de 7 a 1122 con 512 KB) y el delta salia NEGATIVO. Con la particion de 1 MB las
# recolecciones de la ventana son 2-3 y el delta es lo que crecen las FILAS PARSEADAS -- medido: 219.944 B,
# mas que los 134.937 B del archivo, que no se materializan. Un control que depende de donde caiga una
# recoleccion no dice nada del arreglo, asi que el proxy baja a nota y la propiedad se mide con el CONTADOR
# del lector, que es lo unico que distingue "por trozos" de "de un tiron": la difftest del host lee la tabla
# REAL por trozos y dice cuantas veces le pidio bytes y cual fue el pedido mayor (el sector del MPQ, 4.096 B).
# Con el lector viejo la tabla no sale (0 filas): esa es la difftest de DC-002.47.
if [ -n "$S_GS_VOLVIO" ] && [ -n "$S_GS_ENTRA" ]; then
    nota "la marca de la lectura del arreglo crece $((S_GS_VOLVIO - S_GS_ENTRA)) B con un archivo de $LECTURA_BYTES B (lo que crecen las FILAS; con 1 MB por lado hay 2-3 recolecciones en la ventana y no 1.121)"
fi
PEDIDOS_LINE="$((cd "$SRC/console/dc/std" && go test ./difftest/ -run TablaRealLvlPrestPorTrozos -v) 2>&1 | grep -a 'pedidos=' | tail -1)"
PEDIDOS="$(printf '%s' "$PEDIDOS_LINE" | sed 's/.*pedidos=\([0-9]*\).*/\1/')"
MAYOR="$(printf '%s' "$PEDIDOS_LINE" | sed 's/.*pedido mayor=\([0-9]*\) B.*/\1/')"
if [ -n "${PEDIDOS_LINE:-}" ] && [ "${PEDIDOS:-0}" -ge 30 ] && [ "${MAYOR:-99999}" -le 4096 ]; then
    verde "control 7: la tabla se lleva POR TROZOS -- la difftest del host le pidio al lector $PEDIDOS veces y el pedido mayor fue de $MAYOR B ($LECTURA_BYTES B de archivo): no se la pide de un tiron"
else
    rojo "control 7: la difftest del host no dice que la lectura sea por trozos ('$PEDIDOS_LINE')"
fi
if [ -n "$M_FREE_ENTRA" ]; then
    MARGEN=$((M_FREE_ENTRA - LECTURA_BYTES))
    if [ "$MARGEN" -gt 0 ]; then
        nota "criterio 3 (mutante, informativo): el ReadAll YA NO MUERE -- entra con $M_FREE_ENTRA B libres para un archivo de $LECTURA_BYTES B (le sobran $MARGEN B). La pared de DC-002.43 no se reproduce con el runtime de hoy"
    else
        nota "criterio 3 (mutante, informativo): el ReadAll NO ENTRA -- $M_FREE_ENTRA B libres para $LECTURA_BYTES B (le faltan $(( -MARGEN )) B): la pared de DC-002.43 sigue viva"
    fi
    if printf '%s' "$M_VOLVIO" | grep -q .; then
        nota "                  el mutante TERMINO la lectura (no murio adentro): $(printf '%s' "$M_VOLVIO" | cut -c1-95)"
    fi
else
    nota "criterio 3 (mutante, informativo): no hay marca de la lectura del mutante en la captura"
fi

# ================================================================================================
# 10. LOS NUMEROS Y DONDE MURIO CADA CORRIDA (el reporte)
# ================================================================================================
paso "10. los numeros"
for e in sano mutante; do
    L="$SAL/run-$e/flycast.log"
    MAXS=$(log_de "$SAL/run-$e" | grep -ao 'gsemi=[0-9]*' | sed 's/gsemi=//' | sort -n | tail -1)
    MAXU=$(log_de "$SAL/run-$e" | grep -ao 'gc-por-umbral-bytes-usados v=[0-9]*' | sed 's/.*v=//' | sort -n | tail -1)
    MARKS=$(log_de "$SAL/run-$e" | grep -ac 'ARQ-CARGA')
    TROZOS=$(log_de "$SAL/run-$e" | grep -ac 'ARQ-CARGA lee-volvio')
    TABLAS=$(log_de "$SAL/run-$e" | grep -ac 'Loaded /cd/juego')
    MUERTE=$(log_de "$SAL/run-$e" | grep -aE '^fatal error|^panic:|^Memory: arena=' | head -2 | tr '\n' ' ')
    MAXV=$(log_de "$SAL/run-$e" | grep -ao 'gc-vivo v=[0-9]*' | sed 's/.*v=//' | sort -n | tail -1)
    echo "   -- $e: marcas=$MARKS lecturas-terminadas=$TROZOS assets-de-MPQ=$TABLAS"
    echo "      semiespacio: max en las marcas=${MAXS:-?} max del runtime=${MAXU:-?} de $TECHO"
    echo "      dato VIVO maximo (gc-vivo, lo que sobrevive a una copia): ${MAXV:-?} de $TECHO"
    echo "      ultima marca: $(log_de "$SAL/run-$e" | grep -a 'ARQ-CARGA' | tail -1 | cut -c1-100)"
    echo "      muerte: ${MUERTE:-MUDA}"
done

paso "11. el dato VIVO: cuanto queda del semiespacio despues de la recoleccion (DC-002.45)"
# El numero que decide la tarjeta SIGUIENTE: con la lectura por trozos puesta, lo que llena el semiespacio ya
# no es el archivo sino el dato VIVO del arranque (las tablas parseadas que el RecordManager se queda). Se mide
# con la marca `gc-vivo` de `instrumentar-vivo-245.py` (bytes copiados al final de la recoleccion).
for e in sano mutante; do
    V=$(log_de "$SAL/run-$e" | grep -ao 'gc-vivo v=[0-9]*' | sed 's/.*v=//' | sort -n | tail -1)
    if [ -n "${V:-}" ]; then
        P=$((V * 1000 / TECHO))
        nota "$e: dato vivo maximo $V B de $TECHO ($((P / 10)).$((P % 10)) %), quedan $((TECHO - V)) B"
    else
        nota "$e: sin marcas gc-vivo (el runtime no trae instrumentar-vivo-245.py; no es un fallo del arreglo)"
    fi
done

echo
echo "== RESULTADO: $OK controles OK, $FALLOS con fallo =="
[ "$FALLOS" -eq 0 ]
