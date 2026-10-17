#!/bin/bash
# CORPUS DE DC-002.18: el `config.json` del MOTOR en la consola -- guardarlo y leerlo, byte a byte igual
# al archivo del host.
#
# QUE SE MIDE (todo en Flycast, con el codigo del motor de verdad: `d2config.Configuration`):
#
#  CONTROL 1 (guardar): `config.Save()` escribe /rd/json218/config.json con los MISMOS bytes que el host
#    (json.MarshalIndent del json de Go sobre una struct espejo sin metodos). El numero de bytes se compara
#    contra el de los bytes horneados: no alcanza con "no fallo".
#  CONTROL 2 (leer): el archivo se lee con el camino del motor (`json.NewDecoder(archivo).Decode(config)`,
#    d2app/app.go:277), se vuelve a escribir lo leido y los bytes tienen que ser otra vez los del host.
#  CONTROL 3 (Encoder): `json.NewEncoder(...).Encode(config)` deja el compacto con su salto de linea, y lo
#    que escribio se lee de vuelta igual (lo lee el Unmarshal).
#  CONTROL 4 (variante): la misma vuelta con la configuracion de los bordes: lista nil (que es `null` en
#    json), campos vacios, numeros negativos, mapa con una lista vacia y una clave que se omite.
#
# EL "ANTES" (ROJO, MEDIDO): el primer mutante (`sin-marshal`) saca el `MarshalJSON` que agrego esta
# tarjeta, o sea que reproduce EXACTAMENTE el estado del arbol en DC-002.17 (commit 34e35a36): ahi
# `Save()` se caia con
#     json: en la consola no hay reflexion: solo se pueden serializar los tipos que saben hacerlo ellos
#     mismos (MarshalJSON/UnmarshalJSON)
# y los cuatro controles se ponen ROJOS. Los otros mutantes atacan el resto del arreglo.
#
# MUTANTES (4):
#   'sin-marshal'  : `Configuration.MarshalJSON` desactivado  -> 1,2,3,4 ROJOS (es el estado de DC-002.17)
#   'sin-decode'   : `Decoder.Decode` vuelve a devolver ErrSinReflexion (como antes) -> 2 ROJO
#   'sin-indent'   : `MarshalIndent` devuelve el compacto sin reacomodar -> 1 ROJO (los bytes no son)
#   'sin-campo'    : el marshaller se olvida de `D2SDir` -> 1 ROJO (los bytes no son)
#
# Uso:  bash scripts/console/checar-json-218.sh [dir_gox] [dir_build] [segundos]
#       La gox es propia (/tmp/gox-json218 por defecto): el build de la stdlib la rehace desde cero y no
#       hay que pisarle la gox a otra corrida.
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

GEN="${1:-/tmp/gox-json218}"
BUILD="${2:-/tmp/dcbuild/json-config-218}"
SEGS="${3:-20}"
REPO="${REPO:-$(cd "$HERE/../.." && pwd)}"
POC="$REPO/console/dc/poc/json-config"
JSON="$REPO/console/dc/std/encoding/json/json.go"
D2JSON="$REPO/d2core/d2config/d2config_json.go"
PAQUETES_MOTOR="d2common/d2enum d2core/d2input/d2gamepad d2core/d2config"
CAZADOS=0
TOTAL=4

for f in "$JSON" "$D2JSON" "$POC/main.go"; do
    [ -f "$f" ] || { echo "falta $f" >&2; exit 1; }
done

# si el arreglo no esta en el arbol, lo que se mediria es otra cosa: mejor no correr que mentir
grep -q 'func (c \*Configuration) MarshalJSON' "$D2JSON" || {
    echo "$D2JSON no tiene MarshalJSON: el arreglo de DC-002.18 no esta, no hay que medir." >&2
    exit 1
}
grep -q 'return unmarshal(datos, v)' "$JSON" || {
    echo "$JSON no pasa el Decode por Unmarshal: el arreglo de DC-002.18 no esta, no hay que medir." >&2
    exit 1
}

# ---------------------------------------------------------------- compilar y correr
compilar() {
    export GODC_GOX="$GEN"
    bash "$HERE/build-std-gox.sh" "$GEN" > "$BUILD.std.log" 2>&1 || { echo "   LA STDLIB NO COMPILA (ver $BUILD.std.log)"; return 1; }
    # shellcheck disable=SC2086
    PAQUETES="$PAQUETES_MOTOR" bash "$HERE/build-motor-gox.sh" "$GEN" > "$BUILD.gox.log" 2>&1 || { echo "   LOS PAQUETES DEL MOTOR NO COMPILAN (ver $BUILD.gox.log)"; return 1; }
    mkdir -p "$BUILD/gen"
    # el generador: la configuracion y los bytes esperados, calculados por el json de Go en el host
    (cd "$REPO" && go run ./console/dc/gen-config-json) > "$BUILD/gen/config_gen.go" 2> "$BUILD.gen.log" || {
        echo "   EL GENERADOR FALLO (ver $BUILD.gen.log)"; return 1; }
    bash "$HERE/build-dc.sh" "$POC" "$BUILD" > "$BUILD.poc.log" 2>&1 || { echo "   EL POC NO ENLAZA (ver $BUILD.poc.log)"; return 1; }
}

correr() { # $1 = dir de la corrida
    local dir="$1"
    rm -rf "$dir"
    (cd "$REPO" && CAPTURE=none bash "$HERE/run-flycast.sh" "$BUILD/prog.elf" "$SEGS" "$dir") > "$dir.salida" 2>&1
    [ -f "$dir/flycast.log" ]
}

# ---------------------------------------------------------------- los cuatro controles
# (los largos esperados se calculan DESPUES de compilar: los hornea el generador)

ctrl_guardado() { # $1 = log
    grep -qaF "JSON-CONFIG prueba guardado bytes= $LARGO_PRUEBA esperados= $LARGO_PRUEBA igual= true" "$1"
}

ctrl_leer() {
    grep -qaF "JSON-CONFIG prueba vuelta bytes= $LARGO_PRUEBA igual= true" "$1"
}

ctrl_encoder() {
    grep -qaE 'JSON-CONFIG prueba encoder bytes= [0-9]+ vuelta igual= true' "$1"
}

ctrl_variante() {
    grep -qaF "JSON-CONFIG variante guardado bytes= $LARGO_VARIANTE esperados= $LARGO_VARIANTE igual= true" "$1" &&
        grep -qaF "JSON-CONFIG variante vuelta bytes= $LARGO_VARIANTE igual= true" "$1"
}

controles() { # $1 = log; imprime los cuatro y devuelve cuantos dan bien
    local log="$1" bien=0
    if ctrl_guardado "$log"; then echo "   control 1 (guardar el config):        VERDE"; bien=$((bien + 1));
    else echo "   control 1 (guardar el config):        ROJO"; fi
    if ctrl_leer "$log"; then echo "   control 2 (leerlo con el Decoder):    VERDE"; bien=$((bien + 1));
    else echo "   control 2 (leerlo con el Decoder):    ROJO"; fi
    if ctrl_encoder "$log"; then echo "   control 3 (el Encoder del json):      VERDE"; bien=$((bien + 1));
    else echo "   control 3 (el Encoder del json):      ROJO"; fi
    if ctrl_variante "$log"; then echo "   control 4 (la variante de bordes):    VERDE"; bien=$((bien + 1));
    else echo "   control 4 (la variante de bordes):    ROJO"; fi
    echo "$bien"
}

mostrar() { # $1 = log
    grep -a 'JSON-CONFIG\|JSON-CONSOLA' "$1" | tr -d '\r' | sed 's/^/     /'
}

echo "== 1. control sano: con el arreglo puesto, los cuatro controles"
compilar || exit 1

# los largos esperados: los hornea el generador en el paso de arriba
LARGO_PRUEBA="$(python3 - "$BUILD/gen/config_gen.go" <<'PY'
import json, re, sys
t = open(sys.argv[1]).read()
m = re.search(r'const esperadoJSON = (".*?")$', t, re.M)
print(len(json.loads(m.group(1))))
PY
)"
LARGO_VARIANTE="$(python3 - "$BUILD/gen/config_gen.go" <<'PY'
import json, re, sys
t = open(sys.argv[1]).read()
m = re.search(r'const esperadoVarianteJSON = (".*?")$', t, re.M)
print(len(json.loads(m.group(1))))
PY
)"
echo "   bytes esperados (los calculo el json de Go en el host): prueba=$LARGO_PRUEBA variante=$LARGO_VARIANTE"

correr /tmp/checar-218-sano || exit 1
BIEN="$(controles /tmp/checar-218-sano/flycast.log | sed -n '$p')"
controles /tmp/checar-218-sano/flycast.log | sed -n '1,4p'
if [ "$BIEN" -ne 4 ]; then
    echo "   EL CONTROL SANO NO DA ($BIEN/4): no se sigue, primero hay que ver por que" >&2
    mostrar /tmp/checar-218-sano/flycast.log >&2
    exit 1
fi
echo "   OK: 4/4 controles verdes con el codigo de ahora"
mostrar /tmp/checar-218-sano/flycast.log

# ---------------------------------------------------------------- mutacion generica
# El respaldo es una copia propia (nunca `git checkout --`): en medio de la tarjeta hay arreglos SIN
# COMMITEAR en esos mismos archivos. Despues de cada mutante se verifica el sha256.
mutar() { # <archivo> <buscar> <poner> ; devuelve 1 si el patron no es unico
    local arch="$1" buscar="$2" poner="$3"
    python3 - "$arch" "$buscar" "$poner" <<'PY' || return 1
import sys
ruta, buscar, poner = sys.argv[1], sys.argv[2], sys.argv[3]
t = open(ruta).read()
n = t.count(buscar)
if n != 1:
    print("el texto a mutar aparece %d veces (tiene que ser 1)" % n, file=sys.stderr)
    sys.exit(2)
open(ruta, "w").write(t.replace(buscar, poner))
PY
}

restaurar() { # <archivo> <copia> <sha>
    local arch="$1" copia="$2" sha="$3"
    cp "$copia" "$arch"
    if [ "$(sha256sum "$arch" | awk '{print $1}')" != "$sha" ]; then
        echo "   EL ARBOL QUEDO MUTADO: no coincide el sha256 de $arch" >&2
        return 1
    fi
    echo "   arbol restaurado ($(basename "$arch"))"
}

exportar() { # <archivo> ; imprime la copia y el sha
    local arch="$1"
    local copia
    copia="$(mktemp /tmp/checar-218-XXXXXX)"
    cp "$arch" "$copia"
    echo "$copia $(sha256sum "$arch" | awk '{print $1}')"
}

probar_mutante() { # <nombre> <archivo> <buscar> <poner> <control que tiene que caer>
    local nombre="$1" arch="$2" buscar="$3" poner="$4" tiene_que_caer="$5"
    echo
    echo "== mutante '$nombre'"
    local copia sha
    read -r copia sha <<<"$(exportar "$arch")"
    if ! mutar "$arch" "$buscar" "$poner"; then
        echo "   NO SE APLICO LA MUTACION (patron sin match unico): el mutante no prueba nada" >&2
        restaurar "$arch" "$copia" "$sha" || exit 1
        rm -f "$copia"
        return
    fi
    if compilar && correr "/tmp/checar-218-mut-$nombre"; then
        local log="/tmp/checar-218-mut-$nombre/flycast.log"
        controles "$log" | sed -n '1,4p'
        if "$tiene_que_caer" "$log"; then
            echo "   NO LO CAZO: $tiene_que_caer sigue VERDE con el mutante puesto" >&2
        else
            echo "   cazado: el control que tenia que caer cayo"
            grep -a 'FALLO\|primera diferencia\|igual= false' "$log" | tr -d '\r' | head -3 | sed 's/^/     /'
            CAZADOS=$((CAZADOS + 1))
        fi
    else
        echo "   NO LO CAZO: con el mutante no compila/enlaza/corre, y un mutante que no compila no prueba nada" >&2
    fi
    restaurar "$arch" "$copia" "$sha" || exit 1
    rm -f "$copia"
}

# ---------------------------------------------------------------- los 4 mutantes
probar_mutante sin-marshal "$D2JSON" \
    'func (c *Configuration) MarshalJSON() ([]byte, error) {' \
    'func (c *Configuration) marshalJSONDesactivado() ([]byte, error) {   // MUTANTE sin-marshal' \
    ctrl_guardado

probar_mutante sin-decode "$JSON" \
    '	return unmarshal(datos, v)
' \
    '	_ = datos
	return ErrSinReflexion // MUTANTE sin-decode
' \
    ctrl_leer

probar_mutante sin-indent "$JSON" \
    '	return Indent(b, prefijo, sangria)
}' \
    '	return b, nil // MUTANTE sin-indent
}' \
    ctrl_guardado

probar_mutante sin-campo "$D2JSON" \
    '	e.campoCadena("D2SDir", c.D2SDir)
' \
    '' \
    ctrl_guardado

# ---------------------------------------------------------------- control sano de vuelta
echo
echo "== control sano de vuelta (el arbol entero, sin mutar)"
if compilar && correr /tmp/checar-218-vuelta; then
    BIEN="$(controles /tmp/checar-218-vuelta/flycast.log | sed -n '$p')"
    controles /tmp/checar-218-vuelta/flycast.log | sed -n '1,4p'
    [ "$BIEN" -eq 4 ] || { echo "   NO VOLVIERON LOS CUATRO CONTROLES ($BIEN/4)" >&2; exit 1; }
    echo "   control sano VERDE"
else
    echo "   NO VOLVIO EL CONTROL SANO" >&2
    exit 1
fi

echo
echo "== $CAZADOS/$TOTAL mutantes cazados =="
[ "$CAZADOS" -eq "$TOTAL" ]
