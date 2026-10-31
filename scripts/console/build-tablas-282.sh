#!/bin/bash
# DC-002.82 -- compilar y enlazar el POC de LAS TABLAS Y EL COMPOSITOR DEL MOTOR: carga las tablas de
# `d2resource.DataDictionaries()` en la consola y despues compone el pueblo con `d2mapengine`.
#
# POR QUE UN SCRIPT PROPIO. El POC importa paquetes del MOTOR (d2asset, d2records, d2mapengine) que no son
# stdlib y que no estan en el arbol de enlace de los POC anteriores. Se arma un arbol de ENLACE con enlaces
# simbolicos a los .gox del motor y una orden.txt = stdlib + el cierre de lo que este POC importa. El cierre
# NO se escribe a mano: sale de `go list -deps` (con -e, porque los paquetes importan "kos", que el host no
# resuelve).
#
# SE RECOMPILAN LOS PAQUETES QUE GOBIERNAN ESTA MEDICION (d2txt, d2records, d2asset, d2mapstamp,
# d2mapentity, d2mapengine, mas la cadena del arena de tiles de DC-002.78/.80) para que los numeros sean los
# del codigo de hoy y no los de un .gox viejo.
#
# LA IMAGEN LLEVA DOS MPQ: PATCH_D2.MPQ (las tablas: LvlTypes/LvlPrest/Objects/...) y D2DATA.MPQ (los .ds1
# y los .dt1). Sin el parche el compositor no tiene Records; sin los datos no tiene mapa.
#
# EL LIENZO DE REFERENCIA ES EL MISMO CODIGO. nivel.go NO vive en este directorio: se copia al directorio de
# build desde el POC del personaje (la referencia de DC-002.38) y se compara su sha256 contra la copia del
# verificador del host, igual que en DC-002.70, .75, .76, .77 y .81. Asi el crc32 del lienzo del pueblo
# tiene que dar el mismo numero que en las tarjetas anteriores.
#
# Uso:  scripts/console/build-tablas-282.sh [dir_salida]
#       BASE_GOX=...    arbol del motor compilado (por defecto /tmp/gox-med)
#       SALTAR_GEN=1    reusar el arbol de enlace ya armado
#       SOLO_PALANCAS=1 resolver las palancas, imprimirlas, escribir el knobs.go y SALIR sin compilar
#
# LAS PALANCAS NO SE ESCRIBEN ACA (DC-002.90). Se leen de `console/dc/poc/tablas-mapengine/knobs.go`, que
# es la unica fuente de verdad, y se escriben en el knobs.go del directorio de build. Cualquiera de
# TABLAS / MAPA / ENTIDADES / LIENZO / GCENTRE / GCINHIBE / ARENA / TILES se puede pisar por entorno; si la
# pisada se aparta de knobs.go el script lo avisa EN GRANDE con el tamano del binario de referencia, porque
# el binario de las palancas viejas (TABLAS=1 LIENZO=1 ARENA=1, 6.863.464 B) NO ENTRA en la consola (OOM).
# La corrida de referencia (DC-002.88/.89) es la que sale SIN pasar una sola variable: 1ST_READ 6.859.016 B.
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
SRC="$REPO/console/dc/poc/tablas-mapengine"
POC_REF="$REPO/console/dc/poc/personaje-mapa"
VERIF="$REPO/console/dc/verif-personaje-238"
BASE_GOX="${BASE_GOX:-/tmp/gox-med}"
OUT="${1:-/tmp/dcbuild/tablas282}"
GEN="$OUT/gox-enlace"
PREFIJO="github.com/OpenDiablo2/OpenDiablo2"
SRCB="$OUT/src"

# LOS PAQUETES DEL MOTOR QUE ESTE BUILD RECOMPILA EN EL ARBOL, en orden (cada uno se compila contra los
# .gox de los anteriores). La lista se usa DOS veces: la lee el chequeo del arbol (DC-002.91) para no
# quejarse de los que este mismo script rehace, y la recorre el paso 1/6. Una sola lista, porque dos se
# separan (la leccion de DC-002.90).
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
 d2core/d2render/pvr
 d2core/d2map/d2mapentity
 d2core/d2map/d2mapstamp
 d2core/d2map/d2mapengine"

[ -f "$BASE_GOX/orden.txt" ] || { echo "falta el arbol del motor en $BASE_GOX (corre build-motor-all.sh)" >&2; exit 1; }
mkdir -p "$OUT"

# ============================== LAS PALANCAS (DC-002.90) ==============================
# UNA SOLA FUENTE DE VERDAD: el `knobs.go` del POC. El script lee de ahi los ocho valores y los escribe en
# el knobs.go del directorio de build; el entorno solo puede PISARLOS y, cuando los pisa, lo dice EN GRANDE
# con el tamano del binario de referencia al lado.
#
# POR QUE (medido en DC-002.89, commit 258412ca). Hasta DC-002.89 este script traia las palancas escritas a
# mano con OTROS valores que knobs.go (TABLAS=1 LIENZO=1 ARENA=1 contra 3/0/0), asi que correrlo sin
# variables daba un 1ST_READ de 6.863.464 B que NO ENTRA en la consola: `fatal error: out of memory` en la
# cuarta tabla (arena 5.034.368 B, libre 576 B) antes de componer el pueblo. Con las palancas de las
# tarjetas el MISMO codigo mide 6.859.016 B y las 3 fases salen con crc -1596781041 / -205471136 y pico
# 9.525.408 B. Dos fuentes de verdad para el mismo numero cuestan una corrida entera: ahora hay una.
POC_KNOBS="$SRC/knobs.go"
REF_1ST_READ=6859016     # corrida de referencia DC-002.88/.89; lo verifica scripts/console/checar-palancas-290.sh
PISADAS=()

[ -f "$POC_KNOBS" ] || { echo "falta $POC_KNOBS: no hay de donde leer las palancas" >&2; exit 1; }

leer_knob() {  # $1 = const de knobs.go -> imprime su valor tal cual (numero o true/false), sirva `X = v` o `const X = v`
    local v
    v="$(awk -v n="$1" '
        {
            if ($1 == "const") { nom = $2; igual = $3; val = $4 } else { nom = $1; igual = $2; val = $3 }
            if (nom == n && igual == "=" && !visto) { print val; visto = 1; exit }
        }' "$POC_KNOBS")"
    [ -n "$v" ] || { echo "knobs.go ($POC_KNOBS) no declara $1" >&2; exit 1; }
    printf '%s' "$v"
}

palanca() {  # $1 = variable de entorno, $2 = const de knobs.go, $3 = num|bool
    local env="$1" knob="$2" tipo="$3" v pedido
    v="$(leer_knob "$knob")"
    case "$tipo" in
        num)  case "$v" in ''|*[!0-9]*) echo "knobs.go: $knob no es un numero ($v)" >&2; exit 1 ;; esac ;;
        bool) case "$v" in true) v=1 ;; false) v=0 ;;
                   *) echo "knobs.go: $knob tiene que ser true o false ($v)" >&2; exit 1 ;; esac ;;
    esac
    if [[ -v $env ]]; then
        pedido="${!env}"
        case "$tipo" in
            num)  case "$pedido" in ''|*[!0-9]*) echo "$env=$pedido no es un numero" >&2; exit 1 ;; esac ;;
            bool) case "$pedido" in 0|1) ;; *) echo "$env=$pedido tiene que ser 0 o 1" >&2; exit 1 ;; esac ;;
        esac
        # pisar con el MISMO valor no es una pisada: el binario sigue siendo el de la referencia
        [ "$pedido" = "$v" ] || PISADAS+=("$env=$pedido  (knobs.go dice $v)")
        export "$env=$pedido"
    else
        export "$env=$v"
    fi
}

palanca TABLAS    modoTablas        num
palanca MAPA      hacerMapa         bool
palanca ENTIDADES conEntidades      bool
palanca LIENZO    lienzoReferencia  bool
palanca GCENTRE   gcEntreTablas     bool
palanca GCINHIBE  gcInhibidoTablas  bool
palanca ARENA     pedirArenaPrimero bool
palanca TILES     tilesDelStamp     bool

bool_go() { [ "$1" = "1" ] && echo true || echo false; }

escribir_knobs() {  # $1 = destino. Las palancas YA resueltas, con la misma forma que knobs.go.
    {
        printf 'package main\n\n'
        printf '// Palancas generadas por build-tablas-282.sh a partir del knobs.go del POC (DC-002.90).\n'
        printf '// El porque de cada una esta alla: aca solo viajan los valores resueltos.\n'
        printf 'const modoTablas = %s\n' "$TABLAS"
        printf 'const hacerMapa = %s\n' "$(bool_go "$MAPA")"
        printf 'const conEntidades = %s\n' "$(bool_go "$ENTIDADES")"
        printf 'const lienzoReferencia = %s\n' "$(bool_go "$LIENZO")"
        printf 'const gcEntreTablas = %s\n' "$(bool_go "$GCENTRE")"
        printf 'const gcInhibidoTablas = %s\n' "$(bool_go "$GCINHIBE")"
        printf 'const pedirArenaPrimero = %s\n' "$(bool_go "$ARENA")"
        printf 'const tilesDelStamp = %s\n' "$(bool_go "$TILES")"
    } > "$1"
}

echo "== palancas (fuente unica: ${POC_KNOBS#"$REPO"/})"
printf '   TABLAS=%s MAPA=%s ENTIDADES=%s LIENZO=%s GCENTRE=%s GCINHIBE=%s ARENA=%s TILES=%s\n' \
    "$TABLAS" "$MAPA" "$ENTIDADES" "$LIENZO" "$GCENTRE" "$GCINHIBE" "$ARENA" "$TILES"
if [ "${#PISADAS[@]}" -eq 0 ]; then
    echo "   son las de la corrida de referencia (DC-002.88/.89): ese binario mide $REF_1ST_READ B"
else
    {
        echo ""
        echo "###########################################################################################"
        echo "#  OJO: EL ENTORNO ESTA PISANDO LAS PALANCAS DE knobs.go"
        for p in "${PISADAS[@]}"; do echo "#    $p"; done
        echo "#  Este binario NO es el de la corrida de referencia (DC-002.88/.89, 1ST_READ $REF_1ST_READ B)."
        echo "#  Y ojo con el default viejo del script (TABLAS=1 LIENZO=1 ARENA=1): ese binario mide"
        echo "#  6.863.464 B y NO ENTRA -- muere con 'fatal error: out of memory' en la cuarta tabla"
        echo "#  (arena 5.034.368 B, libre 576 B), medido en DC-002.89. Si lo que queres es medir la"
        echo "#  referencia, corré el script SIN variables: ahora las saca de knobs.go."
        echo "###########################################################################################"
        echo ""
    } >&2
fi

if [ "${SOLO_PALANCAS:-0}" = "1" ]; then
    escribir_knobs "$OUT/knobs.go"
    echo "   SOLO_PALANCAS=1: no se compila nada; el knobs.go resuelto queda en $OUT/knobs.go"
    exit 0
fi

# ============================== EL ARBOL DE .gox (DC-002.91) ==============================
# ANTES de compilar: si un .gox del arbol es mas viejo que su fuente, gccgo muere por un simbolo que SI
# existe en el repo y el mensaje apunta al CODIGO (medido en DC-002.90: `zlib.NuevoInflador` con
# /tmp/gox-med del 30/10 13:22 contra la stdlib del 30/10 19:24). El chequeo dice el paquete, las dos
# fechas y la receta, y aborta. Los paquetes de PAQUETES_MOTOR no abortan: los rehace este mismo build.
BASE_GOX="$BASE_GOX" RECOMPILA="$PAQUETES_MOTOR" "$HERE/revisar-arbol-gox.sh"

echo "== 0/6 el archivo compartido del mapa (el mismo del POC y del verificador)"
for f in nivel.go; do
    if ! cmp -s "$POC_REF/$f" "$VERIF/$f"; then
        echo "   FALLO: $POC_REF/$f y $VERIF/$f NO son el mismo archivo" >&2
        exit 1
    fi
    echo "   $f $(sha256sum "$POC_REF/$f" | cut -c1-16)"
done

echo "== 1/6 los paquetes del motor que se compilan aca (el arena de tiles, los records y el compositor)"
# El orden importa: cada paquete se compila contra los .gox que ya estan en el arbol. La lista es
# PAQUETES_MOTOR (arriba), la misma que lee el chequeo del arbol de .gox.
for p in $PAQUETES_MOTOR; do
    mapfile -t SRC_P < <(ls "$REPO/$p"/*.go | grep -v '_test\.go$' |
        grep -vE '_(darwin|windows|js|wasm|plan9|freebsd|netbsd|openbsd|linux|android|ios|solaris|aix)\.go$' |
        grep -vE '_(386|amd64|arm|arm64|ppc64|ppc64le|mips|mipsle|mips64|mips64le|riscv64|s390x|loong64|wasm)\.go$' |
        grep -vE '/(marca_carga_host)\.go$')
    mkdir -p "$(dirname "$BASE_GOX/$PREFIJO/$p.gox")"
    sh-elf-gccgo $GODC_GOFLAGS -I "$BASE_GOX" \
        -fgo-pkgpath="$PREFIJO/$p" \
        -c "${SRC_P[@]}" \
        -o "$BASE_GOX/$PREFIJO/$p.gox"
    printf '   %-28s %9s B\n' "$(basename "$p").gox" "$(stat -c%s "$BASE_GOX/$PREFIJO/$p.gox")"
done

# el cierre del POC, con `go list -deps` sobre los paquetes del MOTOR que importa
PAQUETES="$(cd "$REPO" && go list -deps -e -tags dreamcast \
    ./d2core/d2map/d2mapengine ./d2core/d2asset \
    ./d2common/d2fileformats/d2ds1 ./d2common/d2fileformats/d2dt1 ./d2common/d2fileformats/d2mpq \
    ./d2common/d2resource ./d2common/d2interface ./d2common/d2enum 2>/dev/null |
    grep "$PREFIJO" | sed "s|.*$PREFIJO/||" | sort | tr '\n' ' ')"
[ -n "${PAQUETES// /}" ] || { echo "la lista de paquetes quedo vacia (go list -deps fallo?)" >&2; exit 1; }

echo "== 2/6 el arbol de enlace ($GEN)"
if [ "${SALTAR_GEN:-0}" != "1" ]; then
    rm -rf "$GEN"
    mkdir -p "$GEN"
    # enlaces simbolicos, no copias: los .gox del motor son decenas de MB y no se tocan desde aca
    cp -rs "$BASE_GOX/." "$GEN"/
fi
rm -f "$GEN/orden.txt"
cp "$BASE_GOX/orden.txt" "$GEN/orden.txt"
for p in $PAQUETES; do
    f="$GEN/$PREFIJO/$p.gox"
    [ -f "$f" ] || { echo "   FALTA $p.gox en el arbol del motor: no se puede enlazar lo que no existe" >&2; exit 1; }
    grep -qxF "$f" "$GEN/orden.txt" || echo "$f" >> "$GEN/orden.txt"
done
echo "   paquetes del motor en el enlace: $(printf '%s\n' $PAQUETES | wc -l)"
echo "   objetos en orden.txt: $(wc -l < "$GEN/orden.txt")"

echo "== 3/6 el directorio de fuentes del POC (con el mapa compartido)"
rm -rf "$SRCB"
mkdir -p "$SRCB"
cp "$SRC"/*.go "$SRCB"/
cp -r "$SRC/csrc" "$SRCB"/
cp "$POC_REF/nivel.go" "$SRCB"/

# las palancas (el porque de cada una, en knobs.go del POC; aca solo se escriben las resueltas)
escribir_knobs "$SRCB/knobs.go"
echo "   palancas escritas en $SRCB/knobs.go (modoTablas=$TABLAS lienzoReferencia=$(bool_go "$LIENZO") pedirArenaPrimero=$(bool_go "$ARENA"))"
echo "   $(ls "$SRCB"/*.go | wc -l) archivos .go: $(cd "$SRCB" && ls *.go | tr '\n' ' ')"

echo "== 4/6 compilando y enlazando contra los .gox del motor"
GODC_GOX="$GEN" bash "$HERE/build-dc.sh" "$SRCB" "$OUT" > "$OUT/build.log" 2>&1 || {
    echo "   FALLO el build (ver $OUT/build.log)"; tail -25 "$OUT/build.log"; exit 1; }

echo "== 5/6 la imagen con PATCH_D2.MPQ y D2DATA.MPQ en /JUEGO"
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

echo "== 6/6 lo que salio"
for f in prog.elf 1ST_READ.BIN disc.iso; do
    [ -f "$OUT/$f" ] && printf '   %-14s %10s B\n' "$f" "$(stat -c%s "$OUT/$f")"
done
printf '   codigo (.text)  %10s B\n' "$(sh-elf-size -A "$OUT/prog.elf" | awk '$1==".text"{print $2}')"
printf '   rodata          %10s B\n' "$(sh-elf-size -A "$OUT/prog.elf" | awk '$1==".rodata"{print $2}')"
printf '   data+bss        %10s B\n' "$(sh-elf-size -A "$OUT/prog.elf" | awk '$1==".data"||$1==".bss"{s+=$2} END{print s}')"
[ -f "$OUT/disc.gdi" ] && echo "   imagen:         $OUT/disc.gdi"

# ============================== EL NUMERO DE LA REFERENCIA (DC-002.90) ==============================
# El binario de la corrida de referencia tiene un tamano medido y el monton de la consola se come 1 byte de
# imagen por cada byte de codigo (ley de DC-002.82), asi que crecer por aca es acercarse al OOM. Si las
# palancas son las de knobs.go y el numero no da, el codigo (o el arbol de gox) se movio: se dice con
# numeros y no se deja pasar como si nada.
TAM_BIN="$(stat -c%s "$OUT/1ST_READ.BIN")"
if [ "${#PISADAS[@]}" -gt 0 ]; then
    printf '   OJO 1ST_READ.BIN %s B con palancas pisadas: NO comparable con la referencia (%s B)\n' \
        "$TAM_BIN" "$REF_1ST_READ"
elif [ "$TAM_BIN" != "$REF_1ST_READ" ]; then
    {
        echo ""
        echo "###########################################################################################"
        echo "#  OJO: LAS PALANCAS SON LAS DE LA REFERENCIA PERO EL BINARIO NO MIDE LO MISMO"
        echo "#    referencia DC-002.88/.89: $REF_1ST_READ B"
        echo "#    medido ahora:              $TAM_BIN B  ($((TAM_BIN - REF_1ST_READ)) B)"
        echo "#  El codigo o el arbol de .gox se movio respecto de esa corrida. El techo de monton de la"
        echo "#  consola paga esa diferencia 1:1 (ley de DC-002.82): si crecio, puede morir con 'fatal"
        echo "#  error: out of memory'. Actualiza el numero de referencia A PROPOSITO (build-tablas-282.sh"
        echo "#  y checar-palancas-290.sh) cuando el cambio este medido en la consola."
        echo "###########################################################################################"
        echo ""
    } >&2
else
    echo "   1ST_READ.BIN = $REF_1ST_READ B: es el binario de la corrida de referencia (DC-002.88/.89)"
fi
