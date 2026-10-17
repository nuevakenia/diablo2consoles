#!/bin/bash
# MUTADOR DE DC-002.16: la bandera (?i) del regexp de la consola, medido en el ARRANQUE DEL MOTOR.
#
# Por que un mutador aparte del de la stdlib (scripts/console/mutar-std-host.sh, mutantes 35 y 36): esos
# dos prueban el corpus del HOST (la prueba diferencial del regexp contra el de Go). Este prueba la otra
# punta de la misma fuente: que el ARRANQUE DEL MOTOR EN LA CONSOLA se ponga ROJO si el defecto vuelve.
# El defecto original no lo veia ninguna suite: el motor moria ANTES de main por un MustCompile de nivel
# de paquete, y el unico sintoma era un panic sin pila que leer.
#
# Fases (cada una recompila SOLO el regexp.gox, reenlaza el motor-arranque y corre Flycast):
#   1. CONTROL SANO -- con el arreglo: "arranca N marcas" (N lineas MOTOR-ARRANQUE) y 0 lineas con
#      "panic:". Medido: 9 marcas y 0 panics.
#   2. MUTANTE 'prefijo-de-tres' -- el corte del prefijo vuelve a p[3:] (el off-by-one real: "(?i)" mide
#      CUATRO bytes) y el arranque tiene que morir ANTES de main: 0 marcas MOTOR-ARRANQUE y el panic del
#      regexp en el log. Es el mutante del defecto de esta tarjeta.
#   3. CONTROL SANO DE VUELTA -- con la fuente restaurada (copia propia + sha256, nunca `git checkout --`)
#      vuelve el veredicto de la fase 1.
#
# POR QUE RECOMPILA SOLO regexp.gox Y NO CORRE build-std-gox.sh: ese script arranca con un `rm -rf` del
# directorio de .gox, asi que rehacer la stdlib BORRA la gox del motor (los 142 paquetes + orden-motor.txt)
# y el enlace del arranque se cae con "falta orden-motor.txt". Medido: la primera version de este script
# hizo exactamente eso en su fase 1 y hubo que recompilar el motor entero (5 min) para volver. Recompilar
# un solo .gox es lo mismo que hace checar-init-215.sh con d2common/d2skill: se copian las fuentes de
# regexp a un directorio de trabajo, se reescribe el prefijo "d2std/" -> "" (como el build de verdad) y se
# compila con su pkgpath. ~1 s por fase en vez de 5 min.
#
# Uso:  bash scripts/console/mutar-regexp-216.sh [dir_gox] [dir_build] [segundos]
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

GEN="${1:-${GODC_GOX:-/tmp/gox-00216}}"
BUILD="${2:-/tmp/dcbuild/motor-216-mut}"
SEGS="${3:-15}"
REPO="${REPO:-$(cd "$HERE/../.." && pwd)}"
REGEXPG="$REPO/console/dc/std/regexp/regexp.go"
TMP="$(mktemp -d)"
CAZADOS=0
TOTAL=1
# el resultado de la ultima corrida (los pone correr(): NO dentro de un $( ), que seria un subshell y se
# perderian -- medido: la primera version del script murio con "VER_MARCAS: variable sin asignar")
SALIDA=""
VER_MARCAS=0
VER_PANIC=0
VER_TRACE=""

[ -f "$REGEXPG" ] || { echo "falta $REGEXPG" >&2; exit 1; }
[ -f "$GEN/orden-motor.txt" ] || { echo "falta $GEN/orden-motor.txt: corre build-motor-all.sh '$GEN'" >&2; exit 1; }
grep -q 'p = p\[4:\]' "$REGEXPG" || {
    echo "regexp.go NO tiene el arreglo de DC-002.16 (el corte del prefijo tiene que ser p[4:]):" >&2
    grep -n 'p = p\[3:\]\|p = p\[4:\]' "$REGEXPG" >&2
    exit 1
}

# la restauracion es una COPIA PROPIA (nunca `git checkout --`: en el hito hay WIP sin commitear en los
# mismos archivos). Y si el script muere a la mitad, ademas de restaurar la fuente se rehace la gox: si
# no, queda una gox mutada en GEN y la proxima medicion sale con el defecto puesto sin que se entienda.
EN_CURSO=""
restaurar_al_morir() {
    if [ -n "$EN_CURSO" ] && [ -f "$TMP/orig" ]; then
        cp "$TMP/orig" "$REGEXPG"
        echo "== el script murio a la mitad: regexp.go restaurado; rehaciendo regexp.gox en $GEN =="
        compilar_regexp >/dev/null 2>&1
    fi
    rm -rf "$TMP"
}
trap 'restaurar_al_morir' EXIT

aplicar() { # $1 = buscar, $2 = poner
    cp "$REGEXPG" "$TMP/orig"
    EN_CURSO=1
    sha256sum "$TMP/orig" | awk '{print $1}' > "$TMP/orig.sha"
    python3 - "$REGEXPG" "$1" "$2" <<'PY' || return 1
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

restaurar() {
    cp "$TMP/orig" "$REGEXPG"
    if [ "$(sha256sum "$REGEXPG" | awk '{print $1}')" != "$(cat "$TMP/orig.sha")" ]; then
        echo "   LA FUENTE QUEDO MUTADA: no coinciden los sha256 de regexp.go"
        return 1
    fi
    EN_CURSO=""
    return 0
}

# recompila SOLO regexp.gox dentro de $GEN. No usa build-std-gox.sh a proposito: ese hace `rm -rf` de
# todo el directorio de .gox y se llevaria puesto el motor (ver arriba).
compilar_regexp() {
    local src="$TMP/src/regexp"
    rm -rf "$TMP/src"
    mkdir -p "$src"
    for f in "$REPO/console/dc/std/regexp"/*.go; do
        [ -f "$f" ] || continue
        case "$f" in *_test.go) continue ;; esac
        sed 's|"d2std/|"|g' "$f" > "$src/$(basename "$f")"
    done
    sh-elf-gccgo $GODC_GOFLAGS -I "$GEN" -fgo-pkgpath="regexp" -c "$src"/*.go -o "$GEN/regexp.gox"
    printf "   regexp.gox %s bytes  sha256=%s\n" "$(stat -c%s "$GEN/regexp.gox")" \
        "$(sha256sum "$GEN/regexp.gox" | awk '{print $1}')"
}

correr() { # $1 = etiqueta. Deja el resultado en SALIDA / VER_MARCAS / VER_PANIC / VER_TRACE
    local etiqueta="$1"
    SALIDA="/tmp/mutar-regexp-216-$etiqueta"
    VER_MARCAS=0
    VER_PANIC=0
    VER_TRACE=""
    echo "   recompilando SOLO regexp.gox en $GEN con la fuente $etiqueta"
    if ! compilar_regexp >"$TMP/gox-$etiqueta.log" 2>&1; then
        echo "   no-compila-el-regexp"; tail -3 "$TMP/gox-$etiqueta.log" | sed 's/^/     /'; return 1
    fi
    cat "$TMP/gox-$etiqueta.log"
    if ! bash "$HERE/build-motor-arranque.sh" "$BUILD" "$GEN" >"$TMP/boot-$etiqueta.log" 2>&1; then
        echo "   no-enlaza"; tail -3 "$TMP/boot-$etiqueta.log" | sed 's/^/     /'; return 1
    fi
    rm -rf "$SALIDA"
    CAPTURE=none bash "$HERE/run-flycast.sh" "$BUILD/prog-emu.elf" "$SEGS" "$SALIDA" > "$SALIDA.salida" 2>&1
    VER_MARCAS="$(grep -ac 'MOTOR-ARRANQUE' "$SALIDA/flycast.log")"
    VER_PANIC="$(grep -ac 'panic:' "$SALIDA/flycast.log")"
    VER_TRACE="$(grep -am1 'PANIC-TRAZA' "$SALIDA/flycast.log" | tr -d '\r')"
    return 0
}

echo "== 1. control sano: con el arreglo, el motor llega a main y no paniquea"
correr sano || exit 1
echo "   marcas MOTOR-ARRANQUE: $VER_MARCAS (tienen que ser > 0)   lineas con \"panic:\": $VER_PANIC (0)"
if [ "$VER_MARCAS" -gt 0 ] && [ "$VER_PANIC" -eq 0 ]; then
    echo "   control sano VERDE"
else
    echo "   EL CONTROL SANO NO DA (revisar $SALIDA/flycast.log antes de seguir)" >&2
    exit 1
fi

echo
echo "== mutante 'prefijo-de-tres': el corte del prefijo (?i) vuelve a p[3:] (el off-by-one real)"
if ! aplicar $'\t\tp = p[4:]' $'\t\tp = p[3:]'; then
    echo "   NO SE APLICO LA MUTACION: el mutante no prueba nada" >&2
    exit 1
fi
grep -n 'p = p\[3:\]\|p = p\[4:\]' "$REGEXPG" | sed 's/^/     /'
if ! correr mutante; then
    echo "   el mutante no llego a correr (revisar arriba)" >&2
    restaurar
    exit 1
fi
echo "   marcas MOTOR-ARRANQUE: $VER_MARCAS (tienen que ser 0: el arranque no llega a main)"
echo "   lineas con \"panic:\": $VER_PANIC (tiene que ser > 0)"
[ -n "$VER_TRACE" ] && echo "   $VER_TRACE"
if [ "$VER_MARCAS" -eq 0 ] && [ "$VER_PANIC" -gt 0 ]; then
    echo "   cazado: el arranque vuelve a morir ANTES de main con el panic del regexp"
    CAZADOS=$((CAZADOS + 1))
else
    echo "   NO LO CAZO (marcas=$VER_MARCAS panics=$VER_PANIC): revisar $SALIDA/flycast.log" >&2
fi
restaurar || exit 1

echo
echo "== control sano de vuelta (la fuente repuesta)"
correr vuelta || exit 1
echo "   marcas MOTOR-ARRANQUE: $VER_MARCAS   lineas con \"panic:\": $VER_PANIC"
if [ "$VER_MARCAS" -gt 0 ] && [ "$VER_PANIC" -eq 0 ]; then
    echo "   control sano VERDE"
else
    echo "   NO VOLVIO EL CONTROL 1 (revisar $SALIDA/flycast.log)" >&2
    exit 1
fi

echo
echo "== $CAZADOS/$TOTAL mutantes cazados =="
[ "$CAZADOS" -eq "$TOTAL" ]
