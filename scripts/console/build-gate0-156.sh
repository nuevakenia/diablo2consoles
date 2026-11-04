#!/bin/bash
# DC-002.156: EL BUILD DEL GATE 0 CON EL ARRANQUE DE CONSOLA -- la imagen sin lo que el pueblo no usa.
#
# POR QUE EXISTE (el numero de DC-002.155, que es el que manda aca). Con el binario del motor ENTERO
# (`build-gate0-155.sh`, 145 paquetes) el pueblo compone hasta el arena de bloques de los `.dt1` y muere:
# le faltan **2.188.360 B**. La causa es la imagen (11.155.164 B cargados contra los 6.320.128 B del POC que
# SI compone el pueblo), no las tablas ni el semiespacio.
#
# QUE HACE DISTINTO ESTE SCRIPT. Arma un arbol de .gox PROPIO (`$GEN`, clonado por hardlinks del arbol del
# motor completo) y en ese arbol:
#
#   1. compila `d2app` y `d2game/d2gamescreen` con la LISTA BLANCA del arranque de consola
#      (`DC_GATE0=1` en `palancas-del-build.sh`): el arranque de consola de DC-002.156 y la pantalla del
#      pueblo. El resto del paquete (el camino de la PC, el heroe, la red, las 107 pantallas) NO se compila.
#   2. escribe la lista de enlace por ALCANCE (`alcance-156.py`): los paquetes que se alcanzan desde el
#      main, `d2app` y la pantalla del pueblo. Lo que no se alcanza no entra a la imagen.
#
# EL ARBOL ORIGINAL NO SE TOCA: el clon es por hardlinks y cada .gox que se recompila se BORRA antes
# (`rm -f`), asi que el inodo compartido no se trunca y `/tmp/dc155/gox` (el del motor completo, el que usan
# el POC jugable y los demas POC) queda como estaba.
#
# Uso:  scripts/console/build-gate0-156.sh [dir_salida] [dir_gox_motor] [dir_gox_gate0]
#       Antes: bash scripts/console/build-std-gox.sh /tmp/dc155/gox
#              bash scripts/console/build-motor-all.sh /tmp/dc155/gox /tmp/dc155/log-motor
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"
# shellcheck source=palancas-del-build.sh
. "$HERE/palancas-del-build.sh"

REPO="$(cd "$HERE/../.." && pwd)"
SAL="${1:-/tmp/dcbuild/gate0-pueblo-156}"
GOX="${2:-${GODC_GOX:-/tmp/dc155/gox}}"
GEN="${3:-/tmp/dc156/gox}"
PREFIJO="github.com/OpenDiablo2/OpenDiablo2"

[ -f "$GOX/orden-motor.txt" ] || {
    echo "falta $GOX/orden-motor.txt (el arbol del motor completo): corre build-motor-all.sh" >&2
    exit 1
}

# el clon: solo se borra un arbol que se llama gox y vive en un directorio dc156 (a mano no se toca nada)
case "$GEN" in
    */dc156/gox|*/dc156*/gox) rm -rf "$GEN" ;;
    *)
        echo "no borro $GEN: el arbol del Gate 0 tiene que ser .../dc156.../gox" >&2
        exit 1
        ;;
esac
mkdir -p "$(dirname "$GEN")"
cp -al "$GOX" "$GEN"
echo "== arbol de .gox del Gate 0: $GEN (clon por hardlinks de $GOX)"

# ---- 1. los dos paquetes con LISTA BLANCA (DC_GATE0=1) ----
export DC_GATE0=1
echo "== $(palancas_del_build)"

compilar_uno() {  # <paquete>
    local paq="$1" out="$GEN/$PREFIJO/$1.gox"
    local -a srcs=()
    mapfile -t srcs < <(fuentes_del_paquete "$REPO" "$paq")
    if [ "${#srcs[@]}" -eq 0 ]; then
        echo "FALLA: $paq se quedo sin fuentes con la lista blanca del Gate 0" >&2
        exit 1
    fi
    # OJO: borrar ANTES de compilar. El clon es por hardlinks: escribir encima truncaria el .gox del arbol
    # original, y el POC jugable se quedaria sin el paquete real.
    rm -f "$out"
    if ! sh-elf-gccgo $GODC_GOFLAGS -I "$GEN" -fgo-pkgpath="$PREFIJO/$paq" -c "${srcs[@]}" -o "$out" \
        > "$SAL/compila-${paq//\//_}.log" 2>&1; then
        echo "FALLA compilando $paq (log en $SAL/compila-${paq//\//_}.log):" >&2
        head -20 "$SAL/compila-${paq//\//_}.log" >&2
        exit 1
    fi
    printf '  %-40s %9d B  (%d archivo(s): %s)\n' "$paq" "$(stat -c%s "$out")" "${#srcs[@]}" \
        "$(for f in "${srcs[@]}"; do basename "$f"; done | tr '\n' ' ')"
}

mkdir -p "$SAL"
echo "== 1/4 compilando el arranque de consola y la pantalla del pueblo (lista blanca)"
# OJO CON EL ORDEN, MEDIDO: primero la PANTALLA y despues `d2app`. El .gox que se recompila se escribe en el
# arbol, pero el que lo IMPORTA graba en su propia data de exportacion si ese paquete tiene init o no. Con
# `d2app` primero, compilaba contra la pantalla VIEJA (la hardlink del arbol completo, que trae init porque
# tiene variables de paquete) y el enlace del main moria despues con
#   main.o: in function `__go_init_main': undefined reference to `...d2gamescreen..import'
# porque la pantalla nueva (dos archivos, sin variables de paquete) no define ese simbolo.
compilar_uno d2game/d2gamescreen
compilar_uno d2app

# ---- 2. la lista de enlace por alcance ----
echo "== 2/4 la lista de enlace: solo lo que el pueblo ALCANZA"
"$DC_PYTHON" "$HERE/alcance-156.py" \
    d2app d2core/d2render/pvr \
    --solo-archivos "d2app=$(printf '%s,' "${PALANCA_GATE0_APP[@]}")" \
    --solo-archivos "d2game/d2gamescreen=$(printf '%s,' "${PALANCA_GATE0_PANTALLAS[@]}")" \
    --gox "$GEN" \
    --lista "$GEN/orden-motor-gate0.txt" > "$SAL/alcance.txt" 2>&1 || {
        echo "FALLA el analisis de alcance (ver $SAL/alcance.txt)" >&2
        exit 1
    }
sed 's/^/   /' "$SAL/alcance.txt" | head -14
cp "$GEN/orden-motor.txt" "$SAL/orden-motor-completo.txt"
mv "$GEN/orden-motor-gate0.txt" "$GEN/orden-motor.txt"
echo "   enlace: $(wc -l < "$GEN/orden-motor.txt") paquetes (el motor completo son $(wc -l < "$SAL/orden-motor-completo.txt"))"

# ---- 3. el binario (el mismo camino de DC-002.155: MPQ del disco, .eh_frame fuera, malloc interpuesto) ----
echo "== 3/4 el binario del Gate 0"
JUEGO="$SAL/juego"
mkdir -p "$JUEGO"
D2="${D2_DIR:-$HOME/.wine-d2/drive_c/Games/Diablo II}"
for mpq in d2data.mpq patch_D2.mpq; do
    [ -f "$D2/$mpq" ] || { echo "falta $D2/$mpq" >&2; exit 1; }
    ln -sf "$D2/$mpq" "$JUEGO/$mpq"
done

POC=gate0-pueblo \
DC_RUTA_DATOS=/cd/juego \
DC_MPQ_JUEGO="d2data.mpq,patch_d2.mpq" \
DC_SIN_SELLOS=1 \
DC_DISCO_JUEGO="$JUEGO" \
DC_EH_FINAL=1 \
DC_LDEXTRA="-Wl,--wrap=malloc" \
    bash "$HERE/build-motor-arranque.sh" "$SAL" "$GEN"

# ---- 4. la imagen cargada, contra el objetivo de la tarjeta ----
echo
echo "== 4/4 la imagen (lo que la tarjeta mide)"
IMAGEN=$(stat -c%s "$SAL/1ST_READ.BIN")
BSS=$(sh-elf-size -A "$SAL/prog.elf" | awk '$1 == ".bss" { print $2 }')
CARGADA=$((IMAGEN + BSS))
CARGADA_155=11155164
OBJETIVO=8960804
printf '   1ST_READ.BIN          %9d B\n' "$IMAGEN"
printf '   .bss                  %9d B\n' "$BSS"
printf '   imagen cargada        %9d B   (DC-002.155: %d B; objetivo <= %d B)\n' "$CARGADA" "$CARGADA_155" "$OBJETIVO"
printf '   diferencia            %9d B menos\n' "$((CARGADA_155 - CARGADA))"
if [ "$CARGADA" -le "$OBJETIVO" ]; then
    echo "   CUMPLE el objetivo de la tarjeta"
else
    echo "   NO CUMPLE: faltan $((CARGADA - OBJETIVO)) B para el objetivo" >&2
fi
echo
echo "== listo: $SAL/disc.gdi (Flycast) y $SAL/prog-emu.elf (ciclo rapido)"
