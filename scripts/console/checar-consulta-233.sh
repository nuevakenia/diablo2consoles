#!/bin/bash
# CONTROL 2b FOCADO (DC-002.39b): LA CONSULTA DE LOS 21.253 NOMBRES, repetida N vueltas sobre el MISMO
# runtime (el que manda $DC_ROOT) para poder decir "estable en N corridas" con un numero comparable.
#
# Es la seccion 2 del corpus de DC-002.33 (`checar-tablas-233.sh`) SOLA: mismos comandos, mismo POC
# (`tablas-mpq`), misma extraccion de numeros y mismo criterio (la consola tiene que encontrar lo MISMO que
# el host en los 3 MPQ, con `largas= 0` y el total en 21.253). Existe porque el residuo de la consulta es
# INTERMITENTE -- 0..6 nombres por corrida (evidencia dc-002.39-nombre-corrompido.txt) -- y el corpus entero
# tarda minutos por vuelta: para medir "estable" hay que repetir barato. El corpus sigue siendo la barrera;
# esto es el instrumento de repeticion.
#
# Uso: bash scripts/console/checar-consulta-233.sh [vueltas] [segundos] [dir_salida] [dir_gox] [dir_mpq]
#   vueltas:    por defecto 3
#   segundos:   por defecto 75 (la ventana de Flycast; la consulta de los 3 MPQ entra holgada)
#   dir_salida: por defecto ~/.hermes/cache/scratch/consulta-233
#   dir_gox:    por defecto ${GODC_GOX:-/tmp/gox-233}
#   dir_mpq:    por defecto la instalacion de Diablo II
#
# Corre con el runtime que diga $DC_ROOT (una copia privada armada con los scripts `fix-libgodc-*.py`):
#   DC_ROOT=/tmp/dcroot-39b-mut bash scripts/console/checar-consulta-233.sh      # el MUTANTE: tiene que dar ROJO
#   DC_ROOT=/tmp/dcroot-39b-fix bash scripts/console/checar-consulta-233.sh      # con el arreglo: verde
# Sale 0 si TODAS las vueltas cierran en 21.253 y 1 si alguna queda corta (asi sirve de control del mutante).
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

VUELTAS="${1:-3}"
SEGS="${2:-75}"
SAL="${3:-/home/dgalli/.hermes/cache/scratch/consulta-233}"
GEN="${4:-${GODC_GOX:-/tmp/gox-233}}"
MPQ="${5:-$HOME/.wine-d2/drive_c/Games/Diablo II}"
# POC: `tablas-mpq` (el de la consulta) o `tablas-mpq-viva` (el mismo bucle con la vigia del runtime, para
# ver DENTRO de una recoleccion; su vigia vive en el runtime, no en su csrc/: ver DC-002.39c)
POC="${POC:-tablas-mpq}"
MPQ_LISTA="patch_d2.mpq,d2data.mpq,d2char.mpq"
MPQ_ARCHIVOS="patch_d2.mpq d2data.mpq d2char.mpq"
REPO="${REPO:-$(cd "$HERE/../.." && pwd)}"
ESPERADO=21253

verde() { echo "   OK: $*"; }
rojo() { echo "   FALLO: $*"; }
nota() { echo "   $*"; }

# un campo de la linea de un MPQ (igual que el corpus, para que los numeros sean comparables)
campo() { # $1 = log, $2 = patron del archivo (fijo), $3 = nombre del campo
    tr -d '\r' < "$1" | grep -aF "$2" | grep -aF "$3=" | head -1 | sed "s/.*$3 *= *\([0-9]*\).*/\1/"
}

echo "== 0. el QUIEN: el runtime que se va a medir"
nota "DC_ROOT= $DC_ROOT"
nota "LIBGODC= $LIBGODC"
nota "libgodc.a: $(stat -c%s "$LIBGODC/libgodc.a" 2>/dev/null || echo SIN-ARMA) bytes, sha256 $(sha256sum "$LIBGODC/libgodc.a" 2>/dev/null | cut -c1-16)"
nota "candidato interior (DC-002.39b): $("$DC_PYTHON" "$HERE/fix-libgodc-candidato.py" --estado "$LIBGODC" 2>&1 | tail -1)"

mkdir -p "$SAL/juego"
for n in $MPQ_ARCHIVOS; do
    real="$(ls -1 "$MPQ" | grep -ix "$n" | head -1)"
    [ -n "$real" ] || { echo "   no encuentro $n en $MPQ"; exit 1; }
    ln -f "$MPQ/$real" "$SAL/juego/$n" 2>/dev/null || cp "$MPQ/$real" "$SAL/juego/$n"
done
nota "los 3 MPQ: $(ls "$SAL/juego" | tr '\n' ' ')"

# los nombres que consulta la consola (los escribe el MISMO codigo del host) y la referencia del host
(cd "$REPO" && go run ./console/dc/sonda-tablas-mpq -mpqdir "$SAL/juego" -mpq "$MPQ_LISTA" -nombres-dir "$SAL/juego") \
    > "$SAL/nombres-host.txt" 2>&1
(cd "$REPO" && go run ./console/dc/sonda-tablas-mpq -mpqdir "$SAL/juego" -mpq "$MPQ_LISTA") \
    > "$SAL/tablas-host.txt" 2>&1
nota "nombres para la consola: $(cat "$SAL"/juego/nombres_*.txt 2>/dev/null | wc -l) lineas"

echo "== 1. el POC $POC (el que hace la consulta), armado contra $LIBGODC"
MKISO_JOLIET=0 DC_RUTA_DATOS=/cd/juego DC_MPQ_JUEGO="$MPQ_LISTA" DC_SIN_SELLOS=1 DC_DISCO_JUEGO="$SAL/juego" \
    POC="$POC" bash "$HERE/build-motor-arranque.sh" "$SAL/tablas" "$GEN" > "$SAL/tablas.build.log" 2>&1
if [ ! -e "$SAL/tablas/disc.gdi" ]; then
    rojo "el POC tablas-mpq NO se armo (ver $SAL/tablas.build.log)"
    tail -6 "$SAL/tablas.build.log" | sed 's/^/     /'
    exit 1
fi
nota "POC armado: $SAL/tablas/disc.gdi"

echo "== 2. la consulta de los 3 MPQ, $VUELTAS vueltas (ventana de $SEGS s cada una)"
VERDES=0
ROJAS=0
TOTALES=""
for v in $(seq 1 "$VUELTAS"); do
    [ -e "$SAL/run-$v" ] && mv "$SAL/run-$v" "$SAL/run-$v.viejo-$(date +%s)"
    CAPTURE=none bash "$HERE/run-flycast.sh" "$SAL/tablas/disc.gdi" "$SEGS" "$SAL/run-$v" >/dev/null 2>&1
    log="$SAL/run-$v/flycast.log"

    total=0
    linea=""
    bien=1
    for f in $MPQ_ARCHIVOS; do
        nc="$(campo "$log" "$f " nombres)"
        ec="$(campo "$log" "$f " contenidos)"
        lc="$(campo "$log" "$f " largas)"
        eh="$(campo "$SAL/tablas-host.txt" "$f " contenidos)"
        total=$((total + ${ec:-0}))
        linea="$linea $f:$ec/$eh"
        [ -n "$nc" ] && [ -n "$ec" ] && [ "$ec" = "$eh" ] && [ "${lc:-1}" = "0" ] || bien=0
    done

    if [ "$bien" = "1" ] && [ "$total" = "$ESPERADO" ]; then
        VERDES=$((VERDES + 1))
        verde "vuelta $v: la consulta encuentra $total de $ESPERADO ($linea)"
    else
        ROJAS=$((ROJAS + 1))
        rojo "vuelta $v: la consulta da $total de $ESPERADO ($linea); el POC que falla dice:"
        tr -d '\r' < "$log" | grep -a 'NO-ENCONTRADO\|FALLO\|TABLAS-MPQ-TOTAL' | head -4 | sed 's/^ *//;s/^/     /'
    fi
    TOTALES="$TOTALES $total"
done

echo "== RESULTADO: $VERDES de $VUELTAS vueltas con los $ESPERADO nombres (totales:$TOTALES)"
[ "$ROJAS" = "0" ] || exit 1
