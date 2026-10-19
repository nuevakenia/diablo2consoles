#!/bin/bash
# CORPUS DE DC-002.36: LOS PUNTEROS INTERIORES DEL GC (las cadenas que apuntan al medio de otro objeto).
#
# EL DEFECTO (medido antes de este arreglo, y por eso la tarjeta lo lleva al corpus)
#   `strings.Fields(s)`, `strings.Split(s)` y el decodificador de json devuelven CADENAS QUE SON PEDAZOS
#   del buffer que se leyo: el puntero de la cadena cae en el MEDIO de un objeto del monton. En Go eso es
#   legal -- el recolector tiene que mantener vivo el objeto que CONTIENE el puntero -- pero el GC de
#   libgodc RECHAZA a proposito los punteros interiores (`gc_is_valid_object_start`: "rejecting interior
#   pointers"). Entonces el buffer grande se queda sin referencias validas, se lo lleva la recoleccion y
#   las cadenas pasan a mostrar los bytes de OTRO objeto.
#
#   Se ve en dos lugares, y el segundo es el que importa:
#     - el POC `menu-dc-236` carga 3 de las 11 texturas y las otras 8 fallan con `file does not exist`
#       (su NOMBRE ya no es el nombre);
#     - el POC `tablas-mpq` imprime el largo del tercer nombre como `-1795502946` (DC-002.39).
#
# QUE MIDE ESTE CORPUS (con el POC `sonda-cadenas`, que es el instrumento minimo y sin PVR)
#   A  las cadenas son PEDAZOS de una reserva grande que despues nadie referencia   -> el defecto
#   B  las mismas cadenas en CAMPOS de objetos del monton (el camino preciso del GC) -> el defecto
#   C  las mismas cadenas, cada una con reserva PROPIA (control sano de la sonda)   -> tiene que andar
#      SIEMPRE: con el defecto y sin el. Si C se cae, el que esta mal es el instrumento, no el runtime.
#   La huella es el CRC-32 de todas las cadenas pegadas, medido ANTES y DESPUES de 5 recolecciones
#   forzadas con runtime.GC() (deterministas: no hay que llenar el semiespacio de basura para que el GC
#   corra, que fue el primer instrumento y moria en `GC to-space overflow` a los 3,4 s).
#
# EL MUTANTE (obligatorio: una suite verde no prueba nada)
#   `fix-libgodc-interior.py --quitar --build` deja el arbol con la cadena documentada del runtime y SIN
#   este arreglo, y el corpus tiene que ponerse ROJO en A y en B (y C tiene que seguir verde). Despues se
#   vuelve a aplicar y la ultima corrida es el CONTROL SANO.
#
# Uso:  bash scripts/console/checar-cadenas-236.sh [segundos_consola]
#   SAL (variable): /home/dgalli/.hermes/cache/scratch/corpus-cadenas-236
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

SEGS="${1:-25}"
REPO="${REPO:-$(cd "$HERE/../.." && pwd)}"
SAL="${SAL:-/home/dgalli/.hermes/cache/scratch/corpus-cadenas-236}"
POC="sonda-cadenas"
FUENTE="$REPO/console/dc/poc/$POC"
FIX="$HERE/fix-libgodc-interior.py"

OK=0
FALLOS=0
nota() { echo "   $*"; }
verde() { OK=$((OK + 1)); echo "   OK: $*"; }
rojo() { FALLOS=$((FALLOS + 1)); echo "   FALLO: $*"; }

mkdir -p "$SAL"

# la corrida de la sonda: arma el POC y lo corre en Flycast. Deja el log en $SAL/run-<nombre>/flycast.log
correr() { # $1 = nombre de la corrida
    local d="$SAL/$1"
    rm -rf "$d"
    mkdir -p "$d"
    bash "$HERE/build-dc.sh" "$FUENTE" "$d/build" > "$d/build.log" 2>&1 || return 1
    CAPTURE=none bash "$HERE/run-flycast.sh" "$d/build/disc.gdi" "$SEGS" "$d" >/dev/null 2>&1
    return 0
}

# el veredicto de un caso en una corrida: OK / ROJO / (vacio si la sonda no imprimio el caso)
veredicto() { # $1 = log, $2 = caso (A|B|C)
    tr -d '\r' < "$1" | grep -a "SONDA-CADENAS: $2 " | tail -1 |
        sed -n "s/.*SONDA-CADENAS: $2 \([A-Z]*\).*/\1/p"
}

huella() { # $1 = log, $2 = etiqueta (ANTES|DESPUES)
    tr -d '\r' < "$1" | grep -a "SONDA-CADENAS: $2 " | tail -1 |
        sed -n 's/.*A= *\([0-9]*\).*/\1/p'
}

# ================================================================================================
echo "== 0. el estado del arbol del runtime y el del vendor"
"$DC_PYTHON" "$FIX" --estado > "$SAL/estado.log" 2>&1
nota "$(tail -1 "$SAL/estado.log")"

if grep -qa 'interior: APLICADO' "$SAL/estado.log"; then
    verde "control 0: el runtime tiene el arreglo de punteros interiores puesto"
else
    rojo "control 0: el runtime NO tiene el arreglo (correr $FIX --build antes de medir)"
fi

SEMI_KB="$(grep -a '^#define GC_SEMISPACE_SIZE_KB' "$LIBGODC/runtime/godc_config.h" 2>/dev/null | head -1 | grep -oE '[0-9]+$' | head -1)"
if [ "${SEMI_KB:-0}" = "512" ]; then
    verde "control 0b: el semiespacio del GC sigue en 512 KiB (el valor de DC-002.25: los numeros de abajo son comparables)"
else
    rojo "control 0b: el semiespacio esta en ${SEMI_KB:-?} KB y no en 512"
fi

# ================================================================================================
echo "== 1. la sonda con el arreglo puesto (la que tiene que quedar VERDE)"
if correr sano; then
    L="$SAL/sano/flycast.log"
    nota "la consola, tal cual:"
    tr -d '\r' < "$L" | grep -a 'SONDA-CADENAS' | head -10 | sed 's/^/     /'

    for caso in A B C; do
        V="$(veredicto "$L" "$caso")"
        if [ "$V" = "OK" ]; then
            verde "control 1: caso $caso OK ($(huella "$L" ANTES) == $(huella "$L" DESPUES), $(tr -d '\r' < "$L" | grep -ac "SONDA-CADENAS: $caso OK") caso(s))"
        else
            rojo "control 1: caso $caso sale '${V:-sin veredicto}' y tenia que salir OK (ver $L)"
        fi
    done

    RECOL="$(tr -d '\r' < "$L" | grep -a 'SONDA-CADENAS: recolecciones=' | tail -1 | sed 's/.*recolecciones= *\([0-9]*\).*/\1/')"
    if [ "${RECOL:-0}" = "5" ]; then
        verde "control 1b: las 5 recolecciones forzadas corrieron y la sonda cerro (sin GC to-space overflow)"
    else
        rojo "control 1b: la sonda hizo ${RECOL:-0} de 5 recolecciones (o murio antes: ver $L)"
    fi

    if [ "$(tr -d '\r' < "$L" | grep -ac 'GC to-space overflow')" = "0" ]; then
        verde "control 1c: la corrida NO tiene 'GC to-space overflow' (el instrumento no se pasa del semiespacio)"
    else
        rojo "control 1c: la corrida murio por 'GC to-space overflow' (el instrumento mide otra cosa)"
    fi
else
    rojo "la sonda NO COMPILA/ARMA con el arreglo puesto (ver $SAL/sano/build.log)"
fi

# ================================================================================================
echo "== 2. el MUTANTE: el runtime SIN el arreglo (la cadena documentada sola)"
"$DC_PYTHON" "$FIX" --quitar --build > "$SAL/mutante-fix.log" 2>&1
nota "$(grep -a 'interior:' "$SAL/mutante-fix.log" | tail -1)"
nota "$(grep -a 'libgodc.a:' "$SAL/mutante-fix.log" | tail -1)"

if grep -qa 'interior: QUITADO' "$SAL/mutante-fix.log"; then
    verde "control 2: el arbol quedo con el arreglo QUITADO (el mutante es de verdad)"
else
    rojo "control 2: no se pudo quitar el arreglo (ver $SAL/mutante-fix.log)"
fi

if correr mutante; then
    LM="$SAL/mutante/flycast.log"
    nota "la consola, tal cual:"
    tr -d '\r' < "$LM" | grep -a 'SONDA-CADENAS' | head -10 | sed 's/^/     /'

    for caso in A B; do
        V="$(veredicto "$LM" "$caso")"
        if [ "$V" = "ROJO" ]; then
            verde "control 2b: con el mutante el caso $caso sale ROJO ($(huella "$LM" ANTES) -> $(huella "$LM" DESPUES)): la suite lo caza"
        else
            rojo "control 2b: con el mutante el caso $caso sale '${V:-sin veredicto}' y tenia que salir ROJO: el corpus NO mide el defecto"
        fi
    done

    V="$(veredicto "$LM" C)"
    if [ "$V" = "OK" ]; then
        verde "control 2c: con el mutante el caso C (control sano de la sonda) SIGUE verde: el que se cae es el defecto, no el instrumento"
    else
        rojo "control 2c: con el mutante el caso C sale '${V:-sin veredicto}': el instrumento no es confiable"
    fi
else
    rojo "la sonda NO COMPILA/ARMA con el mutante (ver $SAL/mutante/build.log)"
fi

# ================================================================================================
echo "== 3. el arbol vuelve como estaba (CONTROL SANO al final)"
"$DC_PYTHON" "$FIX" --build > "$SAL/sano-fix.log" 2>&1
nota "$(grep -a 'interior:' "$SAL/sano-fix.log" | tail -1)"
nota "$(grep -a 'libgodc.a:' "$SAL/sano-fix.log" | tail -1)"

if grep -qa 'interior: APLICADO' "$SAL/sano-fix.log"; then
    verde "control 3: el runtime quedo con el arreglo puesto otra vez"
else
    rojo "control 3: el runtime NO quedo con el arreglo (ver $SAL/sano-fix.log)"
fi

if correr sano2; then
    L2="$SAL/sano2/flycast.log"
    for caso in A B C; do
        V="$(veredicto "$L2" "$caso")"
        if [ "$V" = "OK" ]; then
            verde "control 3b: el control sano final, caso $caso OK"
        else
            rojo "control 3b: el control sano final, caso $caso sale '${V:-sin veredicto}'"
        fi
    done
else
    rojo "la sonda no se pudo armar en el control sano final (ver $SAL/sano2/build.log)"
fi

# ================================================================================================
echo
echo "== RESUMEN: las cadenas interiores (A y B) y el control sano (C)"
echo "   con el arreglo:  A=$(veredicto "$SAL/sano/flycast.log" A) B=$(veredicto "$SAL/sano/flycast.log" B) C=$(veredicto "$SAL/sano/flycast.log" C)   (huella $(huella "$SAL/sano/flycast.log" ANTES))"
echo "   con el mutante:  A=$(veredicto "$SAL/mutante/flycast.log" A) B=$(veredicto "$SAL/mutante/flycast.log" B) C=$(veredicto "$SAL/mutante/flycast.log" C)   ($(huella "$SAL/mutante/flycast.log" ANTES) -> $(huella "$SAL/mutante/flycast.log" DESPUES))"
echo "   y al final:      A=$(veredicto "$SAL/sano2/flycast.log" A) B=$(veredicto "$SAL/sano2/flycast.log" B) C=$(veredicto "$SAL/sano2/flycast.log" C)   (huella $(huella "$SAL/sano2/flycast.log" ANTES))"
echo "   la sonda:        $FUENTE/main.go"
echo "   el arreglo:      $FIX"
echo "   logs:            $SAL/sano/flycast.log  $SAL/mutante/flycast.log  $SAL/sano2/flycast.log"
echo
echo "== RESULTADO: $OK controles OK, $FALLOS con fallo"
[ "$FALLOS" -eq 0 ]
