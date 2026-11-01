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
#       DC-002.109: FASE=encendido prende el ciclo de las tres fases, ORDEN_FASES=a|b elige el orden, y
#       TPRESET=/TARCHIVO= el `.ds1` de la TRANSICION (por defecto el preset 1 del pueblo, archivo 1 =
#       TownE1.ds1). Sin FASE el binario es la corrida de referencia de DC-002.106/.108.
#       DC-002.110: ARENA_DS1=encendido pone las rejillas de celdas del `.ds1` en UN bloque del monton de C (medido:
#       la carga del `.ds1` baja de +422.776 B a +42.232 B de semiespacio, pero el monton de C no tiene los 186.960 B
#       en el pico de componer y la corrida muere con `gc_external_alloc: OOM`); apagado -- el default de knobs.go --
#       es el camino de DC-002.109, donde VOLVER muere en el semiespacio.
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"
# shellcheck source=palancas-del-build.sh
. "$HERE/palancas-del-build.sh"   # DC-002.107: las exclusiones del build, por el camino unico

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
aviso_mutante_lookups
if [ "${SALTAR_MOTOR:-0}" = "1" ]; then
    echo "   SALTAR_MOTOR=1: se reusan los .gox del arbol $BASE_GOX (no cambio ningun paquete del motor)"
    PAQUETES_MOTOR=""
fi
for p in $PAQUETES_MOTOR; do
    # DC-002.107 -- LA EXCLUSION DE LA TABLA DE OBJECTLOOKUPS POR EL CAMINO UNICO. En DC-002.103 esta
    # exclusion se escribio ACA, copiada de build-motor-all.sh: eso es una segunda fuente de verdad, y es
    # lo mismo que dejo sin la exclusion a los builds de DC-002.101/.102 (1.788.960 B de imagen de regalo:
    # `object_lookup_record_data.go` es UN simbolo de .data de 1.672.892 B mas 116.048 B de sus cadenas).
    # El lugar de la tabla es el DISCO (`/JUEGO/OBJECTLOOKUPS.PACK`, que el motor lee por registro) y la
    # lista de lo que entra al build la da `palancas-del-build.sh`, no este archivo.
    mapfile -t SRC_P < <(fuentes_del_paquete "$REPO" "$p")
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
#
# DC-002.108: el prefijo NO siempre es el del arbol que se esta usando. `orden.txt` se escribe cuando se arma el
# arbol, y si despues el arbol se COPIA a otro lado (paso en esta tarjeta: /tmp es tmpfs y estaba lleno, asi que
# el arbol se copio a ~/gox108) las rutas siguen apuntando al original. Con el `sed` de un solo prefijo el
# enlazador veia los DOS (el de `orden.txt` y el del arbol de enlace) y moria con "multiple definition of ...".
# Ahora el prefijo se DEDUCE del propio archivo (la carpeta de su primera linea) y se reescribe ese.
RAIZ_ORDEN="$(sed -n '1{s|/[^/]*$||;p}' "$BASE_GOX/orden.txt")"
sed -e "s|^$BASE_GOX/|$GEN/|" -e "s|^$RAIZ_ORDEN/|$GEN/|" "$BASE_GOX/orden.txt" > "$GEN/orden.txt"
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
[ -n "${ENVRAM:-}" ] && { case "$ENVRAM" in 0) v=false ;; 1) v=true ;; *) v="$ENVRAM" ;; esac; pisa_knob cacheEnVram "$v"; }
[ -n "${PEDAZO:-}" ] && pisa_knob tramoEnPedazos "$PEDAZO"
[ -n "${SONDA:-}" ] && pisa_knob pedazoLoDiceElMonton "$SONDA"
[ -n "${TRAMO_ARRANQUE:-}" ] && pisa_knob tramoAlArranque "$TRAMO_ARRANQUE"
[ -n "${SOLTAR:-}" ] && pisa_knob soltarDespuesDeComponer "$SOLTAR" 1

# DC-002.109: el ciclo de las tres fases y su orden. FASE= ("" | a | b) y ORDEN_FASES= (a | b). El resto de las
# palancas de la tarjeta (el ds1 de la transicion) se pisan con TPRESET= / TARCHIVO=.
[ -n "${FASE:-}" ] && pisa_knob cicloDeFases "$FASE" 1
[ -n "${ORDEN_FASES:-}" ] && pisa_knob ordenDeLasFases "$ORDEN_FASES" 1
[ -n "${DIAGNOSTICO:-}" ] && { case "$DIAGNOSTICO" in 0) v=false ;; 1) v=true ;; *) v="$DIAGNOSTICO" ;; esac; pisa_knob diagnosticoDeLasImagenes "$v"; }
[ -n "${TPRESET:-}" ] && pisa_knob transicionPreset "$TPRESET"
[ -n "${TARCHIVO:-}" ] && pisa_knob transicionArchivo "$TARCHIVO"

[ -n "${ARENA_DS1:-}" ] && pisa_knob arenaDelDS1 "$ARENA_DS1" 1

echo "   soltarDespuesDeComponer=$(leer_knob soltarDespuesDeComponer) cacheEnVram=$(leer_knob cacheEnVram) tramoDeLaCache=$(leer_knob tramoDeLaCache) tramoEnPedazos=$(leer_knob tramoEnPedazos) pedazoLoDiceElMonton=$(leer_knob pedazoLoDiceElMonton) tramoAlArranque=$(leer_knob tramoAlArranque)"
echo "   cicloDeFases='$(leer_knob cicloDeFases)' ordenDeLasFases=$(leer_knob ordenDeLasFases) transicion=$(leer_knob transicionPreset)/$(leer_knob transicionArchivo) arenaDelDS1=$(leer_knob arenaDelDS1)"

echo "   $(ls "$SRCB"/*.go | wc -l) archivos .go: $(cd "$SRCB" && ls *.go | tr '\n' ' ')"

echo "== 4/5 compilando y enlazando contra los .gox del motor"
GODC_GOX="$GEN" bash "$HERE/build-dc.sh" "$SRCB" "$OUT" > "$OUT/build.log" 2>&1 || {
    echo "   FALLO el build (ver $OUT/build.log)"; tail -25 "$OUT/build.log"; exit 1; }

# DC-002.107 -- LA GUARDIA: el build NO SALE con la tabla de objectLookups adentro. Lo que se mide no es
# la bandera que se paso sino el BINARIO que se produjo: un simbolo de datos de mas de 1 MB en un ejecutable
# de la consola es, hoy, el volcado de la tabla (1.672.892 B, medidos con `sh-elf-nm -S` en DC-002.107).
if lleva_la_tabla_de_lookups "$OUT/prog.elf"; then
    echo "   FALLA: $OUT/prog.elf lleva la tabla de objectLookups:" >&2
    lleva_la_tabla_de_lookups "$OUT/prog.elf" | sed 's/^/      /' >&2
    echo "   la lista de fuentes del build es scripts/console/palancas-del-build.sh (DC-002.107)" >&2
    # el binario que salio NO se puede quedar con el nombre que viaja a la imagen: se renombra para que
    # nadie lo empaquete por error, y queda a la vista como evidencia del build fallado
    [ -f "$OUT/1ST_READ.BIN" ] && mv -f "$OUT/1ST_READ.BIN" "$OUT/1ST_READ.BIN.CON-LA-TABLA"
    exit 1
fi

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
