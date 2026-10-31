#!/bin/bash
# DC-002.90 -- CONTROL: LAS PALANCAS DEL BUILD TIENEN UNA SOLA FUENTE DE VERDAD, Y EL BINARIO DE LA CORRIDA
# DE REFERENCIA SALE SIN PASAR UNA SOLA VARIABLE.
#
# POR QUE EXISTE. En DC-002.89 se pago una corrida entera por tener dos fuentes de verdad para el mismo
# numero: `build-tablas-282.sh` escribia las palancas con sus propios valores por defecto (TABLAS=1 LIENZO=1
# ARENA=1) mientras las corridas de las tarjetas usaban otras (TABLAS=3 LIENZO=0 ARENA=0). El binario del
# default medía 6.863.464 B y NO ENTRA en la consola (`fatal error: out of memory` en la cuarta tabla, arena
# 5.034.368 B con 576 B libres); el de la referencia mide 6.859.016 B y compone el pueblo en 3 fases con
# crc -1596781041 / -205471136 y pico 9.525.408 B. Este script es la prueba de que eso no vuelve:
#
#   1. `knobs.go` del POC dice LAS PALANCAS DE LA CORRIDA DE REFERENCIA (3/true/false/false/true/true/false/true)
#   2. `build-tablas-282.sh` SIN variables resuelve esas mismas palancas y NO avisa de ninguna pisada
#   3. si el entorno pisa una palanca, el build lo AVISA EN GRANDE y la pisada llega al knobs.go generado
#   4. el build completo sin variables (esta es la cara: compila y enlaza todo) deja un 1ST_READ.BIN de
#      6.859.016 B y un `knobs.go` de build identico en VALORES al del POC
#   5. (solo con CORRER=1) ese binario, corrido 240 s en Flycast, NO imprime 'fatal error'/'out of memory'
#      y saca las 3 fases del pueblo con los crc de la referencia (-1596781041 / -205471136) y pico
#      9.525.408 B: es la prueba de que "no OOMea", no solo de que mide lo mismo
#
# Uso:  scripts/console/checar-palancas-290.sh [dir_salida]
#       SALTAR_BUILD=1   reusa el 1ST_READ.BIN que ya esta en el directorio de salida (no recompila)
#       CORRER=1         ademas lo corre 240 s en Flycast y exige las 3 fases sin OOM (los crc de la
#                        referencia): es la cara de "el binario no muere", no solo el tamano
#       BASE_GOX=...     arbol de .gox del motor (por defecto el de build-tablas-282.sh, /tmp/gox-med).
#                        OJO: tiene que estar hecho con la stdlib de HOY: build-std-gox.sh <dir> y
#                        build-motor-all.sh <dir> <dir_log>. Si esta viejo, el build muere por un simbolo
#                        que falta (medido en DC-002.90: zlib.NuevoInflador de DC-002.80 no estaba).
# sale: 0 = todo en verde; 1 = alguna prueba en rojo; 3 = no se pudo correr
set -o pipefail

REPO="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source=env.sh
. "$REPO/scripts/console/env.sh" >/dev/null 2>&1 || true   # PATH del toolchain (sh-elf-size) y DC_PYTHON
SALIDA="${1:-/tmp/dcbuild/chequeo-290}"
KNOBS="$REPO/console/dc/poc/tablas-mapengine/knobs.go"
BUILD="$REPO/scripts/console/build-tablas-282.sh"
REF_1ST_READ=6859016

# las ocho palancas de la corrida de referencia (DC-002.88 commit d2b59155 / DC-002.89 commit 258412ca)
PALANCAS="modoTablas=3 hacerMapa=true conEntidades=false lienzoReferencia=false gcEntreTablas=true gcInhibidoTablas=true pedirArenaPrimero=false tilesDelStamp=true"

verdes=0
rojas=0
ok()  { echo "   OK    $*"; verdes=$((verdes + 1)); }
mal() { echo "   ROJO  $*"; rojas=$((rojas + 1)); }

# el valor de un const de un knobs.go (los comentarios no cuentan; sirve `X = v` y `const X = v`)
leer() {
    awk -v n="$1" '
        {
            if ($1 == "const") { nom = $2; igual = $3; val = $4 } else { nom = $1; igual = $2; val = $3 }
            if (nom == n && igual == "=" && !visto) { print val; visto = 1; exit }
        }' "$2"
}

# las palancas del entorno las decide el script, no esta prueba
unset TABLAS MAPA ENTIDADES LIENZO GCENTRE GCINHIBE ARENA TILES SOLO_PALANCAS

[ -f "$KNOBS" ] || { echo "FALLA: no esta $KNOBS"; exit 3; }
[ -x "$BUILD" ] || [ -f "$BUILD" ] || { echo "FALLA: no esta $BUILD"; exit 3; }
mkdir -p "$SALIDA" || { echo "FALLA: no puedo crear $SALIDA"; exit 3; }

echo "== 1/4 el knobs.go del POC dice las palancas de la corrida de referencia"
for par in $PALANCAS; do
    nombre="${par%%=*}"
    quiero="${par#*=}"
    tengo="$(leer "$nombre" "$KNOBS")"
    if [ "$tengo" = "$quiero" ]; then
        ok "knobs.go: $nombre = $tengo"
    else
        mal "knobs.go: $nombre = '${tengo:-<no esta>}' y la corrida de referencia usa '$quiero'"
    fi
done

echo "== 2/4 el build SIN variables sale de knobs.go y no avisa de ninguna pisada"
SIN="$SALIDA/palancas-sin-variables"
rm -rf "$SIN"
SALIDA_SIN="$(SOLO_PALANCAS=1 bash "$BUILD" "$SIN" 2>&1)"
ESTADO=$?
if [ "$ESTADO" -ne 0 ]; then
    mal "el build en modo SOLO_PALANCAS salio con estado $ESTADO"
    printf '%s\n' "$SALIDA_SIN" | tail -5
else
    obtiene="$(printf '%s\n' "$SALIDA_SIN" | grep -a '^   TABLAS=' | head -1)"
    case "$obtiene" in
        *"TABLAS=3 MAPA=1 ENTIDADES=0 LIENZO=0 GCENTRE=1 GCINHIBE=1 ARENA=0 TILES=1"*)
            ok "resuelve $obtiene" ;;
        *)  mal "resuelve '${obtiene:-<sin linea de palancas>}' y la referencia es TABLAS=3 MAPA=1 ENTIDADES=0 LIENZO=0 GCENTRE=1 GCINHIBE=1 ARENA=0 TILES=1" ;;
    esac
    if printf '%s\n' "$SALIDA_SIN" | grep -qa 'PISANDO LAS PALANCAS'; then
        mal "avisa de una pisada del entorno cuando no se paso ninguna variable"
    else
        ok "sin aviso de pisada (no hay dos fuentes de verdad)"
    fi
    # el knobs.go que el build escribiria tiene que decir, en VALORES, lo mismo que el del POC
    for par in $PALANCAS; do
        nombre="${par%%=*}"
        quiero="$(leer "$nombre" "$KNOBS")"
        generado="$(leer "$nombre" "$SIN/knobs.go")"
        if [ "$generado" = "$quiero" ]; then
            ok "knobs.go generado: $nombre = $generado (igual al del POC)"
        else
            mal "knobs.go generado: $nombre = '${generado:-<no esta>}' contra '$quiero' del POC"
        fi
    done
fi

echo "== 3/4 si el entorno pisa una palanca, el build lo dice EN GRANDE y la pisada llega al knobs.go"
PIS="$SALIDA/palancas-pisadas"
rm -rf "$PIS"
SALIDA_PIS="$(LIENZO=1 SOLO_PALANCAS=1 bash "$BUILD" "$PIS" 2>&1)"
if printf '%s\n' "$SALIDA_PIS" | grep -qa 'PISANDO LAS PALANCAS'; then
    ok "avisa que el entorno pisa las palancas"
    if printf '%s\n' "$SALIDA_PIS" | grep -qa 'LIENZO=1  (knobs.go dice 0)'; then
        ok "dice cual se piso y contra que valor: 'LIENZO=1 (knobs.go dice 0)'"
    else
        mal "avisa de la pisada pero NO dice cual ni contra que valor de knobs.go"
    fi
    if printf '%s\n' "$SALIDA_PIS" | grep -qa "$REF_1ST_READ B"; then
        ok "al lado del aviso va el tamano del binario de referencia ($REF_1ST_READ B)"
    else
        mal "el aviso no dice cuanto mide el binario de la referencia ($REF_1ST_READ B)"
    fi
else
    mal "con LIENZO=1 NO avisa de la pisada (el silencio es como se cuela un binario que OOMea)"
fi
if [ "$(leer lienzoReferencia "$PIS/knobs.go")" = "true" ]; then
    ok "la pisada llega al knobs.go generado (lienzoReferencia = true)"
else
    mal "la pisada NO llega al knobs.go generado (lienzoReferencia = '$(leer lienzoReferencia "$PIS/knobs.go")')"
fi

echo "== 4/4 el build completo SIN variables da el 1ST_READ de la referencia ($REF_1ST_READ B)"
echo "   (arbol de .gox: ${BASE_GOX:-el de build-tablas-282.sh, /tmp/gox-med})"
if [ "${SALTAR_BUILD:-0}" = "1" ] && [ -f "$SALIDA/1ST_READ.BIN" ]; then
    echo "   SALTAR_BUILD=1: se reusa $SALIDA/1ST_READ.BIN ($(stat -c%s "$SALIDA/1ST_READ.BIN") B)"
else
    SALIDA_BUILD="$(bash "$BUILD" "$SALIDA" 2>&1)"
    ESTADO_BUILD=$?
    printf '%s\n' "$SALIDA_BUILD" | sed 's/^/   | /'
    if [ "$ESTADO_BUILD" -ne 0 ]; then
        mal "el build completo fallo (estado $ESTADO_BUILD; el log queda en $SALIDA/build.log)"
        case "$SALIDA_BUILD" in
            *"undefined identifier"*|*"undefined:"*)
                echo "   pista: un simbolo que falta casi siempre es el ARBOL DE .GOX viejo respecto de la stdlib"
                echo "          del repo. Se rehace con:  scripts/console/build-std-gox.sh <dir> &&"
                echo "                                    scripts/console/build-motor-all.sh <dir> <dir_log>"
                echo "          y se pasa con BASE_GOX=<dir> (medido en DC-002.90: zlib.NuevoInflador de DC-002.80"
                echo "          no estaba en /tmp/gox-med, que se armo el 30/10 a las 13:22)." ;;
        esac
    fi
fi

if [ -f "$SALIDA/1ST_READ.BIN" ]; then
    TAM="$(stat -c%s "$SALIDA/1ST_READ.BIN")"
    SHA="$(sha256sum "$SALIDA/1ST_READ.BIN" | cut -c1-16)"
    if [ "$TAM" = "$REF_1ST_READ" ]; then
        ok "1ST_READ.BIN = $TAM B, sha256 ${SHA}... (la corrida de referencia de DC-002.88/.89)"
    else
        mal "1ST_READ.BIN = $TAM B ($((TAM - REF_1ST_READ)) B contra la referencia). Si el codigo crecio a proposito, actualiza REF_1ST_READ en este script Y en build-tablas-282.sh; si no, algo se movio de mas"
    fi
    iguales=0
    distintas=0
    for par in $PALANCAS; do
        nombre="${par%%=*}"
        quiero="$(leer "$nombre" "$KNOBS")"
        delbuild="$(leer "$nombre" "$SALIDA/src/knobs.go")"
        if [ "$delbuild" = "$quiero" ]; then
            iguales=$((iguales + 1))
        else
            distintas=$((distintas + 1))
            mal "el knobs.go DEL BUILD dice $nombre = '${delbuild:-<no esta>}' contra '$quiero' del POC: el binario no es el de la referencia"
        fi
    done
    [ "$distintas" -eq 0 ] && ok "el knobs.go del build coincide en valores con el del POC ($iguales/8)"
    if [ -f "$SALIDA/prog.elf" ]; then
        echo "   (.text $(sh-elf-size -A "$SALIDA/prog.elf" | awk '$1==".text"{print $2}') B, .rodata $(sh-elf-size -A "$SALIDA/prog.elf" | awk '$1==".rodata"{print $2}') B, imagen $SALIDA/disc.gdi)"
    fi
else
    mal "no hay $SALIDA/1ST_READ.BIN: no se puede verificar el numero de la referencia"
fi

if [ "${CORRER:-0}" = "1" ]; then
    echo "== 5/5 la corrida: 240 s en Flycast, las 3 fases del pueblo y ningun OOM"
    if [ ! -f "$SALIDA/disc.gdi" ]; then
        mal "no hay $SALIDA/disc.gdi: no se puede correr"
    else
        CAPTURE=none bash "$REPO/scripts/console/run-flycast.sh" "$SALIDA/disc.gdi" 240 "$SALIDA/corrida" \
            > "$SALIDA/corrida.log" 2>&1
        LOG="$SALIDA/corrida/flycast.log"
        if [ ! -f "$LOG" ]; then
            mal "no quedo el log de la corrida ($LOG; ver $SALIDA/corrida.log)"
        else
            tr -d '\r' < "$LOG" > "$SALIDA/corrida-serie.txt"
            if grep -qa 'fatal error\|out of memory' "$SALIDA/corrida-serie.txt"; then
                mal "el binario SE MURIO en la consola:"
                grep -a -m2 'fatal error\|out of memory' "$SALIDA/corrida-serie.txt" | sed 's/^/         /'
            else
                ok "la consola no imprimio 'fatal error' ni 'out of memory'"
            fi
            # las tres fases de DC-002.88/.89, con su numero exacto: si el pueblo cambiara, esto se pone rojo
            for esperado in \
                "MAPA: a) ENTRA AL PUEBLO CASILLAS pisos= 2337 paredes= 338 crcGrilla= -1596781041" \
                "MAPA: b) CAMBIA A LA TRANSICION ESTE CASILLAS pisos= 369 paredes= 39 crcGrilla= -205471136" \
                "MAPA: c) VUELVE AL PUEBLO CASILLAS pisos= 2337 paredes= 338 crcGrilla= -1596781041"; do
                if grep -qaF "$esperado" "$SALIDA/corrida-serie.txt"; then
                    ok "${esperado#MAPA: }"
                else
                    mal "no salio: ${esperado#MAPA: }"
                fi
            done
            if grep -qaF 'PICO usado= 9525408' "$SALIDA/corrida-serie.txt"; then
                ok "pico del monton 9.525.408 B (el de DC-002.88/.89)"
            else
                mal "el pico del monton no es 9.525.408 B: $(grep -a -m1 -o 'PICO usado= [0-9]*' "$SALIDA/corrida-serie.txt")"
            fi
            if grep -qaF 'GRILLA OK' "$SALIDA/corrida-serie.txt"; then
                ok "GRILLA OK: el pueblo de la vuelta es identico al de la entrada"
            else
                mal "no salio 'GRILLA OK'"
            fi
        fi
    fi
fi

echo
echo "== resumen: $verdes en verde, $rojas en rojo"
if [ "$rojas" -eq 0 ]; then
    echo "== VERDE: una sola fuente de verdad y el binario de la referencia verificado"
    exit 0
fi
echo "== ROJO: las palancas del build volvieron a tener dos fuentes de verdad (o el binario dejo de ser el de la referencia)"
exit 1
