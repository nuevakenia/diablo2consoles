#!/bin/bash
# MEDICION DE DC-002.17: QUE pide escribir el motor en el arranque y con QUE camino.
#
# Por que este script y no una deduccion leyendo el codigo: el arranque del motor llega a main (DC-002.16)
# y muere en `Run()` con "operation not supported on read-only filesystem". Ese texto lo produce NUESTRO
# os de la consola (console/dc/std/os/os.go:286, ErrNoSePuede): son los stubs declarados de escritura
# (MkdirAll/Mkdir/Remove/Rename/Create). Lo que NO se sabia es CUAL de esas llamadas hace el motor y con
# que camino. Ese es el dato que este script mide: agrega un medidor (noSePuede imprime "OS-ESCRITURA
# ... path=...") en esos cinco stubs, recompila SOLO os.gox, reenlaza el motor-arranque y corre Flycast.
#
# Recompila SOLO os.gox dentro de $GEN, y NO usa build-std-gox.sh: ese arranca con un `rm -rf` del
# directorio de .gox y se llevaria puesto el motor (los 142 paquetes + orden-motor.txt) -- medido en
# DC-002.16 (ver README seccion 32).
#
# Uso:  bash scripts/console/medir-escritura-217.sh [dir_gox] [dir_build] [segundos]
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

GEN="${1:-${GODC_GOX:-/tmp/gox-00216}}"
BUILD="${2:-/tmp/dcbuild/motor-217}"
SEGS="${3:-15}"
REPO="${REPO:-$(cd "$HERE/../.." && pwd)}"
OSGO="$REPO/console/dc/std/os/os.go"
TMP="$(mktemp -d)"
SALIDA="/tmp/dc-run-escritura-217"

[ -f "$OSGO" ] || { echo "falta $OSGO" >&2; exit 1; }
# el medidor tiene que estar en la fuente: si no, la corrida mediria la nada (mejor no correr que mentir)
grep -q 'OS-ESCRITURA' "$OSGO" || {
    echo "os.go NO tiene el medidor OS-ESCRITURA (DC-002.17): no hay que medir." >&2
    exit 1
}

compilar_os() {
    local src="$TMP/src/os"
    rm -rf "$TMP/src"
    mkdir -p "$src"
    for f in "$REPO/console/dc/std/os"/*.go; do
        [ -f "$f" ] || continue
        case "$f" in *_test.go) continue ;; esac
        sed 's|"d2std/|"|g' "$f" > "$src/$(basename "$f")"
    done
    sh-elf-gccgo $GODC_GOFLAGS -I "$GEN" -fgo-pkgpath="os" -c "$src"/*.go -o "$GEN/os.gox"
    printf "   os.gox %s bytes  sha256=%s\n" "$(stat -c%s "$GEN/os.gox")" \
        "$(sha256sum "$GEN/os.gox" | awk '{print $1}')"
}

# POR QUE HAY QUE RECOMPILAR EL PAQUETE DEL MOTOR Y NO ALCANZA CON EL os.gox:
# gccgo compila con -O2 (env.sh: GODC_GOFLAGS="-O2 ...") y las funciones de una linea de la stdlib que
# llama el motor quedan INLINADAS dentro del .gox del motor. Medido con `sh-elf-nm`:
#
#   d2config.gox:  U _os.ErrNoSePuede        <- referencia la VARIABLE del error
#                  (y NO tiene U _os.MkdirAll: la llamada quedo inlinada adentro)
#
# o sea que el mkdir del config no pasa por os.gox, pasa por una copia inlinada en d2config.gox que se
# compilo ANTES del medidor: por eso el medidor estaba en el binario y no imprimia nada. Recompilar el
# paquete del motor (misma receta que build-motor-gox.sh) vuelve a dejar la llamada a la vista.
compilar_paquete_motor() {
    local p="$1"                      # ej. d2core/d2config
    local prefijo="github.com/OpenDiablo2/OpenDiablo2"
    local out="$GEN/$prefijo/$p.gox"
    local srcs
    srcs="$(ls "$REPO/$p"/*.go | grep -v _test.go)"
    [ -n "$srcs" ] || { echo "   no encuentro fuentes de $p" >&2; return 1; }
    # shellcheck disable=SC2086
    sh-elf-gccgo $GODC_GOFLAGS -I "$GEN" -I "$GEN" -fgo-pkgpath="$prefijo/$p" -c $srcs -o "$out"
    printf "   %s.gox %s bytes  sha256=%s\n" "$(basename "$p")" "$(stat -c%s "$out")" \
        "$(sha256sum "$out" | awk '{print $1}')"
}

echo "== 1. recompilando SOLO os.gox en $GEN (con el medidor)"
if ! compilar_os > "$TMP/gox.log" 2>&1; then
    echo "   no-compila-el-os"; tail -5 "$TMP/gox.log" | sed 's/^/     /'; rm -rf "$TMP"; exit 1
fi
cat "$TMP/gox.log"

# y los paquetes del MOTOR que tengan la llamada inlinada (ver arriba). Por defecto el del arranque.
MOTOR_PKGS="${MOTOR_PKGS:-d2core/d2config}"
for p in $MOTOR_PKGS; do
    echo "   recompilando el paquete del motor $p (la llamada inlinada vuelve a la vista)"
    if ! compilar_paquete_motor "$p" >> "$TMP/gox.log" 2>&1; then
        echo "   no-compila-$p"; tail -5 "$TMP/gox.log" | sed 's/^/     /'; rm -rf "$TMP"; exit 1
    fi
    tail -1 "$TMP/gox.log" | sed 's/^/   /'
done

echo "== 2. reenlazando el motor-arranque en $BUILD"
if ! bash "$HERE/build-motor-arranque.sh" "$BUILD" "$GEN" > "$TMP/boot.log" 2>&1; then
    echo "   no-enlaza"; tail -5 "$TMP/boot.log" | sed 's/^/     /'; rm -rf "$TMP"; exit 1
fi
ls -l "$BUILD/prog-emu.elf" | awk '{print "   prog-emu.elf " $5 " bytes"}'

echo "== 3. corriendo Flycast $SEGS s"
rm -rf "$SALIDA"
CAPTURE=none bash "$HERE/run-flycast.sh" "$BUILD/prog-emu.elf" "$SEGS" "$SALIDA" > "$SALIDA.salida" 2>&1
LOG="$SALIDA/flycast.log"
[ -f "$LOG" ] || { echo "   no hay $LOG"; tail -5 "$SALIDA.salida" | sed 's/^/     /'; rm -rf "$TMP"; exit 1; }

echo "== 4. el arranque, en orden (marcas y pedidos de escritura)"
grep -a 'MOTOR-ARRANQUE\|OS-ESCRITURA\|panic:' "$LOG" | tr -d '\r' | sed 's/^/   /'

MARCAS=$(grep -ac 'MOTOR-ARRANQUE' "$LOG" || true)
ESCRIT=$(grep -ac 'OS-ESCRITURA' "$LOG" || true)
PANICS=$(grep -ac 'panic:' "$LOG" || true)
echo
echo "== RESUMEN MEDIDO"
echo "   marcas MOTOR-ARRANQUE: $MARCAS"
echo "   pedidos de escritura OS-ESCRITURA: $ESCRIT"
echo "   lineas con panic:: $PANICS"
echo "   el que pide escribir (unico por operacion+path):"
grep -a 'OS-ESCRITURA' "$LOG" | tr -d '\r' | sed 's/^[[:space:]]*//' | sort -u | sed 's/^/     /'
rm -rf "$TMP"
