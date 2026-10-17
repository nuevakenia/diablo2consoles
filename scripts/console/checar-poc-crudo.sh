#!/bin/bash
# La prueba del AVISO de build-dc.sh para los POC de asset CRUDO (DC-002.12).
#
# Por que existe: los POC motor-dc6, motor-pvr y motor-300 no traen un .h horneado sino los BYTES CRUDOS
# del MPQ como var de Go, y ademas necesitan los .gox del MOTOR. Armarlos con build-dc.sh a mano no es el
# camino, y lo que contestaba era el compilador:
#
#     console/dc/poc/motor-pvr/main.go:53:30: error: reference to undefined name ‘assetDC6’
#
# Ese error manda a buscar el problema donde NO esta: en DC-002.10 quedo escrito en
# console/dc/poc/motor-pvr/ESTADO.md como "el horneador no genera los bytes" y "hay que ensenarle a leer
# asset.conf", cuando el camino correcto ya existia (build-motor-dc6.sh hornea los bytes crudos con
# console/dc/bake-dc6 y compila los .gox del motor). Medido en DC-002.12: el POC compila, corre y da las
# mismas 61 lineas que el host.
#
# Este script es el corpus de ese aviso:
#   control sano: build-dc.sh sobre un POC crudo AVISA (exit 2, con el nombre del script correcto) y NO
#                 llega a compilar.
#   mutante:      el aviso sacado (reemplazo EXACTO de la condicion, una sola vez) -> el corpus tiene que
#                 volver a ver el error viejo. Si el mutante queda "avisado", el corpus no mide el aviso.
#
# Uso:  scripts/console/checar-poc-crudo.sh
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
BUILD="$HERE/build-dc.sh"
POC="$REPO/console/dc/poc/motor-pvr"
OUT="${OUT:-/tmp/checar-poc-crudo}"
SALIDA="${SALIDA:-/tmp/checar-poc-crudo.log}"
BACKUP="/tmp/checar-poc-crudo-build-dc-original.sh"

[ -f "$POC/asset.conf" ] || { echo "no encuentro $POC/asset.conf" >&2; exit 1; }
[ -f "$BUILD" ] || { echo "no encuentro $BUILD" >&2; exit 1; }

cp "$BUILD" "$BACKUP"
SHA="$(sha256sum "$BACKUP" | cut -d' ' -f1)"
echo "respaldo: $BACKUP ($SHA)"

# corpus: corre build-dc.sh sobre el POC crudo y dice que paso.
#   "avisado <rc>"  el aviso salio, con el nombre del script correcto
#   "crudo <rc>"    el compilador se quejo de assetDC6: el aviso no esta
#   "otro <rc>"     cualquier otra cosa (se imprime la salida para verla)
corpus() {
    rm -rf "$OUT"

    bash "$BUILD" "$POC" "$OUT" > "$SALIDA" 2>&1
    local rc=$?

    if grep -q 'es un POC de asset CRUDO' "$SALIDA" && grep -q 'build-motor-pvr.sh' "$SALIDA"; then
        echo "avisado $rc"
    elif grep -q 'undefined name' "$SALIDA" && grep -q 'assetDC6' "$SALIDA"; then
        echo "crudo $rc"
    else
        echo "otro $rc"
    fi
}

# mutar <archivo> <viejo> <nuevo>: aplica si el texto aparece una sola vez
mutar() {
    python3 - "$1" "$2" "$3" <<'PY'
import pathlib, sys
p = pathlib.Path(sys.argv[1]); viejo = sys.argv[2]; nuevo = sys.argv[3]
t = p.read_text(); n = t.count(viejo)
if n != 1:
    sys.exit("el texto a mutar aparece %d veces (esperaba 1)" % n)
p.write_text(t.replace(viejo, nuevo, 1))
PY
}

restaurar() {
    cp "$BACKUP" "$BUILD"
    local a
    a="$(sha256sum "$BUILD" | cut -d' ' -f1)"
    [ "$a" = "$SHA" ] || { echo "NO RESTAURO BIEN build-dc.sh" >&2; exit 1; }
}

echo
echo "== control sano (el aviso puesto)"
leido="$(corpus)"
estado="$(echo "$leido" | cut -d' ' -f1)"
rc="$(echo "$leido" | cut -d' ' -f2)"
echo "   build-dc.sh sobre console/dc/poc/motor-pvr: $estado (exit $rc)"
if [ "$estado" != "avisado" ]; then
    echo "   EL CONTROL SANO NO AVISO (se esperaba 'avisado' y dio '$estado'): la salida esta en $SALIDA" >&2
    sed -n '1,8p' "$SALIDA" | sed 's/^/     /' >&2
    exit 1
fi
sed -n '1,3p' "$SALIDA" | sed 's/^/     /'

TOTAL=1
CAZADOS=0

echo
echo "== mutante 'sin-aviso' (se saca la condicion del aviso en scripts/console/build-dc.sh)"
if mutar "$BUILD" '[ -f "$SRC/asset.conf" ]' '[ -f "$SRC/no-existe.conf" ]'; then
    leido="$(corpus)"
    estado="$(echo "$leido" | cut -d' ' -f1)"
    rc="$(echo "$leido" | cut -d' ' -f2)"
    echo "   build-dc.sh con el aviso sacado: $estado (exit $rc)"
    if [ "$estado" = "crudo" ]; then
        echo "   cazado: sin el aviso el error vuelve a ser el que manda a buscar donde no esta"
        grep -m1 'undefined name' "$SALIDA" | sed 's/^/     /'
        CAZADOS=1
    else
        echo "   NO LO CAZO: con el aviso sacado el corpus siguio en '$estado' (esperaba 'crudo')" >&2
    fi
else
    echo "   NO SE PUDO APLICAR EL MUTANTE (el texto no esta como se esperaba)" >&2
fi
restaurar

echo
echo "== control sano de vuelta (el aviso repuesto)"
leido="$(corpus)"
estado="$(echo "$leido" | cut -d' ' -f1)"
rc="$(echo "$leido" | cut -d' ' -f2)"
echo "   build-dc.sh: $estado (exit $rc)"
if [ "$estado" != "avisado" ]; then
    echo "   EL AVISO NO VOLVIO" >&2; exit 1
fi
if [ "$rc" != "2" ]; then
    echo "   el codigo de salida esperado era 2 y fue $rc" >&2; exit 1
fi
echo "   control sano VERDE (avisado, exit 2)"

echo
echo "== $CAZADOS/$TOTAL mutantes cazados =="
[ "$CAZADOS" -eq "$TOTAL" ]
