#!/usr/bin/env bash
# preflight.sh -- muestra TODOS los errores de un paquete del motor de una sola vez.
#
# POR QUE EXISTE: compilar el motor entero tarda minutos y gccgo se detiene en el PRIMER error, asi que el
# ciclo natural es "arreglo un simbolo, espero 4 minutos, aparece el siguiente". Con -fmax-errors el
# compilador tira TODOS los errores del paquete de una vez y en segundos, asi que se arreglan todos juntos.
#
# MEDIDO: d2render/ebiten paso de 4 minutos por simbolo a 6 simbolos en 8 segundos.
#
# OJO, ACA HABIA UN DEFECTO GRAVE (medido y arreglado): esto compilaba con -fsyntax-only, y GCCGO SE CAE con
# -fsyntax-only en cuanto el paquete tiene un struct con un campo que es un slice ([]uint8 y compania):
#
#   internal compiler error: in fill_in_array, at go/go-gcc.cc:1184
#
# SIN DECIR NADA DEL CODIGO. O sea que la herramienta para saber "que le falta a este paquete" contestaba
# "no compila" con un defecto del compilador, y el que la usaba se ponia a buscar un simbolo que no faltaba.
# Medido con un paquete de 6 lineas (`type T struct { p []uint8 }`): con -fsyntax-only, el ICE; con -c,
# compila y sale un .o de 10.828 bytes.
#
# POR ESO AHORA COMPILA DE VERDAD (-c a un archivo temporal). Es un poco mas lento, pero un numero que
# miente no sirve de nada. Y si aparece el ICE igual, se avisa como defecto del compilador y NO como error
# del paquete: son dos cosas distintas y confundirlas cuesta horas.
#
# Uso:  scripts/console/preflight.sh <paquete>      (ej: d2core/d2render/ebiten)
#       scripts/console/preflight.sh --todos        (todos los que todavia no compilan)
#
# OJO CON EL FILTRO DE PLATAFORMA: hay que excluir los _darwin.go/_windows.go igual que hace
# build-motor-all.sh. Si no, sale un error por el "import C" de un archivo que no tiene nada que ver con la
# Dreamcast, y se persigue un fantasma (me paso).

set -u
HERE="$(cd "$(dirname "$0")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)"
# shellcheck source=env.sh
source "$HERE/env.sh"

GEN="${GODC_GOX:-$HOME/dreamcast/gox}"
PREFIJO="${GODC_PREFIJO:-github.com/OpenDiablo2/OpenDiablo2}"

listar_fuentes() {
    # la misma lista que usa el build de verdad: sin pruebas y sin archivos de otra plataforma
    ls "$REPO/$1"/*.go 2>/dev/null | grep -v _test.go |
        grep -vE '_(darwin|windows|js|wasm|plan9|freebsd|netbsd|openbsd|linux|android|ios|solaris|aix)\.go$' |
        grep -vE '_(386|amd64|arm|arm64|ppc64|ppc64le|mips|mipsle|mips64|mips64le|riscv64|s390x|loong64|wasm)\.go$'
}

un_paquete() {
    local p="$1"
    local srcs
    srcs=$(listar_fuentes "$p")
    if [ -z "$srcs" ]; then
        echo "$p: no se encontro el paquete"
        return 1
    fi
    echo "=== $p ==="
    # shellcheck disable=SC2086
    salida="$(cd "$REPO/$p" && sh-elf-gccgo $GODC_GOFLAGS -I "$GEN" -fgo-pkgpath="$PREFIJO/$p" \
        -fmax-errors=200 -c $srcs -o "$GEN/$PREFIJO/$p.preflight.o" 2>&1)"

    # el ICE del compilador NO es un error del paquete: se dice aparte, con el nombre que tiene
    if echo "$salida" | grep -q 'internal compiler error'; then
        echo "$p: DEFECTO DEL COMPILADOR (no del codigo): $(echo "$salida" | grep -m1 'internal compiler error')"
        return 0
    fi

    echo "$salida" | grep -E 'error:' | sed "s|$REPO/||" | head -60
}

if [ "${1:-}" = "--todos" ]; then
    # los que ya tienen .gox son los que compilan; el resto es lo que falta
    for p in $(cd "$REPO" && go list ./d2core/... ./d2game/... ./d2app/... ./d2common/... ./d2script/... 2>/dev/null |
        sed 's|github.com/OpenDiablo2/OpenDiablo2/||'); do
        # los .gox del motor NO estan en $GEN/<pkg>.gox sino en $GEN/<pkgpath completo>.gox (asi los
        # escribe build-motor-all.sh): buscar en el primero daba que ninguno estaba compilado y el
        # --todos listaba los 142.
        [ -f "$GEN/$PREFIJO/$p.gox" ] && continue
        [ -d "$REPO/$p" ] || continue
        un_paquete "$p"
        echo
    done
else
    for p in "$@"; do
        un_paquete "$p"
    done
fi
