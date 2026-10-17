#!/bin/bash
# CORPUS DE DC-002.15: el init que escribia en un mapa nil, su arreglo y el panic que queda.
#
# Lo que mide:
#  1. CONTROL SANO -- el panic del mapa nil NO vuelve: el arranque recompilado corre y su panic, si hay,
#     es OTRO, y la traza de libgodc lo identifica (hoy: un *regexp.Error, o sea un MustCompile de nivel
#     de paquete que no compila en la consola).
#  2. MUTANTE `init-de-archivo` -- se deshace el arreglo (doTable vuelve a crearse dentro de un init() de
#     class.go) y el corpus se tiene que poner ROJO: el arranque vuelve a morir en
#     "assignment to entry in nil map". Es el mutante del defecto de esta tarjeta.
#  3. CONTROL SANO DE VUELTA -- con el arreglo repuesto, vuelve el veredicto del punto 1.
#
# Por que el control no pide "MOTOR-ARRANQUE > 0": el arranque del motor tiene OTRO defecto delante (el
# regexp de d2mapgen, medido y declarado en la tarjeta DC-002.16). Lo que esta tarjeta mide es que el mapa
# nil dejo de ser el que corta, no que el motor llegue a main.
#
# Uso:  bash scripts/console/checar-init-215.sh [dir_gox] [dir_build] [segundos]
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

GEN="${1:-/tmp/gox-0028}"
BUILD="${2:-/tmp/dcbuild/motor-215}"
SEGS="${3:-15}"
PAQ="d2common/d2skill"
REPO="${REPO:-$(cd "$HERE/../.." && pwd)}"
CLASS="$REPO/$PAQ/class.go"
FIJO=/tmp/checar-215-class-fijo.go
CAZADOS=0
TOTAL=1

[ -f "$CLASS" ] || { echo "falta $CLASS" >&2; exit 1; }
grep -q '^var doTable = map\[int\]doFn{' "$CLASS" || {
    echo "class.go NO tiene el arreglo de DC-002.15 (doTable tiene que ser una variable inicializada):" >&2
    echo "  corre python3 scripts/console/fix-d2skill-dotable-215.py" >&2
    exit 1
}

recompilar() {
    local srcs
    srcs=$(ls "$HERE/../../$PAQ"/*.go | grep -v _test.go)
    # shellcheck disable=SC2086
    sh-elf-gccgo $GODC_GOFLAGS -I "$GEN" -fgo-pkgpath="github.com/OpenDiablo2/OpenDiablo2/$PAQ" -c $srcs \
        -o "$GEN/github.com/OpenDiablo2/OpenDiablo2/$PAQ.gox" || return 1
}

correr() { # $1 = dir de la corrida -> imprime el veredicto
    recompilar || { echo "no-compila"; return; }
    bash "$HERE/build-motor-arranque.sh" "$BUILD" "$GEN" > "$BUILD.build.log" 2>&1 || { echo "no-enlaza"; return; }
    rm -rf "$1"
    CAPTURE=none bash "$HERE/run-flycast.sh" "$BUILD/prog-emu.elf" "$SEGS" "$1" > "$1.salida" 2>&1
    if grep -qa 'nil map' "$1/flycast.log"; then
        echo "mapa-nil"
    elif grep -qa 'MOTOR-ARRANQUE' "$1/flycast.log"; then
        echo "arranca $(grep -ac 'MOTOR-ARRANQUE' "$1/flycast.log") marcas"
    else
        # la traza de libgodc (scripts/console/fix-libgodc-map-traza.py): pc del que paniqueo y tipo del dato
        local pc tipo nombre
        pc=$(grep -a -m1 'PANIC-TRAZA' "$1/flycast.log" | tr -d '\r' | sed 's/.*pc=\([0-9a-f]*\).*/\1/')
        tipo=$(grep -a -m1 'PANIC-TRAZA' "$1/flycast.log" | tr -d '\r' | sed 's/.*tipo=\([0-9a-f]*\).*/\1/')
        nombre=$(sh-elf-nm -n "$BUILD/prog.elf" 2>/dev/null | awk -v t="$((16#$tipo))" '{a=strtonum("0x" $1); if (a<=t) n=$3} END{print n}')
        if [ -n "$pc" ]; then
            echo "otro-panico pc=$pc (el llamado, pc-4, es: $(sh-elf-addr2line -f -e "$BUILD/prog.elf" "$(printf '%x' $((0x$pc - 4)))" 2>/dev/null | head -2 | tr '\n' ' ')) dato=<${nombre:-?>}>"
        else
            echo "otro-panico sin-traza"
        fi
    fi
}

echo "== 1. control sano: el panic del mapa nil no vuelve"
v="$(correr /tmp/checar-215-sano)"
echo "   veredicto: $v"
case "$v" in
mapa-nil*)
    echo "   EL CONTROL 1 NO DA: sigue muriendo en el mapa nil" >&2
    exit 1
    ;;
otro-panico*regexp*)
    echo "   OK: el mapa nil quedo atras y el panic que queda esta identificado (regexp)"
    ;;
otro-panico*)
    echo "   OK a medias: el mapa nil quedo atras, pero el panic nuevo NO se identifico como regexp:" >&2
    echo "     $v" >&2
    ;;
*)
    echo "   el arranque paso de largo (revisar el log antes de creerlo): $v"
    ;;
esac

echo
echo "== mutante 'init-de-archivo' (doTable vuelve a crearse en un init() de class.go)"
cp "$CLASS" "$FIJO"
python3 - "$CLASS" <<'PY'
import pathlib, sys
p = pathlib.Path(sys.argv[1])
t = p.read_text()
i = t.index("var doTable = map[int]doFn{\n")
j = t.index("\n}\n", i) + 3
cuerpo = t[i + len("var doTable = map[int]doFn{\n"):j - 3]
cuerpo = "\n".join(("\t" + l) if l.strip() else l for l in cuerpo.split("\n"))
nuevo = ("var doTable map[int]doFn\n\nfunc init() {\n\tdoTable = map[int]doFn{\n" + cuerpo + "\n\t}\n}\n")
p.write_text(t[:i] + nuevo + t[j:])
print("   class.go mutado: doTable vuelve al init() del archivo")
PY
v="$(correr /tmp/checar-215-mutante)"
echo "   veredicto: $v"
case "$v" in
mapa-nil*)
    echo "   cazado: el arranque vuelve a morir en el mapa nil, que es el defecto de esta tarjeta"
    CAZADOS=$((CAZADOS + 1))
    ;;
*) echo "   NO LO CAZO ($v)" >&2 ;;
esac

echo
echo "== control sano de vuelta (el arreglo repuesto)"
cp "$FIJO" "$CLASS"
v="$(correr /tmp/checar-215-vuelta)"
echo "   veredicto: $v"
case "$v" in
mapa-nil*) echo "   NO VOLVIO EL CONTROL 1" >&2; exit 1 ;;
*) echo "   control sano VERDE" ;;
esac

echo
echo "== $CAZADOS/$TOTAL mutantes cazados =="
[ "$CAZADOS" -eq "$TOTAL" ]
