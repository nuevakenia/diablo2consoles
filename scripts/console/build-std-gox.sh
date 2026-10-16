#!/bin/bash
# Compila la biblioteca estandar minima (console/dc/std) para SH-4, en formato .gox.
#
# Los paquetes se importan entre si como "d2std/xxx" para que la prueba diferencial del host
# funcione contra el modulo d2std. En la consola, en cambio, el motor pide "fmt", "strings", etc. a
# secas: si se compilaran las dos formas habria DOS juegos de paquetes distintos y los tipos no
# encajarian entre si (un io.Writer de cada lado no es el mismo tipo). Por eso el build copia las
# fuentes a un directorio de trabajo y reescribe "d2std/" -> "": un solo juego de paquetes, con los
# nombres que el motor espera encontrar.
#
# Uso:  scripts/console/build-std-gox.sh [dir_salida]      (por defecto $DC_ROOT/gox)
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

STD="${STD:-$HERE/../../console/dc/std}"
GEN="${1:-${GODC_GOX:-$DC_ROOT/gox}}"

# orden de dependencias: cada paquete necesita los .gox de los anteriores
# (io va despues de errors porque io.EOF es un errors.New, DC-002.3)
# Orden de dependencias. ojo: heap importa sort, y csv importa encoding.
# OJO: encoding/csv NO va todavia: el paquete que quedo escrito (a medias) no compila en SH-4
# ("non-integer len argument to make" en csv.go:344). Bloquea a d2equip, d2statlist, d2txt y d2s, asi
# que hay que arreglarlo, pero primero el resto.
# encoding/csv: compila a mano (con el sed aplicado) pero NO desde el build: al entrar en el orden, el
# build de la stdlib se corta ahi y no genera los paquetes siguientes (quedo en 9 .gox en vez de ~20).
# Hay que ver por que (sospecha: el dst/src le queda un archivo con los imports sin el sed). Anotado.
ORDEN="${ORDEN:-unicode unicode/utf8 errors io bytes strings strconv fmt math math/bits math/rand hash hash/fnv encoding/binary encoding/hex container/ring sort container/heap image/color image os log time bufio encoding/csv path/filepath runtime sync/atomic sync}"

if [ ! -d "$STD/${ORDEN%% *}" ]; then
    echo "no encuentro el primer paquete ($STD/${ORDEN%% *})" >&2
    exit 1
fi

rm -rf "$GEN"
mkdir -p "$GEN/src" "$GEN/d2std"
for pkg in $ORDEN; do
    dst="$GEN/src/$pkg"
    mkdir -p "$dst"
    for f in "$STD/$pkg"/*.go; do
        [ -f "$f" ] || continue
        sed 's|"d2std/|"|g' "$f" > "$dst/$(basename "$f")"
    done
    mkdir -p "$(dirname "$GEN/$pkg")"
    # el archivo .gox que escribe gccgo es a la vez la cabecera de importacion Y el objeto con el
    # codigo (ELF reubicable): sirve para importar y hay que pasar el mismo archivo al enlazador
    sh-elf-gccgo $GODC_GOFLAGS -I "$GEN" -fgo-pkgpath="$pkg" -c "$dst"/*.go -o "$GEN/$pkg.gox"
    printf "  %-16s %8s bytes\n" "$pkg.gox" "$(stat -c%s "$GEN/$pkg.gox")"
    echo "$GEN/$pkg.gox" >> "$GEN/orden.txt"
done

echo "stdlib minima compilada para SH-4 en $GEN"
