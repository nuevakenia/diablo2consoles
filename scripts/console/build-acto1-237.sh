#!/bin/bash
# DC-002.37: el ELF del POC del ACTO 1 y el DISCO con el d2data.mpq (267 MB) del juego en /JUEGO.
#
# Que lleva el disco:
#   /JUEGO/D2DATA.MPQ  el MPQ GRANDE (65.536 ranuras de hash): es el que tiene los .ds1 del acto 1 y sus DT1.
#                      Se abre en la consola con las tablas COMPACTAS de DC-002.33 (antes reventaba el
#                      semiespacio del recolector: DC-002.28). El .ds1 se lee por partes: no hace falta
#                      bajarlo entero.
# y el arranque de siempre (IP.BIN + 1ST_READ.BIN, los dos ya generados por build-dc.sh).
#
# Uso:  scripts/console/build-acto1-237.sh [dir_salida]
#       SALTAR_STDLIB=1 / SALTAR_MOTOR=1 para reusar el gox ya compilado (con GODC_GOX propio).
#       DS1_INEXISTENTE=1 -> compila el MUTANTE (el .ds1 del nivel apunta a uno que no existe).
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
POC="acto1-disco"
SRC="$REPO/console/dc/poc/$POC"
OUT="${1:-${DC237_OUT:-/tmp/dcbuild/acto1-disco}}"
GEN="${GODC_GOX:-$DC_ROOT/gox}"

D2DIR="${D2_DIR:-$HOME/.wine-d2/drive_c/Games/Diablo II}"

mkdir -p "$OUT" "$OUT/host"

echo "== 1/4 la stdlib minima y el MOTOR que este POC importa, en $GEN"
if [ "${SALTAR_STDLIB:-0}" != "1" ]; then
    # OJO: build-std-gox.sh arranca con un `rm -rf "$GEN"`: si se corre DESPUES del motor, se lleva los
    # .gox del motor y el enlace queda sin ellos. Por eso el orden es stdlib -> motor, y para reusar un
    # arbol ya armado se pasa SALTAR_STDLIB=1.
    bash "$HERE/build-std-gox.sh" "$GEN" > "$OUT/stdlib.log" 2>&1 || {
        echo "FALLO la stdlib (ver $OUT/stdlib.log)"; exit 1; }
    echo "   stdlib OK ($(ls "$GEN"/*.gox 2>/dev/null | wc -l) .gox)"
else
    echo "   stdlib SALTEADA (SALTAR_STDLIB=1)"
fi

# LOS PAQUETES DEL MOTOR QUE ENTRAN AL BINARIO: los 3 que importa el POC y su CIERRE DE DEPENDENCIAS
# (medido con `go list -deps` en el host: 11 paquetes internos y ni uno mas). NO se enlaza el motor
# entero (205 .gox, ~44 MB de ELF): la consola tiene 16 MB y este POC no los usa.
#
# `build-motor-gox.sh` (y NO `build-motor-all.sh`) porque este ultimo escribe `orden-motor.txt`, que es la
# lista que usa el build del ARRANQUE; el enlace de `build-dc.sh` lee `orden.txt`, y es ahi donde tienen que
# quedar los .gox de este POC. Medido: con build-motor-all.sh el enlace moria con 40 referencias sin
# resolver (d2mpq.FromFile, d2ds1.Layer.Tile, d2dt1.LoadDT1, ...): el motor nunca entraba al ELF.
PAQUETES237="d2common/d2enum d2common/d2math d2common/d2math/d2vector d2common/d2path \
d2networking/d2client/d2clientconnectiontype d2common/d2interface d2common/d2datautils \
d2common/d2data/d2compression d2common/d2fileformats/d2ds1 d2common/d2fileformats/d2dt1 \
d2common/d2fileformats/d2mpq"

if [ "${SALTAR_MOTOR:-0}" != "1" ]; then
    PAQUETES="$PAQUETES237" bash "$HERE/build-motor-gox.sh" "$GEN" > "$OUT/motor.log" 2>&1 || {
        echo "FALLO el motor (ver $OUT/motor.log)"; tail -5 "$OUT/motor.log"; exit 1; }
    N_MOTOR="$(printf '%s\n' $PAQUETES237 | wc -l)"
    echo "   motor OK: $N_MOTOR paquetes (d2mpq, d2ds1, d2dt1 y su cierre de dependencias)"
    for p in $PAQUETES237; do
        grep -qxF "$GEN/github.com/OpenDiablo2/OpenDiablo2/$p.gox" "$GEN/orden.txt" || {
            echo "   FALLO: $p.gox no quedo en orden.txt (el enlace no lo veria)"; exit 1; }
    done
    echo "   los $N_MOTOR .gox estan en orden.txt ($(wc -l < "$GEN/orden.txt") objetos para el linker)"
else
    echo "   motor SALTEADO (SALTAR_MOTOR=1)"
fi

echo
echo "== 2/4 el ELF del POC ($POC)"
TMPPOC="$OUT/poc"
rm -rf "$TMPPOC"
mkdir -p "$TMPPOC"
cp -r "$SRC/." "$TMPPOC/"

if [ "${DS1_INEXISTENTE:-0}" = "1" ]; then
    # el MUTANTE (criterio 3 de la tarjeta): el nivel apunta a un .ds1 que no existe. Es la MISMA corrida con
    # otro nombre: si la pantalla cambia igual, el criterio no vale.
    sed -i 's|townN1\.ds1|no-existe-este-nivel.ds1|' "$TMPPOC/main.go"
    echo "   MUTANTE: el .ds1 del nivel apunta a no-existe-este-nivel.ds1"
fi

GODC_GOX="$GEN" bash "$HERE/build-dc.sh" "$TMPPOC" "$OUT"

echo
echo "== 3/4 la ISO del disco con el d2data.mpq en /JUEGO"
# El nombre REAL del archivo en el disco: `d2data.mpq` en el PC, y va como D2DATA.MPQ al ISO9660 (que no
# distingue caja, pero el VFS de KOS si resuelve el nombre exacto que se escribio).
MPQ_REAL="$(ls "$D2DIR" | grep -ix 'd2data.mpq' | head -1)"
[ -n "$MPQ_REAL" ] || { echo "no encuentro d2data.mpq en $D2DIR"; exit 1; }

LISTA="$OUT/iso-lista.txt"
printf '%s:/JUEGO/D2DATA.MPQ;1\n' "$D2DIR/$MPQ_REAL" > "$LISTA"

# SIN el arbol Joliet (medido en DC-002.35 y DC-002.27): con Joliet el driver iso9660 de KOS monta Joliet y
# despues no encuentra NINGUNA ruta.
MKISO_JOLIET=0 "$DC_PYTHON" "$HERE/mkiso.py" "$OUT/disc.iso" "$OUT/IP.BIN" "$OUT/1ST_READ.BIN" "@$LISTA" | tail -2
"$DC_PYTHON" "$HERE/mkgdi.py" "$OUT" | tail -2

echo
echo "== 4/4 numeros del host (los MISMOS que tiene que dar la consola)"
if [ "${DS1_INEXISTENTE:-0}" != "1" ]; then
    ( cd "$REPO" && go run ./console/dc/verif-acto1-237 -png "$OUT/host/referencia.png" ) | tee "$OUT/host/verif.txt"
fi

echo
echo "ELF:    $OUT/prog.elf   ($(stat -c%s "$OUT/prog.elf") bytes)"
echo "ISO:    $OUT/disc.iso  ($(stat -c%s "$OUT/disc.iso") bytes)"
echo "GDI:    $OUT/disc.gdi"
