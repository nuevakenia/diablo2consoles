#!/bin/bash
# DC-002.18: MIDE EL ARRANQUE DEL MOTOR delante y detras del arreglo del serializador.
#
# QUE MIDE (con el arranque del motor entero en Flycast, el mismo binario de build-motor-arranque.sh):
#
#   - cuantas serializaciones hace el arranque y SOBRE QUE TIPOS (marcas JSON-CONSOLA del instrumento: el
#     instrumento vive en el json de la consola -- `json.Instrumento` -- y el que imprime es el POC del
#     arranque, con la lectura del descriptor de tipo de las sondas de DC-002.2f);
#   - cuantas marcas MOTOR-ARRANQUE salen y hasta donde llega Run();
#   - el error EXACTO con el que queda (si queda con alguno).
#
# EL "ANTES" NO SE DEDUCE: se reproduce sacando el arreglo (el mutante 'sin-marshal' desactiva
# `Configuration.MarshalJSON`), que es EXACTAMENTE el estado del arbol en DC-002.17 (commit 34e35a36).
# Despues se repone y se vuelve a medir. Los dos numeros son de la misma maquina y del mismo binario de
# alrededor.
#
# Uso:  bash scripts/console/medir-arranque-218.sh [dir_gox] [dir_build] [segundos]
#       El dir_gox es el del MOTOR ENTERO: se arma con
#           bash scripts/console/build-motor-all.sh /tmp/gox-00218 /tmp/dcbuild/motor-all-218
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

GEN="${1:-/tmp/gox-00218}"
BUILD="${2:-/tmp/dcbuild/motor-218}"
SEGS="${3:-20}"
REPO="${REPO:-$(cd "$HERE/../.." && pwd)}"
D2JSON="$REPO/d2core/d2config/d2config_json.go"

[ -f "$GEN/orden.txt" ] || { echo "falta $GEN/orden.txt (la stdlib minima)" >&2; exit 1; }
[ -f "$GEN/orden-motor.txt" ] || { echo "falta $GEN/orden-motor.txt (el motor: build-motor-all.sh)" >&2; exit 1; }
[ -f "$D2JSON" ] || { echo "falta $D2JSON (el arreglo de DC-002.18)" >&2; exit 1; }

# el gox del d2config solo (1,8 s): para volver a medir alcanza con rehacer ESE paquete y reenlazar.
rehacer_d2config() {
    export GODC_GOX="$GEN"
    PAQUETES="d2core/d2config" bash "$HERE/build-motor-gox.sh" "$GEN" >> "$BUILD.gox.log" 2>&1 || {
        echo "   no se pudo rehacer d2config.gox (ver $BUILD.gox.log)" >&2; return 1; }
}

correr() { # $1 = dir de la corrida
    local dir="$1"
    bash "$HERE/build-motor-arranque.sh" "$BUILD" "$GEN" > "$dir.build.log" 2>&1 || {
        echo "   no-enlaza (ver $dir.build.log)" >&2; return 1; }
    rm -rf "$dir"
    (cd "$REPO" && CAPTURE=none bash "$HERE/run-flycast.sh" "$BUILD/prog-emu.elf" "$SEGS" "$dir") > "$dir.salida" 2>&1
    [ -f "$dir/flycast.log" ]
}

resumen() { # $1 = dir de la corrida; imprime los numeros
    local log="$1/flycast.log"
    echo "   marcas MOTOR-ARRANQUE:      $(grep -ac 'MOTOR-ARRANQUE' "$log")"
    echo "   serializaciones (JSON-CONSOLA): $(grep -ac 'JSON-CONSOLA' "$log")"
    echo "   puntos y tipos:"
    grep -a 'JSON-CONSOLA' "$log" | tr -d '\r' | sed 's/^[[:space:]]*/     /' | sed 's/desc= [0-9]*//' | sed 's/  */ /g'
    echo "   marcas MOTOR-ARRANQUE (todas):"
    grep -a 'MOTOR-ARRANQUE' "$log" | tr -d '\r' | sed 's/^/     /'
    echo "   el error exacto que queda:"
    if grep -qa 'Run() termino con error' "$log"; then
        grep -a 'Run() termino con error' "$log" | tr -d '\r' | sed 's/^/     /'
    elif grep -qa 'Run() volvio sin error' "$log"; then
        echo "     (ninguno: Run() volvio sin error)"
    else
        echo "     (no llego a volver de Run(): mirar las marcas)"
    fi
    echo "   el overlay del motor al final:"
    grep -a 'overlay del motor' "$log" | tr -d '\r' | sed 's/^/     /'
}

copiar_y_sha() { # <archivo> ; imprime "copia sha"
    local copia
    copia="$(mktemp /tmp/medir-218-XXXXXX)"
    cp "$1" "$copia"
    echo "$copia $(sha256sum "$1" | awk '{print $1}')"
}

echo "== 1. ANTES (el arreglo sacado: es el estado de DC-002.17, commit 34e35a36)"
read -r COPIA SHA <<<"$(copiar_y_sha "$D2JSON")"
python3 - "$D2JSON" <<'PY'
import sys
ruta = sys.argv[1]
t = open(ruta).read()
a = "func (c *Configuration) MarshalJSON() ([]byte, error) {"
n = t.count(a)
if n != 1:
    sys.exit("el texto a mutar aparece %d veces (tiene que ser 1)" % n)
open(ruta, "w").write(t.replace(a, "func (c *Configuration) marshalJSONDesactivado() ([]byte, error) {  // MUTANTE sin-marshal", 1))
print("   d2config_json.go mutado: MarshalJSON fuera (es el estado de DC-002.17)")
PY
rehacer_d2config || { cp "$COPIA" "$D2JSON"; exit 1; }
if correr /tmp/medir-218-antes; then
    resumen /tmp/medir-218-antes
else
    echo "   ni siquiera enlaza/corre sin el arreglo" >&2
fi
cp "$COPIA" "$D2JSON"
[ "$(sha256sum "$D2JSON" | awk '{print $1}')" = "$SHA" ] || { echo "EL ARBOL QUEDO MUTADO" >&2; exit 1; }
rm -f "$COPIA"
echo "   arbol restaurado (sha256 ok)"

echo
echo "== 2. DESPUES (el arreglo puesto)"
rehacer_d2config || exit 1
if correr /tmp/medir-218-despues; then
    resumen /tmp/medir-218-despues
else
    echo "   no-enlaza o no-corre CON el arreglo" >&2
    exit 1
fi

echo
echo "== 3. los dos numeros, al lado"
for d in /tmp/medir-218-antes /tmp/medir-218-despues; do
    [ -f "$d/flycast.log" ] || continue
    printf '   %-24s marcas=%s serializaciones=%s\n' "$(basename "$d")" \
        "$(grep -ac 'MOTOR-ARRANQUE' "$d/flycast.log")" "$(grep -ac 'JSON-CONSOLA' "$d/flycast.log")"
done
