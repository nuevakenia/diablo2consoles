#!/bin/bash
# DC-002.50 -- GOLPEAR ENEMIGOS: el POC del personaje con un BICHO del acto 1 en el mapa y un GOLPE a mano
# limpia que le baja la vida, con el disco y los TRES MPQ que necesita en /JUEGO (D2DATA, D2CHAR y PATCH_D2).
#
# QUE CAMBIA RESPECTO DE build-anim-249.sh (y por que)
# ---------------------------------------------------------------------------------------------------
# `console/dc/poc/personaje-mapa/main.go` trae un tercer interruptor de COMPILACION, APAGADO en el arbol:
#
#   var bestia = false   -> true: compone los CUATRO modos del bicho (NU parado, GH golpeado, DT muriendo,
#                                 DD muerto) y el ciclo A1 del heroe, pone el bicho en el mapa, y el boton Y
#                                 del control golpea: el dano sale del dado del motor y la vida de las tablas.
#   var golpeSordo = false -> true: EL MUTANTE del criterio 4 (el boton se lee y se informa pero NO golpea).
#
# La vida NO esta escrita en el codigo: sale de monstats.txt (d2data.mpq) y MonLvl.txt (patch_D2.mpq) con el
# camino de siembra del motor, en `monstruo.go`, que es EL MISMO archivo que usa el verificador del host
# (`console/dc/verif-monstruo-250`). El script compara los tres archivos compartidos antes de compilar.
#
# Uso:  scripts/console/build-monstruo-250.sh [dir_salida]
#       BESTIA=0        -> el binario de DC-002.49 (sin bicho), para comparar
#       GOLPE_SORDO=1   -> el MUTANTE (el boton se lee, la vida no cambia)
#       SALTAR_STDLIB=1 / SALTAR_MOTOR=1 + GODC_GOX=~/dreamcast/gox  -> reusar los .gox ya compilados
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
POC="personaje-mapa"
# DC-002.49b: POC_DIR permite compilar una COPIA del POC (por ejemplo con el giro de la direccion aplicado)
# sin tocar el archivo del repo, que otra tarjeta puede estar editando en ese momento. Sin POC_DIR todo queda
# como estaba: el POC del repo. La verificacion de los tres archivos compartidos se hace igual.
SRC="${POC_DIR:-$REPO/console/dc/poc/$POC}"
VERIF="$REPO/console/dc/verif-monstruo-250"
OUT="${1:-${DC250_OUT:-/tmp/dcbuild/personaje-mapa-monstruo250}}"
GEN="${GODC_GOX:-$DC_ROOT/gox}"

D2DIR="${D2_DIR:-$HOME/.wine-d2/drive_c/Games/Diablo II}"

mkdir -p "$OUT" "$OUT/host"

echo "== 0/6 los TRES archivos compartidos con el host (tienen que ser EL MISMO archivo)"
for f in nivel.go personaje.go monstruo.go; do
    if ! cmp -s "$SRC/$f" "$VERIF/$f"; then
        echo "   FALLO: $SRC/$f y $VERIF/$f NO son el mismo archivo"
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

# los paquetes del motor que entran al binario: los de DC-002.49 mas los CUATRO del monstruo (las tablas del
# excel, la vida del motor, el dado del motor y los modos NU/GH/DT/DD).
PAQUETES250="d2common/d2enum d2common/d2math d2common/d2geom d2common/d2math/d2vector d2common/d2path \
d2networking/d2client/d2clientconnectiontype d2common/d2interface d2common/d2datautils \
d2common/d2data/d2compression d2common/d2fileformats/d2ds1 d2common/d2fileformats/d2dt1 \
d2common/d2fileformats/d2mpq d2common/d2fileformats/d2cof d2common/d2fileformats/d2dcc \
d2common/d2fileformats/d2dc6 d2common/d2fileformats/d2txt d2common/d2monstats \
d2common/d2math/d2rand d2common/d2monster"

if [ "${SALTAR_MOTOR:-0}" != "1" ]; then
    PAQUETES="$PAQUETES250" bash "$HERE/build-motor-gox.sh" "$GEN" > "$OUT/motor.log" 2>&1 || {
        echo "FALLO el motor (ver $OUT/motor.log)"; tail -5 "$OUT/motor.log"; exit 1; }
    echo "   motor OK: $(printf '%s\n' $PAQUETES250 | wc -l) paquetes"
else
    echo "   motor SALTEADO (SALTAR_MOTOR=1)"
fi

echo
echo "== 2/6 el ELF del POC, con los interruptores de DC-002.50"
TMPPOC="$OUT/poc"
rm -rf "$TMPPOC"
mkdir -p "$TMPPOC"
cp -r "$SRC/." "$TMPPOC/"

# el golpe necesita los ciclos: bestia=ON exige animar=ON (el POC se planta si no)
sed -i 's|^var animar = false$|var animar = true|' "$TMPPOC/main.go"
grep -q '^var animar = true$' "$TMPPOC/main.go" || {
    echo "   FALLO: no pude encender el ciclo (no encontre 'var animar = false')"; exit 1; }
echo "   ANIMAR=ON: el ciclo del personaje corre (TN / TW / A1)"

if [ "${BESTIA:-1}" = "1" ]; then
    sed -i 's|^var bestia = false$|var bestia = true|' "$TMPPOC/main.go"
    grep -q '^var bestia = true$' "$TMPPOC/main.go" || {
        echo "   FALLO: no pude encender el bicho (no encontre 'var bestia = false')"; exit 1; }
    echo "   BESTIA=ON: el Returned en el mapa y el boton Y golpea"
else
    echo "   BESTIA=0: el binario de DC-002.49 (sin bicho)"
fi

if [ "${GOLPE_SORDO:-0}" = "1" ]; then
    sed -i 's|^var golpeSordo = false$|var golpeSordo = true|' "$TMPPOC/main.go"
    grep -q '^var golpeSordo = true$' "$TMPPOC/main.go" || {
        echo "   FALLO: no pude poner el mutante (no encontre 'var golpeSordo = false')"; exit 1; }
    echo "   MUTANTE: golpeSordo=true (el boton se lee, la vida no cambia)"
fi

GODC_GOX="$GEN" bash "$HERE/build-dc.sh" "$TMPPOC" "$OUT"

echo
echo "== 3/6 la ISO con los TRES MPQ en /JUEGO"
MPQ_DATA="$(ls "$D2DIR" | grep -ix 'd2data.mpq' | head -1)"
MPQ_CHARS="$(ls "$D2DIR" | grep -ix 'd2char.mpq' | head -1)"
MPQ_PATCH="$(ls "$D2DIR" | grep -ix 'patch_d2.mpq' | head -1)"
[ -n "$MPQ_DATA" ] || { echo "no encuentro d2data.mpq en $D2DIR"; exit 1; }
[ -n "$MPQ_CHARS" ] || { echo "no encuentro d2char.mpq en $D2DIR"; exit 1; }
[ -n "$MPQ_PATCH" ] || { echo "no encuentro patch_d2.mpq en $D2DIR"; exit 1; }

LISTA="$OUT/iso-lista.txt"
printf '%s:/JUEGO/D2DATA.MPQ;1\n%s:/JUEGO/D2CHAR.MPQ;1\n%s:/JUEGO/PATCH_D2.MPQ;1\n' \
    "$D2DIR/$MPQ_DATA" "$D2DIR/$MPQ_CHARS" "$D2DIR/$MPQ_PATCH" > "$LISTA"

MKISO_JOLIET=0 "$DC_PYTHON" "$HERE/mkiso.py" "$OUT/disc.iso" "$OUT/IP.BIN" "$OUT/1ST_READ.BIN" "@$LISTA" | tail -2
"$DC_PYTHON" "$HERE/mkgdi.py" "$OUT" | tail -2

echo
echo "== 4/6 el CENSO del BICHO y del GOLPE en el host (la referencia contra la que se mide la consola)"
( cd "$REPO" && go run ./console/dc/verif-monstruo-250 -mpq "$D2DIR/d2data.mpq" -png "$OUT/host" ) \
    | tee "$OUT/host/censo.txt" | grep -E 'VIDA|GOLPE|ANIM|DISTINTOS'

echo
echo "== 5/6 el sha256 de lo comparado (los tres .go compartidos y el binario)"
( cd "$REPO" && sha256sum console/dc/poc/$POC/nivel.go console/dc/poc/$POC/personaje.go \
    console/dc/poc/$POC/monstruo.go console/dc/verif-monstruo-250/nivel.go \
    console/dc/verif-monstruo-250/personaje.go console/dc/verif-monstruo-250/monstruo.go ) | tee "$OUT/host/sha256.txt"
sha256sum "$OUT/prog.elf" | tee -a "$OUT/host/sha256.txt"

echo
echo "== 6/6 los numeros del binario"
echo "ELF:    $OUT/prog.elf   ($(stat -c%s "$OUT/prog.elf") bytes)"
echo "ISO:    $OUT/disc.iso  ($(stat -c%s "$OUT/disc.iso") bytes)"
echo "GDI:    $OUT/disc.gdi"
echo "bestia=$(grep -c '^var bestia = true$' "$TMPPOC/main.go") golpeSordo=$(grep -c '^var golpeSordo = true$' "$TMPPOC/main.go")"
