#!/bin/bash
# Mutador del CONTROL de costo del POC de la stdlib (DC-002.8b).
#
# EL DEFECTO (medido antes de arreglarlo): el POC imprimia
#   STD-TEST: costo float64 (control, no tiene que ser 0): 0
# porque el control miraba bits64(suma)&1, y la suma de los bucles termina PAR: el bit bajo es siempre
# 0. O sea que el control no se ponia en rojo NUNCA, ni aunque el compilador se comiera un bucle y las
# mediciones de costo fueran mentira. Un control que no discrimina no es un control.
#
# EL ARREGLO: el control compara la suma contra un VALOR CONOCIDO (piso 1e7: los 1e5 terminos de la raiz
# dan ~2,1e7 por si solos) y ademas imprime el digest de la suma.
#
# LOS MUTANTES (uno por cada forma de mentir, y ninguno probabilistico: los dos tienen que ponerse rojos
# SIEMPRE):
#   raiz-sin-acumular : el bucle de la raiz corre pero no acumula  -> lo caza el PISO (control=0).
#   exp-distinto      : un termino da otro valor (cambia el resultado sin bajar el piso) -> lo caza el
#                       DIGEST (control=1 igual, pero la suma cambia).
#
# El respaldo es una copia propia + sha256 (nunca `git checkout --`: en el hito puede haber WIP de otra
# tarjeta en el mismo archivo).
#
# Uso:  scripts/console/mutar-std-consola.sh       (usa Flycast: ~40 s por caso)
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
POC_GO="$REPO/console/dc/poc/stdlib/main.go"
OUT="${OUT:-/tmp/dcbuild/stdlib}"
SEGS="${SEGS:-25}"
RUN="${RUN:-/tmp/dc-run-8b-mutante}"
BAK="/tmp/stdlib-main-original.go"

[ -f "$POC_GO" ] || { echo "no encuentro $POC_GO" >&2; exit 1; }
cp "$POC_GO" "$BAK"
SHA_ORIG="$(sha256sum "$BAK" | cut -d' ' -f1)"
echo "respaldo: $BAK ($SHA_ORIG)"

# corpus: compila el POC (con el mutante puesto), lo corre en Flycast y saca del log las tres cosas que
# importan: el valor del control, el digest de la suma y el recuento de casos de la suite.
# Imprime "control digest casos".
corpus() {
    local log="$RUN/flycast.log" linea control digest casos

    bash "$HERE/build-dc.sh" "$REPO/console/dc/poc/stdlib" "$OUT" >/dev/null 2>&1 || { echo "no-medido"; return; }
    rm -rf "$RUN"
    bash "$HERE/run-flycast.sh" "$OUT/prog.elf" "$SEGS" "$RUN" >/dev/null 2>&1
    [ -f "$log" ] || { echo "no-medido"; return; }

    linea="$(grep -a 'control, no tiene que ser 0' "$log" | tr -d '\r' | head -1)"
    control="$(echo "$linea" | sed -n 's/.*): \([0-9][0-9]*\) (.*/\1/p')"
    digest="$(echo "$linea" | sed -n 's/.*suma_bits=\([0-9a-f][0-9a-f]*\)).*/\1/p')"
    casos="$(grep -a '^STD-TEST: casos' "$log" | tr -d '\r' | sed 's/^STD-TEST: casos //' | head -1)"

    [ -n "$control" ] || { echo "no-medido"; return; }
    echo "$control ${digest:-?} ${casos:-?}"
}

mutar() { # <viejo> <nuevo>
    python3 - "$POC_GO" "$1" "$2" <<'PY'
import pathlib, sys
p = pathlib.Path(sys.argv[1]); viejo = sys.argv[2]; nuevo = sys.argv[3]
t = p.read_text(); n = t.count(viejo)
if n != 1:
    sys.exit("el texto a mutar aparece %d veces (esperaba 1)" % n)
p.write_text(t.replace(viejo, nuevo, 1))
PY
}

restaurar() {
    cp "$BAK" "$POC_GO"
    [ "$(sha256sum "$POC_GO" | cut -d' ' -f1)" = "$SHA_ORIG" ] || { echo "NO RESTAURO BIEN" >&2; exit 1; }
}

echo
echo "== control sano: el control tiene que dar 1 y la suite 0 mal (y de aca sale el digest de referencia)"
leido="$(corpus)"
CTRL="$(echo "$leido" | cut -d' ' -f1)"
DIGEST_REF="$(echo "$leido" | cut -d' ' -f2)"
CASOS="$(echo "$leido" | cut -d' ' -f3-)"
echo "   control=$CTRL digest=$DIGEST_REF casos=$CASOS"
if [ "$CTRL" != "1" ] || [ "$leido" = "no-medido" ]; then
    echo "   EL CONTROL SANO NO ESTA VERDE (se esperaba 1)" >&2
    restaurar
    exit 1
fi
case "$CASOS" in
    *"mal 0"*) ;;
    *) echo "   la suite no cerro con 0 mal: $CASOS" >&2; restaurar; exit 1 ;;
esac

cazados=0
total=0

for caso in \
    "raiz-sin-acumular|suma += math.Sqrt(x + float64(i))|suma += 0" \
    "exp-distinto|suma += math.Exp(0.5) + math.Log(2.5)|suma += math.Exp(0.6) + math.Log(2.5)"
do
    IFS='|' read -r nombre viejo nuevo <<<"$caso"
    total=$((total + 1))
    echo
    echo "== mutante '$nombre'"
    if ! mutar "$viejo" "$nuevo"; then
        echo "   NO SE PUDO APLICAR"; restaurar; continue
    fi
    leido="$(corpus)"
    c="$(echo "$leido" | cut -d' ' -f1)"; d="$(echo "$leido" | cut -d' ' -f2)"
    echo "   con el mutante: control=$c digest=$d"
    if [ "$leido" = "no-medido" ]; then
        echo "   NO SE PUDO MEDIR (el POC no arranco o el arbol no compila)"
    elif [ "$c" != "$CTRL" ]; then
        echo "   cazado por el PISO: el control se puso en 0 (antes $CTRL)"
        cazados=$((cazados + 1))
    elif [ "$d" != "$DIGEST_REF" ]; then
        echo "   cazado por el DIGEST: el control sigue en $c pero la suma cambio de $DIGEST_REF a $d"
        cazados=$((cazados + 1))
    else
        echo "   NO LO CAZO: el control y el digest dieron igual con el mutante puesto"
    fi
    restaurar
done

echo
echo "== control sano otra vez (con los archivos restaurados)"
leido="$(corpus)"
echo "   control=$(echo "$leido" | cut -d' ' -f1) digest=$(echo "$leido" | cut -d' ' -f2)"
if [ "$(echo "$leido" | cut -d' ' -f1)" != "1" ] || [ "$(echo "$leido" | cut -d' ' -f2)" != "$DIGEST_REF" ]; then
    echo "   EL CONTROL SANO NO VOLVIO (el arbol cambio bajo los pies?)" >&2
    exit 1
fi

echo
echo "== $cazados/$total mutantes cazados =="
[ "$cazados" -eq "$total" ]
