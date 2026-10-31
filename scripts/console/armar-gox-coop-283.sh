#!/bin/bash
# Compila el CIERRE del POC del cooperativo en un arbol de .gox nuevo (DC-002.83).
#
# POR QUE NO build-motor-all.sh: el arbol que se usa aca es uno NUEVO (build-std-gox.sh lo dejo con la
# stdlib minima y los terceros) y solo hace falta el cierre que el POC importa -- 60 y pico de paquetes en
# vez de los 145 del motor entero. El orden sale de `go list -deps`, que ya viene topologico.
#
# Uso:  armar-gox-coop-283.sh <dir_gox>
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
GEN="${1:?uso: armar-gox-coop-283.sh <dir_gox>}"
PREFIJO="github.com/OpenDiablo2/OpenDiablo2"

[ -f "$GEN/fmt.gox" ] || { echo "falta la stdlib minima en $GEN (correr build-std-gox.sh)" >&2; exit 1; }

PAQUETES="$(cd "$REPO" && go list -deps -e -tags dreamcast \
    ./d2core/d2asset ./d2core/d2render/pvr ./d2core/d2inventory \
    ./d2common/d2fileformats/d2ds1 ./d2common/d2fileformats/d2dt1 \
    ./d2common/d2fileformats/d2mpq ./d2common/d2fileformats/d2animdata \
    ./d2common/d2path ./d2common/d2math/d2vector 2>/dev/null |
    grep "$PREFIJO" | sed "s|.*$PREFIJO/||" | awk '!visto[$0]++')"

[ -n "$PAQUETES" ] || { echo "la lista de paquetes quedo vacia (go list -deps fallo?)" >&2; exit 1; }
echo "== paquetes del cierre: $(printf '%s\n' $PAQUETES | wc -l)"

FALLOS=""
LOGDIR="${LOGDIR:-/tmp/t5e0bae71/gox283-logs}"
mkdir -p "$LOGDIR"
for p in $PAQUETES; do
    salida="$GEN/$PREFIJO/$p.gox"
    [ -f "$salida" ] && continue

    mapfile -t SRC_P < <(ls "$REPO/$p"/*.go 2>/dev/null | grep -v '_test\.go$' |
        grep -vE '_(darwin|windows|js|wasm|plan9|freebsd|netbsd|openbsd|linux|android|ios|solaris|aix)\.go$' |
        grep -vE '_(386|amd64|arm|arm64|ppc64|ppc64le|mips|mipsle|mips64|mips64le|riscv64|s390x|loong64|wasm)\.go$')
    [ "${#SRC_P[@]}" -gt 0 ] || { echo "   SIN FUENTES: $p"; FALLOS="$FALLOS $p"; continue; }

    mkdir -p "$(dirname "$salida")"
    LOG="${LOGDIR}/$(echo "$p" | tr / _).log"
    if sh-elf-gccgo $GODC_GOFLAGS -I "$GEN" -fgo-pkgpath="$PREFIJO/$p" -c "${SRC_P[@]}" -o "$salida" 2>"$LOG"; then
        grep -qxF "$salida" "$GEN/orden.txt" || echo "$salida" >> "$GEN/orden.txt"
    else
        echo "   FALLO $p (ver $LOG)"
        FALLOS="$FALLOS $p"
    fi
done

echo "== gox en el arbol: $(find "$GEN" -name '*.gox' | wc -l)  (de 0 bytes: $(find "$GEN" -name '*.gox' -size 0 | wc -l))"
echo "== objetos en orden.txt: $(grep -c . "$GEN/orden.txt")"
if [ -n "$FALLOS" ]; then
    echo "== FALLARON:$FALLOS"
    exit 1
fi
echo "== el cierre compilo entero"
