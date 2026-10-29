#!/bin/bash
# DC-002.62 -- EL PRIMER SONIDO POR EL AICA: el golpe y el recoger.
#
# QUE CAMBIA RESPECTO DE build-items-261.sh (y por que)
# ---------------------------------------------------------------------------------------------------
# `console/dc/poc/personaje-mapa/` trae UN archivo nuevo, `sonido.go`, y dos lineas de enganche
# (`main.go`: el play del golpe en el flanco del boton Y de DC-002.50, y el arranque del AICA antes del
# loop; `objetos.go`: el play de la recogida en `recoger()` de DC-002.61). El archivo trae los
# interruptores APAGADOS en el arbol:
#
#   var sonido = false       -> true: el AICA arranca (snd_init), los dos PCM se cargan y los dos
#                                     momentos suenan.
#   var sonidoSordo = false  -> true: EL MUTANTE del criterio 3 (los momentos se alcanzan, el AICA no
#                                     recibe un solo play: `total` queda en 0).
#
# LOS PCM LOS GENERA EL BUILD (`scripts/console/gen-pcm-262.py` -> $OUT/gen/sonidos_gen.go): los
# sonidos del juego viven en d2sfx.mpq, que NO viaja en la ISO del POC. El generador es deterministico
# (mismo crc32 en cualquier host) y deja cada efecto con su tamano, sus muestras y su tasa, que es lo
# que pide el criterio 2.
#
# El resto NO cambia una linea: el mapa, el personaje, el bicho, el golpe y los items son los mismos, y
# la ISO con los tres MPQ y la verificacion de los tres archivos compartidos con el host las sigue
# haciendo build-monstruo-250.sh (el script de la casa), igual que en DC-002.61.
#
# Uso:  scripts/console/build-sonido-262.sh [dir_salida]
#       SONIDO=0          -> el BASELINE de esta tarjeta: el binario de DC-002.61 SIN una sola llamada
#                            al AICA (para comparar el log y el tamano).
#       SONIDO_SORDO=1    -> el MUTANTE del criterio 3: los momentos se alcanzan, no hay play.
#       PCM_SILENCIO=1    -> el MUTANTE del generador: PCM validos por tamano y tasa, con las muestras
#                            en cero.
#       ITEMS=0           -> sin items (el recoger no se puede alcanzar; sirve para aislar el golpe).
#       ITEMS_AUTO=1      -> enciende el instrumento de DC-002.61 (camina y recoge 3 items: asi suena
#                            tambien el efecto de RECOGER sin teclado).
#       GOLPEAUTO=0       -> apaga el instrumento del golpe de DC-002.50 (default 1 en este script:
#                            el emulador corre sin teclado y sin el no hay 10 golpes que medir).
#       SALTAR_STDLIB=1 / SALTAR_MOTOR=1 + GODC_GOX=~/dreamcast/gox  -> reusar los .gox ya compilados
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
POC="personaje-mapa"
OUT="${1:-${DC262_OUT:-/tmp/dcbuild/sonido-262}}"
SRC="$REPO/console/dc/poc/$POC"
COPIA="$OUT/poc-sonido"

mkdir -p "$OUT" "$COPIA"
cp -r "$SRC/." "$COPIA/"

echo "== 0/3 los interruptores del sonido (en la COPIA, el arbol no se toca)"
if [ "${SONIDO:-1}" = "1" ]; then
    sed -i 's|^var sonido = false$|var sonido = true|' "$COPIA/sonido.go"
    grep -q '^var sonido = true$' "$COPIA/sonido.go" || {
        echo "   FALLO: no pude encender el sonido (no encontre 'var sonido = false')"; exit 1; }
    echo "   SONIDO=ON: snd_init, los dos PCM al AICA y los dos momentos suenan"
else
    echo "   SONIDO=0: el BASELINE (el binario de DC-002.61, sin una linea de audio)"
fi

if [ "${SONIDO_SORDO:-0}" = "1" ]; then
    sed -i 's|^var sonidoSordo = false$|var sonidoSordo = true|' "$COPIA/sonido.go"
    grep -q '^var sonidoSordo = true$' "$COPIA/sonido.go" || {
        echo "   FALLO: no pude poner el mutante (no encontre 'var sonidoSordo = false')"; exit 1; }
    echo "   MUTANTE: sonidoSordo=true (los momentos se alcanzan, el AICA no recibe el play)"
fi

if [ "${ITEMS:-1}" = "1" ]; then
    sed -i 's|^var items = false$|var items = true|' "$COPIA/objetos.go"
    grep -q '^var items = true$' "$COPIA/objetos.go" || {
        echo "   FALLO: no pude encender los items (no encontre 'var items = false')"; exit 1; }
    echo "   ITEMS=ON: los 41 objetos del suelo y el boton A recoge (es el momento que suena RECOGER)"
else
    echo "   ITEMS=0: sin items en el suelo (el recoger no se alcanza)"
fi

if [ "${ITEMS_AUTO:-0}" = "1" ]; then
    sed -i 's|^var itemsAuto = false$|var itemsAuto = true|' "$COPIA/objetos.go"
    grep -q '^var itemsAuto = true$' "$COPIA/objetos.go" || {
        echo "   FALLO: no pude encender el instrumento de los items"; exit 1; }
    echo "   ITEMS_AUTO=ON: camina y recoge 3 items solo (suena el efecto de RECOGER)"
fi

echo
echo "== 1/3 los dos PCM, generados por el build"
if [ "${PCM_SILENCIO:-0}" = "1" ]; then
    python3 "$HERE/gen-pcm-262.py" "$OUT/gen" --silencio | tee "$OUT/pcm.txt"
    echo "   MUTANTE: PCM con las muestras en CERO (mismo tamano y tasa, no suena)"
else
    python3 "$HERE/gen-pcm-262.py" "$OUT/gen" | tee "$OUT/pcm.txt"
fi

echo
echo "== 2/3 el resto del camino es el de la casa: POC_DIR=<copia> build-monstruo-250.sh $OUT"
POC_DIR="$COPIA" BESTIA="${BESTIA:-1}" GOLPEAUTO="${GOLPEAUTO:-1}" GOLPE_SORDO="${GOLPE_SORDO:-0}" \
    bash "$HERE/build-monstruo-250.sh" "$OUT"

echo
echo "== 3/3 los numeros de ESTA tarjeta"
# OJO: los interruptores se informan sobre $OUT/poc (la copia que build-monstruo-250 compila, con sus
# propios sed de animar/bestia/golpeAuto) y NO sobre $COPIA, que es la que armo este script.
echo "sonido=$(grep -c '^var sonido = true$' "$OUT/poc/sonido.go") \
sonidoSordo=$(grep -c '^var sonidoSordo = true$' "$OUT/poc/sonido.go") \
items=$(grep -c '^var items = true$' "$OUT/poc/objetos.go") \
itemsAuto=$(grep -c '^var itemsAuto = true$' "$OUT/poc/objetos.go") \
golpeAuto=$(grep -c '^var golpeAuto = true$' "$OUT/poc/main.go") \
bestia=$(grep -c '^var bestia = true$' "$OUT/poc/main.go")"
echo "ELF: $OUT/prog.elf ($(stat -c%s "$OUT/prog.elf") bytes)"
echo "GDI: $OUT/disc.gdi"
echo "PCM: gen/sonidos_gen.go ($(stat -c%s "$OUT/gen/sonidos_gen.go") bytes)"
