#!/bin/bash
# DC-002.155: RECOMPILAR SOLO LOS PAQUETES QUE SE TOCARON, sin rearmar los 42 del motor.
#
# POR QUE. `build-motor-all.sh` recorre TODOS los paquetes del motor y `build-std-gox.sh` borra el arbol de
# .gox y recompila la stdlib entera: una corrida de los dos son ~6 minutos, y tocar una linea de `d2app`
# costaba los 6 minutos completos. Como los .gox son archivos por paquete y `orden-motor.txt` guarda RUTAS
# (no contenido), recompilar UN paquete es reescribir su .gox en el mismo lugar: la lista de enlace queda igual.
#
# LO QUE NO HACE: no agrega paquetes nuevos a la lista (para eso esta build-motor-all.sh) y no toca la stdlib
# (`console/dc/std`): los paquetes de terceros que vienen de `console/dc/engine` SI se pueden, con la ruta de
# importacion completa.
#
# Uso:
#   scripts/console/recompilar-motor-155.sh <dir_gox> <paquete|ruta:import> ...
#
#   scripts/console/recompilar-motor-155.sh /tmp/dc155/gox d2app d2core/d2gui
#   scripts/console/recompilar-motor-155.sh /tmp/dc155/gox console/dc/engine/ebiten/v2:github.com/hajimehoshi/ebiten/v2
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"
# shellcheck source=palancas-del-build.sh
. "$HERE/palancas-del-build.sh"

REPO="$(cd "$HERE/../.." && pwd)"
GEN="${1:?uso: recompilar-motor-155.sh <dir_gox> <paquete...>}"
shift

[ -d "$GEN" ] || { echo "no existe el arbol de .gox: $GEN" >&2; exit 1; }

fallos=0

for entrada in "$@"; do
    # dos formas: un paquete del motor (`d2app`) o `carpeta:ruta/de/importacion` (los de console/dc/engine)
    case "$entrada" in
        *:*) fuente="${entrada%%:*}"; pkgpath="${entrada##*:}" ;;
        *)   fuente="$entrada";       pkgpath="github.com/OpenDiablo2/OpenDiablo2/$entrada" ;;
    esac

    dir="$REPO/$fuente"
    [ -d "$dir" ] || { echo "no existe $dir" >&2; fallos=$((fallos + 1)); continue; }

    # la MISMA lista de fuentes que usa el build completo (por NOMBRE: gccgo no filtra por `//go:build`)
    mapfile -t srcs < <(fuentes_del_paquete "$REPO" "$fuente")
    if [ "${#srcs[@]}" -eq 0 ]; then
        echo "  $entrada: sin fuentes para compilar" >&2
        fallos=$((fallos + 1))
        continue
    fi

    out="$GEN/$pkgpath.gox"
    mkdir -p "$(dirname "$out")"
    logp="$(mktemp)"

    if sh-elf-gccgo $GODC_GOFLAGS -I "$GEN" -fgo-pkgpath="$pkgpath" -c "${srcs[@]}" -o "$out" > "$logp" 2>&1; then
        printf '  %-52s %8s bytes  (%d archivo(s))\n' "$pkgpath" "$(stat -c%s "$out")" "${#srcs[@]}"

        # si el .gox no estaba en la lista de enlace, se agrega: un paquete compilado que no se enlaza es un
        # paquete que no existe para el binario (defecto que ya tuvo esta receta con los terceros)
        lista="$GEN/orden-motor.txt"
        case "$pkgpath" in
            github.com/hajimehoshi/*|golang.org/*|github.com/gravestench/*|github.com/pkg/*|github.com/robertkrimen/*|github.com/google/*)
                lista="$GEN/orden.txt" ;;
        esac
        grep -qxF "$out" "$lista" 2>/dev/null || echo "$out" >> "$lista"
    else
        echo "  ERROR en $entrada:" >&2
        head -6 "$logp" >&2
        fallos=$((fallos + 1))
    fi

    rm -f "$logp"
done

if [ "$fallos" -ne 0 ]; then
    echo "== $fallos paquete(s) no compilaron" >&2
    exit 1
fi

echo "== los .gox pedidos quedaron recompilados en $GEN"
