#!/bin/bash
# DC-002.46: LA PRUEBA EN CONTRA DEL CORPUS DE DC-002.40.
#
# Arma un arbol de Dreamcast APARTE (no toca ~/dreamcast) con el runtime vendorizado MUTADO en UNA linea:
# el itab de `get_itab` vuelve al monton MOVIBLE de Go (`gc_alloc`) en vez del monton de C
# (`gc_external_alloc`, el arreglo de DC-002.39e). Rearma `libgodc.a` en ese arbol y lo deja listo para que
# `checar-compr-240.sh` corra contra el con `DC_ROOT=<dir> LIBGODC=<dir>/libgodc`.
#
# POR QUE ESTE MUTANTE Y NO OTRO: es, literalmente, el defecto que tenia el runtime cuando el corpus se midio
# en 9/13 (DC-002.43). El `itab_cache` vive en `.bss` y el recolector de la consola NO barre `.bss`: con el
# itab en el monton movible, la recoleccion lo copia y el cache queda apuntando a la mitad vieja; el
# `ReadFile("(listfile)")` del tercer MPQ salta a un pedazo de DATO y la consola se REARMA. Medido con el
# corpus: 9/15, con `0c` (el control que lo nombra) y los cuatro del MOTOR (7, 8, 9, 10) y el 11 en ROJO.
#
# Uso:
#   bash scripts/console/mutar-runtime-itab-246.sh <dcroot>
#   DC_ROOT=<dcroot> LIBGODC=<dcroot>/libgodc bash scripts/console/checar-compr-240.sh <gox> "" 90 sano
#
# Sale con != 0 si no pudo armar el arbol: el mutante sin rearmar no prueba nada (y el corpus mide el .a que
# haya, o sea el sano, sin decirlo).
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

ROOT="${1:?uso: mutar-runtime-itab-246.sh <dcroot>}"
ORIG="$DC_ROOT"

rm -rf "$ROOT"; mkdir -p "$ROOT"
cp -a "$ORIG/libgodc" "$ROOT/libgodc" || exit 1
for d in kos kos-ports sh-elf; do ln -sfn "$ORIG/$d" "$ROOT/$d"; done

IFACE="$ROOT/libgodc/runtime/interface_dreamcast.c"
cp "$IFACE" "$IFACE.sano"

"${DC_PYTHON:-python3}" - "$IFACE" <<'PY' || exit 1
import sys
p = sys.argv[1]
t = open(p).read()
viejo = "Itab *itab = (Itab *)gc_external_alloc(total_size);"
nuevo = "Itab *itab = (Itab *)gc_alloc(total_size, NULL); /* MUTANTE DC-002.46: el itab en el monton MOVIBLE */"
n = t.count(viejo)
if n != 1:
    print("el texto a mutar aparece %d veces (tiene que ser 1): el mutante no prueba nada" % n, file=sys.stderr)
    sys.exit(2)
open(p, "w").write(t.replace(viejo, nuevo))
print("mutado: gc_external_alloc(total_size) -> gc_alloc(total_size, NULL)")
PY

# el .a SE REARMA en el arbol mutado: sin esto el corpus enlaza el runtime sano y el mutante es invisible.
(
    export DC_ROOT="$ROOT" LIBGODC="$ROOT/libgodc" KOS_BASE="$ROOT/kos" KOS_PORTS="$ROOT/kos-ports"
    cd "$ROOT/libgodc" && make libgodc.a
) > "$ROOT/make.log" 2>&1

if [ ! -f "$ROOT/libgodc/libgodc.a" ] ||
   [ ! "$ROOT/libgodc/libgodc.a" -nt "$ROOT/libgodc/runtime/interface_dreamcast.c" ]; then
    echo "NO SE REARMO libgodc.a en el arbol mutado (ver $ROOT/make.log)" >&2
    tail -10 "$ROOT/make.log" >&2
    exit 1
fi

echo "arbol MUTADO: $ROOT"
echo "libgodc.a: $(stat -c%s "$ROOT/libgodc/libgodc.a") B sha256 $(sha256sum "$ROOT/libgodc/libgodc.a" | cut -c1-16)"
echo "esperado en el corpus: 9/15 con 0c, 7, 8, 9, 10 y 11 en ROJO"
