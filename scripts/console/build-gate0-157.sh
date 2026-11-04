#!/bin/bash
# DC-002.157: EL BUILD DEL GATE 0 CON LAS PALANCAS DEL CUADRO -- el mismo Gate 0 de DC-002.156, con el mapa
# estatico, la conversion por zona y la presentacion quieta adentro.
#
# POR QUE EXISTE. DC-002.156 midio el cuadro del pueblo en Flycast: 1.869 / 1.885 / 1.883 ms, con 881 ms en el
# dibujado del motor y 58 ms en la conversion. El 95 % de eso es el mapa, y el mapa del pueblo NO CAMBIA
# cuando la camara no se movio. Esta tarjeta trae cuatro palancas (una en el renderer del motor y tres en la
# presentacion del PVR) y una corrida que las prende de a una para medir cuanto paga cada una.
#
# ES EL MISMO BUILD QUE `build-gate0-156.sh` (misma lista blanca, mismo alcance, mismo enlace): lo unico que
# cambia es el arbol de .gox del Gate 0 (`/tmp/dc157/gox`) para no pisar el de 156, y el directorio de salida.
# El binario que sale de aca es el de la tarjeta, y el que sale de 156 queda como estaba para comparar.
#
# ANTES DE CORRER ESTO, si se toco el shim del PVR (`console/dc/engine/ebiten/v2`) o el motor:
#   bash scripts/console/recompilar-motor-155.sh /tmp/dc155/gox \
#        console/dc/engine/ebiten/v2:github.com/hajimehoshi/ebiten/v2 d2core/d2render/ebiten \
#        d2core/d2map/d2maprenderer
#
# Uso:  scripts/console/build-gate0-157.sh [dir_salida] [dir_gox_motor] [dir_gox_gate0]
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"
# shellcheck source=palancas-del-build.sh
. "$HERE/palancas-del-build.sh"

REPO="$(cd "$HERE/../.." && pwd)"
SAL="${1:-/tmp/dcbuild/gate0-pueblo-157}"
GOX="${2:-${GODC_GOX:-/tmp/dc155/gox}}"
GEN="${3:-/tmp/dc157/gox}"
PREFIJO="github.com/OpenDiablo2/OpenDiablo2"

[ -f "$GOX/orden-motor.txt" ] || {
    echo "falta $GOX/orden-motor.txt (el arbol del motor completo): corre build-motor-all.sh" >&2
    exit 1
}

# el clon: solo se borra un arbol que se llama gox y vive en un directorio dc157 (a mano no se toca nada)
case "$GEN" in
    */dc157/gox|*/dc157*/gox) rm -rf "$GEN" ;;
    *)
        echo "no borro $GEN: el arbol del Gate 0 tiene que ser .../dc157.../gox" >&2
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
# OJO CON EL ORDEN, MEDIDO: primero la PANTALLA y despues `d2app` (ver la cabecera de build-gate0-156.sh: al
# reves, el enlace del main muere con `undefined reference to ...d2gamescreen..import`).
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

# ---- 4. la imagen cargada, contra el objetivo de la tarjeta y contra el Gate 0 de 156 ----
echo
echo "== 4/4 la imagen (lo que la tarjeta mide)"
IMAGEN=$(stat -c%s "$SAL/1ST_READ.BIN")
BSS=$(sh-elf-size -A "$SAL/prog.elf" | awk '$1 == ".bss" { print $2 }')
CARGADA=$((IMAGEN + BSS))
CARGADA_155=11155164
CARGADA_156=5901892
OBJETIVO=8960804
printf '   1ST_READ.BIN          %9d B\n' "$IMAGEN"
printf '   .bss                  %9d B\n' "$BSS"
printf '   imagen cargada        %9d B   (DC-002.156: %d B; DC-002.155: %d B; objetivo <= %d B)\n' \
    "$CARGADA" "$CARGADA_156" "$CARGADA_155" "$OBJETIVO"
printf '   diferencia contra 156 %+9d B\n' "$((CARGADA - CARGADA_156))"
if [ "$CARGADA" -le "$OBJETIVO" ]; then
    echo "   CUMPLE el objetivo de la tarjeta"
else
    echo "   NO CUMPLE: faltan $((CARGADA - OBJETIVO)) B para el objetivo" >&2
fi
echo "   sha256 del ELF: $(sha256sum "$SAL/prog.elf" | cut -d' ' -f1)"
echo
echo "== listo: $SAL/disc.gdi (Flycast) y $SAL/prog-emu.elf (ciclo rapido)"
