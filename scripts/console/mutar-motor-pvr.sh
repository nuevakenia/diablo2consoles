#!/bin/bash
# Mutador de DC-002.9 (motor-pvr): la comparacion de la CAPTURA tiene que CAZAR los cambios del camino
# del color.
#
# Por que existe (regla del proyecto): una comparacion que da 100% no prueba nada por si sola. Este
# script cambia UNA cosa por vez en el POC que pinta (el puente a C del PVR y el frame que se dibuja),
# recompila, corre el emulador y compara la captura contra la referencia del host. Cada mutante tiene
# que poner la comparacion en ROJO. Si alguno queda verde, el hueco es de la comparacion (estaria
# midiendo algo que no depende de lo que dice medir).
#
# Los mutantes son los del camino del COLOR, que es lo que agrega DC-002.9:
#   alpha0 : el indice 0 deja de ser transparente (pinta el color de la paleta 0)
#   uv1    : la UV derecha pasa a 1.0 y muestrea el relleno de la textura (el arte sale corrido)
#   pal1   : la textura usa el BANCO 1 de la paleta, que esta vacio (todo transparente)
#   frame1 : se dibuja el frame 1 con la referencia del frame 0
#
# El respaldo es una copia propia + sha256 (no `git checkout --`: el arbol puede tener WIP de otra
# tarjeta encima).
#
# Uso:  scripts/console/mutar-motor-pvr.sh
#       Necesita la stdlib minima ya compilada en $GODC_GOX (scripts/console/build-std-gox.sh).
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
OUT="${OUT:-/tmp/dcbuild/motor-pvr}"
POC="$REPO/console/dc/poc/motor-pvr"
CSHIM="$POC/csrc/cshim.c"
MAIN="$POC/main.go"
BAK_CSHIM="/tmp/motor-pvr-cshim-original.c"
BAK_MAIN="/tmp/motor-pvr-main-original.go"
ESPERADO_ARTE="${ESPERADO_ARTE:-4051}"

for f in "$CSHIM" "$MAIN"; do
    [ -f "$f" ] || { echo "no encuentro $f" >&2; exit 1; }
done
[ -f "${GODC_GOX:-$DC_ROOT/gox}/fmt.gox" ] || {
    echo "falta la stdlib minima en ${GODC_GOX:-$DC_ROOT/gox}: corre scripts/console/build-std-gox.sh" >&2
    exit 1
}

cp "$CSHIM" "$BAK_CSHIM"
cp "$MAIN" "$BAK_MAIN"
SHA_CSHIM="$(sha256sum "$BAK_CSHIM" | cut -d' ' -f1)"
SHA_MAIN="$(sha256sum "$BAK_MAIN" | cut -d' ' -f1)"
echo "respaldos: $BAK_CSHIM ($SHA_CSHIM), $BAK_MAIN ($SHA_MAIN)"

# corpus: compila el POC (con el mutante puesto), lo corre en Flycast con captura y compara la captura
# contra la referencia del frame 0 que deja el comparador del host. Imprime "arte transparentes
# veredicto": los pixeles de arte, los pixeles transparentes y el veredicto de compare-capture.py.
#
# OJO (medido: el mutante 'alpha0' quedaba verde con la primera version de este mutador): mirar SOLO los
# pixeles de arte no alcanza, porque el indice 0 transparente no cambia ni un pixel de arte y si cambia
# los transparentes. El veredicto del comparador es el que mira las DOS cosas.
#
# SALTAR_STDLIB=1: la stdlib no cambia entre mutantes y rehacerla borra la gox compartida (DC-002.9 se
# midio con otra sesion trabajando el repo, en su propia gox).
corpus() {
    local ref="$OUT/ref-frame0.png" cap salida arte trans veredicto

    SALTAR_STDLIB=1 bash "$HERE/build-motor-pvr.sh" "$OUT" >/dev/null 2>&1 || { echo "no-medido"; return; }
    CAPTURE=spectacle bash "$HERE/run-flycast.sh" "$OUT/prog.elf" 12 "$OUT/run" >/dev/null 2>&1
    (cd "$REPO" && go run ./console/dc/motorcmp -dc6 "$OUT/d2logo.dc6" -pal "$OUT/d2logo.pal" \
        -png "$ref" >/dev/null 2>&1) || { echo "no-medido"; return; }

    cap="$(ls -t "$OUT"/run/f*.png 2>/dev/null | head -1)"
    [ -n "$cap" ] || { echo "no-medido"; return; }

    salida="$(python3 "$HERE/compare-capture.py" --captura "$cap" --referencia "$ref" \
        --fondo 89 0 140 --pos 40 300 2>/dev/null)"
    [ -n "$salida" ] || { echo "no-medido"; return; }

    arte="$(echo "$salida" | awk '/pixeles de arte:/ {print $4}')"
    trans="$(echo "$salida" | awk '/pixeles transparentes:/ {print $3}')"
    veredicto="$(echo "$salida" | awk '/^RESULTADO:/ {print $2}')"
    echo "${arte:-?} ${trans:-?} ${veredicto:-?}"
}

# mutar <archivo> <viejo> <nuevo>: aplica si el texto aparece 1 sola vez
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
    cp "$BAK_CSHIM" "$CSHIM"
    cp "$BAK_MAIN" "$MAIN"
    local a b
    a="$(sha256sum "$CSHIM" | cut -d' ' -f1)"
    b="$(sha256sum "$MAIN" | cut -d' ' -f1)"
    [ "$a" = "$SHA_CSHIM" ] && [ "$b" = "$SHA_MAIN" ] || { echo "NO RESTAURO BIEN" >&2; exit 1; }
}

ALPHA_OK='unsigned int alpha = (i == 0) ? 0u : 1u;'
ALPHA_MUT='unsigned int alpha = 1u;'
UV_OK='    u1 = ((float)W - 0.5f) / (float)PW;'
UV_MUT='    u1 = 1.0f;'
PAL_OK='PVR_TXRFMT_8BPP_PAL(0)'
PAL_MUT='PVR_TXRFMT_8BPP_PAL(1)'
FRAME_OK='idx := d.DecodeFrame(0)'
FRAME_MUT='idx := d.DecodeFrame(1)'

cazados=0
total=0

for caso in \
    "alpha0|$CSHIM|$ALPHA_OK|$ALPHA_MUT" \
    "uv1|$CSHIM|$UV_OK|$UV_MUT" \
    "pal1|$CSHIM|$PAL_OK|$PAL_MUT" \
    "frame1|$MAIN|$FRAME_OK|$FRAME_MUT"
do
    IFS='|' read -r nombre archivo viejo nuevo <<<"$caso"
    total=$((total + 1))
    echo
    echo "== mutante '$nombre' (en ${archivo#$REPO/})"
    if ! mutar "$archivo" "$viejo" "$nuevo"; then
        echo "   NO SE PUDO APLICAR (el texto no esta como se esperaba)"; restaurar; continue
    fi
    leido="$(corpus)"
    arte="$(echo "$leido" | cut -d' ' -f1)"
    trans="$(echo "$leido" | cut -d' ' -f2)"
    veredicto="$(echo "$leido" | cut -d' ' -f3)"
    echo "   captura con el mutante: arte=$arte transparentes=$trans veredicto=${veredicto:-?}"
    if [ "${veredicto:-}" = "NO" ]; then
        echo "   cazado: la comparacion se puso ROJA"
        cazados=$((cazados + 1))
    elif [ "$leido" = "no-medido" ]; then
        echo "   NO SE PUDO MEDIR (captura: ventana tapada, o el emulador no arranco)"
    else
        echo "   NO LO CAZO: la comparacion siguio VERDE con el mutante puesto"
    fi
    restaurar
done

echo
echo "== control sano (todo restaurado)"
if [ "$(sha256sum "$CSHIM" | cut -d' ' -f1)" != "$SHA_CSHIM" ] ||
   [ "$(sha256sum "$MAIN" | cut -d' ' -f1)" != "$SHA_MAIN" ]; then
    echo "los archivos no volvieron al original" >&2; exit 1
fi
leido="$(corpus)"
arte="$(echo "$leido" | cut -d' ' -f1)"
trans="$(echo "$leido" | cut -d' ' -f2)"
veredicto="$(echo "$leido" | cut -d' ' -f3)"
echo "   captura: arte=$arte transparentes=$trans veredicto=${veredicto:-?}"
if [ "$arte" = "$ESPERADO_ARTE/$ESPERADO_ARTE" ] && [ "$veredicto" = "OK" ]; then
    echo "   control sano VERDE ($arte)"
else
    echo "   CONTROL SANO NO ESTA VERDE (se esperaba $ESPERADO_ARTE/$ESPERADO_ARTE y veredicto OK)" >&2
    exit 1
fi

echo
echo "== $cazados/$total mutantes cazados =="
[ "$cazados" -eq "$total" ]
