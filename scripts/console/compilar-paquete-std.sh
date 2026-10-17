#!/bin/bash
# DC-002.19: recompila UN paquete de la stdlib minima (console/dc/std/<paquete>) a <gox>/<paquete>.gox.
#
# POR QUE NO SIRVE build-std-gox.sh PARA ESTO: ese script arranca borrando el arbol de .gox entero
# (`rm -rf "$GEN"`), asi que se lleva puesto el motor (los 142 paquetes y orden-motor.txt) y hay que
# recompilarlo todo (minutos). Cuando lo que cambio es UN paquete de la stdlib (o su C), con esto alcanza.
#
# OJO (medido en DC-002.19): si el arbol de .gox se COPIA de otro lado, su orden.txt/orden-motor.txt traen
# las rutas ABSOLUTAS del original y los scripts de enlace (build-dc.sh, build-motor-arranque.sh) enlazan
# los .gox VIEJOS sin decir nada: el sintoma es una referencia indefinida a un tipo del paquete que si
# esta, o -- peor -- un binario que se enlaza y corre con el codigo viejo. Al copiar: reescribir las rutas.
#
# Uso:  scripts/console/compilar-paquete-std.sh <paquete> [dir_gox]
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

PKG="${1:?uso: compilar-paquete-std.sh <paquete> [dir_gox]}"
GEN="${2:-${GODC_GOX:-$DC_ROOT/gox}}"
REPO="$(cd "$HERE/../.." && pwd)"
SRC="$REPO/console/dc/std/$PKG"

[ -d "$SRC" ] || { echo "no encuentro el paquete $SRC" >&2; exit 1; }
[ -d "$GEN" ] || { echo "no existe el arbol de .gox $GEN" >&2; exit 1; }

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
mkdir -p "$TMP/$PKG"

for f in "$SRC"/*.go; do
    [ -f "$f" ] || continue
    case "$f" in *_test.go) continue ;; esac
    sed 's|"d2std/|"|g' "$f" > "$TMP/$PKG/$(basename "$f")"
done

sh-elf-gccgo $GODC_GOFLAGS -I "$GEN" -fgo-pkgpath="$PKG" -c "$TMP/$PKG"/*.go -o "$GEN/$PKG.gox"
printf "%s.gox %s bytes sha256=%s\n" "$PKG" "$(stat -c%s "$GEN/$PKG.gox")" \
    "$(sha256sum "$GEN/$PKG.gox" | awk '{print $1}')"
