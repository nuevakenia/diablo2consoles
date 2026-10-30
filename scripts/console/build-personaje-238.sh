#!/bin/bash
# DC-002.38: el ELF del POC del PERSONAJE EN EL MAPA y el DISCO con los DOS MPQ que necesita en /JUEGO:
#   /JUEGO/D2DATA.MPQ  el mapa del acto 1 (.ds1 + DT1) y las paletas
#   /JUEGO/D2CHAR.MPQ  el COF del heroe y los .dcc de sus capas
#
# El POC se llama `personaje-mapa` y comparte con el verificador del host los DOS archivos de siempre:
# `nivel.go` (el compositor del nivel de DC-002.37) y `personaje.go` (el compuesto del personaje). El sha256
# de los cuatro archivos queda en la evidencia: si se edita uno solo de un lado, la comparacion no vale.
#
# Uso:  scripts/console/build-personaje-238.sh [dir_salida]
#       SALTAR_STDLIB=1 / SALTAR_MOTOR=1 para reusar el gox ya compilado (con GODC_GOX propio).
#       CONTROL_SORDO=1 -> compila el MUTANTE del criterio 3 (el control se lee y se informa, pero NO mueve).
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
POC="personaje-mapa"
SRC="$REPO/console/dc/poc/$POC"
OUT="${1:-${DC238_OUT:-/tmp/dcbuild/personaje-mapa}}"
GEN="${GODC_GOX:-$DC_ROOT/gox}"

D2DIR="${D2_DIR:-$HOME/.wine-d2/drive_c/Games/Diablo II}"

mkdir -p "$OUT" "$OUT/host"

echo "== 0/5 los dos archivos compartidos (los mismos del host)"
for f in nivel.go personaje.go; do
    if ! cmp -s "$SRC/$f" "$REPO/console/dc/verif-personaje-238/$f"; then
        echo "   FALLO: $SRC/$f y console/dc/verif-personaje-238/$f NO son el mismo archivo"
        exit 1
    fi
    echo "   $f $(sha256sum "$SRC/$f" | cut -c1-16)"
done

echo "== 1/5 la stdlib minima y el MOTOR que este POC importa, en $GEN"
if [ "${SALTAR_STDLIB:-0}" != "1" ]; then
    bash "$HERE/build-std-gox.sh" "$GEN" > "$OUT/stdlib.log" 2>&1 || {
        echo "FALLO la stdlib (ver $OUT/stdlib.log)"; exit 1; }
    echo "   stdlib OK ($(ls "$GEN"/*.gox 2>/dev/null | wc -l) .gox)"
else
    echo "   stdlib SALTEADA (SALTAR_STDLIB=1)"
fi

# LOS PAQUETES DEL MOTOR QUE ENTRAN AL BINARIO: los de DC-002.37 mas los tres del personaje
# (d2cof, d2dcc, d2dc6) y su cierre de dependencias.
# DC-002.73a: el cierre se MEDIA mal. Desde DC-002.49/.50 el POC trae `monstruo.go`, que importa
# d2fileformats/d2txt + d2monstats + d2math/d2rand + d2monster (las tablas del excel, la vida y el dado del
# motor, los modos NU/GH/DT/DD), y esta lista no los tenia: el enlace moria con
# `undefined reference to ...d2monstats.Class..p`, `...d2monster.Mode.String`, `...d2txt..types`,
# `d2rand.Seed..p` (los cuatro, medidos). La lista es ahora la MISMA que la de build-monstruo-250.sh
# (PAQUETES250, 19 paquetes) y se verifica sola abajo contra `go list -deps -tags dreamcast` del POC.
PAQUETES238="d2common/d2enum d2common/d2math d2common/d2geom d2common/d2math/d2vector d2common/d2path \
d2networking/d2client/d2clientconnectiontype d2common/d2interface d2common/d2datautils \
d2common/d2data/d2compression d2common/d2fileformats/d2ds1 d2common/d2fileformats/d2dt1 \
d2common/d2fileformats/d2mpq d2common/d2fileformats/d2cof d2common/d2fileformats/d2dcc \
d2common/d2fileformats/d2dc6 d2common/d2fileformats/d2txt d2common/d2monstats \
d2common/d2math/d2rand d2common/d2monster"

# El cierre del POC medido con `go list -deps` (no a mano): si alguien vuelve a agregar un import al POC sin
# tocar la lista de arriba, esto lo dice ANTES de llegar al enlazador con un `undefined reference` de 200
# lineas. Solo se exige un sentido (todo lo que el POC importa tiene que estar en la lista); la lista puede
# traer de mas (d2geom, d2data/d2compression).
CIERRE_POC="$( ( cd "$REPO" && go list -deps -e -tags dreamcast ./console/dc/poc/$POC 2>/dev/null ) \
    | grep "^github.com/OpenDiablo2/OpenDiablo2/" | sed 's|.*OpenDiablo2/||' \
    | grep -v "^console/" | sort || true)"    # sin el paquete del POC mismo (es el main, no un .gox)
# OJO CON EL -tags: los .go del POC llevan `//go:build dreamcast`, y sin la etiqueta `go list` ni los mira
# (medido: sin -tags el cierre sale 16 y deja afuera d2dcc/d2dc6/d2geom, que si entran al enlace).
LISTA238=" $(printf '%s ' $PAQUETES238)"   # sin comillas: colapsa los saltos de linea de la lista
FALTAN=""
for p in $CIERRE_POC; do
    case "$LISTA238" in
        *" $p "*) ;;
        *) FALTAN="$FALTAN $p" ;;
    esac
done
if [ -n "$FALTAN" ]; then
    echo "   AVISO: el POC importa paquetes que no estan en PAQUETES238:$FALTAN"
    echo "          (el enlace va a fallar con undefined reference: agregalos a la lista)"
else
    echo "   el cierre del POC ($(printf '%s\n' $CIERRE_POC | wc -l) paquetes del motor) esta en PAQUETES238"
fi

if [ "${SALTAR_MOTOR:-0}" != "1" ]; then
    PAQUETES="$PAQUETES238" bash "$HERE/build-motor-gox.sh" "$GEN" > "$OUT/motor.log" 2>&1 || {
        echo "FALLO el motor (ver $OUT/motor.log)"; tail -5 "$OUT/motor.log"; exit 1; }
    N_MOTOR="$(printf '%s\n' $PAQUETES238 | wc -l)"
    echo "   motor OK: $N_MOTOR paquetes"
    for p in $PAQUETES238; do
        grep -qxF "$GEN/github.com/OpenDiablo2/OpenDiablo2/$p.gox" "$GEN/orden.txt" || {
            echo "   FALLO: $p.gox no quedo en orden.txt (el enlace no lo veria)"; exit 1; }
    done
    echo "   los $N_MOTOR .gox estan en orden.txt ($(wc -l < "$GEN/orden.txt") objetos para el linker)"
else
    echo "   motor SALTEADO (SALTAR_MOTOR=1)"
fi

echo
echo "== 2/5 el ELF del POC ($POC)"
TMPPOC="$OUT/poc"
rm -rf "$TMPPOC"
mkdir -p "$TMPPOC"
cp -r "$SRC/." "$TMPPOC/"

if [ "${CONTROL_SORDO:-0}" = "1" ]; then
    # el MUTANTE (criterio 3 de la tarjeta): el control se sigue leyendo y se sigue informando, pero NO se
    # engancha al movimiento. Es la MISMA corrida con otra tecla: si la posicion cambia igual, el criterio
    # (que el movimiento sea el del control) no vale.
    sed -i 's|^var controlSordo = false$|var controlSordo = true|' "$TMPPOC/main.go"
    grep -q '^var controlSordo = true$' "$TMPPOC/main.go" || {
        echo "   FALLO: no pude poner el mutante (no encontre 'var controlSordo = false')"; exit 1; }
    echo "   MUTANTE: controlSordo=true (el control se informa, no mueve)"
fi

GODC_GOX="$GEN" bash "$HERE/build-dc.sh" "$TMPPOC" "$OUT"

echo
echo "== 3/5 la ISO con los DOS MPQ en /JUEGO"
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
echo "== 4/5 los numeros del host (los MISMOS que tiene que dar la consola)"
if [ "${CONTROL_SORDO:-0}" != "1" ]; then
    ( cd "$REPO" && go run ./console/dc/verif-personaje-238 \
        -png "$OUT/host/referencia.png" -png-mapa "$OUT/host/mapa.png" ) | tee "$OUT/host/verif.txt"
fi

echo
echo "== 5/5 el sha256 de lo que se comparo (para la evidencia)"
( cd "$REPO" && sha256sum console/dc/poc/personaje-mapa/nivel.go console/dc/poc/personaje-mapa/personaje.go \
    console/dc/verif-personaje-238/nivel.go console/dc/verif-personaje-238/personaje.go ) \
    | tee "$OUT/host/sha256.txt"

echo
echo "ELF:    $OUT/prog.elf   ($(stat -c%s "$OUT/prog.elf") bytes)"
echo "ISO:    $OUT/disc.iso  ($(stat -c%s "$OUT/disc.iso") bytes)"
echo "GDI:    $OUT/disc.gdi"
