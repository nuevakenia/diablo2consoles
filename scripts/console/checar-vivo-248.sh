#!/bin/bash
# DC-002.48: EL TECHO DEL DATO VIVO DEL ARRANQUE, COMO CONTROL (no como numero suelto en un log).
#
# QUE MIDE
#   El dato VIVO maximo de una corrida del arranque del motor en Flycast (la marca `gc-vivo` del runtime,
#   `instrumentar-vivo-245.py`: lo que sobrevive a una copia), el avance del arranque (assets del MPQ pedidos y
#   lecturas de tabla terminadas) y el techo de la particion. Lee la corrida que deja
#   `checar-trozos-245.sh` en `$1/run-sano/flycast.log` (o la que se le pase con -corrida).
#
# POR QUE ES UN CONTROL Y NO UN `nota`
#   Una suite verde con el dato vivo otra vez al 98,9 % se lee igual que una que lo bajo: el numero tiene que
#   hacer FALLAR la corrida. Este control es el que caza el mutante de las palancas de esta tarjeta
#   (`mutar-vivo-248.py --pristino`): con los 36 campos muertos del record de Objects de vuelta y las cadenas
#   sin copiar, el dato vivo vuelve a 1.037.096 B y este control se pone ROJO.
#
# Uso:
#   bash scripts/console/checar-vivo-248.sh <dir_de_la_corrida> [techo_del_dato_vivo]
#     <dir_de_la_corrida>   directorio que tiene run-sano/flycast.log (el $SAL del corpus)
#     techo_del_dato_vivo   por defecto 950000 B (el sano medido queda por debajo; el pristino en 1.037.096)
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh" 2>/dev/null

SAL="${1:?uso: checar-vivo-248.sh <dir_de_la_corrida> [techo_del_dato_vivo]}"
VIVO_MAX="${2:-950000}"
PISO_ASSETS="${PISO_ASSETS:-6}"
PISO_LECTURAS="${PISO_LECTURAS:-5}"
LOG="$SAL/run-sano/flycast.log"
TECHO="$("$DC_PYTHON" "$HERE/fix-libgodc-semiespacio-245.py" "${LIBGODC:-$HOME/dreamcast/libgodc}" --estado 2>/dev/null |
    grep -m1 'cada semiespacio' | sed 's/.*cada semiespacio: *\([0-9]*\) B.*/\1/')"
[ -n "${TECHO:-}" ] || TECHO=$((1024 * 1024))

OK=0
FALLOS=0
verde() { OK=$((OK + 1)); echo "   OK: $*"; }
rojo() { FALLOS=$((FALLOS + 1)); echo "   FALLO: $*"; }

[ -s "$LOG" ] || { echo "   FALLO: no hay corrida en $LOG"; exit 1; }
log_de() { tr -d '\r' < "$1"; }

MAXV="$(log_de "$LOG" | grep -ao 'gc-vivo v=[0-9]*' | sed 's/.*v=//' | sort -n | tail -1)"
ASSETS="$(log_de "$LOG" | grep -ac 'Loaded /cd/juego')"
LECTURAS="$(log_de "$LOG" | grep -ac 'ARQ-CARGA lee-volvio')"
MARCAS="$(log_de "$LOG" | grep -ac 'gc-vivo')"
ULTIMA="$(log_de "$LOG" | grep -a 'ARQ-CARGA lee-entra' | tail -1 | sed 's/.*lee-entra //' | cut -c1-60)"

echo "   corrida: $LOG"
echo "   dato vivo maximo: ${MAXV:-?} B de $TECHO   (techo de este control: $VIVO_MAX B)"
echo "   arranque: $ASSETS assets del MPQ, $LECTURAS lecturas terminadas; ultima pedida: $ULTIMA"

# 1. el instrumento tiene que haber medido (sin marcas no hay numero y el control no puede decir verde)
if [ "${MARCAS:-0}" -ge 1 ]; then
    verde "control 1: la corrida trae $MARCAS marcas 'gc-vivo' (el dato vivo se midio)"
else
    rojo "control 1: la corrida NO trae marcas 'gc-vivo': sin instrumento no hay numero que valga"
fi

# 2. el dato vivo por DEBAJO del techo declarado
if [ -n "${MAXV:-}" ] && [ "$MAXV" -lt "$VIVO_MAX" ]; then
    verde "control 2: el dato vivo maximo queda en $MAXV B, $((VIVO_MAX - MAXV)) B por debajo del techo de $VIVO_MAX B ($((MAXV * 1000 / TECHO / 10)).$((MAXV * 1000 / TECHO % 10)) % del semiespacio de $TECHO)"
else
    rojo "control 2: el dato vivo maximo es ${MAXV:-?} B y el techo de este control es $VIVO_MAX B (las palancas no estan puestas o no hicieron nada)"
fi

# 3. el arranque llega IGUAL O MAS LEJOS que antes de las palancas (los assets y las tablas no se pierden)
if [ "${ASSETS:-0}" -ge "$PISO_ASSETS" ]; then
    verde "control 3: el arranque llega a $ASSETS assets del MPQ (piso $PISO_ASSETS)"
else
    rojo "control 3: el arranque se queda en $ASSETS assets del MPQ (piso $PISO_ASSETS): las palancas costaron avance"
fi

if [ "${LECTURAS:-0}" -ge "$PISO_LECTURAS" ]; then
    verde "control 4: $LECTURAS lecturas de tabla terminadas (piso $PISO_LECTURAS)"
else
    rojo "control 4: $LECTURAS lecturas terminadas (piso $PISO_LECTURAS)"
fi

echo "   == RESULTADO: $OK controles OK, $FALLOS con fallo =="
[ "$FALLOS" -eq 0 ]
