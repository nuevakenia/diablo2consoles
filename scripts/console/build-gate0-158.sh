#!/bin/bash
# DC-002.158: EL BUILD DEL GATE 0 CON EL RELOJ DE LA CONSOLA ARREGLADO.
#
# POR QUE EXISTE. DC-002.157 encontro que `d2util.Now()` adelantaba 2x en la consola y dejo el cuadro del
# pueblo medido con el reloj del PVR (que da bien). Esta tarjeta arregla el reloj (`console/dc/std/time/
# time.go`) y el numero que hay que volver a ver es el `CUADRO n ms=` de la pantalla, que es el UNICO del
# proyecto medido con `d2util.Now()`: tiene que pasar de 1.781 ms a ~880 ms en el mismo cuadro (el 11).
#
# ES EL MISMO BUILD QUE `build-gate0-157.sh` (misma lista blanca, mismo alcance, mismo enlace, mismo plan de
# siete tramos en caliente -- todo eso es del motor y no cambia): lo unico distinto son los directorios de
# salida y el paso 0, y ese paso NO es un tramite: el `orden.txt` del arbol de .gox apunta por ruta absoluta al
# arbol ORIGINAL (/tmp/dc155/gox), asi que sin refrescar el clon Y reapuntar el listado el binario sale con el
# `time.gox` de las 13:38 y el `CUADRO n ms=` sigue diciendo 1.781 ms (medido: paso tal cual en la primera
# corrida de este script).
#
# ANTES DE CORRER ESTO:
#   bash scripts/console/build-std-gox.sh                     # la stdlib con el reloj arreglado
#   (el motor no se recompila: no se toco ni un archivo de d2core/d2game)
#
# Uso:  scripts/console/build-gate0-158.sh [dir_salida] [dir_gox_motor] [dir_gox_gate0]
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
. "$HERE/env.sh"
# shellcheck source=palancas-del-build.sh
. "$HERE/palancas-del-build.sh"

REPO="$(cd "$HERE/../.." && pwd)"
SAL="${1:-/tmp/dcbuild/gate0-pueblo-158}"
GOX="${2:-${GODC_GOX:-/tmp/dc155/gox}}"
GEN="${3:-/tmp/dc158/gox}"
PREFIJO="github.com/OpenDiablo2/OpenDiablo2"

[ -f "$GOX/orden-motor.txt" ] || {
    echo "falta $GOX/orden-motor.txt (el arbol del motor completo): corre build-motor-all.sh" >&2
    exit 1
}

# el clon: solo se borra un arbol que se llama gox y vive en un directorio dc158 (a mano no se toca nada)
case "$GEN" in
    */dc158/gox|*/dc158*/gox) rm -rf "$GEN" ;;
    *)
        echo "no borro $GEN: el arbol del Gate 0 tiene que ser .../dc158.../gox" >&2
        exit 1
        ;;
esac
mkdir -p "$(dirname "$GEN")"
cp -al "$GOX" "$GEN"
echo "== arbol de .gox del Gate 0: $GEN (clon por hardlinks de $GOX)"

# ---- 0. LA STDLIB: HAY QUE REFRESCARLA EN EL CLON *Y* REAPUNTAR orden.txt ----
#
# OJO, MEDIDO EN ESTA TARJETA (dos corridas): el `orden.txt` del arbol de .gox NO apunta a $DC_ROOT/gox, apunta
# al arbol ORIGINAL del motor (/tmp/dc155/gox/....gox, rutas absolutas grabadas cuando se armo ese arbol). O
# sea que el enlace toma la stdlib de /tmp/dc155/gox y NADA de lo que se haga en el clon entra al binario: dos
# builds de esta tarjeta, uno refrescando el clon y otro sin refrescar, dieron el MISMO sha256 de ELF
# (40784853...) y el mismo `CUADRO 11 ms= 1781` -- el reloj del time.gox de las 13:38.
#
# La prueba de que el arreglo ENTRO al binario no es el sha: es el simbolo `_time.baseUnix`, que solo tiene el
# time.gox nuevo (`sh-elf-nm <gox>/time.gox | grep baseUnix`). Se verifica al final, en el ELF (paso 4).
#
# Los dos pasos, y los dos son necesarios:
#   1. refrescar los .gox de la stdlib DENTRO del clon, desde $DC_ROOT/gox. Con rm + cp, NUNCA escribiendo
#      encima: el clon es por hardlinks y escribir encima truncaria el .gox de /tmp/dc155/gox, que es el
#      binario de referencia de DC-002.157.
#   2. reapuntar $GEN/orden.txt para que sus lineas apunten al clon y no a /tmp/dc155/gox.
std_gox="${GODC_GOX:-$DC_ROOT/gox}"
[ -f "$std_gox/orden.txt" ] || {
    echo "falta $std_gox/orden.txt (la stdlib minima): corre scripts/console/build-std-gox.sh" >&2
    exit 1
}
if [ ! "$std_gox/time.gox" -nt "$REPO/console/dc/std/time/time.go" ]; then
    echo "el time.gox de $std_gox es MAS VIEJO que console/dc/std/time/time.go." >&2
    echo "Corre scripts/console/build-std-gox.sh antes de este build." >&2
    exit 1
fi

refrescados=0
while IFS= read -r ruta; do
    [ -n "$ruta" ] || continue
    paq="${ruta#"$std_gox"/}"
    [ -f "$std_gox/$paq" ] || continue
    if ! cmp -s "$std_gox/$paq" "$GEN/$paq"; then
        rm -f "$GEN/$paq"
        mkdir -p "$GEN/$(dirname "$paq")"
        cp "$std_gox/$paq" "$GEN/$paq"
        refrescados=$((refrescados + 1))
    fi
done < "$std_gox/orden.txt"
echo "   stdlib: $refrescados .gox del clon refrescados desde $std_gox"

# reapunto del listado: de $GOX (el arbol original) al clon
sed "s|^$GOX/|$GEN/|" "$GEN/orden.txt" > "$GEN/orden.txt.clon"
if grep -qv "^$GEN/" "$GEN/orden.txt.clon"; then
    echo "FALLA: quedan lineas de orden.txt que no apuntan al clon ($GEN):" >&2
    grep -v "^$GEN/" "$GEN/orden.txt.clon" | head -5 >&2
    exit 1
fi
mv "$GEN/orden.txt.clon" "$GEN/orden.txt"
echo "   orden.txt: $(wc -l < "$GEN/orden.txt") paquetes reapuntados a $GEN (antes: $GOX)"

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

# ---- 4. la imagen cargada, contra el objetivo de la tarjeta y contra el Gate 0 de 157 ----
echo
echo "== 4/4 la imagen (lo que la tarjeta mide)"
IMAGEN=$(stat -c%s "$SAL/1ST_READ.BIN")
BSS=$(sh-elf-size -A "$SAL/prog.elf" | awk '$1 == ".bss" { print $2 }')
CARGADA=$((IMAGEN + BSS))
CARGADA_155=11155164
CARGADA_156=5901892
CARGADA_157=5910180
OBJETIVO=8960804
printf '   1ST_READ.BIN          %9d B\n' "$IMAGEN"
printf '   .bss                  %9d B\n' "$BSS"
printf '   imagen cargada        %9d B   (DC-002.157: %d B; 156: %d; 155: %d; objetivo <= %d B)\n' \
    "$CARGADA" "$CARGADA_157" "$CARGADA_156" "$CARGADA_155" "$OBJETIVO"
printf '   diferencia contra 157 %+9d B\n' "$((CARGADA - CARGADA_157))"
if [ "$CARGADA" -le "$OBJETIVO" ]; then
    echo "   CUMPLE el objetivo de la tarjeta"
else
    echo "   NO CUMPLE: faltan $((CARGADA - OBJETIVO)) B para el objetivo" >&2
fi
echo "   sha256 del ELF: $(sha256sum "$SAL/prog.elf" | cut -d' ' -f1)"
# LA PRUEBA DE QUE EL ARREGLO ESTA ADENTRO. El sha no sirve (dos builds con stdlib distinta dieron el mismo):
# lo que sirve es el simbolo `_time.baseUnix`, que solo existe en el time.gox con el reloj arreglado.
# OJO CON `grep -q` ACA: con `set -o pipefail`, el `grep -q` corta el pipe al primer match, `sh-elf-nm` muere
# por SIGPIPE y el pipe queda en 141 -- el "no aparece" era mentira y el guard daba FALLA con el simbolo
# presente (medido: paso en la primera corrida con el arreglo puesto). Se cuenta con -c y se lee el numero.
simbolos_reloj="$(sh-elf-nm "$SAL/prog.elf" | grep -c "_time\.baseUnix" || true)"
if [ "${simbolos_reloj:-0}" -lt 1 ]; then
    echo "FALLA: el ELF no trae el reloj arreglado (no aparece _time.baseUnix): enlazo un time.gox viejo." >&2
    exit 1
fi
echo "   reloj: _time.baseUnix presente en el ELF (el time.gox de la stdlib arreglada entro al enlace)"
echo
echo "== listo: $SAL/disc.gdi (Flycast) y $SAL/prog-emu.elf (ciclo rapido)"
echo "   bash scripts/console/run-flycast.sh $SAL/disc.gdi 185 /tmp/capturas158"
