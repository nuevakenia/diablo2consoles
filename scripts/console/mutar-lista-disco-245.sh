#!/bin/bash
# MUTADORES DE DC-002.45: EL LISTADO DEL DISCO POR EL `fs_readdir` DEL DRIVER.
#
# El corpus (scripts/console/checar-lista-disco-245.sh) da 20 controles verdes. Eso NO prueba nada por si
# solo: hay que romper cada cosa que dice medir y exigir que se ponga ROJO. Este script hace eso, uno por
# uno, sobre el POC `lista-disco` (que es el que da los numeros finos: entradas, nombres, bytes y memoria):
#
#   1. el puente NO consulta el driver (vuelve al manifiesto del horneador) -> la marca suelta desaparece
#      y el listado baja a 3. Es el defecto de la tarjeta, exacto.
#   2. el driver no salta "." y ".."                 -> 6 entradas y un "." como entrada del usuario
#   3. el driver no salta el indice (MANIFIESTO.TXT) -> el indice sale como si fuera un archivo del juego
#   4. el driver no trae el TAMANO (de->size)        -> los 3 MPQ salen con 0 bytes
#
# Si un mutante queda VERDE, el hueco es del corpus y hay que taparlo: no se sigue.
#
# Costo: recompilar el paquete os (10 s) + el POC (15 s) + 25 s de Flycast por mutante. NO rehace la capa
# entera de .gox: en una copia de la capa se recompila SOLO el paquete os, que es lo unico que cambia.
#
# Restaurar NO se hace con git checkout --: se respalda y se verifica con sha256sum (el arbol del hito
# puede tener arreglos sin commitear).
#
# Uso:  bash scripts/console/mutar-lista-disco-245.sh
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$HERE/../.." || exit 1
source scripts/console/env.sh

SAL="${SAL:-/home/dgalli/.hermes/cache/scratch/c245}"
GEN="${GEN:-$SAL/gox}"
GENMUT="$SAL/gox-mut"
POCDIR="$SAL/poc-lista-mut"
OS=console/dc/std/os/os.go
CSRC=console/dc/std/csrc/os_dc.c
RESP="$SAL/mutar"

mkdir -p "$RESP"
# la capa de trabajo: copia de la buena, con SOLO el paquete os recompilado por mutante
[ -d "$GENMUT" ] || cp -a "$GEN" "$GENMUT"

mkdir -p "$GENMUT"
python3 "$HERE/reescribe-rutas-gox.py" "$GEN" "$GENMUT" "$GENMUT" >/dev/null 2>&1 || true

cp "$OS" "$RESP/os.go.orig"
cp "$CSRC" "$RESP/os_dc.c.orig"
SHA_OS="$(sha256sum "$OS" | cut -d' ' -f1)"
SHA_C="$(sha256sum "$CSRC" | cut -d' ' -f1)"
echo "os.go   de partida: sha256 $SHA_OS"
echo "os_dc.c de partida: sha256 $SHA_C"

restaura() {
    cp "$RESP/os.go.orig" "$OS"
    cp "$RESP/os_dc.c.orig" "$CSRC"
}

muta() {  # $1 archivo, $2 buscar, $3 poner
    python3 - "$1" "$2" "$3" <<'PY'
import sys
ruta, buscar, poner = sys.argv[1], sys.argv[2], sys.argv[3]
texto = open(ruta).read()
if buscar not in texto:
    print("  no encontre el fragmento a mutar", file=sys.stderr)
    sys.exit(3)
open(ruta, "w").write(texto.replace(buscar, poner, 1))
PY
}

corre() {  # compila el paquete os mutado + el POC y lo corre; deja el log en $RESP/log
    if ! bash scripts/console/compilar-paquete-std.sh os "$GENMUT" > "$RESP/build.log" 2>&1; then
        echo "(no compilo el paquete os mutado)" > "$RESP/log"
        return 1
    fi
    if ! GODC_GOX="$GENMUT" bash scripts/console/build-dc.sh console/dc/poc/lista-disco "$POCDIR" >> "$RESP/build.log" 2>&1; then
        echo "(no compilo el POC con el paquete mutado)" > "$RESP/log"
        return 1
    fi
    DC_MARCA_SUELTA="$SAL/marca/MARCA_SUELTA.DAT" bash scripts/console/mkdisco.sh "$POCDIR" "$SAL/juego3" > "$RESP/disco.log" 2>&1
    export FLYCAST="${FLYCAST:-$HOME/dreamcast/dl/squashfs-root/AppRun}"
    export CAPTURE=none
    python3 - "$RESP/run" <<'PY'
import shutil, sys
shutil.rmtree(sys.argv[1], ignore_errors=True)
PY
    bash scripts/console/run-flycast.sh "$POCDIR/disc.gdi" 25 "$RESP/run" > "$RESP/run.salida" 2>&1
    grep -aE 'LISTA-DISCO|ARQ-CARGA instrumento' "$RESP/run/flycast.log" 2>/dev/null | tr -d '\r' > "$RESP/log"
    [ -s "$RESP/log" ] || echo "(el POC no imprimio nada)" > "$RESP/log"
}

echo
echo "== control: sin mutar el POC tiene que dar 4 entradas CON la marca suelta de 78 bytes =="
restaura
corre
CONTROL_MAL=0
grep -q 'entradas= 4' "$RESP/log" || CONTROL_MAL=1
grep -q 'marca_suelta.dat dir= false bytes= 78' "$RESP/log" || CONTROL_MAL=1
grep -q 'patch_d2.mpq dir= false bytes= 5022588' "$RESP/log" || CONTROL_MAL=1
grep -q 'antes-de-40-listados cfree=2232' "$RESP/log" || CONTROL_MAL=1
[ "$CONTROL_MAL" = 0 ] && echo "  CONTROL SANO: 4 entradas, la marca suelta con 78 bytes y cfree= 2232" \
                       || { echo "  CONTROL MAL: con el codigo sin tocar el POC no da lo esperado. Se corta."; exit 1; }

cazados=0
total=0
probar() {  # $1 nombre, $2 archivo, $3 buscar, $4 poner, $5 lo que TIENE que verse al romperlo
    total=$((total + 1))
    echo
    echo "== mutante $total: $1 =="
    restaura
    if ! muta "$2" "$3" "$4"; then
        echo "  no se pudo aplicar el mutante"; return
    fi
    corre
    local debe="$5"
    if grep -qE "$debe" "$RESP/log"; then
        cazados=$((cazados + 1))
        echo "  CAZADO: el POC se puso ROJO -> $(grep -aE "$debe" "$RESP/log" | head -1)"
    else
        echo "  VERDE: el mutante NO se caza (el hueco es del corpus). Salida:"
        echo "  ---"
        sed -n '1,12p' "$RESP/log" | sed 's/^/  /'
        echo "  ---"
    fi
}

# 1. el puente deja de preguntarle al driver: es el defecto de la tarjeta (listar por el manifiesto).
#    Se rompe en el C, que es donde vive la llamada: el "driver" contesta -4 (no pudo abrir) y os.ReadDir
#    cae al respaldo. El listado baja a 3 y la marca suelta (que el horneador no conoce) desaparece.
probar "el puente no consulta al driver (contesta -4 y cae al manifiesto del horneador)" \
    "$CSRC" \
    '    fd = fs_open(ruta, O_RDONLY | O_DIR);
    if (fd < 0) {
        return -4;
    }' \
    '    fd = fs_open(ruta, O_RDONLY | O_DIR);
    if (fd >= 0) {
        fs_close(fd);
        return -4;      /* MUTANTE */
    }
    if (fd < 0) {
        return -4;
    }' \
    'entradas= 3|entradas= 0'

# 2. el driver no salta "." y "..": los pone el VFS (fs.c:fs_readdir), no son entradas del usuario
probar "el driver no salta . y .." \
    "$CSRC" \
    'if (strcmp(de->name, ".") == 0 || strcmp(de->name, "..") == 0) {' \
    'if (0) {   /* MUTANTE */' \
    '\] \. dir=|entradas= 6'

# 3. el driver no salta el INDICE del directorio (el manifiesto es el indice, no un archivo del juego)
probar "el driver lista el MANIFIESTO.TXT como entrada" \
    "$CSRC" \
    'if (nombre_es_indice(de->name)) {' \
    'if (0) {    /* MUTANTE */' \
    'manifiesto\.txt'

# 4. el driver no trae el TAMANO de la entrada (los 3 MPQ saldrian con 0 bytes)
probar "el driver no trae el tamano (de->size)" \
    "$CSRC" \
    '            tam = de->size;' \
    '            tam = 0;   /* MUTANTE */' \
    'd2char\.mpq dir= false bytes= 0'

echo
restaura
echo "== restaurando =="
SHA_OS_F="$(sha256sum "$OS" | cut -d' ' -f1)"
SHA_C_F="$(sha256sum "$CSRC" | cut -d' ' -f1)"
if [ "$SHA_OS_F" = "$SHA_OS" ] && [ "$SHA_C_F" = "$SHA_C" ]; then
    echo "   os.go   sha256 $SHA_OS_F  (igual)"
    echo "   os_dc.c sha256 $SHA_C_F  (igual)"
else
    echo "   OJO: no quedo igual. os.go $SHA_OS_F / os_dc.c $SHA_C_F"
    exit 1
fi
# la capa buena no se toco: el mutante fue sobre $GENMUT
echo "   la capa de trabajo ($GEN) no se toco; los mutantes fueron sobre $GENMUT"

echo
echo "== VEREDICTO: $cazados de $total mutantes cazados =="
[ "$cazados" = "$total" ] || exit 1
