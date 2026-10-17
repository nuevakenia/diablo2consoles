#!/bin/bash
# Regla de la casa de la stdlib minima (console/dc/std): un paquete de d2std NO importa la biblioteca
# estandar de Go. Lo que necesita lo escribe adentro (go.mod, DC-002) y lo importa como "d2std/xxx".
#
# POR QUE ESTO ES UN CHEQUEO Y NO UN COMENTARIO (medido en DC-002.8a):
#   io/ioutil/ioutil.go decia `import "io"`. En la CONSOLA ese nombre resuelve a NUESTRO io.gox (el
#   build reescribe "d2std/" -> ""), pero en el HOST resuelve al io de Go. El mismo texto significaba
#   dos cosas distintas segun quien compilara, y la prueba del host terminaba compilando contra la
#   biblioteca de Go sin decirlo: el defecto era INVISIBLE justamente en el host, que es donde se
#   comparaba. Un nombre desnudo no se distingue de una dependencia externa; uno con "d2std/" si.
#
# Lo unico de afuera permitido es "unsafe": es una primitiva del lenguaje (no un paquete de la
# biblioteca) y el compilador de la consola la tiene.
#
# difftest/ y cmd/ quedan afuera a proposito: son las herramientas del HOST, que comparan contra la
# biblioteca estandar de verdad. Es su trabajo importarla.
#
# Uso:  scripts/console/checar-imports-d2std.sh      (0 = limpio, 1 = hay violaciones)
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STD="${STD:-$HERE/../../console/dc/std}"

if [ ! -d "$STD" ]; then
    echo "no encuentro la stdlib minima en $STD" >&2
    exit 2
fi

PAQUETES=0
ARCHIVOS=0
IMPORTS=0
VIOLACIONES=0
DETALLE=""

for d in $(find "$STD" -type d -not -path '*/difftest*' -not -path '*/cmd*' | sort); do
    hay_go=0
    for f in "$d"/*.go; do
        [ -f "$f" ] || continue
        hay_go=1
        ARCHIVOS=$((ARCHIVOS + 1))
    done
    [ "$hay_go" = 1 ] || continue
    PAQUETES=$((PAQUETES + 1))
    for f in "$d"/*.go; do
        [ -f "$f" ] || continue
        # imprime "linea:ruta" por cada import de afuera
        salida="$(awk '
            function chequear(l,   p) {
                if (match(l, /"[^"]+"/)) {
                    p = substr(l, RSTART + 1, RLENGTH - 2)
                    if (p != "unsafe" && p != "C" && p !~ /^d2std\//) {
                        printf "%d:%s\n", FNR, p
                    }
                }
            }
            /^[[:space:]]*import[[:space:]]*\(/ { enc = 1; next }
            enc && /^[[:space:]]*\)/           { enc = 0; next }
            enc                                { chequear($0); next }
            /^[[:space:]]*import[[:space:]]+"/ { chequear($0) }
        ' "$f")"
        [ -n "$salida" ] || continue
        IMPORTS=$((IMPORTS + $(printf '%s\n' "$salida" | wc -l)))
        while IFS= read -r v; do
            [ -n "$v" ] || continue
            VIOLACIONES=$((VIOLACIONES + 1))
            DETALLE="$DETALLE
  ${f#$STD/}:${v%:*}  import \"${v##*:}\""
        done <<< "$salida"
    done
done

echo "== imports de la stdlib de Go en los paquetes de d2std =="
echo "paquetes revisados: $PAQUETES ($ARCHIVOS archivos .go; difftest/ y cmd/ no se revisan)"
echo "imports de afuera: $IMPORTS   (violaciones: $VIOLACIONES; lo permitido es \"unsafe\")"
if [ "$VIOLACIONES" -gt 0 ]; then
    echo "$DETALLE"
    echo
    echo "ROJO: esos imports resuelven a la biblioteca de Go en el host. Van como \"d2std/...\"."
    exit 1
fi
echo "VERDE: ningun paquete de d2std importa la biblioteca estandar de Go"
exit 0
