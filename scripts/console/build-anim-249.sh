#!/bin/bash
# DC-002.49 -- EL PERSONAJE ANIMADO EN LA CONSOLA: el ELF del POC del personaje con el CICLO andando y el
# disco con los DOS MPQ que necesita en /JUEGO (/JUEGO/D2DATA.MPQ y /JUEGO/D2CHAR.MPQ).
#
# QUE CAMBIA RESPECTO DE build-personaje-238.sh (y por que)
# ---------------------------------------------------------------------------------------------------
# `console/dc/poc/personaje-mapa/main.go` trae dos interruptores de COMPILACION, los dos APAGADOS en el
# arbol (asi el mismo POC sigue sirviendo de instrumento de DC-002.38, cuyo corpus compara el compuesto NU
# cuadro 0 contra el host):
#
#   var animar     = false   -> true: compone los ciclos TN (parado, 16 cuadros) y TW (caminando, 8) y los
#                                   corre con el reloj de la consola, con el estado eligiendo el modo;
#   var cicloSordo = false   -> true: EL MUTANTE del criterio 3 (el cuadro se queda en 0, o sea el mismo
#                                   personaje tieso de DC-002.38: dos capturas quietas tienen que dar 0 px).
#
# Los dos se prenden con un sed sobre la copia del build, igual que CONTROL_SORDO en DC-002.38, y el sha256
# de cada binario queda en la evidencia: el medido y el mutante NO son el mismo archivo.
#
# Uso:  scripts/console/build-anim-249.sh [dir_salida]
#       ANIMAR=0          -> el binario de DC-002.38 (sin ciclo), para comparar
#       CICLO_SORDO=1     -> el MUTANTE (ciclo quieto en el cuadro 0)
#       SALTAR_STDLIB=1 / SALTAR_MOTOR=1 + GODC_GOX=~/dreamcast/gox  -> reusar los .gox ya compilados
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
POC="personaje-mapa"
SRC="$REPO/console/dc/poc/$POC"
OUT="${1:-${DC249_OUT:-/tmp/dcbuild/personaje-mapa-anim249}}"
GEN="${GODC_GOX:-$DC_ROOT/gox}"

D2DIR="${D2_DIR:-$HOME/.wine-d2/drive_c/Games/Diablo II}"

mkdir -p "$OUT" "$OUT/host"

echo "== 0/6 los dos archivos compartidos con el host (tienen que ser EL MISMO archivo)"
for f in nivel.go personaje.go; do
    if ! cmp -s "$SRC/$f" "$REPO/console/dc/verif-personaje-238/$f"; then
        echo "   FALLO: $SRC/$f y console/dc/verif-personaje-238/$f NO son el mismo archivo"
        exit 1
    fi
    echo "   $f $(sha256sum "$SRC/$f" | cut -c1-16)"
done

echo "== 1/6 la stdlib minima y el MOTOR que este POC importa, en $GEN"
if [ "${SALTAR_STDLIB:-0}" != "1" ]; then
    bash "$HERE/build-std-gox.sh" "$GEN" > "$OUT/stdlib.log" 2>&1 || {
        echo "FALLO la stdlib (ver $OUT/stdlib.log)"; exit 1; }
    echo "   stdlib OK ($(ls "$GEN"/*.gox 2>/dev/null | wc -l) .gox)"
else
    echo "   stdlib SALTEADA (SALTAR_STDLIB=1)"
fi

# los paquetes del motor que entran al binario: los de DC-002.37 mas los tres del personaje
PAQUETES249="d2common/d2enum d2common/d2math d2common/d2geom d2common/d2math/d2vector d2common/d2path \
d2networking/d2client/d2clientconnectiontype d2common/d2interface d2common/d2datautils \
d2common/d2data/d2compression d2common/d2fileformats/d2ds1 d2common/d2fileformats/d2dt1 \
d2common/d2fileformats/d2mpq d2common/d2fileformats/d2cof d2common/d2fileformats/d2dcc \
d2common/d2fileformats/d2dc6"

if [ "${SALTAR_MOTOR:-0}" != "1" ]; then
    PAQUETES="$PAQUETES249" bash "$HERE/build-motor-gox.sh" "$GEN" > "$OUT/motor.log" 2>&1 || {
        echo "FALLO el motor (ver $OUT/motor.log)"; tail -5 "$OUT/motor.log"; exit 1; }
    echo "   motor OK: $(printf '%s\n' $PAQUETES249 | wc -l) paquetes"
else
    echo "   motor SALTEADO (SALTAR_MOTOR=1)"
fi

echo
echo "== 2/6 el ELF del POC, con los interruptores de DC-002.49"
TMPPOC="$OUT/poc"
rm -rf "$TMPPOC"
mkdir -p "$TMPPOC"
cp -r "$SRC/." "$TMPPOC/"

if [ "${ANIMAR:-1}" = "1" ]; then
    sed -i 's|^var animar = false$|var animar = true|' "$TMPPOC/main.go"
    grep -q '^var animar = true$' "$TMPPOC/main.go" || {
        echo "   FALLO: no pude encender el ciclo (no encontre 'var animar = false')"; exit 1; }
    echo "   ANIMAR=1: el ciclo corre (TN quieto / TW caminando)"
else
    echo "   ANIMAR=0: el binario de DC-002.38 (sin ciclo)"
fi

if [ "${CICLO_SORDO:-0}" = "1" ]; then
    sed -i 's|^var cicloSordo = false$|var cicloSordo = true|' "$TMPPOC/main.go"
    grep -q '^var cicloSordo = true$' "$TMPPOC/main.go" || {
        echo "   FALLO: no pude poner el mutante (no encontre 'var cicloSordo = false')"; exit 1; }
    echo "   MUTANTE: cicloSordo=true (el cuadro se queda en 0)"
fi

GODC_GOX="$GEN" bash "$HERE/build-dc.sh" "$TMPPOC" "$OUT"

echo
echo "== 3/6 la ISO con los DOS MPQ en /JUEGO"
MPQ_DATA="$(ls "$D2DIR" | grep -ix 'd2data.mpq' | head -1)"
MPQ_CHARS="$(ls "$D2DIR" | grep -ix 'd2char.mpq' | head -1)"
[ -n "$MPQ_DATA" ] || { echo "no encuentro d2data.mpq en $D2DIR"; exit 1; }
[ -n "$MPQ_CHARS" ] || { echo "no encuentro d2char.mpq en $D2DIR"; exit 1; }

LISTA="$OUT/iso-lista.txt"
printf '%s:/JUEGO/D2DATA.MPQ;1\n%s:/JUEGO/D2CHAR.MPQ;1\n' \
    "$D2DIR/$MPQ_DATA" "$D2DIR/$MPQ_CHARS" > "$LISTA"

MKISO_JOLIET=0 "$DC_PYTHON" "$HERE/mkiso.py" "$OUT/disc.iso" "$OUT/IP.BIN" "$OUT/1ST_READ.BIN" "@$LISTA" | tail -2
"$DC_PYTHON" "$HERE/mkgdi.py" "$OUT" | tail -2

echo
echo "== 4/6 el CENSO del CICLO en el host (la referencia contra la que se mide la consola)"
( cd "$REPO" && go run ./console/dc/verif-personaje-238 -ciclo TN,TW -png-ciclo "$OUT/host" ) \
    | tee "$OUT/host/censo.txt" | grep -E 'MITAD|RESUMEN'

echo
echo "== 5/6 el sha256 de lo comparado (los dos .go compartidos y los DOS binarios)"
( cd "$REPO" && sha256sum console/dc/poc/personaje-mapa/nivel.go console/dc/poc/personaje-mapa/personaje.go \
    console/dc/verif-personaje-238/nivel.go console/dc/verif-personaje-238/personaje.go ) | tee "$OUT/host/sha256.txt"
sha256sum "$OUT/prog.elf" | tee -a "$OUT/host/sha256.txt"

echo
echo "== 6/6 los numeros del binario"
echo "ELF:    $OUT/prog.elf   ($(stat -c%s "$OUT/prog.elf") bytes)"
echo "ISO:    $OUT/disc.iso  ($(stat -c%s "$OUT/disc.iso") bytes)"
echo "GDI:    $OUT/disc.gdi"
grep -c '^var animar = true$' "$TMPPOC/main.go" >/dev/null && echo "animar=ON cicloSordo=$(grep -c '^var cicloSordo = true$' "$TMPPOC/main.go")"
