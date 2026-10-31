#!/bin/bash
# DC-002.92 -- EL BARRIDO DEL GC FUERA DE RAM: el comando que da VERDE con el arreglo y ROJO con el mutante.
#
# La sonda es `console/dc/poc/gc-fuera-ram` (20 lineas de Go, sin el motor) y el arreglo es
# `scripts/console/fix-libgodc-tipos.py`. Este script hace el ciclo entero de UNA direccion y da el veredicto;
# la otra direccion es el mismo comando con otro argumento.
#
#   checar-gc-fuera-ram-292.sh mutante      # el runtime SIN el arreglo -> tiene que dar ROJO
#   checar-gc-fuera-ram-292.sh arreglo      # el runtime CON el arreglo -> tiene que dar VERDE
#   checar-gc-fuera-ram-292.sh veredicto <log>   # solo lee un log ya corrido
#
# VERDE = el barrido del GC NO se sale de RAM: cero lineas GC-FUERA-RAM y al menos una gc-tipo-fuera-imagen
#         (el descriptor falso se rechaza por no estar en la imagen y el objeto se barre conservador y acotado).
# ROJO  = el barrido se va de RAM: sale GC-FUERA-RAM con la direccion donde terminaria (`-barra`), que es mayor
#         que el tope de RAM 0x8d000000.
#
# EL VENDOR. El arreglo se aplica en una COPIA del toolchain (`$LIB92`, por defecto /tmp/dc306/dcroot/libgodc),
# nunca en ~/dreamcast ni en el repo. El directorio tiene que existir ya (una copia de libgodc + enlaces a kos,
# kos-ports y sh-elf):
#
#   mkdir -p /tmp/dc306/dcroot && cd /tmp/dc306/dcroot
#   ln -s ~/dreamcast/kos kos; ln -s ~/dreamcast/kos-ports kos-ports; ln -s ~/dreamcast/sh-elf sh-elf
#   cp -a ~/dreamcast/libgodc libgodc
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)"
MODO="${1:?uso: checar-gc-fuera-ram-292.sh <mutante|arreglo|veredicto> [log]}"
LIB92="${LIB92:-/tmp/dc306/dcroot/libgodc}"
DCROOT="$(cd "$LIB92/.." && pwd)"
POC="$REPO/console/dc/poc/gc-fuera-ram"
GOX="${GODC_GOX:-/tmp/gox-med}"       # el arbol de la stdlib (runtime.gox): la sonda importa runtime
SECS="${SECS:-20}"

# veredicto <log>: el numero que manda
veredicto() {
    local L="$1" fuera tipo obj barra
    [ -f "$L" ] || { echo "no existe el log $L"; return 2; }
    fuera=$(tr -d '\r' < "$L" | grep -acF "ARQ-MARCA GC-FUERA-RAM-barra")
    tipo=$(tr -d '\r' < "$L" | grep -acF "ARQ-MARCA gc-tipo-fuera-imagen")
    obj=$(tr -d '\r' < "$L" | grep -aF "ARQ-MARCA GC-FUERA-RAM-obj" | head -1 | sed 's/.*v=//')
    barra=$(tr -d '\r' < "$L" | grep -aF "ARQ-MARCA GC-FUERA-RAM-barra" | head -1 | sed 's/.*v=//')
    echo "   barridos que se salian de RAM (GC-FUERA-RAM): $fuera"
    echo "   descriptores rechazados por no estar en la imagen (gc-tipo-fuera-imagen): $tipo"
    [ -n "${obj:-}" ] && echo "   primer objeto: $obj   barra (donde terminaria): $barra  (tope de RAM 2365587456 = 0x8d000000)"
    if [ "$fuera" -gt 0 ]; then
        echo "   ROJO: el barrido se sale de RAM"
        return 1
    fi
    if [ "$tipo" -gt 0 ]; then
        echo "   VERDE: el descriptor falso se rechaza y el barrido no sale de RAM"
        return 0
    fi
    echo "   SIN VEREDICTO: ni una cosa ni la otra (¿se corrio el binario? ¿llego a main?)"
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

OUT="${OUT:-/tmp/dcbuild/gc92-$MODO}"
echo "== 1/4 el runtime ($MODO) en $LIB92"
python3 "$HERE/fix-libgodc-tipos.py" "$LIB92" $QUITAR --build > "$OUT.toolchain.log" 2>&1 || {
    echo "   FALLO (ver $OUT.toolchain.log)"; tail -5 "$OUT.toolchain.log"; exit 1; }
grep -a "sensor:" "$OUT.toolchain.log"

echo "== 2/4 el ELF + la imagen de la sonda"
DC_ROOT="$DCROOT" GODC_GOX="$GOX" bash "$HERE/build-dc.sh" "$POC" "$OUT" > "$OUT.build.log" 2>&1 || {
    echo "   FALLO el build (ver $OUT.build.log)"; tail -10 "$OUT.build.log"; exit 1; }
printf '   1ST_READ.BIN %s B   prog.elf %s B\n' "$(stat -c%s "$OUT/1ST_READ.BIN")" "$(stat -c%s "$OUT/prog.elf")"

echo "== 3/4 la corrida (${SECS}s, sin capturas: la sonda no dibuja)"
CAPTURE=none bash "$HERE/run-flycast.sh" "$OUT/prog.elf" "$SECS" "$OUT/run" > "$OUT.run.log" 2>&1

echo "== 4/4 el veredicto"
veredicto "$OUT/run/flycast.log"
