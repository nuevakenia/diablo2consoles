#!/bin/bash
# DC-002.14: EL RECORTE DEL BINARIO DEL MOTOR (medido, no supuesto).
#
# DC-002.13 midio que el motor entero enlazado no entra en los 16 MB (faltan 1.486.848 bytes para que el GC
# de Go consiga su semiespacio de 2 MB) y DC-002.14 midio DONDE esta el peso (scripts/console/pesar-motor.py).
# Este script mueve las dos palancas que el linker y el compilador de la consola tienen y las MIDE:
#
#   A  -ffunction-sections -fdata-sections  (cada funcion y cada dato en su PROPIA seccion, medido: hace
#      falta ademas porque es lo unico que permite atribuir bytes a un paquete sin mentir; en el ELF de una
#      sola .text los tamanos se solapan 2,86x) + -Wl,--gc-sections (el linker tira lo que nadie referencia
#      desde el entry point).
#   B  A + -fno-asynchronous-unwind-tables: el motor no usa excepciones de C++, asi que .eh_frame (que en
#      A pesa cientos de KB) es tabla de desenrollado que nadie lee.
#
# Los .gox van a un arbol PROPIO (por defecto /tmp/gox-214-gc) para no pisar el de las otras tarjetas: el
# otro arbol (/tmp/gox-0028) es el que mide el binario de DC-002.13 y tiene que quedar igual.
#
# Uso:  bash scripts/console/recorte-214.sh [dir_gox] [dir_salida] [A|B]
#         SALTAR_GOX=1   no recompila los .gox (usa el arbol que ya este); sirve para REENLAZAR rapido
#         POR_DEFECTO:    /tmp/gox-214-gc  /tmp/dcbuild/motor-214  A
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

GEN="${1:-/tmp/gox-214-gc}"
OUT="${2:-/tmp/dcbuild/motor-214}"
VAR="${3:-A}"

case "$VAR" in
A | B) ;;
*)
    echo "variante desconocida '$VAR' (A = secciones + gc-sections, B = A + sin .eh_frame)" >&2
    exit 2
    ;;
esac

FLAGS="-ffunction-sections -fdata-sections"
[ "$VAR" = "B" ] && FLAGS="$FLAGS -fno-asynchronous-unwind-tables"

export GODC_GOX="$GEN"
export GODC_GOFLAGS="$GODC_GOFLAGS $FLAGS"
export DC_LDEXTRA="-Wl,--gc-sections"

echo "== RECORTE variante $VAR"
echo "   gox:  $GEN"
echo "   salida: $OUT"
echo "   flags: $FLAGS"
echo "   enlace: DC_LDEXTRA=$DC_LDEXTRA"

if [ -z "${SALTAR_GOX:-}" ]; then
    echo "== 1/3 la stdlib minima y los terceros con los flags del recorte"
    bash "$HERE/build-std-gox.sh" | tail -2
    echo "== 2/3 el motor (142 paquetes) con los flags del recorte"
    bash "$HERE/build-motor-all.sh" "$GEN" "/tmp/dcbuild/motor-all-214-$VAR" |
        grep -E '^== (RESULTADO|bloqueados)'
else
    echo "== 1-2/3 SALTAR_GOX: se usa el gox que ya este en $GEN"
fi

echo "== 3/3 enlazando el motor entero"
bash "$HERE/build-motor-arranque.sh" "$OUT" "$GEN" | grep -E '^(ELF|ELF \(emu\)|1ST_READ|== 3/4)'

echo
echo "== SECCIONES QUE SE CARGAN (lo que decide si entra en los 16 MB)"
sh-elf-size -A "$OUT/prog.elf" |
    awk 'NR>1 && $1 !~ /^(\.debug|\.go_export|\.comment|\.symtab|\.strtab|Total)/ && $2+0>0 {print "   " $0}'
echo
echo "== LA CUENTA (la misma de DC-002.13, para comparar pera con pera)"
python3 - "$OUT/prog.elf" /tmp/dcbuild/motor-arranque/prog.elf "$HERE/medir-cargado.py" <<'FIN'
import subprocess, sys

MEDIR = sys.argv[3]

def cargado(elf):
    out = subprocess.run([sys.executable, MEDIR, elf], capture_output=True, text=True).stdout.splitlines()
    return int(out[0].rsplit(":", 1)[1].split()[0]) if out else 0

RAM = 16777216          # 16 MB de la consola
FALTABAN = 1486848      # medido en DC-002.13: RAM que le faltaba al GC para su semiespacio de 2 MB
nuevo, base = cargado(sys.argv[1]), cargado(sys.argv[2])
print(f"   cargado (baseline DC-002.13): {base}")
print(f"   cargado (recorte)           : {nuevo}")
print(f"   recortado                   : {base - nuevo} bytes")
print(f"   le faltaban al GC           : {FALTABAN} bytes")
print(f"   margen sobre lo que faltaba : {base - nuevo - FALTABAN} bytes")
print(f"   sigue cargando de los {RAM}  : {100 * nuevo / RAM:.2f}%")
print(f"   VEREDICTO por tamano        : {'ENTRA' if base - nuevo >= FALTABAN else 'NO ALCANZA'}"
      f" (el que manda es el arranque: run-flycast.sh + checar-recorte-214.sh)")
FIN
