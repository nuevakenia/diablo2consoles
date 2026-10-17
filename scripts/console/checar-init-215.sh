#!/bin/bash
# CORPUS DE DC-002.15 (apretado en DC-002.16): el init que escribia en un mapa nil, su arreglo y el
# arranque del motor que quedo detras.
#
# Lo que mide:
#  1. CONTROL SANO -- el panic del mapa nil NO vuelve y el arranque LLEGA A MAIN: el veredicto tiene que
#     ser "arranca N marcas" (N lineas MOTOR-ARRANQUE) y el log no puede tener ningun panic. Desde
#     DC-002.16 el segundo defecto que estaba delante (el regexp con (?i) de d2mapgen) esta cerrado, asi
#     que el control ya puede pedir esto: si el panic del regexp vuelve, este control se pone ROJO.
#  2. MUTANTE `init-de-archivo` -- se deshace el arreglo (doTable vuelve a crearse dentro de un init() de
#     class.go) y el corpus se tiene que poner ROJO: el arranque vuelve a morir en
#     "assignment to entry in nil map". Es el mutante del defecto de DC-002.15.
#  3. CONTROL SANO DE VUELTA -- con el arreglo repuesto, vuelve el veredicto del punto 1.
#
# Medido, con la gox de DC-002.16 (/tmp/gox-00216: la stdlib minima con el regexp arreglado):
#   antes  (gox de DC-002.15): "otro-panico pc=8c44b4ba dato=<_regexp.Error..p>" y 0 marcas MOTOR-ARRANQUE
#   ahora  (gox de DC-002.16): "arranca 9 marcas", 0 lineas con "panic:"
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

echo "== 1. control sano: el panic del mapa nil no vuelve, y el arranque llega a main"
v="$(correr /tmp/checar-215-sano)"
echo "   veredicto: $v"
case "$v" in
mapa-nil*)
    echo "   EL CONTROL 1 NO DA: sigue muriendo en el mapa nil" >&2
    exit 1
    ;;
otro-panico*regexp*|otro-panico*_regexp*)
    echo "   EL CONTROL 1 NO DA: VOLVIO el panic del regexp que cerro DC-002.16 (el arranque no llega a main):" >&2
    echo "     $v" >&2
    exit 1
    ;;
arranca*)
    echo "   OK: el arranque llega a main ($v)"
    # y que no sea "llega a main y despues paniquea": las marcas no alcanzan si el log tiene un panic
    n_panic="$(grep -ac 'panic:' /tmp/checar-215-sano/flycast.log)"
    echo "   lineas con \"panic:\" en el log: $n_panic (tienen que ser 0)"
    if [ "$n_panic" -ne 0 ]; then
        grep -am1 'PANIC-TRAZA\|panic:' /tmp/checar-215-sano/flycast.log | tr -d '\r' | sed 's/^/     /' >&2
        echo "   EL CONTROL 1 NO DA: el arranque llega a main pero despues paniquea" >&2
        exit 1
    fi
    # las marcas, para que queden en la corrida: el motor entra a d2app y su Run() vuelve con el error
    # que da el sistema de archivos de solo lectura (el defecto que sigue, de otra tarjeta)
    grep -a 'MOTOR-ARRANQUE' /tmp/checar-215-sano/flycast.log | tr -d '\r' | sed 's/^/     /'
    ;;
otro-panico*)
    echo "   OK a medias: el mapa nil quedo atras, pero el panic nuevo NO es del regexp:" >&2
    echo "     $v" >&2
    ;;
*)
    echo "   veredicto inesperado (revisar el log antes de creerlo): $v" >&2
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
