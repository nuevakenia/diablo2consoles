#!/bin/bash
# DC-002.103 -- compilar y enlazar el POC DEL PUEBLO EN PANTALLA (componer -> soltar el arena -> dibujar por el PVR): el pueblo compuesto por el
# MOTOR (`d2mapengine`) y dibujado por el camino de dibujo del MOTOR (`d2maprenderer`) sobre el backend PVR, en la Dreamcast.
#
# POR QUE UN SCRIPT PROPIO. El POC importa medio motor (`d2asset`, `d2mapengine`, `d2maprenderer`, `d2term`,
# `d2render/pvr`) y eso no es stdlib: se arma un arbol de ENLACE con enlaces simbolicos a la gox del motor y
# una orden.txt = stdlib + el cierre de lo que este POC importa. El cierre NO se escribe a mano: sale de
# `go list -deps` (con -e, porque los paquetes de video no resuelven en el host).
#
# SE RECOMPILAN EN EL ARBOL BASE los paquetes que esta tarjeta toca o que el POC importa del compositor y del
# dibujado: `d2core/d2map/d2maprenderer` (palancas de cuadro y cache perezosa), `d2render/pvr`, `d2term`,
# `d2asset`, `d2mapengine`, `d2mapstamp` y la cadena de lectura -- para que lo que corre en Flycast sea el
# codigo de HOY y no un .gox viejo (el defecto de DC-002.90, medido).
#
# LA IMAGEN LLEVA DOS MPQ: PATCH_D2.MPQ (las tablas) y D2DATA.MPQ (los .ds1 y los .dt1).
#
# Uso:  scripts/console/build-mapa-103.sh [dir_salida]
#       BASE_GOX=...   arbol del motor compilado (por defecto /tmp/gox-296, el de DC-002.96/.99)
#       SALTAR_GEN=1    reusar el arbol de enlace ya armado
#       SALTAR_MOTOR=1  NO recompilar los 21 paquetes del motor (la iteracion del POC sola: solo cambia el
#                       main del POC, los .gox del motor son los mismos de la corrida anterior)
#       ORDEN=...       pisa la palanca `orden` de knobs.go (contrato|prestamo|tarjeta). Se AVISA EN GRANDE:
#                       el knobs.go del POC es la unica fuente de verdad (la leccion de DC-002.90)
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
SRC="$REPO/console/dc/poc/mapa-103"
BASE_GOX="${BASE_GOX:-/tmp/gox-296}"
OUT="${1:-/tmp/dcbuild/mapa103}"
GEN="$OUT/gox-enlace"
PREFIJO="github.com/OpenDiablo2/OpenDiablo2"
SRCB="$OUT/src"

# los paquetes que este build recompila en el arbol, en orden (cada uno contra los .gox de los anteriores)
PAQUETES_MOTOR="d2common/d2datautils
 d2common/d2fileformats/d2dt1
 d2common/d2fileformats/d2ds1
 d2common/d2fileformats/d2mpq
 d2common/d2cache
 d2common/d2fileformats/d2txt
 d2common/d2loader/asset/types
 d2common/d2loader/asset
 d2common/d2loader/mpq
 d2common/d2loader/filesystem
 d2common/d2loader
 d2core/d2records
 d2core/d2asset
 d2core/d2term
 d2core/d2render/pvr
 d2core/d2map/d2lightmap
 d2core/d2map/d2mapentity
 d2core/d2map/d2mapstamp
 d2core/d2map/d2mapengine
 d2core/d2map/d2maprenderer
 console/dc/fuenteconsola"

[ -f "$BASE_GOX/orden.txt" ] || { echo "falta el arbol del motor en $BASE_GOX (corre build-motor-all.sh)" >&2; exit 1; }
mkdir -p "$OUT"

# ============================== EL ARBOL DE .gox (DC-002.91) ==============================
BASE_GOX="$BASE_GOX" RECOMPILA="$PAQUETES_MOTOR" "$HERE/revisar-arbol-gox.sh"

echo "== 1/5 los paquetes que se compilan aca (el motor que se dibuja)"
if [ "${SALTAR_MOTOR:-0}" = "1" ]; then
    echo "   SALTAR_MOTOR=1: se reusan los .gox del arbol $BASE_GOX (no cambio ningun paquete del motor)"
    PAQUETES_MOTOR=""
fi
for p in $PAQUETES_MOTOR; do
    mapfile -t SRC_P < <(ls "$REPO/$p"/*.go | grep -v '_test\.go$' |
        grep -vE '_(darwin|windows|js|wasm|plan9|freebsd|netbsd|openbsd|linux|android|ios|solaris|aix)\.go$' |
        grep -vE '_(386|amd64|arm|arm64|ppc64|ppc64le|mips|mipsle|mips64|mips64le|riscv64|s390x|loong64|wasm)\.go$' |
        grep -vE '/(marca_carga_host)\.go$' |
        grep -vE '/(object_lookup_record_data|object_lookups_host)\.go$')
    # DC-002.103 -- LA EXCLUSION QUE FALTABA (defecto encontrado en esta tarjeta, medido). El build de consola
    # (`build-motor-all.sh`, DC-002.42) NO compila el VOLCADO de la tabla de objectLookups ni su lado del host:
    # `object_lookup_record_data.go` es UN simbolo de .data de 1.672.892 B mas 116.048 de sus cadenas =
    # 1.788.960 B de imagen, y el lugar de esa tabla es el DISCO (`/JUEGO/OBJECTLOOKUPS.PACK`, que el motor lee
    # por registro). Los builds de los POC de DC-002.101/.102 (de donde sale este script) NO traian esa
    # exclusion, asi que sus imagenes median 7,03 MB y el monton de C arrancaba 1,79 MB mas abajo que en el
    # binario del motor. Medido en esta tarjeta con `sh-elf-nm -S` sobre el .gox: `_go..C3075 d 01672892`.
    # OJO: gccgo NO respeta `//go:build`, la exclusion es por NOMBRE (medido en DC-002.42).
    mkdir -p "$(dirname "$BASE_GOX/$PREFIJO/$p.gox")"
    sh-elf-gccgo $GODC_GOFLAGS -I "$BASE_GOX" \
        -fgo-pkgpath="$PREFIJO/$p" \
        -c "${SRC_P[@]}" \
        -o "$BASE_GOX/$PREFIJO/$p.gox"
    printf '   %-34s %9s B\n' "$(basename "$p").gox" "$(stat -c%s "$BASE_GOX/$PREFIJO/$p.gox")"
done

# el cierre del POC, con `go list -deps` sobre el paquete del POC
PAQUETES="$(cd "$REPO" && go list -deps -e -tags dreamcast ./console/dc/poc/mapa-103 2>/dev/null |
    grep "$PREFIJO" | sed "s|.*$PREFIJO/||" | grep -vx 'console/dc/poc/mapa-103' |
    sort | tr '\n' ' ')"
[ -n "${PAQUETES// /}" ] || { echo "la lista de paquetes quedo vacia (go list -deps fallo?)" >&2; exit 1; }

echo "== 2/5 el arbol de enlace ($GEN)"
if [ "${SALTAR_GEN:-0}" != "1" ]; then
    rm -rf "$GEN"
    mkdir -p "$GEN"
    # enlaces simbolicos, no copias: los .gox del motor son decenas de MB y no se tocan desde aca
    cp -rs "$BASE_GOX/." "$GEN"/
fi
rm -f "$GEN/orden.txt"
# DC-002.101: las rutas de `orden.txt` vienen con el prefijo del arbol BASE y el arbol de enlace son ENLACES
# SIMBOLICOS a esos mismos archivos: si se copian tal cual y despues se agregan las rutas de `$GEN`, el
# enlazador ve DOS veces el mismo .gox ("multiple definition of ...", medido en este build). Se reescribe el
# prefijo para que cada paquete quede UNA vez, apuntando al arbol de enlace.
sed "s|^$BASE_GOX/|$GEN/|" "$BASE_GOX/orden.txt" > "$GEN/orden.txt"
for p in $PAQUETES; do
    f="$GEN/$PREFIJO/$p.gox"
    [ -f "$f" ] || { echo "   FALTA $p.gox en el arbol del motor: no se puede enlazar lo que no existe" >&2; exit 1; }
    grep -qxF "$f" "$GEN/orden.txt" || echo "$f" >> "$GEN/orden.txt"
done
echo "   paquetes del motor en el enlace: $(printf '%s\n' $PAQUETES | wc -l)"
echo "   objetos en orden.txt: $(wc -l < "$GEN/orden.txt")"

echo "== 3/5 el directorio de fuentes del POC"
rm -rf "$SRCB"
mkdir -p "$SRCB"
cp "$SRC"/*.go "$SRCB"/
cp -r "$SRC/csrc" "$SRCB"/

# ============================== LAS PALANCAS (DC-002.90) ==============================
# UNA SOLA FUENTE DE VERDAD: el knobs.go del POC. El entorno solo puede PISARLO y, cuando lo pisa, se dice
# aca y el valor queda escrito en el knobs.go del directorio de build (el binario dice con cual se compilo).
leer_knob() {  # $1 = const de knobs.go -> imprime su valor tal cual
    awk -v n="$1" '
        {
            if ($1 == "const") { nom = $2; igual = $3; val = $4 } else { nom = $1; igual = $2; val = $3 }
            if (nom == n && igual == "=" && !visto) { print val; visto = 1; exit }
        }' "$SRCB/knobs.go" | tr -d '"'
}

ORDEN_KNOB="$(leer_knob orden)"
[ -n "$ORDEN_KNOB" ] || { echo "   knobs.go no declara 'orden'" >&2; exit 1; }
if [ -n "${ORDEN:-}" ] && [ "$ORDEN" != "$ORDEN_KNOB" ]; then
    echo "   ****************************************************************"
    echo "   * PISADA DE PALANCA: orden=$ORDEN (knobs.go dice $ORDEN_KNOB)  *"
    echo "   * el binario NO es el de la referencia del POC                  *"
    echo "   ****************************************************************"
    sed -i "s/^\torden = \".*\"$/\torden = \"$ORDEN\"/" "$SRCB/knobs.go"
    ORDEN_KNOB="$ORDEN"
fi
echo "   orden=$ORDEN_KNOB cuadros=$(leer_knob cuadros) modoTablas=$(leer_knob modoTablas)"

# DC-002.105: las palancas de esta tarjeta -- cuanto pide el tramo de la cache, si se reserva al arranque y que se
# suelta despues de componer. Misma regla que `orden`: se pisan SOLO por entorno y se avisa en grande.
pisa_knob() {  # $1 = nombre en knobs.go, $2 = valor nuevo (sin comillas), $3 = comillas (1 = cadena)
    local actual
    actual="$(leer_knob "$1")"
    [ -n "$actual" ] || { echo "   knobs.go no declara '$1'" >&2; exit 1; }
    [ "$actual" = "$2" ] && return 0
    echo "   ****************************************************************"
    printf '   * PISADA DE PALANCA: %-24s %s -> %s\n' "$1" "$actual" "$2"
    echo "   * el binario NO es el de la referencia del POC"
    echo "   ****************************************************************"
    if [ "${3:-0}" = "1" ]; then
        sed -i "s/^\t$1 = \".*\"$/\t$1 = \"$2\"/" "$SRCB/knobs.go"
    else
        sed -i "s/^\t$1 = .*/\t$1 = $2/" "$SRCB/knobs.go"
    fi
}

[ -n "${TRAMO_CACHE:-}" ] && pisa_knob tramoDeLaCache "$TRAMO_CACHE"
[ -n "${PEDAZO:-}" ] && pisa_knob tramoEnPedazos "$PEDAZO"
[ -n "${SONDA:-}" ] && pisa_knob pedazoLoDiceElMonton "$SONDA"
[ -n "${TRAMO_ARRANQUE:-}" ] && pisa_knob tramoAlArranque "$TRAMO_ARRANQUE"
[ -n "${SOLTAR:-}" ] && pisa_knob soltarDespuesDeComponer "$SOLTAR" 1

echo "   soltarDespuesDeComponer=$(leer_knob soltarDespuesDeComponer) tramoDeLaCache=$(leer_knob tramoDeLaCache) tramoEnPedazos=$(leer_knob tramoEnPedazos) pedazoLoDiceElMonton=$(leer_knob pedazoLoDiceElMonton) tramoAlArranque=$(leer_knob tramoAlArranque)"

echo "   $(ls "$SRCB"/*.go | wc -l) archivos .go: $(cd "$SRCB" && ls *.go | tr '\n' ' ')"

echo "== 4/5 compilando y enlazando contra los .gox del motor"
GODC_GOX="$GEN" bash "$HERE/build-dc.sh" "$SRCB" "$OUT" > "$OUT/build.log" 2>&1 || {
    echo "   FALLO el build (ver $OUT/build.log)"; tail -25 "$OUT/build.log"; exit 1; }

echo "== 5/5 la imagen con PATCH_D2.MPQ y D2DATA.MPQ en /JUEGO"
D2DIR="${D2_DIR:-$HOME/.wine-d2/drive_c/Games/Diablo II}"
MPQ_PATCH="$(ls "$D2DIR" | grep -ix 'patch_d2.mpq' | head -1)"
MPQ_DATA="$(ls "$D2DIR" | grep -ix 'd2data.mpq' | head -1)"
[ -n "$MPQ_PATCH" ] || { echo "   no encuentro patch_d2.mpq en $D2DIR" >&2; exit 1; }
[ -n "$MPQ_DATA" ] || { echo "   no encuentro d2data.mpq en $D2DIR" >&2; exit 1; }
LISTA="$OUT/iso-lista.txt"
{
    printf '%s:/JUEGO/PATCH_D2.MPQ;1\n' "$D2DIR/$MPQ_PATCH"
    printf '%s:/JUEGO/D2DATA.MPQ;1\n' "$D2DIR/$MPQ_DATA"
} > "$LISTA"
MKISO_JOLIET=0 "$DC_PYTHON" "$HERE/mkiso.py" "$OUT/disc.iso" "$OUT/IP.BIN" "$OUT/1ST_READ.BIN" "@$LISTA" | tail -2
"$DC_PYTHON" "$HERE/mkgdi.py" "$OUT" | tail -2

for f in prog.elf 1ST_READ.BIN disc.iso; do
    [ -f "$OUT/$f" ] && printf '   %-14s %10s B\n' "$f" "$(stat -c%s "$OUT/$f")"
done
printf '   codigo (.text)  %10s B\n' "$(sh-elf-size -A "$OUT/prog.elf" | awk '$1==".text"{print $2}')"
printf '   data+bss        %10s B\n' "$(sh-elf-size -A "$OUT/prog.elf" | awk '$1==".data"||$1==".bss"{s+=$2} END{print s}')"
[ -f "$OUT/disc.gdi" ] && echo "   imagen:         $OUT/disc.gdi"
