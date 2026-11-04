#!/bin/bash
# DC-002.146c -- EL BLINDAJE: el `recover` de libgodc que SI devuelve el control (POC minimo).
#
# QUE SALE DE ACA. Un programa de consola de 40 lineas de Go y 20 de C, sin motor ni assets:
# `runtime_blindaje` de libgodc + un panic de REBANADA de verdad adentro del cuerpo + tres vueltas.
# Es el criterio de la tarjeta: el panic se imprime, el control vuelve y la consola sigue imprimiendo.
#
# LA CONTRA-PRUEBA (BLINDAJE=viejo). El mismo POC, con el `runtime_checkpoint()` de siempre (el que el
# proyecto usaba en el rearme del minimapa): el `setjmp` queda en una funcion de C que ya retorno, asi que
# el `longjmp` del recover salta a un marco muerto. El binario viejo tiene que morir con
# `kernel panic: unhandled IRQ/Exception`. Los dos binarios son el mismo codigo Go con una const distinta.
#
# ANTES DE CORRER ESTO: libgodc tiene que tener el arreglo y el .a rehecho.
#     python3 scripts/console/fix-libgodc-blindaje.py --build
# (libgodc.a es estatico: sin reenlazar, el binario viejo no tiene la funcion nueva y el enlace falla con
#  `undefined reference to '_runtime_blindaje_go'`, que es el aviso de que falta el paso de arriba.)
#
# Uso:
#   bash scripts/console/build-blindaje-146c.sh [salida_dir]
#   BLINDAJE=viejo bash scripts/console/build-blindaje-146c.sh /tmp/dcbuild/blindaje146c-viejo
#   CAPTURE=none bash scripts/console/run-flycast.sh <salida>/prog.elf 20 <capturas>
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
SRC="$REPO/console/dc/poc/sonda-blindaje-146c"
MODO="${BLINDAJE:-blindaje}"
case "$MODO" in
    blindaje|viejo) ;;
    *) echo "ERROR: BLINDAJE=$MODO (son 'blindaje' o 'viejo')" >&2; exit 2 ;;
esac
OUT="${1:-/tmp/dcbuild/blindaje146c-$MODO}"
SRCB="$OUT/src"

mkdir -p "$OUT"
rm -rf "$SRCB"
mkdir -p "$SRCB"
cp "$SRC"/*.go "$SRCB"/
cp -r "$SRC/csrc" "$SRCB"/

# LA PALANCA: una sola fuente de verdad (knobs.go del POC); el entorno solo la PISA, y cuando la pisa se
# dice aca y el valor queda escrito en el knobs.go del directorio de build.
ACTUAL="$(awk '$1=="modo" && $2=="=" {print $3}' "$SRCB/knobs.go" | tr -d '"')"
if [ "$MODO" != "$ACTUAL" ]; then
    echo "   ******************************************************************"
    echo "   * PISADA DE PALANCA: modo=$ACTUAL -> $MODO"
    echo "   * el binario NO es el de la referencia del POC"
    echo "   ******************************************************************"
    sed -i "s/^\tmodo    = \".*\"$/\tmodo    = \"$MODO\"/" "$SRCB/knobs.go"
fi
MODO_COMPILADO="$(awk '$1=="modo" && $2=="=" {print $3}' "$SRCB/knobs.go" | tr -d '"')"
[ "$MODO_COMPILADO" = "$MODO" ] || { echo "ERROR: no pude pisar la palanca modo en knobs.go" >&2; exit 1; }
echo "   modo=$MODO_COMPILADO vueltas=$(awk '$1=="vueltas" {print $3}' "$SRCB/knobs.go")"

echo "== compilando y enlazando (build-dc.sh)"
# OJO, MEDIDO: este POC no importa NADA (solo `println`), asi que no puede usar el arbol de la stdlib.
# Con `-I $DC_ROOT/gox` (que es el default de build-dc.sh cuando ese arbol existe) gccgo encuentra el
# `runtime.gox` del arbol y emite la seccion `go..typelists` con una referencia a `runtime..types`, que
# libgodc NO define: el enlace muere con `undefined reference to 'runtime..types'` (le pasa igual al POC
# `hello` con el arbol de hoy). Con un -I vacio no hay referencia. Si algun dia este POC importa un
# paquete, hay que pasar GODC_GOX=<arbol> Y enlazar tambien el runtime.gox del arbol.
mkdir -p "$OUT/sin-gox"
GODC_GOX="${GODC_GOX:-$OUT/sin-gox}" bash "$HERE/build-dc.sh" "$SRCB" "$OUT" > "$OUT/build.log" 2>&1 || {
    echo "   FALLO el build (ver $OUT/build.log)"; tail -30 "$OUT/build.log"; exit 1; }
[ -f "$OUT/prog.elf" ] || { echo "   FALLO: no hay $OUT/prog.elf" >&2; exit 1; }

# el arreglo TIENE que estar en el binario de la corrida del criterio: un .a viejo da el mismo aspecto que
# un enlace bueno. En la contra-prueba (viejo) el simbolo NI SIQUIERA entra: el cuerpo no esta.
# OJO: la lista va a un archivo antes de grepearla. `nm ... | grep -q` con `set -o pipefail` da falso
# negativo (grep -q corta la tuberia, nm muere con SIGPIPE y el pipe queda en 141: medido aca).
SIMBOLOS="$OUT/simbolos.txt"
sh-elf-nm "$OUT/prog.elf" > "$SIMBOLOS"
if [ "$MODO_COMPILADO" = "blindaje" ]; then
    if ! grep -q "T _runtime_blindaje_go" "$SIMBOLOS"; then
        echo "   FALLO: $OUT/prog.elf NO lleva runtime_blindaje_go (¿libgodc.a sin el arreglo?)" >&2
        exit 1
    fi
    echo "   runtime_blindaje_go en el ELF: si"
else
    if grep -q "_runtime_blindaje" "$SIMBOLOS"; then
        echo "   OJO: la contra-prueba lleva runtime_blindaje en el binario (no deberia llamarlo)"
    else
        echo "   contra-prueba: el binario NO lleva runtime_blindaje (el cuerpo viejo no lo llama)"
    fi
    if ! grep -q "T _d2c_checkpoint_viejo" "$SIMBOLOS"; then
        echo "   FALLO: la contra-prueba no lleva d2c_checkpoint_viejo" >&2
        exit 1
    fi
    echo "   d2c_checkpoint_viejo en el ELF: si"
fi

printf '   prog.elf       %10s B\n' "$(stat -c%s "$OUT/prog.elf")"
printf '   codigo (.text) %10s B\n' "$(sh-elf-size -A "$OUT/prog.elf" | awk '$1==".text"{print $2}')"
echo
if [ "$MODO_COMPILADO" = "blindaje" ]; then
    echo "   LO QUE TIENE QUE DECIR LA CONSOLA SERIE (las tres vueltas, en orden):"
else
    echo "   LO QUE TIENE QUE DECIR LA CONSOLA SERIE (la contra-prueba: NO vuelve):"
fi
echo "     BLINDAJE146C: arranca modo= $MODO_COMPILADO vueltas= 3"
echo "     BLINDAJE146C: --- vuelta 0 antes del blindaje ---"
echo "     BLINDAJE146C: dentro del cuerpo, vuelta= 0"
echo "     BLINDAJE146C: leyendo la casilla 3 de una rebanada de 3"
echo "     BLINDAJE146C: PANICO RECOGIDO en el defer, el valor es: (string,\"index out of range\")"
if [ "$MODO_COMPILADO" = "blindaje" ]; then
    echo "     BLINDAJE146C: el blindaje devolvio 1 (0 = el cuerpo volvio solo, 1 = se volvio por el panic)"
    echo "     BLINDAJE146C: SIGUE IMPRIMIENDO despues del panic, vuelta 0"
    echo "     (la vuelta 1 es el cuerpo con defer huerfano: el huerfano NO corre)"
    echo "     BLINDAJE146C: PANICO RECOGIDO (cuerpo con defer huerfano), el valor es: (string,\"index out of range\")"
    echo "     (y al final, con la cadena sana:)"
    echo "     BLINDAJE146C: CIERRE OK las 3 vueltas volvieron y la consola sigue viva"
    echo "     BLINDAJE146C: el defer de main corrio al final: la cadena de defers quedo sana"
    echo "     arch: exit return code 0"
else
    echo "     y despues NADA: el control NO vuelve; el SH-4 se va a basura (medido: inunda la consola serie"
    echo "     con CR/LF hasta que se corta la corrida)"
fi
echo
echo "   ELF:   $OUT/prog.elf"
echo "   GDI:   $OUT/disc.gdi"
