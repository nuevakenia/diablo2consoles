#!/bin/bash
# Corre la prueba diferencial de la stdlib minima (console/dc/std) contra la biblioteca estandar
# real de Go, en los dos anchos de int que importan:
#
#   amd64  -> int de 64 bits (el host)
#   386    -> int de 32 bits, que es el ancho de int del SH-4 de la Dreamcast
#
# Por que los dos: hay defectos que solo se ven con int de 32 bits (el tope de Atoi/ParseInt al
# desbordar, por ejemplo). Una sola corrida deja el hueco abierto; el corpus de la consola se
# calcula con el ancho del destino (32) justamente por eso.
#
# OJO: el paquete os NO se puede probar aca. Su implementacion habla con el VFS de KOS (funciones
# //extern que solo existen en el binario de la consola), asi que en el host no compila. Su prueba es
# la que corre EN la consola: console/dc/poc/vfs + scripts/console/mutar-os-consola.sh.
#
# Uso:  scripts/console/test-std-host.sh
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STD="${STD:-$HERE/../../console/dc/std}"

rc=0
for arch in amd64 386; do
    echo "== prueba diferencial con GOARCH=$arch =="
    # os queda afuera a proposito (necesita KOS); se corre el resto y ademas se exige que os NO compile,
    # para que el dia que alguien le ponga una implementacion de host la prueba lo diga.
    if ! (cd "$STD" && GOARCH="$arch" go test $(go list ./... | grep -v 'd2std/os$') 2>&1 | tail -25); then
        rc=1
    fi
done
echo
if [ "$rc" -eq 0 ]; then
    echo "== los dos anchos: verde =="
else
    echo "== HAY FALLOS (arriba) =="
fi
exit "$rc"
