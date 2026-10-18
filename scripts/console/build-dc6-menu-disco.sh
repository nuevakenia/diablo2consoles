#!/bin/bash
# DC-002.35 -- build del PRIMER HITO VISIBLE: un DC6 real del menu, leido del MPQ DEL DISCO, pintado.
#
# Que arma (y por que asi):
#   1. los .gox de la stdlib minima y los paquetes del MOTOR que este POC importa (d2mpq + d2dc6 +
#      d2pl2 y sus dependencias). No hace falta el motor entero: este POC no lo arrastra.
#   2. el ELF del POC (build-dc.sh) y su ISO/GDI de arranque.
#   3. LA ISO DEL DISCO otra vez, ahora CON LOS ARCHIVOS DEL JUEGO en /JUEGO: el MPQ que el POC abre
#      (patch_d2.mpq, el chico) y la paleta PL2, que se extrae del MPQ del host porque en la consola
#      el MPQ grande (d2data) no se puede abrir todavia (DC-002.28/33: 65.536 entradas revientan el
#      semiespacio del GC). El DC6 en cambio se lee DEL MPQ DEL DISCO: es el punto de la tarjeta.
#
# Uso:  scripts/console/build-dc6-menu-disco.sh [dir_salida]
#       SALTAR_STDLIB=1 / SALTAR_MOTOR=1 para reusar lo ya compilado en $GODC_GOX (el build de la
#       stdlib arranca con un rm -rf de su directorio: con gox propia no molesta a nadie).
#       DC6_INEXISTENTE=1 -> compila el MUTANTE (el asset que no existe).
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
POC="dc6-menu-disco"
SRC="$REPO/console/dc/poc/$POC"
OUT="${1:-${DC35_OUT:-$HOME/.hermes/cache/scratch/dc35/build}}"
GEN="${GODC_GOX:-$DC_ROOT/gox}"

D2DIR="${D2_DIR:-$HOME/.wine-d2/drive_c/Games/Diablo II}"
MPQ_CHICO="${MPQ_CHICO:-patch_d2.mpq}"      # 16.384 entradas: se abre en la consola
MPQ_GRANDE="${MPQ_GRANDE:-d2data.mpq}"      # 65.536 entradas: hoy no (el semiespacio)
DC6_PATH='data\global\ui\BIGMENU\waitingroombkgd.dc6'
PAL_PATH='data\global\palette\act1\pal.pl2'

mkdir -p "$OUT" "$OUT/host"

echo "== 0/5 los archivos del host: el DC6 crudo y la paleta, cada uno de SU MPQ"
# El nombre REAL del archivo en el PC es `patch_D2.mpq` (D mayuscula): si se le pasa el nombre tal como
# viene en la config (minusculas) el MPQ no abre y el error parece un defecto del MPQ.
mpq_real() {
    local buscado="$1" hallado
    hallado="$(ls -1 "$D2DIR" | grep -ix "$buscado" | head -1)"
    [ -n "$hallado" ] || hallado="$(ls -1 "$D2DIR" | grep -i "^$(basename "${buscado%.mpq}").mpq$" | head -1)"
    printf '%s' "$hallado"
}
MPQ_CHICO_REAL="$(mpq_real "$MPQ_CHICO")"
MPQ_GRANDE_REAL="$(mpq_real "$MPQ_GRANDE")"
[ -n "$MPQ_CHICO_REAL" ] || { echo "no encuentro $MPQ_CHICO en $D2DIR"; exit 1; }
[ -n "$MPQ_GRANDE_REAL" ] || { echo "no encuentro $MPQ_GRANDE en $D2DIR"; exit 1; }

if [ ! -f "$OUT/host/$POC.dc6" ]; then
    ( cd "$REPO" && go run ./console/dc/bake-dc6 -mpq "$D2DIR/$MPQ_CHICO_REAL" -dc6 "$DC6_PATH" \
        -out "$OUT/host" -name "$POC" )
else
    echo "   (el DC6 crudo ya estaba en $OUT/host/$POC.dc6: se reusa)"
fi

# La paleta vive en el OTRO MPQ (d2data.mpq): con -solo-pal se pide ahi sin exigir el DC6, que no esta.
if [ ! -f "$OUT/host/$POC.pal" ]; then
    ( cd "$REPO" && go run ./console/dc/bake-dc6 -mpq "$D2DIR/$MPQ_GRANDE_REAL" -pal "$PAL_PATH" \
        -out "$OUT/host" -name "$POC" -solo-pal )
else
    echo "   (la paleta ya estaba en $OUT/host/$POC.pal: se reusa)"
fi

echo
echo "== 1/5 stdlib minima + paquetes del motor para SH-4"
if [ "${SALTAR_STDLIB:-0}" = "1" ]; then
    echo "   (salteada: SALTAR_STDLIB=1)"
else
    bash "$HERE/build-std-gox.sh"
fi

if [ "${SALTAR_MOTOR:-0}" = "1" ]; then
    echo "   (paquetes del motor salteados: SALTAR_MOTOR=1)"
else
    PAQUETES="d2common/d2enum d2common/d2math d2common/d2math/d2vector d2networking/d2client/d2clientconnectiontype d2common/d2interface d2common/d2datautils d2common/d2data/d2compression d2common/d2fileformats/d2dc6 d2common/d2fileformats/d2pl2 d2common/d2fileformats/d2mpq" \
        bash "$HERE/build-motor-gox.sh" "$GEN"
fi

echo
echo "== 2/5 el ELF del POC"
FUENTE="$SRC/main.go"
if [ "${DC6_INEXISTENTE:-0}" = "1" ]; then
    # el MUTANTE: el nombre del asset apunta a uno que no existe (criterio 3 de la tarjeta).
    cp "$SRC/main.go" "$OUT/main.go.mutante"
    python3 - "$OUT/main.go.mutante" <<'PY'
import sys
p = sys.argv[1]
s = open(p, encoding="utf-8").read()
s = s.replace("waitingroombkgd.dc6", "no-existe-este-asset.dc6")
open(p, "w", encoding="utf-8").write(s)
PY
    FUENTE="$OUT/main.go.mutante"
fi

TMPPOC="$OUT/poc-src"
rm -rf "$TMPPOC"
mkdir -p "$TMPPOC"
cp -r "$SRC/." "$TMPPOC/"
if [ "${DC6_INEXISTENTE:-0}" = "1" ]; then
    cp "$OUT/main.go.mutante" "$TMPPOC/main.go"
fi

bash "$HERE/build-dc.sh" "$TMPPOC" "$OUT"

echo
echo "== 3/5 la ISO del disco con los archivos del juego en /JUEGO"
# Se rearma la MISMA ISO de arranque (IP.BIN + 1ST_READ.BIN) agregando los archivos del juego.
# DC-002.35: esto lo hace mkdisco-235.sh, que resuelve el nombre REAL del archivo (en el PC es
# `patch_D2.mpq` con D mayuscula y el ISO9660 no lo abre con otra caja). Antes esta linea le pasaba a
# mkiso.py el nombre tal como viene en la config (en minusculas): el archivo no existia y el disco salia
# sin el MPQ (el POC moria con "MPQ-ABRIR FALLO: file does not exist" y parecia un defecto del MPQ).
bash "$HERE/mkdisco-235.sh" "$OUT"

echo
echo "== 4/5 los numeros del HOST (con el MISMO codigo del motor) y el PNG de referencia"
# verif-dc6-235 imprime el CRC-32 del DC6, los primeros bytes, el CRC por trozo de 64 KiB, los frames, el
# lienzo compuesto y la paleta, y escribe el PNG contra el que se compara la captura. Antes este paso
# llamaba a crc-dc6-235.py con el nombre del POC, que no era el nombre de los archivos horneados
# (pal235.*): los numeros del host salian de un archivo distinto del que leia la consola.
( cd "$REPO" && go run ./console/dc/verif-dc6-235 -dc6 "$OUT/host/$POC.dc6" -pal "$OUT/host/$POC.pal" \
    -out "$OUT/host" -name "$POC" ) | sed 's/^/   /'

echo
echo "== 5/5 lo que quedo"
ls -la "$OUT"/prog.elf "$OUT"/disc.iso "$OUT"/disc.gdi 2>/dev/null | sed 's/^/   /'
