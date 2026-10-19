#!/bin/bash
# CORPUS DE DC-002.37: EL ACTO 1 ARMADO Y PINTADO EN LA DREAMCAST, DESDE LOS .ds1 DEL MPQ DEL DISCO.
#
# QUE MIDE (dos estados, misma cadena de codigo):
#   FIX  el arbol de HEAD: el POC `acto1-disco` lee el .ds1 del nivel de `d2data.mpq` POR EL DRIVER DE
#        ISO9660 de la consola, lo parsea con el parseador del MOTOR, compone la ventana isometrica con el
#        compositor del motor (`nivel.go`, el MISMO archivo que usa el verificador del host), sube el lienzo
#        como textura paletizada de 8 bits y lo pinta por PVR;
#   MUT  el MUTANTE del criterio 3: el MISMO binario compilado con `DS1_INEXISTENTE=1`, o sea con la ruta
#        del nivel apuntando a un .ds1 que no existe. Si la pantalla cambia igual, el criterio no vale.
#
# CONTROLES (con numeros, uno por linea):
#   0a/0b/0c/0d  el HOST: el .ds1 del MPQ (bytes, crc), la composicion (pedidos/compuestos/sin tile/fuera,
#                pixeles no vacios y crc del lienzo), el `nivel.go` IDENTICO en las dos puntas (sha256) y la
#                referencia PNG con la que se compara la captura;
#   1a/1b/1c     las dos imagenes: el ELF del FIX, el D2DATA.MPQ DENTRO del ISO (sha256 del PC) y el mutante
#                como binario DISTINTO (sha256);
#   2a..2h       la corrida BUENA: los numeros de la consola tienen que ser LOS DEL HOST, uno por uno;
#   2i-0         la geometria del area cliente la midio el GESTOR DE VENTANAS (linea `GEO` de la corrida):
#                es la unica fuente independiente del asset;
#   2j           esa caja se valida con el quad de control magenta que el POC dibuja en coordenadas logicas
#                conocidas (520,380 110x90): tiene que caer donde la caja dice (residuo <= 2 px);
#   2i/2k        la captura: pixeles pintados del bloque, color dominante y medio, y la comparacion a 64x64
#                contra la referencia en las CUATRO orientaciones (la que gana tiene que ser la identidad);
#   3a/3b        el MUTANTE: el log dice DS1-LEER FALLO + PANTALLA-VACIA y la captura, con la MISMA geometria
#                que la corrida buena, no tiene ni un pixel pintado.
#
# Uso:  bash scripts/console/checar-acto1-237.sh [segundos_de_la_corrida_buena]
#       SAL=... GODC_GOX=... bash ...        (otra carpeta de trabajo / otra capa de .gox)
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
SAL="${SAL:-$HOME/.hermes/cache/scratch/dc237-corpus}"
GEN="${GODC_GOX:-/tmp/gox-237b}"
# 75 s y no 45: MEDIDO, el POC tarda ~45 s en llegar al primer cuadro (MPQ-ABRIR ~10 s + los 8 DT1 del
# town ~35 s) y la primera marca CUADROS 30 sale en ms=45093. Con 45 s la ultima captura es la de 40 s y
# todavia no hay mapa: el corpus mediria una pantalla negra y diria que el POC no pinta.
SEGS="${1:-75}"
MPQ="${MPQ:-$HOME/.wine-d2/drive_c/Games/Diablo II}"

OK=0
FALLOS=0
nota() { echo "   $*"; }
verde() { OK=$((OK + 1)); echo "   OK: $*"; }
rojo() { FALLOS=$((FALLOS + 1)); echo "   FALLO: $*"; }
paso() { printf '\n== %s\n' "$*"; }
# el valor de una clave `CLAVE= valor` de un log (la ultima que aparece)
val() { grep -a "$1=" "$2" 2>/dev/null | tail -1 | sed -n "s/.*$1= *\([^ ]*\).*/\1/p"; }
# normaliza una linea para COMPARARLA: saca el \r de la consola serie (que escribe \r\n), junta los
# espacios que mete el println de Go entre operandos, y unifica los separadores del host (`a,b` contra
# `a b`). El VALOR comparado es el mismo; antes estos controles fallaban solo por el formato.
limpiar() { tr -d '\r' | sed -e 's|\\|/|g' -e 's/,/ /g' -e 's| */ *|/|g' -e 's/  */ /g' -e 's/= /=/g' -e 's/^ *//' -e 's/ *$//'; }

mkdir -p "$SAL/host" "$SAL/fix" "$SAL/mut"

# ================================================================================================
# 0. EL HOST -- la misma composicion, el mismo archivo de composicion y la referencia
# ================================================================================================
paso "0. el host: el .ds1 del MPQ, la composicion y la referencia"
if (cd "$REPO" && go run ./console/dc/verif-acto1-237 -png "$SAL/host/referencia.png") \
        > "$SAL/host/verif.txt" 2>&1; then
    # OJO: el `val 'bytes'` tomaba la ULTIMA linea con "bytes=" del verificador, que es la de la PALETA
    # (bytes=768), no la del .ds1: de ahi salia "el .ds1 del host no es el esperado (bytes=768 ...)".
    H_DS1_BYTES="$(grep -a 'VERIF237: DS1 data' "$SAL/host/verif.txt" |
        sed -n 's/.*bytes= *\([0-9]*\).*/\1/p' | tail -1)"
    H_DS1_CRC="$(grep -a 'VERIF237: DS1 data' "$SAL/host/verif.txt" | sed -n 's/.*crc32=\([0-9]*\).*/\1/p')"
    H_DS1_LINEA="$(grep -a 'VERIF237: DS1 version' "$SAL/host/verif.txt" | sed 's/^VERIF237: //')"
    H_VENTANA="$(grep -a 'VERIF237: VENTANA' "$SAL/host/verif.txt" | sed 's/^VERIF237: VENTANA //')"
    H_PEDIDOS="$(grep -a 'VERIF237: PEDIDOS' "$SAL/host/verif.txt" | sed 's/^VERIF237: PEDIDOS //')"
    H_COMPUESTO="$(grep -a 'VERIF237: COMPUESTO' "$SAL/host/verif.txt" | sed 's/^VERIF237: COMPUESTO //')"
    H_LIENZO="$(grep -a 'VERIF237: LIENZO pixelesNoVacios' "$SAL/host/verif.txt" | sed 's/^VERIF237: LIENZO //')"
    H_PIXELES="$(grep -a 'VERIF237: LIENZO pixelesPiso' "$SAL/host/verif.txt" | sed 's/^VERIF237: LIENZO //')"
    H_LIENZO_CRC="$(echo "$H_LIENZO" | sed -n 's/.*crc32= *\([0-9]*\).*/\1/p')"
    H_LIENZO_NV="$(echo "$H_LIENZO" | sed -n 's/.*pixelesNoVacios= *\([0-9]*\).*/\1/p')"
    H_PALETA_CRC="$(grep -a 'VERIF237: PALETA' "$SAL/host/verif.txt" | sed -n 's/.*crc32= *\([0-9]*\).*/\1/p')"

    if [ "$H_DS1_BYTES" = "57652" ] && [ "$H_DS1_CRC" = "712973328" ]; then
        verde "control 0a: el .ds1 del nivel en el MPQ: $H_DS1_BYTES B, crc32=$H_DS1_CRC, $H_DS1_LINEA"
    else
        rojo "control 0a: el .ds1 del host no es el esperado (bytes=$H_DS1_BYTES crc32=$H_DS1_CRC, ver $SAL/host/verif.txt)"
    fi

    case "$H_COMPUESTO" in
    *"pisos=1982/1982"*"paredes=373/373"*"sintilePiso=0"*"sintilePared=0"*)
        verde "control 0b: la composicion del host: $H_PEDIDOS -> $H_COMPUESTO ($H_PIXELES)" ;;
    *)
        rojo "control 0b: la composicion del host no es la documentada ($H_PEDIDOS -> $H_COMPUESTO)" ;;
    esac

    if [ -n "$H_LIENZO_CRC" ] && [ "$H_LIENZO_NV" = "258653" ]; then
        verde "control 0c: el lienzo del host: $H_LIENZO y paleta crc32=$H_PALETA_CRC"
    else
        rojo "control 0c: el lienzo del host cambio (pixelesNoVacios=$H_LIENZO_NV crc32=$H_LIENZO_CRC)"
    fi
else
    rojo "control 0a/0b/0c: el verificador del host fallo (ver $SAL/host/verif.txt)"
    tail -5 "$SAL/host/verif.txt"
fi

SHA_POC="$(sha256sum "$REPO/console/dc/poc/acto1-disco/nivel.go" | cut -d' ' -f1)"
SHA_VERIF="$(sha256sum "$REPO/console/dc/verif-acto1-237/nivel.go" | cut -d' ' -f1)"
if [ -n "$SHA_POC" ] && [ "$SHA_POC" = "$SHA_VERIF" ]; then
    verde "control 0d: el compositor es EL MISMO archivo en las dos puntas (nivel.go sha256 ${SHA_POC:0:16}...)"
else
    rojo "control 0d: nivel.go NO es el mismo en el POC y en el verificador ($SHA_POC / $SHA_VERIF)"
fi

if [ -s "$SAL/host/referencia.png" ]; then
    verde "control 0e: la referencia PNG del host: $(stat -c%s "$SAL/host/referencia.png") B"
else
    rojo "control 0e: no hay referencia PNG del host"
fi

# ================================================================================================
# 1. LAS DOS IMAGENES (FIX y MUTANTE)
# ================================================================================================
paso "1. las dos imagenes: FIX y MUTANTE"
if SALTAR_STDLIB=1 SALTAR_MOTOR=1 GODC_GOX="$GEN" \
        bash "$HERE/build-acto1-237.sh" "$SAL/fix" > "$SAL/fix/build.log" 2>&1; then
    GDI_FIX="$SAL/fix/disc.gdi"
    if [ -s "$SAL/fix/prog.elf" ] && [ -s "$SAL/fix/disc.iso" ]; then
        verde "control 1a: la imagen del FIX: prog.elf $(stat -c%s "$SAL/fix/prog.elf") B, disc.iso $(stat -c%s "$SAL/fix/disc.iso") B"
    else
        rojo "control 1a: falta el ELF o el ISO del FIX (ver $SAL/fix/build.log)"
    fi

    MPQ_PC="$(ls -1 "$MPQ" | grep -ix 'd2data.mpq' | head -1)"
    SHA_PC="$(sha256sum "$MPQ/$MPQ_PC" | cut -d' ' -f1)"
    SHA_ISO="$("$DC_PYTHON" "$HERE/leer-iso.py" "$SAL/fix/disc.iso" "/JUEGO/D2DATA.MPQ;1" 2>/dev/null |
        grep -o 'sha256=[0-9a-f]*' | cut -d= -f2)"
    if [ -n "$SHA_ISO" ] && [ "$SHA_ISO" = "$SHA_PC" ]; then
        verde "control 1b: el d2data.mpq DENTRO del ISO es el del PC (sha256 ${SHA_PC:0:16}...)"
    else
        rojo "control 1b: el MPQ del ISO no coincide con el del PC (iso=${SHA_ISO:0:16} pc=${SHA_PC:0:16})"
    fi
else
    rojo "control 1a: el build del FIX fallo (ver $SAL/fix/build.log)"
    tail -8 "$SAL/fix/build.log"
fi

if DS1_INEXISTENTE=1 SALTAR_STDLIB=1 SALTAR_MOTOR=1 GODC_GOX="$GEN" \
        bash "$HERE/build-acto1-237.sh" "$SAL/mut" > "$SAL/mut/build.log" 2>&1; then
    GDI_MUT="$SAL/mut/disc.gdi"
    SHA_ELF_FIX="$(sha256sum "$SAL/fix/prog.elf" 2>/dev/null | cut -d' ' -f1)"
    SHA_ELF_MUT="$(sha256sum "$SAL/mut/prog.elf" 2>/dev/null | cut -d' ' -f1)"
    if [ -n "$SHA_ELF_MUT" ] && [ "$SHA_ELF_MUT" != "$SHA_ELF_FIX" ]; then
        verde "control 1c: el MUTANTE es otro binario (sha256 ${SHA_ELF_MUT:0:16}... contra ${SHA_ELF_FIX:0:16}...) y el sed apunto a un .ds1 inexistente"
    else
        rojo "control 1c: el mutante salio IDENTICO al FIX ($SHA_ELF_MUT): el mutante no muta nada"
    fi
    grep -a 'MUTANTE' "$SAL/mut/build.log" | head -2 | sed 's/^/   /'
else
    rojo "control 1c: el build del MUTANTE fallo (ver $SAL/mut/build.log)"
    tail -8 "$SAL/mut/build.log"
fi

# ================================================================================================
# 2. LA CORRIDA BUENA EN FLYCAST
# ================================================================================================
paso "2. la corrida BUENA en Flycast ($SEGS s)"
rm -rf "$SAL/fix/run"
bash "$HERE/run-flycast-237.sh" "${GDI_FIX:-$SAL/fix/disc.gdi}" "$SEGS" "$SAL/fix/run" 1400 120 \
    > "$SAL/fix/run-corrida.txt" 2>&1
LOG_FIX="$SAL/fix/run/flycast.log"
cp -f "$LOG_FIX" "$SAL/fix/flycast.log" 2>/dev/null
nota "consola serie: $(grep -ac 'HITO237' "$LOG_FIX" 2>/dev/null || echo 0) lineas HITO237 en $LOG_FIX"

C_DS1="$(grep -a 'HITO237: DS1-LEER OK' "$LOG_FIX" | tail -1)"
C_DS1_BYTES="$(echo "$C_DS1" | sed -n 's/.*bytes= *\([0-9]*\).*/\1/p')"
C_DS1_CRC="$(echo "$C_DS1" | sed -n 's/.*crc32= *\([0-9]*\).*/\1/p')"
if [ "$C_DS1_BYTES" = "$H_DS1_BYTES" ] && [ "$C_DS1_CRC" = "$H_DS1_CRC" ]; then
    verde "control 2a: la consola leyo el .ds1 del MPQ DEL DISCO: $C_DS1_BYTES B crc32=$C_DS1_CRC (igual al host)"
else
    rojo "control 2a: el .ds1 de la consola no es el del host (consola bytes=$C_DS1_BYTES crc32=$C_DS1_CRC / host $H_DS1_BYTES $H_DS1_CRC)"
fi

C_DS1_LINEA="$(grep -a 'HITO237: DS1 version' "$LOG_FIX" | sed 's/^.*HITO237: //')"
if [ "$(echo "$C_DS1_LINEA" | limpiar)" = "$(echo "$H_DS1_LINEA" | limpiar)" ]; then
    verde "control 2b: el parseo del MOTOR en la consola: $C_DS1_LINEA (igual al host)"
else
    rojo "control 2b: el parseo del .ds1 en la consola difiere del host ('$C_DS1_LINEA' contra '$H_DS1_LINEA')"
fi

C_VENTANA="$(grep -a 'HITO237: VENTANA' "$LOG_FIX" | sed 's/^.*HITO237: VENTANA //')"
C_PEDIDOS="$(grep -a 'HITO237: PEDIDOS' "$LOG_FIX" | sed 's/^.*HITO237: PEDIDOS //')"
if [ "$(echo "$C_VENTANA" | limpiar)" = "$(echo "$H_VENTANA" | limpiar)" ] &&
    [ "$(echo "$C_PEDIDOS" | limpiar)" = "$(echo "$H_PEDIDOS" | limpiar)" ]; then
    verde "control 2c: la ventana isometrica: $C_VENTANA ($C_PEDIDOS), igual al host"
else
    rojo "control 2c: la ventana/pedidos de la consola difieren ('$C_VENTANA' '$C_PEDIDOS' contra '$H_VENTANA' '$H_PEDIDOS')"
fi

C_DT1="$(grep -ac 'HITO237: DT1 .* crc32=' "$LOG_FIX")"
D_DT1="$(grep -a 'VERIF237: DT1' "$SAL/host/verif.txt" | sed 's/^VERIF237: //' | sort)"
C_DT1_LISTA="$(grep -a 'HITO237: DT1' "$LOG_FIX" | sed 's/^.*HITO237: //' |
    sed 's/ ms=.*//' | sed 's|\\\\|/|g' | sed 's|\\|/|g' | sort)"
D_DT1_LISTA="$(echo "$D_DT1" | sed 's/ ms=.*//' | sed 's|\\\\|/|g' | sed 's|\\|/|g' | sort)"
if [ -n "$C_DT1_LISTA" ] &&
    [ "$(echo "$C_DT1_LISTA" | limpiar)" = "$(echo "$D_DT1_LISTA" | limpiar)" ]; then
    verde "control 2d: los $C_DT1 DT1 que el PROPIO .ds1 lista salen iguales a los del host (bytes y crc32, uno por uno)"
else
    rojo "control 2d: los DT1 de la consola no son los del host (ver $LOG_FIX)"
    diff <(echo "$D_DT1_LISTA") <(echo "$C_DT1_LISTA") | head -6 | sed 's/^/   /'
fi

C_COMPUESTO="$(grep -a 'HITO237: COMPUESTO' "$LOG_FIX" | sed 's/^.*HITO237: COMPUESTO //')"
if [ "$(echo "$C_COMPUESTO" | limpiar)" = "$(echo "$H_COMPUESTO" | limpiar)" ]; then
    verde "control 2e: la composicion en la consola: $C_COMPUESTO (igual al host)"
else
    rojo "control 2e: la composicion de la consola difiere ('$C_COMPUESTO' contra '$H_COMPUESTO')"
fi

C_LIENZO="$(grep -a 'HITO237: LIENZO pixelesNoVacios' "$LOG_FIX" | sed 's/^.*HITO237: LIENZO //')"
C_LIENZO_CRC="$(echo "$C_LIENZO" | sed -n 's/.*crc32= *\([0-9]*\).*/\1/p')"
C_LIENZO_NV="$(echo "$C_LIENZO" | sed -n 's/.*pixelesNoVacios= *\([0-9]*\).*/\1/p')"
if [ "$C_LIENZO_CRC" = "$H_LIENZO_CRC" ] && [ "$C_LIENZO_NV" = "$H_LIENZO_NV" ]; then
    verde "control 2f: el LIENZO de indices es el del host: $C_LIENZO (crc32 igaul, pixel a pixel)"
else
    rojo "control 2f: el lienzo de la consola difiere (pixelesNoVacios=$C_LIENZO_NV crc32=$C_LIENZO_CRC / host $H_LIENZO_NV $H_LIENZO_CRC)"
fi

C_PALETA_CRC="$(grep -a 'HITO237: PALETA' "$LOG_FIX" | sed -n 's/.*crc32= *\([0-9]*\).*/\1/p')"
if [ -n "$C_PALETA_CRC" ] && [ "$C_PALETA_CRC" = "$H_PALETA_CRC" ]; then
    verde "control 2g: la paleta del acto (pal.dat del MPQ) da el mismo ARGB1555 que en el host (crc32=$C_PALETA_CRC)"
else
    rojo "control 2g: la paleta ARGB1555 de la consola no es la del host ($C_PALETA_CRC contra $H_PALETA_CRC)"
fi

C_TEXTURA="$(grep -a 'HITO237: TEXTURA OK' "$LOG_FIX" | sed 's/^.*HITO237: //')"
C_CUADROS_N="$(grep -ac 'HITO237: CUADROS' "$LOG_FIX")"
C_CUADROS_1="$(grep -a 'HITO237: CUADROS' "$LOG_FIX" | head -1 | sed 's/^.*ms= *//; s/[^0-9].*//')"
C_CUADROS_U="$(grep -a 'HITO237: CUADROS' "$LOG_FIX" | tail -1 | sed 's/^.*ms= *//; s/[^0-9].*//')"
# los CUADROS de la primera y de la ultima marca: el ritmo es (cuadros_fin - cuadros_inicio) / segundos.
# (Antes se dividia 30/d, que es el paso entre marcas, y daba 6,6 cuadros/s en vez de 59,8: medido.)
C_CUADROS_1N="$(grep -a 'HITO237: CUADROS' "$LOG_FIX" | head -1 | sed -n 's/.*CUADROS *\([0-9]*\).*/\1/p')"
C_CUADROS_UN="$(grep -a 'HITO237: CUADROS' "$LOG_FIX" | tail -1 | sed -n 's/.*CUADROS *\([0-9]*\).*/\1/p')"
if [ -n "$C_TEXTURA" ] && [ "$C_CUADROS_N" -ge 2 ] && ! grep -aq 'HITO237: TEXTURA FALLO' "$LOG_FIX"; then
    FPS="$(awk -v a="$C_CUADROS_1" -v b="$C_CUADROS_U" -v na="$C_CUADROS_1N" -v nb="$C_CUADROS_UN" \
        'BEGIN{d=(b-a)/1000.0; if(d<=0 || nb<=na){print "?"} else {printf "%.1f", (nb-na)/d}}')"
    verde "control 2h: el lienzo subio a VRAM y se pinto: $C_TEXTURA / $C_CUADROS_N marcas de cuadros ($C_CUADROS_1N..$C_CUADROS_UN cuadros, ~$FPS cuadros/s)"
else
    rojo "control 2h: la textura o los cuadros no salieron ('$C_TEXTURA', marcas de CUADROS=$C_CUADROS_N)"
fi

CAP_FIX="$(ls -1 "$SAL/fix/run"/f*.png 2>/dev/null | sort -V | tail -1)"
# La geometria del AREA CLIENTE la mide el gestor de ventanas (linea `GEO` de la corrida): es la unica
# fuente independiente del asset. El instrumento la usa TAL CUAL y ademas la contrasta con el quad de
# control magenta (control 2j). Antes se deducia de la banda negra y el quad magenta PARTE la banda:
# medido, leia alto=361 donde el cliente mide 453, y con eso el mapa "no coincidia" (11,6 % a +-8).
GEO_CLIENTE="$(grep -a '^GEO ' "$SAL/fix/run-corrida.txt" 2>/dev/null | tail -1 |
    sed -n 's/.*cliente=\([0-9-]*,[0-9-]*,[0-9]*,[0-9]*\).*/\1/p')"
VENTANA_ARGS=""
if [ -n "$GEO_CLIENTE" ]; then
    VENTANA_ARGS="--ventana $GEO_CLIENTE"
    verde "control 2i-0: la geometria del area cliente la midio el gestor de ventanas: $GEO_CLIENTE"
else
    rojo "control 2i-0: la corrida no reporto la geometria del area cliente (sin linea GEO): se mediria por banda negra"
fi

if [ -n "$CAP_FIX" ]; then
    # shellcheck disable=SC2086
    python3 "$HERE/medir-captura-237.py" "$CAP_FIX" "$SAL/host/referencia.png" $VENTANA_ARGS \
        > "$SAL/fix/captura.txt" 2>&1
    cat "$SAL/fix/captura.txt" | sed 's/^/   /'
    NO_FONDO_PCT="$(grep -a 'MEDIDOR237 bloque' "$SAL/fix/captura.txt" | sed -n 's/.*(\([0-9.]*\)%).*/\1/p')"
    REF_PCT="$(grep -a 'MEDIDOR237 refNoFondo=' "$SAL/fix/captura.txt" | sed 's/.*refNoFondo=//')"
    IDENT64="$(grep -a 'MEDIDOR237 identidad64 mas8=' "$SAL/fix/captura.txt" | sed -n 's/.*mas8=\([0-9.]*\).*/\1/p')"
    OTRAS64="$(grep -a 'MEDIDOR237 orient64 ' "$SAL/fix/captura.txt" | sed -n 's/.*mas8=\([0-9.]*\).*/\1/p' | sort -rn | head -1)"
    GRUESO_MAS8="$(grep -a 'MEDIDOR237 grueso ' "$SAL/fix/captura.txt" | sed -n 's/.*mas8=\([0-9.]*\).*/\1/p')"
    QUAD_ESTADO="$(grep -a 'MEDIDOR237 quad=' "$SAL/fix/captura.txt" | sed -n 's/.*quad=\([a-z]*\).*/\1/p')"
    QUAD_DX="$(grep -a 'MEDIDOR237 quad=' "$SAL/fix/captura.txt" | sed -n 's/.*dx=\([-+0-9.]*\).*/\1/p')"
    QUAD_DY="$(grep -a 'MEDIDOR237 quad=' "$SAL/fix/captura.txt" | sed -n 's/.*dy=\([-+0-9.]*\).*/\1/p')"
    DOMINANTE="$(grep -a 'MEDIDOR237 dominante=' "$SAL/fix/captura.txt" | sed 's/.*dominante=//')"
    MEDIA="$(grep -a 'color medio del bloque' "$SAL/fix/captura.txt" | sed 's/^ *//')"
    VENTANA_MEDIDA="$(grep -a 'MEDIDOR237 ventana=' "$SAL/fix/captura.txt" | head -1 | sed 's/.*ventana=//; s/ fuente=.*//')"

    # control 2j: la caja con la que se mide esta VALIDADA por el quad de control (coordenadas logicas
    # conocidas, dibujado sin textura por el mismo camino de dibujo).
    if [ "$QUAD_ESTADO" = "ok" ]; then
        verde "control 2j: el quad de control cae donde la caja dice (dx=$QUAD_DX dy=$QUAD_DY px): la geometria medida es la del asset"
    else
        rojo "control 2j: el quad de control NO cae donde la caja dice (estado='$QUAD_ESTADO'): la geometria no esta validada"
    fi

    # control 2i: el mapa. El veredicto es a 64x64 y SIN buscar alineacion (la caja es la medida, no se
    # ajusta a la medida); las otras tres orientaciones tienen que quedar MUY abajo (medido: 31-36 %).
    if [ -n "$IDENT64" ] && awk -v i="$IDENT64" 'BEGIN{exit !(i >= 95.0)}' &&
        [ -n "$OTRAS64" ] && awk -v i="$IDENT64" -v o="$OTRAS64" 'BEGIN{exit !(i >= 2.0*o)}' &&
        awk -v c="$NO_FONDO_PCT" -v r="$REF_PCT" 'BEGIN{d=c-r; if(d<0)d=-d; exit !(d <= 3.0)}'; then
        verde "control 2i: la captura ES el mapa del host: identidad $IDENT64 % a 64x64 (la mejor de las otras tres: $OTRAS64 %), a 128x128 $GRUESO_MAS8 %; pixeles pintados $NO_FONDO_PCT % contra $REF_PCT % de la referencia"
        verde "control 2k: lo pintado en la consola: $NO_FONDO_PCT % de pixeles que no son fondo, color dominante (rango 16) $DOMINANTE, $MEDIA"
    else
        rojo "control 2i: la captura no coincide con la referencia (identidad $IDENT64 % a 64x64, la mejor de las otras tres $OTRAS64 %, a 128x128 $GRUESO_MAS8 %; pintados $NO_FONDO_PCT % contra $REF_PCT %)"
    fi
else
    rojo "control 2i: la corrida buena no dejo capturas en $SAL/fix/run"
    VENTANA_MEDIDA=""
fi

# ================================================================================================
# 3. EL MUTANTE (criterio 3)
# ================================================================================================
paso "3. el MUTANTE (.ds1 que no existe)"
rm -rf "$SAL/mut/run"
bash "$HERE/run-flycast-237.sh" "${GDI_MUT:-$SAL/mut/disc.gdi}" 20 "$SAL/mut/run" 1400 120 \
    > "$SAL/mut/run-corrida.txt" 2>&1
LOG_MUT="$SAL/mut/run/flycast.log"
cp -f "$LOG_MUT" "$SAL/mut/flycast.log" 2>/dev/null

if grep -aq 'HITO237: DS1-LEER FALLO' "$LOG_MUT" && grep -aq 'HITO237: PANTALLA-VACIA' "$LOG_MUT" &&
    ! grep -aq 'HITO237: DS1-LEER OK' "$LOG_MUT"; then
    verde "control 3a: el mutante no encuentra el nivel y lo dice: $(grep -a 'HITO237: DS1-LEER FALLO' "$LOG_MUT" | sed 's/^.*HITO237: //' | head -1) + PANTALLA-VACIA"
else
    rojo "control 3a: el mutante no dijo DS1-LEER FALLO + PANTALLA-VACIA (o igual leyo el .ds1)"
fi

CAP_MUT="$(ls -1 "$SAL/mut/run"/f*.png 2>/dev/null | sort -V | tail -1)"
GEO_MUT="$(grep -a '^GEO ' "$SAL/mut/run-corrida.txt" 2>/dev/null | tail -1 |
    sed -n 's/.*cliente=\([0-9-]*,[0-9-]*,[0-9]*,[0-9]*\).*/\1/p')"
VENTANA_MUT="${GEO_MUT:-$VENTANA_MEDIDA}"
if [ -n "$CAP_MUT" ] && [ -n "$VENTANA_MUT" ]; then
    python3 "$HERE/medir-captura-237.py" "$CAP_MUT" --ventana "$VENTANA_MUT" \
        > "$SAL/mut/captura.txt" 2>&1
    cat "$SAL/mut/captura.txt" | sed 's/^/   /'
    MUT_PCT="$(grep -a 'MEDIDOR237 bloque' "$SAL/mut/captura.txt" | sed -n 's/.*(\([0-9.]*\)%).*/\1/p')"
    if [ -n "$MUT_PCT" ] && awk -v p="$MUT_PCT" 'BEGIN{exit !(p <= 1.0)}'; then
        verde "control 3b: la pantalla del mutante NO cambio: $MUT_PCT% de pixeles pintados en el MISMO bloque (la buena tenia $NO_FONDO_PCT%)"
    else
        rojo "control 3b: la pantalla del mutante SI tiene pixeles pintados ($MUT_PCT%): el nivel se pinto sin su .ds1"
    fi
else
    rojo "control 3b: no hay captura del mutante o no hay geometria medida de la corrida buena"
fi

# ================================================================================================
# 4. EL INSTRUMENTO DE LA CAPTURA, CON SU PROPIO MUTANTE
# ================================================================================================
paso "4. el instrumento de la captura y su mutante (que no sea ciego)"
if python3 "$HERE/mutar-captura-237.py" "$SAL/host/referencia.png" "$SAL/instrumento" \
        > "$SAL/instrumento.txt" 2>&1; then
    verde "control 4: el medidor de capturas caza los tres casos ($(grep -c 'OK:' "$SAL/instrumento.txt") OK): sano, espejado y vacio"
else
    rojo "control 4: el medidor de capturas es CIEGO en algun caso (ver $SAL/instrumento.txt)"
    grep -a 'FALLO' "$SAL/instrumento.txt" | sed 's/^/   /'
fi

printf '\n== RESULTADO: %d controles OK, %d en rojo (SAL=%s)\n' "$OK" "$FALLOS" "$SAL"
[ "$FALLOS" -eq 0 ]
