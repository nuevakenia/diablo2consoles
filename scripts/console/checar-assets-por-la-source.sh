#!/bin/bash
# LA REGLA DE LOS ASSETS EN EL CAMINO DE CONSOLA (DC-002.95), en una línea para el que la corre.
#
#   scripts/console/checar-assets-por-la-source.sh          -> 0 = verde, 1 = hay lecturas NUEVAS, 2 = no corrió
#   scripts/console/checar-assets-por-la-source.sh -v       -> además muestra el alcance y cada lectura con dueño
#
# QUÉ PROHÍBE, Y POR QUÉ (el porqué completo está en docs/adr/0001-assets-fuera-del-heap-de-go.md):
# decidido por Daniel el 31/10 — los assets grandes del camino de consola viven FUERA del montón de Go, con
# liberación explícita, y el motor no se toca. La consecuencia práctica es una sola: en el camino de consola
# no se lee un asset por `os.ReadFile`, `io.ReadAll`, `mpq.ReadFile` ni `mpq.ReadFileStream`; se pide a la
# Source (`console/dc/fuenteconsola`) con dueño (`Pedir`/`Liberar`, `ReadFileInto`, `LeerEnteroEn`).
#
# La implementación es un programa Go con go/ast (scripts/console/regla-assets-por-la-source): el árbol está
# lleno de comentarios que nombran `io.ReadAll` para explicar por qué NO se usa, y un grep se pondría rojo con
# el comentario que enseña a no hacerlo. El AST ve llamadas, no prosa.
#
# La deuda que ya existía está declarada, una por una, en
# scripts/console/assets-fuera-de-la-source.deuda.txt, con su motivo y su tarjeta. Lo que no está ahí es una
# violación nueva. Un mutador prueba que la regla caza lo que dice cazar:
#   scripts/console/mutar-assets-por-la-source.sh
#
# NOTA DE ENTORNO: esto corre en el HOST con el `go` del host (no con el toolchain de SH-4), así que NO carga
# env.sh a propósito.
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)"

if ! command -v go >/dev/null 2>&1; then
    echo "no hay 'go' en el PATH: la regla corre en el host (ver docs/adr/0001-...)" >&2
    exit 2
fi

cd "$REPO" || exit 2
exec go run ./scripts/console/regla-assets-por-la-source "$@"
