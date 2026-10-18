#!/bin/bash
# DC-002.31: los dos arreglos del proveedor de SISTEMA DE ARCHIVOS del motor se tienen que CAZAR si se los
# saca. Una suite en verde no prueba nada por si sola (regla del proyecto): este script saca UNO por vez,
# RECOMPILA la consola (el .gox del paquete tiene que salir del arbol mutado, no del que ya estaba: eso es
# la trampa de la camada de DC-002.29) y corre las dos mitades del corpus -- la del host y la de la consola.
#
#   nombre : `fullPath` vuelve a dejar las barras invertidas sin normalizar (el defecto del NOMBRE:
#            el motor pide `data\global\ui\...` y en Linux eso no es una ruta)
#   exists : `Exists` vuelve a `os.IsExist(err)` (false SIEMPRE, con el archivo presente incluido)
#
# El mutante del nombre tiene que poner ROJO el load del host Y el de la consola; el del Exists solo los
# controles del Exists. Se comprueba cual se cayo, no solo que "algo" se cayo.
#
# Control sano al final: el arbol restaurado y el corpus entero de las dos mitades en VERDE.
#
# Uso:  bash scripts/console/mutar-sueltos-231.sh [salida-del-corpus]
set -uo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)"
# shellcheck disable=SC1091
. "$HERE/env.sh"

SAL="${1:-$HOME/.hermes/cache/scratch/corpus-231}"
GEN="${GODC_GOX:-/tmp/gox-330}"
SCRIPT="$HERE/fix-d2loader-fs.py"
case "$GEN" in /*) ;; *) echo "GEN tiene que ser un directorio absoluto: $GEN" >&2; exit 1 ;; esac

SHA_SANO="$(python3 "$SCRIPT" --sha | sed -n 's/.* sano=\([0-9a-f]*\).*/\1/p')"
sha_actual() { python3 "$SCRIPT" --sha | sed -n 's/en_arbol=\([0-9a-f]*\).*/\1/p'; }

echo "== mutador de DC-002.31 (source.go sano=$SHA_SANO, layer $GEN)"

# el corpus de las dos mitades (host + consola): imprime el resumen y deja la salida cruda en $1
corpus() {
    local log="$1"
    SUELTOS_SOLO="2 3" bash "$HERE/checar-sueltos-231.sh" "$SAL" "$GEN" > "$log" 2>&1
    echo "$(sed -n 's/.*RESULTADO: \([0-9]*\) controles OK, \([0-9]*\) con fallo.*/\1 \2/p' "$log" | tail -1)"
}

mutar() {
    local modo="$1"
    python3 - "$REPO/d2common/d2loader/filesystem/source.go" "$modo" "$SHA_SANO" <<'PY'
import hashlib
import pathlib
import sys

ruta, modo, sha_sano = pathlib.Path(sys.argv[1]), sys.argv[2], sys.argv[3]
texto = ruta.read_text()

if hashlib.sha256(texto.encode()).hexdigest() != sha_sano:
    sys.exit("el archivo no esta en el estado sano: NO se muta (revisar antes de creerle al resultado)")

if modo == "nombre":
    # el defecto del NOMBRE: la ruta con las barras invertidas del motor, sin normalizar. El import de
    # `strings` se va con el (si no, el paquete no compila y el mutante seria "no compila", no el defecto).
    cambios = [
        ('\t"path/filepath"\n\t"strings"\n', '\t"path/filepath"\n'),
        ('return filepath.Clean(filepath.Join(s.Root, filepath.FromSlash(strings.ReplaceAll(subPath, `\\`, "/"))))',
         'return filepath.Clean(filepath.Join(s.Root, subPath)) // MUTANTE DC-002.31: sin normalizar'),
    ]
else:
    # el defecto del EXISTS: el os.IsExist(err) que es false cuando err == nil
    cambios = [("\treturn err == nil\n", "\treturn os.IsExist(err) // MUTANTE DC-002.31: el Exists de antes\n")]

for viejo, nuevo in cambios:
    if texto.count(viejo) != 1:
        sys.exit("el texto a mutar aparece %d veces (esperaba 1):\n%s" % (texto.count(viejo), viejo[:120]))

    texto = texto.replace(viejo, nuevo, 1)

ruta.write_text(texto)
PY
}

restaurar() {
    python3 "$SCRIPT" --forzar > /dev/null
    local ahora
    ahora="$(sha_actual)"
    [ "$ahora" = "$SHA_SANO" ] || { echo "NO RESTAURO BIEN ($ahora contra $SHA_SANO)" >&2; exit 1; }
    bash "$HERE/recompilar-paquete-motor.sh" d2common/d2loader/filesystem "$GEN" > /dev/null 2>&1
}

cazados=0
total=0

for caso in "nombre:MAL: 2b|MAL: 2e|MAL: 3b|MAL: 3d" "exists:MAL: 2d|MAL: 2f|MAL: 3c|MAL: 3e"; do
    MODO="${caso%%:*}"
    ESPERADOS="${caso#*:}"
    total=$((total + 1))
    echo
    echo "== mutante '$MODO' (source.go sha256 antes=$(sha_actual))"

    if ! mutar "$MODO"; then
        echo "   NO SE PUDO APLICAR (el texto no esta como se esperaba)"; restaurar; continue
    fi

    SHA_MUT="$(sha_actual)"
    if [ "$SHA_MUT" = "$SHA_SANO" ]; then
        echo "   el mutante NO cambio el archivo: no se puede creer el resultado"; restaurar; continue
    fi

    bash "$HERE/recompilar-paquete-motor.sh" d2common/d2loader/filesystem "$GEN" > /dev/null 2>&1 || {
        echo "   el paquete no compila mutado"; restaurar; continue; }

    LOG="$SAL/mutante-$MODO.log"
    leido="$(corpus "$LOG")"
    okk="$(echo "$leido" | cut -d' ' -f1)"; mal="$(echo "$leido" | cut -d' ' -f2)"
    echo "   corpus con el mutante (source.go sha256=$SHA_MUT): $okk OK, $mal con fallo"
    grep -a '^MAL: ' "$LOG" | sed 's/^/      /' | head -8

    cayeron=0
    IFS='|' read -ra LISTA <<<"$ESPERADOS"
    for e in "${LISTA[@]}"; do
        grep -aqF "$e" "$LOG" && cayeron=$((cayeron + 1))
    done

    if [ "${mal:-0}" -gt 0 ] && [ "$cayeron" -eq "${#LISTA[@]}" ]; then
        echo "   cazado: el corpus se puso ROJO ($mal controles) y cayeron los $cayeron controles del defecto"
        cazados=$((cazados + 1))
    else
        echo "   NO LO CAZO: rojo=$mal y cayeron $cayeron de ${#LISTA[@]} controles ($ESPERADOS)"
    fi

    restaurar
done

echo
echo "== control sano (los dos arreglos restaurados)"
if [ "$(sha_actual)" != "$SHA_SANO" ]; then
    echo "el archivo no volvio al estado sano" >&2; exit 1
fi
leido="$(corpus "$SAL/mutante-sano.log")"
okk="$(echo "$leido" | cut -d' ' -f1)"; mal="$(echo "$leido" | cut -d' ' -f2)"
echo "   corpus: $okk OK, $mal con fallo"
if [ "${mal:-1}" -eq 0 ] && [ "${okk:-0}" -eq 14 ]; then
    echo "   control sano VERDE ($okk/14)"
else
    echo "   CONTROL SANO NO ESTA VERDE" >&2
    exit 1
fi

echo
echo "== $cazados/$total mutantes cazados =="
[ "$cazados" -eq "$total" ]
