#!/bin/bash
# DC-002.27: recompila UN paquete del motor a su .gox (y lo deja en orden-motor.txt), sin pagar los
# minutos de build-motor-all.sh (que reintenta los 142).
#
# Para que: medir un arreglo del motor en la consola (editar el .go, recompilar el .gox, enlazar el POC) en
# un par de minutos en vez de compilar el motor entero. OJO: esto NO reemplaza a build-motor-all.sh como
# control (esa es la que dice N/146); es la herramienta de la vuelta corta mientras se mide.
#
# Uso:  scripts/console/recompilar-paquete-motor.sh <paquete> [dir_gox]
#       <paquete> es la ruta del paquete dentro del repo, p.ej. d2common/d2fileformats/d2mpq
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
PAQ="${1:?uso: recompilar-paquete-motor.sh <paquete> [dir_gox]}"
GEN="${2:-${GODC_GOX:-$DC_ROOT/gox}}"
PREFIJO="github.com/OpenDiablo2/OpenDiablo2"

cd "$REPO"

# los mismos filtros que build-motor-all.sh: sin _test.go, sin los sufijos de otras plataformas
# (gccgo no los filtra solo y los intenta compilar), sin el lado del HOST de la tabla de objectLookups
# (el volcado y su escritor/registro), que en la consola NO van en el binario: la tabla viaja en el disco
# (/JUEGO/OBJECTLOOKUPS.PACK) y se lee por registro (DC-002.42), y sin `marca_carga_host.go` (DC-002.43: las
# marcas de carga; el lado de la consola es `marca_carga_godc.go`, que gccgo compila siempre porque NO
# respeta los build tags -- medido).
# DC_TABLA_LOOKUPS=dentro los vuelve a meter: es el mutante de la tarjeta.
SRCS=$(ls "$PAQ"/*.go | grep -v _test.go |
    grep -vE '_(darwin|windows|js|wasm|plan9|freebsd|netbsd|openbsd|linux|android|ios|solaris|aix)\.go$' |
    grep -vE '_(386|amd64|arm|arm64|ppc64|ppc64le|mips|mipsle|mips64|mips64le|riscv64|s390x|loong64|wasm)\.go$' |
    grep -vE '/(marca_carga_host)\.go$')

if [ "${DC_TABLA_LOOKUPS:-fuera}" = "fuera" ]; then
    SRCS=$(echo "$SRCS" | grep -vE '/(object_lookup_record_data|object_lookups_host)\.go$' || true)
fi

[ -n "$SRCS" ] || { echo "no hay fuentes en $PAQ" >&2; exit 1; }

OUT="$GEN/$PREFIJO/$PAQ.gox"
mkdir -p "$(dirname "$OUT")"

# shellcheck disable=SC2086
if sh-elf-gccgo $GODC_GOFLAGS -I "$GEN" -fgo-pkgpath="$PREFIJO/$PAQ" -c $SRCS -o "$OUT" > /tmp/recompilar-$(echo "$PAQ" | tr / _).log 2>&1; then
    grep -qxF "$OUT" "$GEN/orden-motor.txt" 2>/dev/null || echo "$OUT" >> "$GEN/orden-motor.txt"
    echo "RE-MOTOR $PAQ -> $OUT ($(stat -c%s "$OUT") bytes)"
else
    echo "RE-MOTOR $PAQ FALLO:" >&2
    tail -20 /tmp/recompilar-$(echo "$PAQ" | tr / _).log >&2
    exit 1
fi
