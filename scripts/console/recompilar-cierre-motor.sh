#!/bin/bash
# DC-002.29: REARME CON CIERRE. Recompila un paquete del motor Y TODOS los paquetes que lo importan
# (transitivamente), capa por capa, para que la camada de .gox quede CONSISTENTE.
#
# POR QUE: medido en DC-002.29. El `.gox` de gccgo NO es autocontenido (lleva adentro los cuerpos inlineados y
# los numeros de tipo de sus imports, con la ruta de la camada en que se compilaron). Recompilar UN paquete
# solo y reenlazarlo con el resto deja un binario que MUERE DENTRO de los `init` de paquetes de Go
# (justo despues de `main_wrapper-entra`, 0 recolecciones del GC, salto a una direccion basura):
#   - camada de DC-002.27 (anda, llega al cuadro): d2mpq recompilado de las MISMAS fuentes (7ef2ff16) -> muere.
#   - camada coherente de hoy (pasa los init, muere despues de main.main) -> d2mpq recompilado -> igual.
# Y el mismo d2mpq recompilado dentro de una camada TODO recompilado da un 1ST_READ.BIN byte a byte identico
# al de esa camada (0 offsets distintos en .go_export): el flujo esta bien cuando el CONJUNTO es coherente.
#
# Uso:  scripts/console/recompilar-cierre-motor.sh <paquete> [dir_gox] [--fuentes <commit>] [--seco]
#       <paquete>  ruta del paquete en el repo, p.ej. d2common/d2fileformats/d2mpq
#       --fuentes  compila las fuentes de ese commit (git show) en vez del arbol de trabajo
#       --seco     solo dice que compilaria
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
PAQ=""
GEN=""
FUENTES=""
SECO=0
while [ $# -gt 0 ]; do
    case "$1" in
        --fuentes) FUENTES="$2"; shift 2 ;;
        --seco) SECO=1; shift ;;
        *) if [ -z "$PAQ" ]; then PAQ="$1"; elif [ -z "$GEN" ]; then GEN="$1"; fi; shift ;;
    esac
done
[ -n "$PAQ" ] || { echo "uso: recompilar-cierre-motor.sh <paquete> [dir_gox] [--fuentes <commit>] [--seco]" >&2; exit 1; }
GEN="${GEN:-${GODC_GOX:-$DC_ROOT/gox}}"
PREFIJO="github.com/OpenDiablo2/OpenDiablo2"

cd "$REPO"

# el paquete + su cierre, capa por capa (los de adentro primero)
CAPAS=$(python3 "$HERE/cierre-dependientes.py" "$PAQ" --capas)
N=0
for c in $CAPAS; do N=$((N + $(echo "$c" | wc -w))); done
echo "== rearme con CIERRE de $PAQ -> $GEN"
echo "   $N paquetes dependen (transitivamente) de el, en $(echo "$CAPAS" | wc -l) capas"
[ "$SECO" -eq 1 ] && { echo "$PAQ"; echo "$CAPAS" | tr ' ' '\n'; exit 0; }

compila() {   # $1 = paquete
    local p="$1"
    # los mismos filtros que build-motor-all.sh / recompilar-paquete-motor.sh
    local srcs
    srcs=$(ls "$REPO/$p"/*.go | grep -v _test.go |
        grep -vE '_(darwin|windows|js|wasm|plan9|freebsd|netbsd|openbsd|linux|android|ios|solaris|aix)\.go$' |
        grep -vE '_(386|amd64|arm|arm64|ppc64|ppc64le|mips|mipsle|mips64|mips64le|riscv64|s390x|loong64|wasm)\.go$')
    local out="$GEN/$PREFIJO/$p.gox"
    mkdir -p "$(dirname "$out")"
    if [ -n "$FUENTES" ]; then
        # las fuentes del commit pedido, a un dir temporal con el mismo nombre de archivo
        local tmp="/tmp/dc229-fuentes-$FUENTES/$p"
        mkdir -p "$tmp"
        local f caso
        for f in $(git ls-tree --name-only "$FUENTES" "$p/"); do
            caso=$(basename "$f")
            case "$caso" in *_test.go|"") continue;; esac
            git show "$FUENTES:$f" > "$tmp/$caso"
        done
        srcs=$(ls "$tmp"/*.go)
    fi
    # shellcheck disable=SC2086
    if ! sh-elf-gccgo $GODC_GOFLAGS -I "$GEN" -fgo-pkgpath="$PREFIJO/$p" -c $srcs -o "$out" > "/tmp/cierre-$(echo "$p" | tr / _).log" 2>&1; then
        echo "   FALLO $p:"; tail -5 "/tmp/cierre-$(echo "$p" | tr / _).log"; return 1
    fi
    printf "   %-56s %8s bytes\n" "$p.gox" "$(stat -c%s "$out")"
    grep -qxF "$out" "$GEN/orden-motor.txt" 2>/dev/null || echo "$out" >> "$GEN/orden-motor.txt"
    return 0
}

fallan=0
{ echo "$PAQ"; echo "$CAPAS" | tr ' ' '\n'; } > /tmp/dc229-cierre-lista.txt
while read -r p; do
    [ -n "$p" ] || continue
    compila "$p" || fallan=$((fallan + 1))
done < /tmp/dc229-cierre-lista.txt
echo "== rearme con cierre terminado: $N recompilados, $fallan fallaron"
[ "$fallan" -eq 0 ]
