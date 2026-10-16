#!/bin/bash
# Compila paquetes REALES del motor (OpenDiablo2) para SH-4, en formato .gox, con la biblioteca
# estandar minima en el lugar de la de Go.
#
# Por que asi y no una copia retocada: los archivos del motor se importan entre si por su ruta de
# verdad (github.com/OpenDiablo2/OpenDiablo2/...), asi que se compilan con ese mismo pkgpath y no se
# les toca ni una linea. Lo unico que cambia es quien resuelve "fmt", "io", "log": la stdlib minima
# de console/dc/std. Los _test.go se saltean (gccgo no filtra por sufijo).
#
# Uso:  scripts/console/build-motor-gox.sh [dir_salida]     (por defecto $DC_ROOT/gox)
#       El dir_salida tiene que ser el MISMO que usa build-std-gox.sh, porque el enlace resuelve los
#       imports con un solo -I y orden.txt es la lista de objetos que van al linker.
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
GEN="${1:-${GODC_GOX:-$DC_ROOT/gox}}"
STD_GOX="${GODC_GOX:-$DC_ROOT/gox}"
PREFIJO="github.com/OpenDiablo2/OpenDiablo2"

# En orden de dependencias: cada paquete necesita los .gox de los anteriores y los de la stdlib.
#   d2common/d2datautils -> io, bytes, log, fmt
#   d2dc6                -> fmt, d2datautils
#   motorresumen         -> fmt, strconv, d2dc6   (el resumen medible del POC, en un solo lugar)
PAQUETES="${PAQUETES:-d2common/d2datautils d2common/d2fileformats/d2dc6 console/dc/motorresumen}"

[ -f "$STD_GOX/fmt.gox" ] || {
    echo "falta la stdlib minima en $STD_GOX: corre scripts/console/build-std-gox.sh primero" >&2
    exit 1
}

mkdir -p "$GEN"
for p in $PAQUETES; do
    srcs=$(ls "$REPO/$p"/*.go | grep -v _test.go)
    out="$GEN/$PREFIJO/$p.gox"
    mkdir -p "$(dirname "$out")"
    # shellcheck disable=SC2086
    sh-elf-gccgo $GODC_GOFLAGS -I "$STD_GOX" -I "$GEN" -fgo-pkgpath="$PREFIJO/$p" -c $srcs -o "$out"
    printf "  %-42s %8s bytes\n" "$p.gox" "$(stat -c%s "$out")"
    echo "$out" >> "$GEN/orden.txt"
done

echo "paquetes del motor compilados para SH-4 en $GEN"
