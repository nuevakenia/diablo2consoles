#!/bin/bash
# Mutadores de la ALINEACION de tipos de os con io/fs (DC-002.8a).
#
# os.FileInfo, os.DirEntry y os.FileMode SON los de io/fs (alias); con dos tipos distintos, cualquier
# funcion que devuelva uno no sirve donde se pide el otro. El sintoma medido (DC-002.8) fue el motor:
#
#   d2mpq/mpq.go:204: cannot use type []os.FileInfo as type []fs.FileInfo
#
# Ese defecto no lo puede cazar la prueba del host (os no compila ahi) ni un POC que corra: lo caza el
# COMPILADOR. Asi que este mutador es de SOLO COMPILACION, sin Flycast. Tres pasos por mutante:
#
#   control:   sin mutar, (a) la stdlib compila, (b) d2mpq (el motor) compila, (c) el POC del VFS
#              compila (sus identidades de tipo las exige el compilador, seccion 4l de main.go).
#   mutante 1: os.FileInfo propio, y nada mas -> la STDLIB no compila: dirEntryOS.Info() devuelve el
#              FileInfo de os y fs.DirEntry pide el de io/fs.
#   mutante 1b: os.FileInfo propio + os parcheado para que compile (el estado "a mitad de camino" que
#              describe el tablero) -> d2mpq NO compila, con el error exacto de DC-002.8.
#   mutante 2: os.DirEntry propio  -> el POC del VFS NO compila (el []os.DirEntry deja de ser
#                                     []fs.DirEntry). El motor no lo usa donde el compilador lo exija.
#   mutante 3: os.FileMode propio  -> la STDLIB no compila: dirEntryOS.Type() devuelve os.FileMode y
#                                     fs.DirEntry pide fs.FileMode.
#
# La stdlib mutada se compila en un directorio APARTE ($TMP/gox): build-std-gox.sh hace `rm -rf` de su
# salida y, si se apunta al arbol de produccion, se lleva puestos los .gox del MOTOR que necesita
# d2mpq. Con el stdlib mutada PRIMERO en el -I, gccgo resuelve os.gox de ahi y el motor del otro dir.
#
# Restaurar NO es git checkout --: se respalda os.go y se verifica con sha256sum al final.
#
# Uso:  scripts/console/mutar-tipos-os.sh
set -uo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
cd "$HERE/../.." || exit 1
# shellcheck source=env.sh
source scripts/console/env.sh

OS=console/dc/std/os/os.go
PREFIJO=github.com/OpenDiablo2/OpenDiablo2
MPQ=d2common/d2fileformats/d2mpq
POC=console/dc/poc/vfs/main.go
GOX_MOTOR="${GODC_GOX:-$DC_ROOT/gox}"
TMP=/tmp/mutar-tipos
GOX_TMP="$TMP/gox"
mkdir -p "$TMP"

cp "$OS" "$TMP/os.go.orig"
SHA0="$(sha256sum "$OS" | cut -d' ' -f1)"
echo "os.go de partida: sha256 $SHA0"

if [ ! -f "$GOX_MOTOR/$PREFIJO/d2common/d2interface.gox" ]; then
    echo "faltan los .gox del motor en $GOX_MOTOR: los compilo una vez (tarda un par de minutos)"
    bash scripts/console/build-motor-all.sh "$GOX_MOTOR" "$TMP/motor-all" > "$TMP/motor-all.log" 2>&1 ||
        { echo "no pude compilar el motor: ver $TMP/motor-all.log"; exit 1; }
fi

muta() {  # $1 = buscar, $2 = reemplazar
    python3 - "$OS" "$1" "$2" <<'PY'
import sys
ruta, buscar, poner = sys.argv[1], sys.argv[2], sys.argv[3]
texto = open(ruta).read()
if buscar not in texto:
    print("  no encontre el fragmento a mutar", file=sys.stderr)
    sys.exit(3)
open(ruta, "w").write(texto.replace(buscar, poner, 1))
PY
}

# los tres pasos, contra la stdlib mutada de $GOX_TMP; el que se cae devuelve != 0
paso_stdlib() { bash scripts/console/build-std-gox.sh "$GOX_TMP" > "$TMP/stdlib.log" 2>&1; }
paso_motor() {
    local srcs
    srcs=$(ls "$MPQ"/*.go 2>/dev/null | grep -v _test.go)
    # shellcheck disable=SC2086
    sh-elf-gccgo $GODC_GOFLAGS -I "$GOX_TMP" -I "$GOX_MOTOR" -fgo-pkgpath="$PREFIJO/$MPQ" \
        -c $srcs -o "$TMP/mpq.gox" > "$TMP/mpq.log" 2>&1
}

paso_poc() {
    sh-elf-gccgo $GODC_GOFLAGS -I "$GOX_TMP" -I "$GOX_MOTOR" -c "$POC" -o "$TMP/poc.o" > "$TMP/poc.log" 2>&1
}

# algo_se_cae: corre los tres pasos y devuelve el motivo del primero que se cae (vacio = ninguno: la
# mutacion es invisible para todo el corpus)
algo_se_cae() {
    if ! paso_stdlib; then
        echo "la stdlib no compila: $(grep -m1 'error:' "$TMP/stdlib.log" | cut -c1-170)"
        return 0
    fi
    if ! paso_motor; then
        echo "d2mpq no compila: $(grep -m1 'error:' "$TMP/mpq.log" | cut -c1-170)"
        return 0
    fi
    if ! paso_poc; then
        echo "el POC del VFS no compila: $(grep -m1 'error:' "$TMP/poc.log" | cut -c1-170)"
        return 0
    fi
    echo ""
}

echo
echo "== control: sin mutar tienen que compilar los tres =="
motivo="$(algo_se_cae)"
if [ -n "$motivo" ]; then
    echo "CONTROL MAL: $motivo. Se corta."
    cp "$TMP/os.go.orig" "$OS"
    exit 1
fi
echo "  stdlib OK, d2mpq OK, POC del VFS OK (identidades exigidas por el compilador)"

cazados=0
total=0
probar() {  # $1 nombre, $2 buscar, $3 reemplazar, $4 donde se tiene que caer
    total=$((total + 1))
    echo
    echo "== mutante $total: $1 (tiene que caerse en: $4) =="
    cp "$OS" "$TMP/os.go.resp"
    if ! muta "$2" "$3"; then
        echo "  NO SE PUDO APLICAR (se saltea)"
        cp "$TMP/os.go.orig" "$OS"
        return
    fi
    # segundo par de reemplazos, para los mutantes que necesitan dos parches (MUT2_*)
    if [ -n "${MUT2_BUSCAR:-}" ]; then
        if ! muta "$MUT2_BUSCAR" "$MUT2_PONER"; then
            echo "  NO SE PUDO APLICAR EL SEGUNDO PARCHE (se saltea)"
            cp "$TMP/os.go.orig" "$OS"
            MUT2_BUSCAR=""
            return
        fi
    fi
    MUT2_BUSCAR=""
    motivo="$(algo_se_cae)"
    cp "$TMP/os.go.resp" "$OS"
    if [ -z "$motivo" ]; then
        echo "  VERDE: la mutacion NO se ve. Hueco: la alineacion no esta exigida por nadie."
    else
        echo "  ROJO: cazado -> $motivo"
        cazados=$((cazados + 1))
    fi
}

probar "os.FileInfo con tipo propio (y nada mas)" \
    "type FileInfo = fs.FileInfo" \
    "type FileInfo interface {
	Name() string
	Size() int64
	Mode() FileMode
	ModTime() time.Time
	IsDir() bool
	Sys() interface{}
}" \
    "la stdlib (dirEntryOS.Info deja de satisfacer fs.DirEntry)"

# El estado "A MITAD DE CAMINO" que describe el tablero: con os.FileInfo propio, os se cae. Si se lo
# parchea para que compile (Info devolviendo fs.FileInfo, que es la forma en que el tablero dice que el
# motor BAJA a 40 de 142), el que se cae es el motor, con el error EXACTO que se midio en DC-002.8.
MUT2_BUSCAR='func (d dirEntryOS) Info() (FileInfo, error) {'
MUT2_PONER='func (d dirEntryOS) Info() (fs.FileInfo, error) {'
probar "os.FileInfo propio + os parcheado para compilar (el motor es el que se cae)" \
    "type FileInfo = fs.FileInfo" \
    "type FileInfo interface {
	Name() string
	Size() int64
	Mode() FileMode
	ModTime() time.Time
	IsDir() bool
	Sys() interface{}
}" \
    "d2mpq, con el error del tablero"

probar "os.DirEntry con tipo propio" \
    "type DirEntry = fs.DirEntry" \
    "type DirEntry interface {
	Name() string
	IsDir() bool
	Type() FileMode
	Info() (FileInfo, error)
}" \
    "el POC del VFS"

probar "os.FileMode con tipo propio" \
    "type FileMode = fs.FileMode" \
    "type FileMode uint32" \
    "la stdlib"

echo
echo "== restauro y verifico =="
cp "$TMP/os.go.orig" "$OS"
SHA1="$(sha256sum "$OS" | cut -d' ' -f1)"
if [ "$SHA0" != "$SHA1" ]; then
    echo "MAL: os.go no quedo igual (sha256 $SHA1, era $SHA0)"
    exit 1
fi
echo "os.go restaurado, sha256 $SHA1"
echo "(la stdlib de produccion $GOX_MOTOR no se toco: las mutaciones se compilan en $GOX_TMP)"
echo
echo "== mutantes cazados $cazados/$total =="
[ "$cazados" = "$total" ] || exit 1
