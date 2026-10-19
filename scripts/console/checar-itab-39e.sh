#!/bin/bash
# DC-002.39e: EL CORPUS DEL CACHE DE ITABS -- el defecto y su arreglo, medidos en la consola.
#
# QUE MIDE
#   El POC `itab-raiz` (console/dc/poc/itab-raiz) arma tres cosas por vuelta, con recolecciones forzadas:
#     V1  la variable de interfaz de la PILA (control): el barrido del stack mantiene su itab
#     V2  la interfaz que sale del CACHE del runtime y queda VIVA en un slice a traves de la recoleccion
#     V4  el CONTROL de V2: la misma forma, pero con itab ESTATICO de .rodata (conversion concreto->interfaz)
#   V4 aisla la causa: si V4 sobrevive y V2 no, lo que rompe no es "una interfaz viva en un slice" (que es el
#   patron del motor en `bytes.Buffer.ReadFrom`), es el ITAB DEL CACHE.
#
# LOS DOS ESTADOS (el defecto y el arreglo)
#   sano  (el DEFECTO, y tambien el MUTANTE): `fix-libgodc-itab.py --sin-raices` -- solo cuenta, no arregla
#   fix   (el ARREGLO):                         `fix-libgodc-itab.py`              -- registra las ranuras del
#         cache como raices del GC (`gc_add_root`), asi el recolector las actualiza al copiar el itab
#
# USO
#   bash scripts/console/checar-itab-39e.sh                 # con los arboles ya preparados
#   bash scripts/console/checar-itab-39e.sh --preparar      # ademas los arma (cp -a + cadena + make)
#   GODC_GOX=/tmp/gox-head bash scripts/console/checar-itab-39e.sh --preparar
#
# SALIDA: verde si el arreglo imprime el TOTAL con v2mal= 0 y el defecto NO lo imprime (se reinicia).
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)"
SAL="${SAL:-/home/dgalli/.hermes/cache/scratch/itab-39e}"
DCROOT_SANO="${DCROOT_SANO:-$SAL/dcroot-sano}"
DCROOT_FIX="${DCROOT_FIX:-$SAL/dcroot-fix}"
GOX="${GODC_GOX:-/tmp/gox-head}"
SEGS="${SEGS:-25}"
PREPARAR=0
[ "${1:-}" = "--preparar" ] && PREPARAR=1

POC="$REPO/console/dc/poc/itab-raiz"

verde() { echo "   OK: $*"; }
rojo()  { echo "   FALLO: $*"; }
nota()  { echo "   $*"; }

campo() { # $1 = log, $2 = nombre del campo (de la ULTIMA linea que lo traiga)
    tr -d '\r' < "$1" | grep -a 'ITAB-RAIZ' | grep -a "$2=" | tail -1 |
        sed -n "s/.*$2= *\([0-9]*\).*/\1/p"
}

# ---------------------------------------------------------------- preparar los dos arboles
preparar() { # $1 = dir, $2 = (arreglo|mutante)
    local dir="$1" modo="$2"
    mkdir -p "$dir"
    if [ ! -d "$dir/libgodc" ]; then
        cp -a /home/dgalli/dreamcast/libgodc "$dir/libgodc"
        for d in kos kos-ports sh-elf; do ln -sfn "/home/dgalli/dreamcast/$d" "$dir/$d"; done
    fi

    # la cadena completa primero (deja el runtime en el estado sano conocido, YA con el arreglo), DESPUES el
    # paso que decide el estado: sin `--sin-raices` queda el arreglo; con el, el mutante.
    bash "$HERE/fix-libgodc-cadena.sh" "$dir/libgodc" --build > "$dir/cadena.log" 2>&1
    if [ "$modo" = "mutante" ]; then
        python3 "$HERE/fix-libgodc-itab.py" "$dir/libgodc" --sin-raices > "$dir/itab.log" 2>&1
        ( . "$HERE/env.sh" && cd "$dir/libgodc" && make libgodc.a ) > "$dir/make.log" 2>&1
    fi

    echo "   $dir ($modo): libgodc.a $(stat -c%s "$dir/libgodc/libgodc.a") B sha256 $(sha256sum "$dir/libgodc/libgodc.a" | cut -c1-16)"
}

if [ "$PREPARAR" = 1 ]; then
    echo "== 0. los dos arboles del runtime"
    mkdir -p "$SAL"
    [ -d "$DCROOT_SANO/libgodc" ] || preparar "$DCROOT_SANO" mutante
    [ -d "$DCROOT_FIX/libgodc" ] || preparar "$DCROOT_FIX" arreglo
fi

correr() { # $1 = etiqueta, $2 = dcroot
    local etq="$1" root="$2" d="$SAL/$1"
    rm -rf "$d"; mkdir -p "$d"

    DC_ROOT="$root" GODC_GOX="$GOX" bash "$HERE/build-dc.sh" "$POC" "$d/build" > "$d/build.log" 2>&1
    if [ ! -f "$d/build/prog.elf" ]; then
        echo "   BUILD FALLO: ver $d/build.log"; return 1
    fi

    CAPTURE=none bash "$HERE/run-flycast.sh" "$d/build/prog.elf" "$SEGS" "$d" > "$d/run.log" 2>&1
    tr -d '\r' < "$d/flycast.log" | grep -a 'ITAB-RAIZ' > "$d/resumen.txt"

    return 0
}

echo "== 1. el runtime que se mide"
nota "gox= $GOX ($(wc -l < "$GOX/orden.txt" 2>/dev/null || echo 0) paquetes)"
for r in "$DCROOT_SANO" "$DCROOT_FIX"; do
    nota "$r: libgodc.a $(stat -c%s "$r/libgodc/libgodc.a" 2>/dev/null || echo SIN-ARMA) B sha256 $(sha256sum "$r/libgodc/libgodc.a" 2>/dev/null | cut -c1-16)"
done

echo "== 2. el DEFECTO (el cache sin registrar como raiz) -- se espera ROJO"
correr sano "$DCROOT_SANO" || exit 1
LS="$SAL/sano/flycast.log"
AR_S="$(grep -ac 'ITAB-RAIZ arranca' "$LS" || true)"
TOT_S="$(grep -ac 'ITAB-RAIZ-TOTAL' "$LS" || true)"
nota "arranques= $AR_S  (mas de 1 = la consola se REINICIA sola)"
nota "lineas de progreso v=: $(grep -ac 'ITAB-RAIZ v=' "$LS" || true)"
nota "TOTAL impreso= $TOT_S"
if [ "$TOT_S" = "0" ] && [ "${AR_S:-0}" -gt 1 ]; then
    verde "el DEFECTO esta medido: la corrida no llega al final, la consola reinicia (arranques= $AR_S)"
    SANO_ROJO=1
else
    rojo "el defecto NO se reproduce en esta corrida (arranques= $AR_S, TOTAL= $TOT_S)"
    SANO_ROJO=0
fi

echo "== 3. el ARREGLO (las ranuras del cache como raices del GC) -- se espera VERDE"
correr fix "$DCROOT_FIX" || exit 1
LF="$SAL/fix/flycast.log"
AR_F="$(grep -ac 'ITAB-RAIZ arranca' "$LF" || true)"
TOT_F="$(grep -ac 'ITAB-RAIZ-TOTAL' "$LF" || true)"
V2B="$(campo "$LF" v2bien)"; V2M="$(campo "$LF" v2mal)"
V4B="$(campo "$LF" v4bien)"; V4M="$(campo "$LF" v4mal)"
ARM="$(campo "$LF" itabArmados)"
nota "arranques= $AR_F  TOTAL impreso= $TOT_F"
nota "v2bien= $V2B v2mal= $V2M (el cache, vivo a traves de las recolecciones)"
nota "v4bien= $V4B v4mal= $V4M (CONTROL: itab estatico de .rodata)"
nota "itabArmados= $ARM (cuantos itabs se reservaron en el monton de Go)"
if [ "$TOT_F" = "1" ] && [ "${V2M:-1}" = "0" ] && [ "${V4M:-1}" = "0" ] && [ "${AR_F:-0}" = "1" ]; then
    verde "el ARREGLO cierra: v2mal= 0 con $V2B comprobaciones despues de la recoleccion, 1 arranque, sin reinicio"
    FIX_VERDE=1
else
    rojo "el arreglo NO cierra (TOTAL= $TOT_F v2mal= ${V2M:-?} arranques= $AR_F)"
    FIX_VERDE=0
fi

veredicto_salida() {
    echo "== VEREDICTO: el defecto esta medido (ROJO) y el arreglo tambien (VERDE). El MUTANTE es el estado"
    echo "   'mutante' de arriba: el mismo runtime con 'fix-libgodc-itab.py --sin-raices' (las ranuras del"
    echo "   cache SIN registrar como raices del GC) deja la corrida sin TOTAL y con reinicio."
}

echo
if [ "${SANO_ROJO:-0}" = 1 ] && [ "${FIX_VERDE:-0}" = 1 ]; then
    veredicto_salida
    exit 0
fi

echo "== VEREDICTO: la medicion NO cierra (rojo=$SANO_ROJO verde=$FIX_VERDE)"
exit 1
