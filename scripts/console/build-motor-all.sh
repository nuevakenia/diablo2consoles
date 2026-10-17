#!/bin/bash
# DC-002.8: compila los paquetes REALES del motor para SH-4 por CAPAS y reporta N/M con los bloqueos
# reales (paquete + simbolo exacto que falta), en vez de suponer cuanto falta.
#
# Como funciona la cuenta: se intenta compilar todos; los que fallan se reintentan en la capa
# siguiente, porque lo mas probable es que fallaran por una dependencia que si se compilo en esta
# pasada. Se corta cuando no queda ninguno o cuando una capa no desbloquea nada nuevo.
#
# Uso:  scripts/console/build-motor-all.sh [dir_gox] [dir_log]
#       Requiere la stdlib minima ya compilada: scripts/console/build-std-gox.sh
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
GEN="${1:-${GODC_GOX:-$DC_ROOT/gox}}"
LOG="${2:-/tmp/dcbuild/motor-all}"
PREFIJO="github.com/OpenDiablo2/OpenDiablo2"
DIRS="d2common d2core d2game d2app d2networking d2script d2thread"

mkdir -p "$LOG"
rm -f "$LOG"/*.log "$LOG/bloqueos.txt"

# la lista de paquetes del motor: los directorios con .go, sin los de prueba
PAQ=$(cd "$REPO" && find $DIRS -name '*.go' -not -name '*_test.go' | xargs -n1 dirname | sort -u)
TOTAL=$(echo "$PAQ" | wc -l)
OK=0
PENDIENTES="$PAQ"

# orden.txt lo escribe build-motor-gox.sh; aca se arranca de cero para que no se mezcle
: > "$GEN/orden-motor.txt"

for CAPA in $(seq 1 20); do
    FALLARON=""
    for p in $PENDIENTES; do
        # gccgo NO filtra por sufijo de plataforma como hace el go de verdad: si se le pasan
        # _darwin.go o _windows.go los intenta compilar (uno importa "C") y el paquete "falla" por un
        # archivo que no tiene nada que ver con la Dreamcast. Se excluyen a mano.
        srcs=$(ls "$REPO/$p"/*.go 2>/dev/null | grep -v _test.go |
            grep -vE '_(darwin|windows|js|wasm|plan9|freebsd|netbsd|openbsd|linux|android|ios|solaris|aix)\.go$' |
            grep -vE '_(386|amd64|arm|arm64|ppc64|ppc64le|mips|mipsle|mips64|mips64le|riscv64|s390x|loong64|wasm)\.go$')
        [ -n "$srcs" ] || continue
        out="$GEN/$PREFIJO/$p.gox"
        mkdir -p "$(dirname "$out")"
        logp="$LOG/$(echo "$p" | tr / _).log"
        # shellcheck disable=SC2086
        if sh-elf-gccgo $GODC_GOFLAGS -I "$GEN" -fgo-pkgpath="$PREFIJO/$p" -c $srcs -o "$out" > "$logp" 2>&1; then
            OK=$((OK + 1))
            echo "$out" >> "$GEN/orden-motor.txt"
            rm -f "$logp"
        else
            FALLARON="$FALLARON $p"
        fi
    done
    echo "capa $CAPA: compilados $OK de $TOTAL"

    # OJO CON EL ORDEN, QUE ESTABA MAL: PENDIENTES guarda lo que FALTA y ANTERIOR lo de la vuelta previa.
    # El `break` salia ANTES de guardar los que fallaron, asi que el resumen de abajo informaba la lista de
    # la vuelta ANTERIOR: con el motor completo decia "bloqueados: 1" (el que quedaba en la lista vieja) en
    # vez de 0. Un numero de cierre que habla de otro momento es peor que no tenerlo.
    ANTERIOR="$PENDIENTES"
    PENDIENTES="$FALLARON"

    [ -z "${PENDIENTES// /}" ] && break

    # si una capa no desbloqueo nada, seguir no sirve: se corta y se reporta lo que falta
    [ -n "${ANTERIOR// /}" ] && [ "$PENDIENTES" = "$ANTERIOR" ] && {
        echo "capa $CAPA: no se desbloqueo nada nuevo, corto aca"
        break
    }
    PENDIENTES="$FALLARON"
done

echo
echo "== RESULTADO: $OK de $TOTAL paquetes del motor compilan para SH-4 =="
# el conteo de bloqueados: OJO, con la lista vacia `echo | wc -w` da 1 (el echo manda una linea vacia).
# Medido cuando el motor llego a 142: decia "bloqueados: 1" con la lista vacia.
if [ -z "${PENDIENTES// /}" ]; then
    echo "== bloqueados: 0 (el motor compila COMPLETO) =="
else
    echo "== bloqueados: $(echo "$PENDIENTES" | wc -w) =="
fi

{
    echo "# DC-002.8: el motor compilando por capas"
    echo
    echo "compilados: $OK de $TOTAL"
    echo
    echo "## los simbolos que faltan, ordenados por cuantos paquetes bloquean"
    echo "## el avance, paquete por paquete (1 = compila)"
    for p in $PAQ; do
        if grep -qxF "$GEN/$PREFIJO/$p.gox" "$GEN/orden-motor.txt" 2>/dev/null; then
            echo "$p 1"
        else
            echo "$p 0"
        fi
    done
    echo
    echo "## los simbolos que faltan (undefined:), ordenados por cuantos paquetes bloquean"
    grep -h 'undefined:' "$LOG"/*.log 2>/dev/null |
        sed 's/.*undefined: //' | sed 's/[[:space:]]*$//' |
        sort | uniq -c | sort -rn | head -40
    echo
    echo "## los paquetes bloqueados (primera linea de error de cada uno)"
    for p in $PENDIENTES; do
        logp="$LOG/$(echo "$p" | tr / _).log"
        [ -f "$logp" ] || continue
        echo "$p: $(grep -m1 -E 'error|undefined|cannot' "$logp" | cut -c1-160)"
    done
} > "$LOG/bloqueos.txt"

cat "$LOG/bloqueos.txt"
