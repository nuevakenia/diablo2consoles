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
# La lista la amplio DC-002.9 (el motor PINTANDO): para el COLOR hacen falta d2util
# (ImgIndexToRGBA, el camino del propio motor) y d2dat (la paleta del MPQ), y esos arrastran
# d2interface -> d2enum, d2math/d2vector y el tipo de conexion de red. Medido con `go list -deps`:
# son 11 paquetes internos y ni uno mas.
#   d2enum                 -> fmt, log, strconv
#   d2math, d2math/d2vector -> fmt, math
#   d2clientconnectiontype  -> (nada)
#   d2interface            -> d2enum, d2math/d2vector, d2clientconnectiontype, image, image/color
#   d2util/assets          -> bytes, compress/gzip, fmt, image, log, x/image/bmp
#   d2util                 -> d2interface, d2util/assets, ebiten, image/color, sync/atomic, ...
#   d2datautils            -> io, bytes, log, fmt
#   d2dc6                  -> fmt, d2datautils
#   d2dat                  -> fmt, d2interface
#   motorresumen           -> fmt, strconv, d2dc6, d2interface, d2util  (el resumen medible del POC)
PAQUETES="${PAQUETES:-d2common/d2enum d2common/d2math d2common/d2math/d2vector d2networking/d2client/d2clientconnectiontype d2common/d2interface d2common/d2util/assets d2common/d2util d2common/d2datautils d2common/d2fileformats/d2dc6 d2common/d2fileformats/d2dat console/dc/motorresumen}"

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
    # orden.txt: cada .gox se anota UNA sola vez. Si el script se corre dos veces sin rehacer la
    # stdlib (SALTAR_STDLIB=1), el linker recibiria el mismo objeto dos veces y se quejaria de
    # definiciones multiples.
    if [ -f "$GEN/orden.txt" ]; then
        # shellcheck disable=SC2086
        grep -vxF "$out" "$GEN/orden.txt" > "$GEN/orden.txt.nuevo" || true
        mv "$GEN/orden.txt.nuevo" "$GEN/orden.txt"
    fi
    echo "$out" >> "$GEN/orden.txt"
done

# Los paquetes de TERCEROS tambien tienen que ir al ENLACE, no solo al -I. build-std-gox.sh los
# compila pero no los anota en orden.txt, y eso alcanzaba mientras nadie los usara. Medido en
# DC-002.9: un POC que pasa por d2util deja UNDEFINED los simbolos de ebiten (debug_print.go llama
# NewImageFromImage y usa ebiten.Image/DrawImageOptions) y los de x/image/bmp (d2util/assets,
# zipbmp.go). Se anotan al final, que es de quien dependen.
for t in github.com/hajimehoshi/ebiten/v2 github.com/hajimehoshi/ebiten/v2/inpututil \
         github.com/gravestench/akara github.com/google/uuid golang.org/x/image/bmp; do
    f="$GEN/$t.gox"
    [ -f "$f" ] || continue
    grep -qxF "$f" "$GEN/orden.txt" 2>/dev/null || echo "$f" >> "$GEN/orden.txt"
done

echo "paquetes del motor compilados para SH-4 en $GEN"
