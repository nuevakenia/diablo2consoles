#!/bin/bash
# DC-002.39d: LA CADENA DEL RUNTIME VENDORIZADO EN UN SOLO PUNTO DE ENTRADA.
#
# EL DEFECTO (medido, evidencia console/dc/evidencia/dc-002.39d-cadena.txt)
#   Cada `fix-libgodc-*.py` deja el arbol como le conviene, y los que RESETEAN (`git checkout` del archivo
#   pristino) se llevan puesto lo que aplico otro: el ULTIMO que corre es el que manda. Medido paso por paso
#   sobre una copia del vendor (`runtime/gc_copy.c`: sha256, bytes y estado de la Store Queue):
#
#     pristino  (git show HEAD)                                130de736  43749 B   >>26
#     fix-libgodc-gc.py 512   (pristino + ABI + interfaces)     379271ee  44509 B   >>24
#     fix-libgodc-candidato.py (reset + punteros interiores + candidato + marcas)
#                                                               fec6ef13  50074 B   >>26   <- REVIERTE la SQ
#     fix-libgodc-sq.py --sin-pristino (solo la Store Queue)    988e80fc  50834 B   >>24   <- estado completo
#
#   Por eso el 2026-10-19 a las 01:04 el vendor quedo en `>>26` (el ultimo habia sido el candidato) y
#   DC-002.39c midio encima de ese defecto. Al rearmar el `.a` a las 02:54 quedo bien: el vendor de hoy es
#   `988e80fc`, EXACTAMENTE lo que da esta cadena (verificado por sha256, ver la evidencia).
#
# QUE HACE
#   Corre la cadena ENTERA en el orden que importa -- y con la SQ al FINAL, despues del ultimo script que
#   resetea `gc_copy.c` -- desde CUALQUIER estado de partida (pristino, a medio aplicar o sucio):
#
#     1. fix-libgodc-gc.py <KB> <LIB>          pristino + ABI + interfaces + la Store Queue
#     2. fix-libgodc-candidato.py <LIB>        reset + punteros interiores + candidato + marcas ARQ-MARCA
#     3. fix-libgodc-sq.py <LIB> --sin-pristino  LA STORE QUEUE (el paso que faltaba: sin resetear)
#     4. (con --build) el `libgodc.a` al dia
#
# USO
#   bash scripts/console/fix-libgodc-cadena.sh [LIB] [--kb 512] [--build] [--verificar]
#     LIB:   la copia del runtime (por defecto $LIBGODC). Con una copia privada se mide sin tocar el vendor.
#     --kb:  el semiespacio del GC en KiB (por defecto 512, el valor de DC-002.25).
#     --build: limpia los .o/.a y rehace `libgodc.a` (lo que hace falta para que el POC lo vea).
#     --verificar: exige el estado COMPLETO (interiores + candidato + SQ + marcas + el KB pedido) y sale
#                  distinto de 0 si falta algo. Es el control que se le puede poner a un arbol cualquiera:
#                  sirve para el vendor antes de medir con el.
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

KB=512
BUILD=0
VERIFICAR=0
LIB=""
while [ $# -gt 0 ]; do
    case "$1" in
        --kb)        KB="$2"; shift 2 ;;
        --build)     BUILD=1; shift ;;
        --verificar) VERIFICAR=1; shift ;;
        --*)         echo "opcion desconocida: $1" >&2; exit 2 ;;
        *)           LIB="$1"; shift ;;
    esac
done
LIB="${LIB:-$LIBGODC}"
C="$LIB/runtime/gc_copy.c"

[ -f "$C" ] || { echo "no encuentro $C" >&2; exit 2; }

sha() { sha256sum "$C" | cut -c1-16; }
dato() { printf '   %-52s sha %s  %s B  SQ=%s\n' "$1" "$(sha)" "$(wc -c < "$C")" "$(grep -c 'dst_addr >> 24' "$C")"; }

# el veredicto del estado: los marcadores de cada arreglo de la cadena (los textos que deja cada script)
veredicto() { # $1 = archivo
    local f="$1" fallos=0
    [ "$(grep -c 'dst_addr >> 24' "$f")" = "2" ] || { echo "   FALTA la Store Queue (>>24)"; fallos=$((fallos + 1)); }
    [ "$(grep -c 'dst_addr >> 26' "$f")" = "0" ] || { echo "   el arbol tiene la formula VIEJA (>>26): el defecto DC-002.24"; fallos=$((fallos + 1)); }
    [ "$(grep -c 'PUNTEROS INTERIORES' "$f")" -ge 1 ] || { echo "   FALTAN los punteros interiores (DC-002.36)"; fallos=$((fallos + 1)); }
    [ "$(grep -c 'DC-002.39b' "$f")" -ge 1 ] || { echo "   FALTA el candidato del barrido (DC-002.39b)"; fallos=$((fallos + 1)); }
    [ "$(grep -c 'ARQ-MARCA (DC-002.29)' "$f")" -ge 1 ] || { echo "   FALTAN las marcas del arranque (DC-002.29)"; fallos=$((fallos + 1)); }
    local kb
    kb="$(grep -a '^#define GC_SEMISPACE_SIZE_KB' "$LIB/runtime/godc_config.h" 2>/dev/null | head -1 | grep -oE '[0-9]+$')"
    [ "${kb:-0}" = "$KB" ] || { echo "   el semiespacio esta en ${kb:-?} KiB y se pidio $KB"; fallos=$((fallos + 1)); }
    return "$fallos"
}

if [ "$VERIFICAR" = "1" ]; then
    echo "== VERIFICAR $LIB (el estado completo de la cadena, semiespacio $KB KiB)"
    if veredicto "$C"; then
        echo "   OK: la cadena esta completa (sha $(sha), $(wc -c < "$C") B)"
        exit 0
    fi
    echo "   ROJO: la cadena esta INCOMPLETA -> correr: bash $0 $LIB --kb $KB --build"
    exit 1
fi

echo "== la cadena completa del runtime: $LIB (semiespacio $KB KiB)"
dato "0. como estaba (el estado de partida)"
grep -n 'dst_addr >> 2' "$C" | head -2 | sed 's/^/     /'

echo "   1/3 fix-libgodc-gc.py $KB  (pristino + ABI + interfaces + la Store Queue)"
python3 "$HERE/fix-libgodc-gc.py" "$KB" "$LIB" > /tmp/dc-cadena-gc.log 2>&1 || {
    echo "   FALLO: ver /tmp/dc-cadena-gc.log" >&2; tail -5 /tmp/dc-cadena-gc.log >&2; exit 1; }
dato "    despues de gc.py"

echo "   2/3 fix-libgodc-candidato.py  (punteros interiores + candidato + marcas; RESETEA gc_copy.c y se lleva la SQ)"
python3 "$HERE/fix-libgodc-candidato.py" "$LIB" > /tmp/dc-cadena-candidato.log 2>&1 || {
    echo "   FALLO: ver /tmp/dc-cadena-candidato.log" >&2; tail -5 /tmp/dc-cadena-candidato.log >&2; exit 1; }
dato "    despues del candidato (aca la SQ vuelve a la formula vieja: es el paso que se la lleva)"
grep -n 'dst_addr >> 2' "$C" | head -2 | sed 's/^/        /'

echo "   3/3 fix-libgodc-sq.py --sin-pristino  (LA STORE QUEUE, sin resetear: el paso que la cadena no tenia)"
python3 "$HERE/fix-libgodc-sq.py" "$LIB" --sin-pristino > /tmp/dc-cadena-sq.log 2>&1 || {
    echo "   FALLO: ver /tmp/dc-cadena-sq.log" >&2; tail -5 /tmp/dc-cadena-sq.log >&2; exit 1; }
dato "    despues de la SQ (el estado completo)"
grep -n 'dst_addr >> 2' "$C" | head -2 | sed 's/^/        /'

if [ "$BUILD" = "1" ]; then
    echo "   4/4 el libgodc.a al dia"
    rm -f "$LIB"/runtime/*.o "$LIB"/*.a
    make -C "$LIB" -s all > /tmp/dc-cadena-make.log 2>&1 || {
        echo "   FALLO: ver /tmp/dc-cadena-make.log" >&2; tail -5 /tmp/dc-cadena-make.log >&2; exit 1; }
    echo "        libgodc.a: $(stat -c%s "$LIB/libgodc.a") bytes, sha256 $(sha256sum "$LIB/libgodc.a" | cut -c1-16)"
fi

# el control final: el estado completo, siempre el mismo desde cualquier estado de partida
if veredicto "$C"; then
    echo "   CADENA OK: $(sha) $(wc -c < "$C") B  (el estado completo; el mutante es volver a >>26)"
else
    echo "   CADENA INCOMPLETA (ver arriba)" >&2
    exit 1
fi
