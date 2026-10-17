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
# LOS QUE NO COMPILAN EN EL HOST (y por que), medido:
#   os, time, log, runtime, sync/atomic, compress/zlib, compress/gzip: hablan con KOS por //extern
#   (el VFS, el reloj de la RTC, el GC del runtime, los atomicos del SH-4, la zlib de KOS), o sea que
#   su implementacion solo existe dentro del binario de la consola.
#   io/fs e io/ioutil: NO tienen un //extern propio; llegan a KOS por el grafo de imports, y eso es
#   consecuencia de DC-002.8a (los imports de la stdlib minima se pasaron a "d2std/..."): io/fs pide
#   d2std/time (FileInfo.ModTime devuelve un time.Time) e ioutil pide d2std/os, d2std/io y d2std/sort.
#   Antes de ese cambio los dos compilaban en el host, pero compilando contra la biblioteca de Go: el
#   mismo texto `import "io"` significa NUESTRO io en la consola (el build reescribe "d2std/" -> "") y
#   el io de Go en el host, asi que la prueba no estaba probando lo nuestro y no lo decia. El ajuste
#   que lo caza es scripts/console/checar-imports-d2std.sh.
#   Historia de la lista: os siempre; time y log entraron en DC-002.7 (hasta entonces el host venia
#   fallando por ellos: "build failed"); compress/zlib y compress/gzip en DC-002.5b (puente //extern
#   a la zlib de KOS, commit 4bc01457); io/fs e io/ioutil en DC-002.8a.
#
# La prueba tiene DOS mitades y las dos se miden en cada ancho:
#   1. los NO excluidos: suite diferencial verde contra la biblioteca real;
#   2. los excluidos: NO compilan en el host. Si algun dia uno compila (porque alguien le escribe una
#      implementacion de host), esto se pone ROJO en vez de taparlo con una lista que quedo vieja.
#
# El corpus de la consola de todo esto (os, time, log, ioutil, io/fs) es console/dc/poc/vfs, con sus
# mutadores en scripts/console/mutar-os-consola.sh.
#
# Uso:  scripts/console/test-std-host.sh
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STD="${STD:-$HERE/../../console/dc/std}"

KOS="os time log runtime sync/atomic compress/zlib compress/gzip io/fs io/ioutil"
EXCLUIDOS="d2std/($(echo "$KOS" | tr ' ' '|'))\$"
N_KOS=$(echo "$KOS" | wc -w)

rc=0
for arch in amd64 386; do
    echo "== prueba diferencial con GOARCH=$arch =="
    if ! (cd "$STD" && GOARCH="$arch" go test -timeout 60s $(go list ./... | grep -vE "$EXCLUIDOS") 2>&1 | tail -25); then
        rc=1
    fi

    echo "   -- los que hablan con KOS tienen que NO compilar en el host --"
    no_compilan=0
    for p in $KOS; do
        if (cd "$STD" && GOARCH="$arch" go build "./$p" >/dev/null 2>&1); then
            echo "   OJO: d2std/$p COMPILA en el host ($arch) y esta en la lista de excluidos:"
            echo "        la lista quedo vieja (o alguien le puso una implementacion de host sin decirlo)."
            rc=1
        else
            no_compilan=$((no_compilan + 1))
        fi
    done
    echo "   excluidos que NO compilan en el host ($arch): $no_compilan de $N_KOS"
done
echo
if [ "$rc" -eq 0 ]; then
    echo "== los dos anchos: verde (y los $N_KOS que hablan con KOS siguen sin compilar en el host) =="
else
    echo "== HAY FALLOS (arriba) =="
fi
exit "$rc"
