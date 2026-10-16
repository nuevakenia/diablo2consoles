#!/bin/bash
# Mutador de DC-002.2f: los dos arreglos del runtime tienen que CAZARSE si se los saca.
#
# Por que existe (regla del proyecto): una suite en verde no prueba nada por si sola. Este script muta
# UNA cosa por vez en el runtime (libgodc), reconstruye y corre el corpus completo en la consola; el
# mutante tiene que poner el corpus en ROJO. Si alguno pasa desapercibido, la suite estaba de adorno.
#
#   firma   : implements_interface vuelve a mirar solo el nombre del metodo (sin la firma)
#   itabeq  : runtime_ifaceeq vuelve a comparar los punteros de itab
#
# El arbol de libgodc esta fuera del repo, asi que el respaldo es una copia propia + sha256 (no
# `git checkout --`: en ese arbol hay cambios del hito sin commitear).
#
# Uso:  scripts/console/mutar-libgodc-2f.sh
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
IFACE="$LIBGODC/runtime/interface_dreamcast.c"
OUT="/tmp/dcbuild/stdlib"
BAK="/tmp/libgodc-interface-original-2f.c"
ESPERADO_BIEN=593

[ -f "$IFACE" ] || { echo "no encuentro $IFACE" >&2; exit 1; }

cp "$IFACE" "$BAK"
SHA_ORIGINAL="$(sha256sum "$BAK" | cut -d' ' -f1)"
echo "respaldo del runtime: $BAK ($SHA_ORIGINAL)"

# corpus: construye, corre el emulador e imprime "bien mal"
corpus() {
    bash "$HERE/build-dc.sh" "$REPO/console/dc/poc/stdlib" "$OUT" >/dev/null 2>&1 || { echo "0 0"; return; }
    bash "$HERE/run-flycast.sh" "$OUT/prog.elf" 18 "$OUT/run" >/dev/null 2>&1
    grep -a "STD-TEST: casos" "$OUT/run/flycast.log" | tr -d '\r' | tail -1 | awk '{print $5, $7}'
}

# mutar <nombre> <viejo> <nuevo>: aplica si el texto aparece 1 vez, recompila el runtime y lo deja listo
mutar() {
    python3 - "$IFACE" "$2" "$3" <<'PY'
import pathlib, sys
p = pathlib.Path(sys.argv[1]); viejo = sys.argv[2]; nuevo = sys.argv[3]
t = p.read_text(); n = t.count(viejo)
if n != 1:
    sys.exit("el texto a mutar aparece %d veces (esperaba 1)" % n)
p.write_text(t.replace(viejo, nuevo, 1))
PY
}

restaurar() {
    cp "$BAK" "$IFACE"
    local ahora
    ahora="$(sha256sum "$IFACE" | cut -d' ' -f1)"
    [ "$ahora" = "$SHA_ORIGINAL" ] || { echo "NO RESTAURO BIEN ($ahora)" >&2; exit 1; }
    make -C "$LIBGODC" >/dev/null 2>&1
}

# El mutante de la firma no puede dejar a method_types_equal sin usar: el runtime compila con -Werror
# y -Wall, asi que una funcion static sin llamadores es un error de compilacion. Se la llama con el
# mismo descriptor de los dos lados: es "no mirar la firma" y compila igual.
FIRMA_OK='                if (!method_types_equal(lhsMethod->__typ, rhsMethod->__mtyp))'
FIRMA_MUT='                if (!method_types_equal(lhsMethod->__typ, lhsMethod->__typ))'
ITAB_OK='    if (itab_type(itab_from_iface(i1.itab)) != itab_type(itab_from_iface(i2.itab)))'
ITAB_MUT='    if (i1.itab != i2.itab)'

cazados=0
total=0

for caso in "firma|$FIRMA_OK|$FIRMA_MUT" "itabeq|$ITAB_OK|$ITAB_MUT"; do
    IFS='|' read -r nombre viejo nuevo <<<"$caso"
    total=$((total + 1))
    echo
    echo "== mutante '$nombre'"
    if ! mutar "$nombre" "$viejo" "$nuevo"; then
        echo "   NO SE PUDO APLICAR (el texto no esta como se esperaba)"; restaurar; continue
    fi
    make -C "$LIBGODC" >/dev/null 2>&1 || { echo "   el runtime no compila mutado"; restaurar; continue; }
    leido="$(corpus)"
    bien="$(echo "$leido" | cut -d' ' -f1)"; mal="$(echo "$leido" | cut -d' ' -f2)"
    echo "   corpus con el mutante: bien=$bien mal=$mal"
    if [ "${mal:-0}" -gt 0 ]; then
        echo "   cazado: el corpus se puso ROJO ($mal casos de $ESPERADO_BIEN)"
        cazados=$((cazados + 1))
    else
        echo "   NO LO CAZO: el corpus quedo verde con el arreglo sacado"
    fi
    restaurar
done

echo
echo "== control sano (runtime restaurado)"
if [ "$(sha256sum "$IFACE" | cut -d' ' -f1)" != "$SHA_ORIGINAL" ]; then
    echo "el archivo no volvio al original" >&2; exit 1
fi
leido="$(corpus)"
bien="$(echo "$leido" | cut -d' ' -f1)"; mal="$(echo "$leido" | cut -d' ' -f2)"
echo "   corpus: bien=$bien mal=$mal"
if [ "${bien:-0}" -eq "$ESPERADO_BIEN" ] && [ "${mal:-1}" -eq 0 ]; then
    echo "   control sano VERDE ($bien/$ESPERADO_BIEN)"
else
    echo "   CONTROL SANO NO ESTA VERDE" >&2
    exit 1
fi

echo
echo "== $cazados/$total chequeos cazados =="
[ "$cazados" -eq "$total" ]
