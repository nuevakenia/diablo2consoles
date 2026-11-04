#!/bin/bash
# DC-002.146d -- EL RECOLECTOR QUE BARRE UN TIPO CON `__ptrdata` BASURA: el comando que da VERDE con el arreglo y
# ROJO con el mutante.
#
# La sonda es `console/dc/poc/gc-tipo-invalido` (90 lineas de Go, sin el motor) y el arreglo es
# `scripts/console/fix-libgodc-tipos-reales.py`. El caso que arma la sonda es el del defecto medido en DC-002.145:
# un "descriptor de tipo" que SI cae en la imagen (es un global de Go, o sea .bss) con `__ptrdata` = una direccion
# de la imagen y `__gcdata` = 0x36040408 (fuera de RAM). El objeto ANCLA es real y su encabezado pasa a nombrarlo;
# el objeto CONTROL queda sano para medir que la tabla no rompe lo que anda.
#
#   checar-gc-tipo-invalido-146d.sh mutante     # el runtime de DC-002.92 -> tiene que dar ROJO (el defecto)
#   checar-gc-tipo-invalido-146d.sh arreglo     # el runtime con la tabla de tipos -> tiene que dar VERDE
#   checar-gc-tipo-invalido-146d.sh veredicto <log>   # solo lee un log ya corrido
#
# VERDE = el tipo de basura NO se barre: al menos una `gc-tipo-invalido`, CERO `gc-ptrdata-recortado`, CERO
#         `gc-gcdata-fuera-imagen`, y el objeto de CONTROL entero (el barrido sano sigue andando).
# ROJO  = el recolector leyo el `__ptrdata` de basura: sale `gc-ptrdata-recortado` con una direccion (0x8c0.....)
#         y/o `gc-gcdata-fuera-imagen` con un valor fuera de RAM.
#
# OJO: este script toca `~/dreamcast/libgodc` (el toolchain del host) porque es el que usan los builds del motor; el
# fix-libgodc-tipos-reales.py es idempotente y `--quitar` deja el arbol como estaba (el arreglo de DC-002.92). Si se
# quiere medir en una copia, pasar LIB146D=<otro arbol>.
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)"
MODO="${1:?uso: checar-gc-tipo-invalido-146d.sh <mutante|arreglo|veredicto> [log]}"
LIB146D="${LIB146D:-$HOME/dreamcast/libgodc}"
POC="$REPO/console/dc/poc/gc-tipo-invalido"
GOX="${GODC_GOX:-/tmp/gox-med}"       # el arbol de la stdlib (runtime.gox): la sonda importa runtime
SECS="${SECS:-20}"

# veredicto <log>: el numero que manda
veredicto() {
    local L="$1" inv rec gc fuera reg control
    [ -f "$L" ] || { echo "no existe el log $L"; return 2; }
    inv=$(tr -d '\r' < "$L" | grep -acF "ARQ-MARCA gc-tipo-invalido")
    rec=$(tr -d '\r' < "$L" | grep -acF "ARQ-MARCA gc-ptrdata-recortado")
    gc=$(tr -d '\r' < "$L" | grep -acF "ARQ-MARCA gc-gcdata-fuera-imagen")
    fuera=$(tr -d '\r' < "$L" | grep -acF "ARQ-MARCA gc-tipo-fuera-imagen")
    reg=$(tr -d '\r' < "$L" | grep -aF "ARQ-MARCA gc-tipos-registrados" | head -1 | sed 's/.*v=//')
    control=$(tr -d '\r' < "$L" | grep -aF "SONDA-GC146D: ronda= 3" | head -1 | sed 's/.*control= //; s/ *ancla_n=.*//; s/^ *//; s/ *$//')
    echo "   tipos declarados invalidos por el recolector (gc-tipo-invalido): $inv"
    echo "   barridos con __ptrdata basura (gc-ptrdata-recortado): $rec"
    echo "   gcdata fuera de la imagen (gc-gcdata-fuera-imagen): $gc"
    echo "   tipos fuera de la imagen (gc-tipo-fuera-imagen, el chequeo de DC-002.92): $fuera"
    echo "   tipos registrados por el asignador (gc-tipos-registrados): ${reg:-NO SALE (no hay tabla: mutante)}"
    echo "   objeto de CONTROL en la ronda 3: ${control:-no llego}"
    if [ "$rec" -gt 0 ] || [ "$gc" -gt 0 ]; then
        echo "   ROJO: el recolector barrio el tipo de basura (leyo __ptrdata/__gcdata de un descriptor que no lo es)"
        return 1
    fi
    if [ "$inv" -gt 0 ] && [ "${control:-}" = "entero" ]; then
        echo "   VERDE: el tipo invalido se reporta y NO se barre; el objeto sano sigue entero"
        return 0
    fi
    if [ "$inv" -gt 0 ]; then
        echo "   AMARILLO: el tipo se reporta pero el CONTROL no salio entero (¿se corrio el binario entero?)"
        return 2
    fi
    echo "   SIN VEREDICTO: ni una cosa ni la otra (¿llego a main? ¿se corrio el binario?)"
    return 2
}

if [ "$MODO" = "veredicto" ]; then
    veredicto "${2:?falta el log}"
    exit $?
fi

case "$MODO" in
    mutante) QUITAR="--quitar" ;;
    arreglo) QUITAR="" ;;
    *) echo "modo desconocido: $MODO (mutante|arreglo|veredicto)" >&2; exit 2 ;;
esac

OUT="${OUT:-/tmp/dcbuild/gc146d-$MODO}"
echo "== 1/4 el runtime ($MODO) en $LIB146D"
python3 "$HERE/fix-libgodc-tipos-reales.py" "$LIB146D" $QUITAR --build > "$OUT.toolchain.log" 2>&1 || {
    echo "   FALLO (ver $OUT.toolchain.log)"; tail -5 "$OUT.toolchain.log"; exit 1; }
grep -a "arreglo 146d:" "$OUT.toolchain.log"

echo "== 2/4 el ELF + la imagen de la sonda"
DC_ROOT="$(cd "$LIB146D/.." && pwd)" GODC_GOX="$GOX" bash "$HERE/build-dc.sh" "$POC" "$OUT" > "$OUT.build.log" 2>&1 || {
    echo "   FALLO el build (ver $OUT.build.log)"; tail -10 "$OUT.build.log"; exit 1; }
printf '   1ST_READ.BIN %s B   prog.elf %s B\n' "$(stat -c%s "$OUT/1ST_READ.BIN")" "$(stat -c%s "$OUT/prog.elf")"

echo "== 3/4 la corrida (${SECS}s, sin capturas: la sonda no dibuja)"
CAPTURE=none bash "$HERE/run-flycast.sh" "$OUT/prog.elf" "$SECS" "$OUT/run" > "$OUT.run.log" 2>&1

echo "== 4/4 el veredicto"
tr -d '\r' < "$OUT/run/flycast.log" | grep -a "SONDA-GC146D:" | head -20
veredicto "$OUT/run/flycast.log"
