#!/bin/bash
# DC-002.14: EL ALCANCE REAL DEL ENTRY POINT (medido, paquete por paquete).
#
# La pregunta de la tarjeta: "el entry point arrastra el motor entero?". La respuesta no se deduce mirando
# los imports (los imports de Go arrastran el paquete completo, pero eso no dice que SIMBOLOS se usan): se
# mide sacando UN paquete del enlace y viendo si el linker queda contento o si falta algo.
#
#   - si el enlace TERMINA sin el paquete  -> nadie referencia nada suyo: el paquete es ALCANCE MUERTO y
#     se puede sacar (se miden los bytes que ahorra).
#   - si el enlace se cae con "undefined reference" -> el paquete se usa, y los simbolos que faltan dicen
#     DESDE DONDE (es lo que el motor necesita de verdad).
#
# Los .gox ya estan compilados: esto solo reenlaza (1-2 s por paquete), no recompila nada.
#
# Uso:  bash scripts/console/medir-alcance-motor.sh [dir_gox] [dir_salida]
#         SOLO=<subcadena>   mide solo los paquetes cuya ruta contenga eso (para probar rapido)
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

GEN="${1:-/tmp/gox-0028}"
OUT="${2:-/tmp/dcbuild/alcance-214}"
SOLO="${SOLO:-}"          # con SOLO vacio el patron *""* matchea todos los paquetes
LISTA="$GEN/orden-motor.txt"
[ -f "$LISTA" ] || { echo "falta $LISTA" >&2; exit 1; }

mkdir -p "$OUT"
RES="$OUT/alcance.txt"
: > "$RES"

enlazar() { # $1 = archivo de .gox a usar como lista-motor, $2 = salida
    # shellcheck disable=SC2046,SC2086
    kos-cc -o "$2" "$OUT/main.o" $POC_OBJS $STD_C $(tr '\n' ' ' < "$GEN/orden.txt") $(tr '\n' ' ' < "$1") \
        $DC_LDPATHS $DC_LIBS_PRE $DC_LIBS_POST
}

# main.o: se compila UNA vez con el mismo camino que build-motor-arranque.sh
POC="$HERE/../../console/dc/poc/motor-arranque"
echo "compilando el arranque (main.o)"
sh-elf-gccgo $GODC_GOFLAGS -I "$GEN" -c "$POC"/*.go -o "$OUT/main.o" || exit 1
POC_OBJS=""
for f in "$POC"/*.c "$POC"/csrc/*.c; do
    [ -f "$f" ] || continue
    kos-cc -O2 -ml -m4-single -I"$POC" -I"$POC/csrc" -c "$f" -o "$OUT/$(basename "${f%.c}").o" || exit 1
    POC_OBJS="$POC_OBJS $OUT/$(basename "${f%.c}").o"
done
STD_C=""
for f in "$HERE/../../console/dc/std/csrc"/*.c; do
    [ -f "$f" ] || continue
    kos-cc -O2 -ml -m4-single -I"${ZLIB_INC:-$DC_ROOT/kos-ports/zlib/inst/include}" -c "$f" \
        -o "$OUT/stdlib-$(basename "${f%.c}").o" || exit 1
    STD_C="$STD_C $OUT/stdlib-$(basename "${f%.c}").o"
done

echo "== enlace COMPLETO (los $(wc -l < "$LISTA") paquetes del motor) como referencia"
if enlazar "$LISTA" "$OUT/ref.elf" > "$OUT/ref.log" 2>&1; then
    echo "   enlaza: $(stat -c%s "$OUT/ref.elf") bytes" | tee -a "$RES"
else
    echo "   NO enlaza ni completo (revisar $OUT/ref.log)" >&2
    tail -5 "$OUT/ref.log" >&2
    exit 1
fi

echo
echo "== sacando UN paquete por vez"
MUERTO=0
NECESARIO=0
while read -r gox; do
    [ -n "$gox" ] || continue
    pkg="${gox#"$GEN"/}"
    pkg="${pkg%.gox}"
    case "$pkg" in
    *"${SOLO}"*) ;;
    *) continue ;;
    esac
    grep -vxF "$gox" "$LISTA" > "$OUT/sin-lista.txt"
    if enlazar "$OUT/sin-lista.txt" "$OUT/sin.elf" > "$OUT/sin.log" 2>&1; then
        tam=$(stat -c%s "$OUT/sin.elf")
        echo "MUERTO    $pkg   ($tam bytes el ELF sin el)" | tee -a "$RES"
        MUERTO=$((MUERTO + 1))
    else
        simbolos=$(grep -c 'undefined reference' "$OUT/sin.log")
        echo "NECESARIO $pkg   ($simbolos referencias sin definir)" | tee -a "$RES"
        NECESARIO=$((NECESARIO + 1))
    fi
done < "$LISTA"

echo
echo "== RESUMEN: $MUERTO paquetes que el entry point NO usa, $NECESARIO que si"
echo "   detalle en $RES"
