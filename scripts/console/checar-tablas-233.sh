#!/bin/bash
# CORPUS DE DC-002.33: LAS TABLAS DEL MPQ COMPACTAS (los bytes, no un objeto de Go por entrada).
#
# En que consiste la tarjeta: las dos tablas de cada MPQ (la de hash y la de bloque) se guardaban como UN
# OBJETO DE GO POR ENTRADA (`map[uint64]*Hash` + `[]*Block`). Con d2data.mpq (65.536 ranuras de hash,
# 11.083 de bloque) ese conjunto VIVO no entra en el semiespacio del GC de la consola (512 KiB) y el arranque
# del motor muere ahi. El arreglo las guarda como BYTES contiguos (16 por entrada) y el que consulta las lee
# POR INDICE, con el sondeo del formato MPQ. Cada tabla pasa a ser UNA reserva de mas de 64 KiB, o sea del
# monton de C (no del semiespacio).
#
# QUE MIDE, en los tres niveles (host, consola y el motor):
#   HOST  (1) `go test ./d2common/d2fileformats/d2mpq/` verde, incluido el caso que compara la consulta NUEVA
#             (el sondeo sobre los bytes) contra el mapa de objetos VIEJO, nombre por nombre del `(listfile)`
#             de los MPQ del juego (`TestBuscarHashIgualQueElMapaViejo`, con D2_GAME_DIR).
#         (2) `console/dc/sonda-tablas-mpq`: entradas y CHECKSUM de los bytes de cada tabla de cada MPQ, con
#             el mismo codigo del motor.
#   CONSOLA (3) POC `tablas-mpq`: los SEIS mismos numeros por archivo en la consola (criterio 2: el hash de
#             la tabla entera igual en los dos lados). Con las tablas como bytes contiguos la tabla ENTERA se
#             compara con DOS numeros, que es lo que la tarjeta pedia.
#         (4) POC `motor-reservas` (DC-002.28): el cargador del motor abriendo los TRES MPQ del disco. Antes
#             de esta tarjeta este paso MORIA en d2data.mpq con el semiespacio lleno.
#   MOTOR (5) POC `motor-disco`: el arranque de verdad (`d2app.Create().Run()`) con los MPQ del disco. El
#             numero de la tarjeta es el SEMIESPACIO: la marca `gc-por-umbral-bytes-usados` da los bytes
#             usados de los 512 KiB en cada recoleccion, o sea el libre = 524288 - usado.
#   MUTANTE (6) `fix-mpq-tablas-233.py --pristino` deja el arbol como estaba (un objeto de Go por entrada),
#             recompila d2mpq EN EL ARBOL MUTADO y vuelve a medir: el semiespacio tiene que volver a llenarse
#             (criterio 3). Es tambien el "antes" del criterio 1, asi que se corre por defecto; la corrida
#             corta se pide con MUTANTE_CONSOLE=0 (y entonces el corpus dice que la mitad "antes" no se
#             midio).
#
# Uso: bash scripts/console/checar-tablas-233.sh [dir_gox] [dir_mpq] [segundos_reservas] [segundos_motor]
#   GEN (o 1er argumento): la capa de .gox COHERENTE con el arbol (por defecto /tmp/gox-233)
#   MPQ (o 2do argumento): la instalacion de Diablo II (los .mpq)
#   SAL: ~/.hermes/cache/scratch/corpus-233
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

GEN="${1:-${GODC_GOX:-/tmp/gox-233}}"
MPQ="${2:-$HOME/.wine-d2/drive_c/Games/Diablo II}"
SEGS_RES="${3:-75}"
SEGS_MOTOR="${4:-60}"
REPO="${REPO:-$(cd "$HERE/../.." && pwd)}"
SAL="${SAL:-/home/dgalli/.hermes/cache/scratch/corpus-233}"
FIX="$HERE/fix-mpq-tablas-233.py"
LISTA="patch_d2.mpq,d2data.mpq,d2char.mpq"
ARCHIVOS_MPQ="patch_d2.mpq d2data.mpq d2char.mpq"

# la pared medida en DC-002.29/30 con las tablas como OBJETOS: el motor muere con 480.736 bytes usados de los
# 524.288 del semiespacio. Es el numero contra el que se compara el "despues".
PARED_VIEJA=480736

export GODC_GOX="$GEN"

OK=0
FALLOS=0
nota() { echo "   $*"; }
verde() { OK=$((OK + 1)); echo "   OK: $*"; }
rojo() { FALLOS=$((FALLOS + 1)); echo "   FALLO: $*"; }

# un campo de la linea de un MPQ: `campo <log> <"<mpq> "> <nombre del campo>`. Sirve para el host (que
# imprime los seis numeros en UNA linea) y para la consola (que imprime los cuatro de las TABLAS en una y los
# dos de la CONSULTA en otra).
campo() { # $1 = log, $2 = patron del archivo (fijo), $3 = nombre del campo
    tr -d '\r' < "$1" | grep -aF "$2" | grep -aF "$3=" | head -1 | sed "s/.*$3 *= *\([0-9]*\).*/\1/"
}

ultimo_valor() { # $1 = etiqueta de una marca ARQ-MARCA, $2 = log
    tr -d '\r' < "$2" | grep -aF "ARQ-MARCA $1 " | tail -1 | sed 's/.*v=//'
}

libre_del_semiespacio() { # $1 = usado; imprime el libre contra los 512 KiB del GC
    echo $((524288 - ${1:-0}))
}

# las lineas que importan de una corrida, con el prefijo de la seccion
bloque() { # $1 = log, $2 = cuantas lineas del final
    tr -d '\r' < "$1" | grep -a 'ARQ-MARCA\|fatal error\|out of memory\|Memory: arena\|EXCEPTION\|EBITEN-LOOP\|EBITEN-PERFIL\|MOTOR-DISCO\|SELECCION\|DISCO-CONSOLA' |
        tail -"${2:-12}" | sed 's/^ *//;s/^/     /'
}

# aparta una corrida vieja en vez de borrarla (DC-002.30: `mide` podia leer un flycast.log de otro binario;
# DC-002.29: una corrida vieja deja numeros que no son de este build)
apartar() { [ -e "$1" ] && mv "$1" "$1.viejo-$(date +%s)"; return 0; }

corrida_motor() { # $1 = dir del POC ya armado, $2 = dir de salida, $3 = segundos
    apartar "$2"
    CAPTURE=none bash "$HERE/run-flycast.sh" "$1/disc.gdi" "$3" "$2" >/dev/null 2>&1
}

armar() { # $1 = POC, $2 = dir de salida, $3 = lista de MPQ (por defecto la de la tarjeta)
    MKISO_JOLIET=0 DC_RUTA_DATOS=/cd/juego DC_MPQ_JUEGO="${3:-$LISTA}" DC_SIN_SELLOS=1 DC_DISCO_JUEGO="$SAL/juego" \
        POC="$1" bash "$HERE/build-motor-arranque.sh" "$2" "$GEN" > "$2.build.log" 2>&1
}

# ================================================================================================
# 0. EL ESTADO DEL ARBOL Y EL DE LOS PARCHES DE LOS QUE DEPENDE LA MEDICION
# ================================================================================================
echo "== 0. estado del arbol, la carpeta del juego y el parche de la sesion de KOS"
mkdir -p "$SAL"

ESTADO="$("$DC_PYTHON" "$FIX" --estado 2>&1 | tail -1)"
if echo "$ESTADO" | grep -qa '7/7 como el arreglo'; then
    verde "control 0: el arbol esta como el arreglo ($ESTADO)"
else
    rojo "control 0: el arbol NO esta como el arreglo ($ESTADO): correr $FIX --sano antes de medir"
fi

SEMI_KB="$(grep -a '^#define GC_SEMISPACE_SIZE_KB' "$LIBGODC/runtime/godc_config.h" 2>/dev/null | head -1 | grep -oE '[0-9]+$' | head -1)"
if [ "${SEMI_KB:-0}" = "512" ]; then
    verde "control 0c: el semiespacio del GC es de 512 KiB ($LIBGODC/runtime/godc_config.h), el valor de DC-002.25"
else
    rojo "control 0c: el semiespacio esta en ${SEMI_KB:-?} KB y no en 512: los numeros de abajo NO son comparables"
fi

mkdir -p "$SAL/juego"

# el orden es el del motor (`d2setup.CoreMPQs` filtrado): patch_d2 primero. Los nombres van en minusculas,
# que es como los busca el motor (el ISO9660 de KOS compara con tolower; en el host eso se hace a mano).
for n in $ARCHIVOS_MPQ; do
    real="$(ls -1 "$MPQ" | grep -ix "$n" | head -1)"
    [ -n "$real" ] || { echo "   no encuentro $n en $MPQ"; exit 1; }

    ln -f "$MPQ/$real" "$SAL/juego/$n" 2>/dev/null || cp "$MPQ/$real" "$SAL/juego/$n"
done

nota "MPQ del disco: $(ls "$SAL/juego" | tr '\n' ' ') ($(du -sb "$SAL/juego" | cut -f1) bytes)"

if "$DC_PYTHON" "$HERE/fix-kos-iso9660-sesion.py" --ver 2>/dev/null | grep -qa PARCHEADO; then
    verde "control 0b: el driver iso9660 de KOS tiene el parche de la sesion de alta densidad"
else
    rojo "control 0b: falta el parche de KOS (scripts/console/fix-kos-iso9660-sesion.py + make -C \$KOS_BASE)"
fi

# ================================================================================================
# 1. EL HOST: la tabla entera y la consulta nueva contra la vieja
# ================================================================================================
echo "== 1. el host: la tabla como bytes y la consulta (el sondeo) contra el mapa de objetos viejo"
if (cd "$REPO" && go test -count=1 ./d2common/d2fileformats/d2mpq/) > "$SAL/host-test.log" 2>&1; then
    verde "control 1: go test de d2mpq verde ($(grep -c '^ok' "$SAL/host-test.log") paquete(s))"
else
    rojo "control 1: go test de d2mpq en ROJO (ver $SAL/host-test.log)"
fi

# el caso que compara las DOS consultas, nombre por nombre del (listfile), con los MPQ del juego
if (cd "$REPO" && D2_GAME_DIR="$SAL/juego" go test -count=1 -v -run TestBuscarHashIgualQueElMapaViejo \
    ./d2common/d2fileformats/d2mpq/) > "$SAL/host-consulta.log" 2>&1; then
    IGUALES="$(grep -a 'iguales=' "$SAL/host-consulta.log" | sed 's/.*iguales=\([0-9]*\).*/\1/' | awk '{s+=$1} END {print s+0}')"
    DISTINTOS="$(grep -a 'distintos=' "$SAL/host-consulta.log" | sed 's/.*distintos=\([0-9]*\) .*/\1/' | awk '{s+=$1} END {print s+0}')"
    nota "el sondeo contra el mapa viejo, por archivo:"
    grep -a 'entradas_hash=' "$SAL/host-consulta.log" | tr -d '\r' | sed 's/^ *//;s/^/     /'

    if [ "$IGUALES" -gt 20000 ] && [ "$DISTINTOS" = "0" ]; then
        verde "control 1b: la consulta nueva da lo mismo que el mapa viejo ($IGUALES nombres iguales, $DISTINTOS distintos)"
    else
        rojo "control 1b: la consulta nueva NO da lo mismo que el mapa viejo ($IGUALES iguales, $DISTINTOS distintos)"
    fi
else
    rojo "control 1b: el caso que compara las dos consultas esta en ROJO (ver $SAL/host-consulta.log)"
fi

# los numeros de referencia del host
(cd "$REPO" && go run ./console/dc/sonda-tablas-mpq -mpqdir "$SAL/juego" -mpq "$LISTA") > "$SAL/tablas-host.txt" 2>&1
nota "las tablas, medidas en el host (con el mismo codigo del motor):"
grep -a 'TABLAS-HOST' "$SAL/tablas-host.txt" | tr -d '\r' | sed 's/^/     /'

# LOS NOMBRES AL LADO DE LOS MPQ: de aca los toma la CONSOLA para su consulta. No los puede sacar del
# `(listfile)` del MPQ porque en la consola ese `ReadFile` FALLA (medido: `zlib: invalid header`, defecto
# aparte de esta tarjeta). Los escribe el MISMO codigo del host, y el disco los lleva al /JUEGO de la consola
# (scripts/console/mkdisco.sh hornea los .mpq y los .txt).
(cd "$REPO" && go run ./console/dc/sonda-tablas-mpq -mpqdir "$SAL/juego" -mpq "$LISTA" -nombres-dir "$SAL/juego") \
    > "$SAL/nombres-host.txt" 2>&1
nota "nombres para la consola: $(ls "$SAL/juego"/nombres_*.txt 2>/dev/null | wc -l) archivo(s), $(cat "$SAL/juego"/nombres_*.txt | wc -l) lineas"

# ================================================================================================
# 2. LA CONSOLA: el POC tablas-mpq (los mismos seis numeros) y el cargador del motor
# ================================================================================================
TABLAS_OK=""
CONSULTA_OK=""
CONSULTA_TOTAL=0
LARGAS=""

medir_tablas() { # $1 = log, $2 = archivo: compara las DOS tablas (4 numeros) contra el host
    local c vc vh linea_c="" linea_h="" igual=1

    for c in entradasHash entradasBloque checksumHash checksumBloque; do
        vc="$(campo "$1" "$2 " "$c")"
        vh="$(campo "$SAL/tablas-host.txt" "$2 " "$c")"
        linea_c="$linea_c $c=$vc"
        linea_h="$linea_h $c=$vh"

        if [ -z "$vc" ] || [ "$vc" != "$vh" ]; then
            igual=0
        fi
    done

    nota "$2  las DOS tablas -> consola:$linea_c"
    nota "$2  las DOS tablas -> host:   $linea_h"

    if [ "$igual" = "1" ]; then
        TABLAS_OK="$TABLAS_OK $2"
    fi
}

medir_consulta() { # $1 = log, $2 = archivo: la consulta de la consola contra la del host
    local nc ec lc nh eh

    nc="$(campo "$1" "$2 " nombres)"
    ec="$(campo "$1" "$2 " contenidos)"
    lc="$(campo "$1" "$2 " largas)"
    nh="$(campo "$SAL/tablas-host.txt" "$2 " nombres)"
    eh="$(campo "$SAL/tablas-host.txt" "$2 " contenidos)"
    LARGAS="$LARGAS $2:${lc:-?}"

    if [ -n "$nc" ] && [ "$nc" = "$nh" ] && [ "$ec" = "$eh" ] && [ "$lc" = "0" ]; then
        CONSULTA_OK="$CONSULTA_OK $2"
        CONSULTA_TOTAL=$((CONSULTA_TOTAL + ec))
        nota "$2  la consulta: la consola encontro $ec de $nc nombres (el host $eh de $nh), largas=0"
    else
        nota "$2  la consulta: consola nombres=$nc contenidos=$ec largas=$lc contra host nombres=$nh contenidos=$eh"
    fi
}

echo "== 2. la consola: el POC tablas-mpq (las tablas y la consulta) sobre el disco con los 3 MPQ"
T="$SAL/tablas"
if armar tablas-mpq "$T"; then
    corrida_motor "$T" "$SAL/run-tablas" "$SEGS_RES"
    LT="$SAL/run-tablas/flycast.log"

    nota "la consola (POC tablas-mpq), en el orden del motor:"
    grep -a 'TABLAS-MPQ' "$LT" | tr -d '\r' | sed 's/^/     /'

    for f in $ARCHIVOS_MPQ; do
        medir_tablas "$LT" "$f"
        medir_consulta "$LT" "$f"
    done

    nota "las recolecciones del GC en esa corrida: $(grep -ac 'ARQ-MARCA gc_collect' "$LT"); semiespacio usado en la ultima: $(ultimo_valor gc-por-umbral-bytes-usados "$LT")"
else
    rojo "el POC tablas-mpq NO COMPILA/ARMA (ver $T.build.log)"
    tail -5 "$T.build.log" | sed 's/^/     /'
fi

# EL TERCERO, EN SU PROPIA CORRIDA. No es un capricho del corpus: en una misma corrida el nombre del tercero
# se corrompe (ver la nota del defecto en console/dc/poc/tablas-mpq/main.go y el `largoNombre=` del log), asi
# que la unica forma de medir d2char.mpq en la consola es darle una corrida con una sola vuelta.
echo "== 2b. la consola: el TERCERO (d2char.mpq) en su propia corrida, una sola vuelta"
TD="$SAL/tablas-d2char"
if armar tablas-mpq "$TD" d2char.mpq; then
    corrida_motor "$TD" "$SAL/run-tablas-d2char" "$SEGS_RES"
    LTD="$SAL/run-tablas-d2char/flycast.log"

    nota "la consola (POC tablas-mpq), d2char.mpq solo:"
    grep -a 'TABLAS-MPQ' "$LTD" | tr -d '\r' | sed 's/^/     /'

    medir_tablas "$LTD" d2char.mpq
    medir_consulta "$LTD" d2char.mpq
else
    rojo "el POC tablas-mpq (d2char solo) NO COMPILA/ARMA (ver $TD.build.log)"
fi

CUANTAS_TABLAS="$(echo $TABLAS_OK | tr ' ' '\n' | grep -c .)"
if [ "$CUANTAS_TABLAS" = "3" ]; then
    verde "control 2: las DOS tablas (entradas + checksum de sus bytes) de los 3 MPQ son iguales en el host y en la consola"
else
    rojo "control 2: solo $CUANTAS_TABLAS de 3 MPQ dan las mismas tablas (con las tablas iguales:$TABLAS_OK)"
fi

CUANTAS_CONSULTA="$(echo $CONSULTA_OK | tr ' ' '\n' | grep -c .)"
if [ "$CUANTAS_CONSULTA" = "3" ] && [ "$CONSULTA_TOTAL" = "21253" ]; then
    verde "control 2b: la CONSULTA de la consola encuentra los 21.253 nombres (162 + 11.082 + 10.009), los mismos del host"
else
    rojo "control 2b: la consulta de la consola no cierra ($CUANTAS_CONSULTA de 3 archivos, total $CONSULTA_TOTAL de 21253, largas:$LARGAS)"
fi

if [ "$(cat "$SAL/run-tablas/flycast.log" "$LTD" 2>/dev/null | grep -ac 'fatal error')" = "0" ] &&
   [ "$(cat "$SAL/run-tablas/flycast.log" "$LTD" 2>/dev/null | grep -ac 'out of memory')" = "0" ] &&
   [ "$(grep -ac 'TABLAS-MPQ-TOTAL' "$LTD" 2>/dev/null)" -ge 1 ]; then
    verde "control 2c: las dos corridas del POC sin out of memory ni fatal error (la del tercero cierra con TOTAL)"
else
    rojo "control 2c: alguna corrida del POC tiene out of memory/fatal error, o la del tercero no cierra (ver $LT y $LTD)"
fi

# DECLARADO: la corrida de los 3 MPQ puede perder su linea TOTAL (o la impresion entera) por el defecto del
# nombre que se corrompe (DC-002.39). No es un fallo de esta tarjeta, pero se dice con su numero.
if [ "$(grep -ac 'TABLAS-MPQ-TOTAL' "$SAL/run-tablas/flycast.log" 2>/dev/null)" = "0" ]; then
    nota "declarado (DC-002.39): la corrida de los 3 MPQ NO imprime TOTAL -- el nombre del tercero se corrompe y la impresion se lleva la salida puesta; las dos primeras se miden igual"
else
    nota "la corrida de los 3 MPQ cierra con TOTAL: $(grep -a 'TABLAS-MPQ-TOTAL' "$SAL/run-tablas/flycast.log" | tr -d '\r' | tail -1 | sed 's/^ *//')"
fi

echo "== 2c. la consola: el POC motor-reservas (DC-002.28) sobre el mismo disco"
R="$SAL/reservas"
if armar motor-reservas "$R"; then
    corrida_motor "$R" "$SAL/run-reservas" "$SEGS_RES"
else
    rojo "el POC motor-reservas NO COMPILA/ARMA (ver $R.build.log)"
fi

LR="$SAL/run-reservas/flycast.log"
PASO2="$(grep -ac 'CARGADOR-CONSOLA 2 / 3' "$LR")"
ABRIO2="$(tr -d '\r' < "$LR" | grep -ac 'CARGADOR-CONSOLA  mpq.NewSource OK (las dos tablas)')"
if [ "$PASO2" -ge 1 ] && [ "$ABRIO2" -ge 2 ] &&
   [ "$(grep -ac 'fatal error' "$LR")" = "0" ] && [ "$(grep -ac 'out of memory' "$LR")" = "0" ]; then
    verde "control 3: el cargador del motor EMPIEZA d2data.mpq (el archivo que moria) y lo abre con SUS DOS TABLAS ($ABRIO2 aperturas) sin morir en el semiespacio"
else
    rojo "control 3: el cargador del motor no llega a d2data.mpq (paso 2 = $PASO2) o MUERE ($ABRIO2 aperturas) (ver $LR)"
    grep -a 'fatal error\|Memory: arena\|CARGADOR-CONSOLA' "$LR" | tr -d '\r' | tail -6 | sed 's/^/     /'
fi

# DECLARADO: el TERCERO de este POC sale con el NOMBRE VACIO (la ruta queda en `/cd/JUEGO/`), que es el defecto
# del nombre que se corrompe (DC-002.39), no una pared de memoria de esta tarjeta.
ULT3="$(tr -d '\r' < "$LR" | grep -a 'CARGADOR-CONSOLA 3 / 3' | head -1 | sed 's/^ *//')"
if echo "$ULT3" | grep -qa '/cd/JUEGO/$'; then
    nota "declarado (DC-002.39): el TERCERO del cargador sale con el nombre VACIO -> $ULT3 (y el `d2mpq.New FALLO` que le sigue)"
else
    nota "el cargador del motor, en el tercero: ${ULT3:-no llego}"
fi

nota "el monton de C del POC motor-reservas (el arranque y tras las tablas):"
grep -a 'cargador del motor, en el orden\|CARGADOR-CONSOLA  monton tras las tablas' "$LR" | tr -d '\r' | sed 's/^/     /' | tail -4
nota "las recolecciones del GC en ese POC: $(grep -ac 'ARQ-MARCA gc_collect' "$LR"); semiespacio usado en la ultima: $(ultimo_valor gc-por-umbral-bytes-usados "$LR")"

# ================================================================================================
# 3. EL MOTOR: el arranque de verdad, con el instrumento de DC-002.27/28
# ================================================================================================
echo "== 3. el motor entero sobre el mismo disco (donde muere y con cuanto semiespacio)"
M="$SAL/motor"
if armar motor-disco "$M"; then
    corrida_motor "$M" "$SAL/run-motor" "$SEGS_MOTOR"
else
    rojo "el motor no se pudo armar (ver $M.build.log)"
fi

LM="$SAL/run-motor/flycast.log"
USADO=""
if [ -f "$LM" ]; then
    MARCAS="$(grep -ac 'ARQ-MARCA' "$LM")"
    MAIN="$(grep -ac 'main.main-entra' "$LM")"
    CUADROS="$(grep -ac 'EBITEN-LOOP' "$LM")"
    USADO="$(ultimo_valor gc-por-umbral-bytes-usados "$LM")"
    OOM="$(grep -ac 'out of memory\|fatal error' "$LM")"

    nota "marcas=$MARCAS main.main=$MAIN cuadros=$CUADROS gc_collect=$(grep -ac 'ARQ-MARCA gc_collect' "$LM")"
    nota "semiespacio: usado en la ultima recoleccion = ${USADO:-?} de 524288 -> libre $(libre_del_semiespacio "$USADO")"
    nota "donde muere el arranque (despues): $(tr -d '\r' < "$LM" | grep -a 'fatal error\|Memory: arena\|EXCEPTION' | tail -1 | sed 's/^ *//')"
    nota "las ultimas lineas del motor:"
    bloque "$LM" 10

    # LA PARED DE ESTA TARJETA es la del SEMIESPACIO (criterio 1: que el arranque ya NO muera ahi). Si muere
    # despues, en el MONTON DE C, eso es OTRA pared (la arena, declarada desde DC-002.28) y no la de aca: se
    # declara con su numero en vez de esconderse detras del control.
    if [ "$MAIN" -ge 1 ] && [ "${USADO:-999999999}" -lt "$PARED_VIEJA" ]; then
        verde "control 4: el arranque pasa la pared del SEMIESPACIO ($USADO < $PARED_VIEJA usados; libre $(libre_del_semiespacio "$USADO")) y llega a main"
    else
        rojo "control 4: el arranque sigue en la pared del semiespacio (usado=${USADO:-?}, main.main=$MAIN)"
    fi

    MUERTE4="$(tr -d '\r' < "$LM" | grep -a 'fatal error\|Memory: arena' | tr '\n' ' ' | sed 's/  */ /g;s/^ *//')"
    if echo "$MUERTE4" | grep -qa 'gc_external_alloc'; then
        nota "declarado (OTRA pared, la de DC-002.28/34): el arranque muere despues en el MONTON DE C -> $MUERTE4"
    elif [ -n "$MUERTE4" ]; then
        nota "donde muere el arranque: $MUERTE4"
    else
        nota "el arranque NO imprime fatal error: se cuelga mudo (familia de DC-002.29) -- main.main=$MAIN cuadros=$CUADROS"
    fi

    if [ "$CUADROS" -ge 1 ]; then
        verde "control 4b: el motor LLEGA A LOS CUADROS con los MPQ del disco (cuadros=$CUADROS)"
    else
        nota "declarado: el motor NO llega a los cuadros (cuadros=0): muere antes, donde dice la linea de arriba"
    fi
else
    rojo "el motor no corrio (no hay $LM)"
fi

# ================================================================================================
# 4. EL MUTANTE: un objeto de Go por entrada (el "antes" de la tarjeta)
# ================================================================================================
USADOM=""
if [ "${MUTANTE_CONSOLE:-1}" = "1" ]; then
    echo "== 4. mutante: el arbol PRISTINO (un objeto de Go por entrada) y el mismo arranque"
    SHA_ANTES="$(sha256sum "$REPO"/d2common/d2fileformats/d2mpq/*.go | sha256sum | awk '{print $1}')"

    "$DC_PYTHON" "$FIX" --pristino > "$SAL/mutante-estado.log" 2>&1
    nota "$(tail -1 "$SAL/mutante-estado.log")"

    # el .gox del paquete mutado se recompila EN EL ARBOL MUTADO (la trampa de la camada de DC-002.29: sin
    # esto la consola sigue midiendo el codigo viejo y el mutante queda verde).
    bash "$HERE/recompilar-paquete-motor.sh" d2common/d2fileformats/d2mpq "$GEN" > "$SAL/mutante-gox.log" 2>&1
    bash "$HERE/recompilar-paquete-motor.sh" d2common/d2loader/mpq "$GEN" >> "$SAL/mutante-gox.log" 2>&1
    grep -a RE-MOTOR "$SAL/mutante-gox.log" | sed 's/^/     /'

    # el instrumento NUEVO no puede existir en el codigo viejo: dice que el mutante es de verdad el estado
    # anterior y no una copia a medias.
    if armar tablas-mpq "$SAL/tablas-mutante"; then
        rojo "control 5a: el POC tablas-mpq COMPILA con el codigo viejo: el mutante no llego a las tablas por bytes"
    else
        verde "control 5a: con el mutante el POC tablas-mpq NO compila ($(grep -a 'undefined' "$SAL/tablas-mutante.build.log" | head -1 | cut -c1-80))"
    fi

    # los dos POC que existen en los DOS estados
    for POC in motor-reservas motor-disco; do
        D="$SAL/mutante-$POC"
        if armar "$POC" "$D"; then
            corrida_motor "$D" "$SAL/run-mutante-$POC" "$SEGS_MOTOR"
        else
            rojo "el POC $POC no se pudo armar con el mutante (ver $D.build.log)"
        fi
    done

    LB="$SAL/run-mutante-motor-reservas/flycast.log"
    LMUT="$SAL/run-mutante-motor-disco/flycast.log"

    # LA MITAD "ANTES", medida con el MISMO instrumento: con un objeto de Go por entrada el cargador NO llega a
    # abrir las tablas de d2data.mpq (el pedido de su tabla de hash como objetos no entra en el semiespacio) y
    # se queda ahi; con las tablas como bytes si las abre. Se mide por CUANTAS APERTURAS hay y no por el texto
    # del fatal error, porque en esa corrida el motor muere MUDO (sin el mensaje en la serie): es el defecto de
    # DC-002.28/29, y por eso el marcador `OOM-throw` (que si queda) se imprime aparte.
    ABRIOM="$(tr -d '\r' < "$LB" | grep -ac 'CARGADOR-CONSOLA  mpq.NewSource OK (las dos tablas)')"
    ABRIOS="$(tr -d '\r' < "$LR" | grep -ac 'CARGADOR-CONSOLA  mpq.NewSource OK (las dos tablas)')"
    ULTIMO_ANTES="$(tr -d '\r' < "$LB" | grep -a 'CARGADOR-CONSOLA [0-9] / 3' | tail -1 | sed 's/^ *//')"

    if [ "$ABRIOM" -lt 2 ] && [ "$ABRIOS" -ge 2 ]; then
        verde "control 5b: con el mutante el cargador NO PASA de las tablas de d2data.mpq ($ABRIOM aperturas; ultimo paso: $ULTIMO_ANTES) y con las tablas como bytes SI ($ABRIOS aperturas)"
    else
        rojo "control 5b: el mutante abre $ABRIOM (>=2 = no muerde) o el sano abre $ABRIOS (<2 = no pasa): el mutante no muerde donde debe"
    fi

    nota "el mutante en el cargador: marcadores OOM-throw=$(grep -ac 'OOM-throw' "$LB"), fatal error=$(grep -ac 'fatal error' "$LB") (si es 0, murio mudo: familia de DC-002.29; el semiespacio usado queda en los marcadores: $(ultimo_valor gc-por-umbral-bytes-usados "$LB"))"

    nota "el mutante, donde muere (motor-reservas):"
    grep -a 'RESERVA-CONSOLA\|CARGADOR-CONSOLA\|fatal error\|Memory: arena' "$LB" | tr -d '\r' | tail -8 | sed 's/^/     /'

    if [ -f "$LMUT" ]; then
        MAINM="$(grep -ac 'main.main-entra' "$LMUT")"
        USADOM="$(ultimo_valor gc-por-umbral-bytes-usados "$LMUT")"
        MUERTEM="$(tr -d '\r' < "$LMUT" | grep -a 'fatal error\|Memory: arena\|EXCEPTION' | tail -1 | sed 's/^ *//')"

        nota "el mutante, en el MOTOR: marcas=$(grep -ac 'ARQ-MARCA' "$LMUT") main.main=$MAINM cuadros=$(grep -ac 'EBITEN-LOOP' "$LMUT") semiespacio usado=${USADOM:-?} -> libre $(libre_del_semiespacio "$USADOM")"
        nota "el mutante, donde muere (motor): $MUERTEM"

        # el mutante muerde si el motor NO llega a los cuadros con las tablas como objetos (es la mitad
        # "antes" del criterio 1, medida con el MISMO instrumento)
        # LAS DOS configuraciones mueren antes de los cuadros (eso es DC-002.34): lo que se exige aca es que el
        # SITIO de la muerte NO sea el mismo -- el mutante por el semiespacio (mudo), el de las tablas como
        # bytes por el monton de C (con su `gc_external_alloc: OOM`).
        MUDOM="$(grep -ac 'gc_external_alloc' "$LMUT")"
        if [ "$(grep -ac 'EBITEN-LOOP' "$LMUT")" = "0" ] && [ "$MUDOM" = "0" ]; then
            verde "control 5c: con el mutante el motor muere ANTES de los cuadros por el SEMIESPACIO (mudo, sin gc_external_alloc), en ${USADOM:-?} usados; con las tablas como bytes muere despues, en el MONTON DE C"
        else
            rojo "control 5c: el mutante llega a los cuadros ($(grep -ac 'EBITEN-LOOP' "$LMUT")) o muere en el monton de C ($MUDOM): no se distingue"
        fi
    else
        rojo "el motor con el mutante no corrio"
    fi

    # el arbol vuelve como estaba, con la prueba de sha256 (DC-002.29: el mutante pegado ya se comio una
    # corrida de esta familia)
    "$DC_PYTHON" "$FIX" --sano > "$SAL/sano-estado.log" 2>&1
    bash "$HERE/recompilar-paquete-motor.sh" d2common/d2fileformats/d2mpq "$GEN" > "$SAL/sano-gox.log" 2>&1
    bash "$HERE/recompilar-paquete-motor.sh" d2common/d2loader/mpq "$GEN" >> "$SAL/sano-gox.log" 2>&1

    SHA_DESPUES="$(sha256sum "$REPO"/d2common/d2fileformats/d2mpq/*.go | sha256sum | awk '{print $1}')"
    if [ "$SHA_ANTES" = "$SHA_DESPUES" ]; then
        verde "control 6: el arbol quedo con el mismo sha256 del d2mpq ($SHA_DESPUES, sin mutante pegado)"
    else
        rojo "control 6: el arbol NO quedo como estaba ($SHA_ANTES contra $SHA_DESPUES)"
    fi
else
    nota "el mutante no se corrio (MUTANTE_CONSOLE=0): la mitad 'antes' del criterio 1 queda SIN MEDIR"
fi

echo
echo "== RESUMEN: EL SEMIESPACIO (512 KiB = 524288 bytes), usado en la ultima recoleccion del motor"
echo "     despues (las tablas como BYTES):  ${USADO:-no medido} usados -> libre $(libre_del_semiespacio "$USADO")"
echo "     antes   (un objeto por entrada):  ${USADOM:-no medido} usados -> libre $(libre_del_semiespacio "$USADOM")"
echo "     la pared vieja medida en DC-002.29/30: $PARED_VIEJA usados"
echo "     donde muere despues (las tablas como BYTES): $(tr -d '\r' < "$LM" 2>/dev/null | grep -a 'fatal error\|Memory: arena' | tail -2 | tr '\n' ' ' | sed 's/  */ /g')"
echo "     donde muere antes   (un objeto por entrada): $(tr -d '\r' < "$SAL/run-mutante-motor-disco/flycast.log" 2>/dev/null | grep -a 'fatal error\|Memory: arena' | tail -2 | tr '\n' ' ' | sed 's/  */ /g;s/^ *//')"
echo "     (vacio = murio MUDO, sin el mensaje en la serie: familia de DC-002.28/29)"
echo "   la sonda del host:            $SAL/tablas-host.txt"
echo "   la consola (POC tablas-mpq):  $SAL/run-tablas/flycast.log"
echo "   el motor:                     $LM"
echo "   el motor con el mutante:      $SAL/run-mutante-motor-disco/flycast.log"
echo
echo "== RESULTADO: $OK controles OK, $FALLOS con fallo"
[ "$FALLOS" -eq 0 ]
