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
# flag va despues de io/strconv/strings (importa los tres) y antes de encoding/csv (que no lo usa).
# NOTA (DC-002.5b): hasta DC-002.5b este archivo decia que encoding/csv NO podia entrar al orden
# porque "no compila en SH-4 (non-integer len argument to make en csv.go:344)" y que al entrar "el
# build se cortaba ahi". Las DOS cosas eran falsas cuando se escribieron: medido, csv compila solo
# (95.628 bytes de .gox) y el corte era un problema de orden, arreglado en DC-002.8 (commit
# 4420033d). El comentario quedo viejo y confundia: se corrige aca.
# PENDIENTE QUE YA NO ESTA (DC-002.8a): hasta DC-002.8 este archivo decia que os y io/fs tenian TIPOS
# DISTINTOS para lo mismo (FileInfo, DirEntry, FileMode) y que alinearlos bajaba el motor a 40 de 142.
# Las dos cosas eran ciertas ENTONCES y dejaron de serlo en 95e0ef74 (DC-002.8), que hizo el alias; el
# comentario quedo y confundia, porque el tablero lo leia como trabajo pendiente. Medido en DC-002.8a:
# os y io/fs ya comparten los tipos (os los reusa por alias), el motor esta en 93 de 142 antes y despues
# de los cambios de esta tarjeta, y el que vigila la alineacion es scripts/console/mutar-tipos-os.sh.
# Lo que SI se agrego aca (DC-002.8a): los imports de los paquetes de d2std van como "d2std/..." y no
# con el nombre desnudo; el que lo exige es scripts/console/checar-imports-d2std.sh.
ORDEN="${ORDEN:-unicode unicode/utf8 unicode/utf16 errors io bytes strings strconv fmt math math/bits math/rand math/cmplx hash hash/fnv crypto/sha256 encoding/json encoding/binary encoding/hex compress/zlib compress/gzip container/ring sort container/heap image/color image image/draw regexp time io/fs embed os os/exec syscall log bufio flag encoding/csv io/ioutil path/filepath runtime sync/atomic sync}"

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
        # los _test.go van SOLO al host (son la prueba contra la biblioteca de verdad): si entran al
        # build de la consola, gccgo intenta compilar un paquete de prueba que importa al de go.
        case "$f" in *_test.go) continue ;; esac
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

# ============================================================================
# Los paquetes de TERCEROS que el motor importa y que se reescribieron para la consola. No son stdlib,
# pero van al mismo directorio de .gox porque para gccgo da lo mismo: son paquetes con su ruta.
#
# Va aca (y no en un script aparte) para que cualquier build que arme la stdlib los tenga: si faltan,
# el motor se cae en d2util y en los paquetes que lo usan, y el error se ve lejos de la causa.
# ============================================================================
compilar_tercero() {
    local pkgpath="$1" archivo="$2"
    local salida="$DC_ROOT/gox/${pkgpath}.gox"
    mkdir -p "$(dirname "$salida")"
    if sh-elf-gccgo $GODC_GOFLAGS -I "$DC_ROOT/gox" -fgo-pkgpath="$pkgpath" -c "$archivo" -o "$salida" 2>"$salida.log"; then
        echo "  ${pkgpath##*/}.gox  $(stat -c%s "$salida") bytes"
        rm -f "$salida.log"
    else
        echo "  ERROR en $pkgpath:"
        head -4 "$salida.log"
    fi
}

echo "== los paquetes de terceros reescritos para la consola =="
# ebiten: el esqueleto del backend grafico/input/audio (d2core/d2render, d2input, d2audio y d2util)
compilar_tercero "github.com/hajimehoshi/ebiten/v2" "$HERE/../../console/dc/engine/ebiten/v2/ebiten.go"
# inpututil: la parte de input (teclas y textos). Va despues de ebiten, que es de quien depende.
compilar_tercero "github.com/hajimehoshi/ebiten/v2/inpututil" "$HERE/../../console/dc/engine/ebiten/v2/inpututil/inpututil.go"
# akara: el BitSet (d2records/monster_unique_affix_*). Medido: es lo unico que se usa de esa libreria.
compilar_tercero "github.com/gravestench/akara" "$HERE/../../console/dc/engine/akara/akara/akara.go"
# google/uuid: los identificadores de las entidades del mapa (d2mapentity/factory.go). Medido: solo se
# usa uuid.New.
compilar_tercero "github.com/google/uuid" "$HERE/../../console/dc/engine/uuid/uuid.go"
# x/image/bmp: el BMP con los glifos de la fuente del juego (d2util/assets)
compilar_tercero "golang.org/x/image/bmp" "$HERE/../../console/dc/engine/x/image/bmp/bmp.go"
