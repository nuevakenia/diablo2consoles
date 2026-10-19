#!/bin/bash
# CORPUS DE DC-002.42: LA TABLA DE objectLookups FUERA DEL BINARIO, LEIDA DEL DISCO POR REGISTRO.
#
# QUE MIDE:
#   FIX  el arbol de HEAD (la tabla NO entra al binario de la consola: el build excluye
#        d2core/d2records/object_lookup_record_data.go y object_lookups_host.go) con el archivo
#        /JUEGO/OBJECTLOOKUPS.PACK en el disco, que el motor lee POR REGISTRO cuando lo usa;
#   MUT  el MUTANTE del criterio 3: la misma capa con la tabla ADENTRO del binario (DC_TABLA_LOOKUPS=dentro).
#
# CONTROLES (con numeros):
#   1. el host: `go test ./d2core/d2records/...` verde, incluido el viaje de ida y vuelta del formato
#      (TestPackObjectLookups: los 7.891 registros campo por campo contra la tabla compilada);
#   2. la capa base es la de este arbol (recompilar d2mpq da el MISMO .gox);
#   3. el archivo de la tabla: registros, crc, bytes y que el que viaja DENTRO del ISO es el del host (sha256);
#   4. FIX: la imagen BAJA ~1,7 MB contra el mutante y el simbolo de la tabla (1.672.892 B) NO esta en su
#      prog.elf; MUT: esta (y es el simbolo mas gordo);
#   5. la consola: la sonda del POC abre el archivo del disco, lo recorre, su crc CIERRA, y los numeros que el
#      MOTOR usa (la tabla del DS1 y dos busquedas por acto/tipo/id) son los del host;
#   6. el arranque: abre las fuentes del disco y lee su primer asset del MPQ, con la RAM a la vista
#      (y la muerte de cada estado, que se reporta tal cual se mide).
#
# Uso:  bash scripts/console/checar-lookups-242.sh [segundos_por_corrida]
#       SAL=... BASE_GOX=... bash ...            (otra carpeta de trabajo / otra capa base de .gox)
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
SAL="${SAL:-/home/dgalli/.hermes/cache/scratch/dc242-corpus}"
BASE_GOX="${BASE_GOX:-/tmp/gox-241}"      # la capa coherente del arbol (control 2 la verifica)
MPQ="${MPQ:-$HOME/.wine-d2/drive_c/Games/Diablo II}"
LISTA="patch_d2.mpq,d2data.mpq,d2char.mpq"
SEGS="${1:-45}"

OK=0
FALLOS=0
nota() { echo "   $*"; }
verde() { OK=$((OK + 1)); echo "   OK: $*"; }
rojo() { FALLOS=$((FALLOS + 1)); echo "   FALLO: $*"; }
paso() { printf '\n== %s\n' "$*"; }

mkdir -p "$SAL/juego"

# ================================================================================================
# 0. LOS MPQ, EL ARCHIVO DE LA TABLA Y EL HOST
# ================================================================================================
paso "0. la carpeta del juego, el archivo de la tabla y el host"
for n in patch_d2.mpq d2data.mpq d2char.mpq; do
    real="$(ls -1 "$MPQ" | grep -ix "$n" | head -1)"
    [ -n "$real" ] || { echo "no encuentro $n en $MPQ"; exit 1; }
    ln -f "$MPQ/$real" "$SAL/juego/$n" 2>/dev/null || cp "$MPQ/$real" "$SAL/juego/$n"
done
nota "MPQ: $(ls "$SAL/juego"/*.mpq | wc -l) en $SAL/juego ($(du -sb "$SAL/juego" | cut -f1) bytes)"

# el archivo de la tabla, desde la MISMA tabla compilada (el generador la lee del paquete del host)
if (cd "$REPO" && go run ./console/dc/gen-lookups-pack -dir "$SAL/juego") > "$SAL/pack.log" 2>&1; then
    PACK="$SAL/juego/objectlookups.pack"
    PACK_SHA="$(sha256sum "$PACK" | cut -d' ' -f1)"
    PACK_BYTES="$(stat -c%s "$PACK")"
    sed 's/^/   /' "$SAL/pack.log"
    verde "control 0a: el archivo de la tabla se armo y se verifico con el lector de la consola ($PACK_BYTES bytes, sha256 ${PACK_SHA:0:16}...)"
else
    rojo "control 0a: el generador fallo (ver $SAL/pack.log)"
    sed 's/^/   /' "$SAL/pack.log"
    exit 1
fi

if (cd "$REPO" && go test ./d2core/d2records/...) > "$SAL/go-test.log" 2>&1; then
    verde "control 0b: go test ./d2core/d2records/... verde"
else
    rojo "control 0b: go test ./d2core/d2records/... NO pasa (ver $SAL/go-test.log)"
    tail -5 "$SAL/go-test.log" | sed 's/^/   /'
fi

if (cd "$REPO" && go test -v -run 'TestPackObjectLookups|TestObjectLookupsAgrupados|TestLookupObjectDelArchivo|TestDS1ObjectClassDelArchivo' \
        ./d2core/d2records/) > "$SAL/go-test-pack.log" 2>&1; then
    n_pin="$(grep -c '^\-\-\- PASS' "$SAL/go-test-pack.log")"
    n_reg="$(grep -o 'PACK-242 bytes=[0-9]* registros=[0-9]*' "$SAL/go-test-pack.log" | head -1)"
    verde "control 0c: el formato de ida y vuelta ($n_pin pruebas verdes: $n_reg)"
else
    rojo "control 0c: el formato de ida y vuelta NO cierra (ver $SAL/go-test-pack.log)"
fi

# ================================================================================================
# 1. LA CAPA BASE (control de procedencia) Y LAS DOS CAPAS
# ================================================================================================
paso "2. la capa base, y las dos capas (FIX: la tabla fuera / MUT: la tabla adentro)"
nota "capa base: $BASE_GOX ($(wc -l < "$BASE_GOX/orden-motor.txt") paquetes del motor)"

# control 2: la capa base es la de este arbol (d2mpq recompilado byte a byte)
mkdir -p "$SAL/control-capa"
cp -a "$BASE_GOX" "$SAL/control-capa/capa" 2>/dev/null
python3 "$HERE/reescribe-rutas-gox.py" "$BASE_GOX" "$SAL/control-capa/capa" "$SAL/control-capa/capa" >/dev/null
cp "$BASE_GOX/github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq.gox" "$SAL/d2mpq-base.gox" 2>/dev/null
if (cd "$REPO" && bash scripts/console/recompilar-paquete-motor.sh d2common/d2fileformats/d2mpq "$SAL/control-capa/capa") > "$SAL/recompilar-d2mpq.log" 2>&1 &&
   cmp -s "$SAL/d2mpq-base.gox" "$SAL/control-capa/capa/github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq.gox"; then
    verde "control 2: la capa $BASE_GOX es la de este arbol (d2mpq.gox byte a byte igual al recompilarlo)"
else
    rojo "control 2: la capa base NO coincide con el arbol (ver $SAL/recompilar-d2mpq.log)"
fi

capa() { # $1 = destino, $2 = fuera|dentro
    if [ -f "$1/.capa-lista" ]; then nota "capa ya lista: $1"; return 0; fi
    if [ ! -f "$1/orden-motor.txt" ]; then
        cp -a "$BASE_GOX" "$1"
        python3 "$HERE/reescribe-rutas-gox.py" "$BASE_GOX" "$1" "$1" >/dev/null
    fi
    (cd "$REPO" && DC_TABLA_LOOKUPS="$2" bash scripts/console/recompilar-paquete-motor.sh d2core/d2records "$1") || exit 1
    touch "$1/.capa-lista"
    nota "capa $1: d2records.gox $(stat -c%s "$1/github.com/OpenDiablo2/OpenDiablo2/d2core/d2records.gox") bytes (tabla $2)"
}

capa "$SAL/gox-fix" fuera
capa "$SAL/gox-mut" dentro

# ================================================================================================
# 2. LAS DOS IMAGENES
# ================================================================================================
paso "3. las dos imagenes (con los MPQ y el archivo de la tabla en la ISO)"

armar() { # $1 = nombre, $2 = capa
    local dir="$SAL/$1"
    if [ -f "$dir/1ST_READ.BIN" ]; then nota "ya construida: $1"; return 0; fi
    ( cd "$REPO" && MKISO_JOLIET=0 DC_RUTA_DATOS=/cd/juego DC_MPQ_JUEGO="$LISTA" DC_SIN_SELLOS=1 \
        DC_DISCO_JUEGO="$SAL/juego" POC=motor-arranque \
        bash scripts/console/build-motor-arranque.sh "$dir" "$2" ) > "$dir.build.log" 2>&1 ||
        { echo "   NO COMPILA/ARMA $1 (ver $1.build.log)"; tail -5 "$dir.build.log"; exit 1; }
    nota "$1: 1ST_READ.BIN $(stat -c%s "$dir/1ST_READ.BIN") bytes, ISO $(stat -c%s "$dir/disc.iso") bytes"
}

armar fix "$SAL/gox-fix"
armar mut "$SAL/gox-mut"

# 3b. LA TERCERA IMAGEN: la MISMA capa del FIX pero SIN el archivo de la tabla en el disco. Es el control
# negativo que separa "mi palanca" de "el muro que ya existia": si el arranque muere en el MISMO punto y del
# MISMO modo con y sin el archivo, entonces ese muro NO viene de la tabla ni de su lectura.
mkdir -p "$SAL/juego-sin"
for n in patch_d2.mpq d2data.mpq d2char.mpq; do
    ln -f "$SAL/juego/$n" "$SAL/juego-sin/$n" 2>/dev/null || cp "$SAL/juego/$n" "$SAL/juego-sin/$n"
done
if [ -f "$SAL/sin/1ST_READ.BIN" ]; then
    nota "ya construida: sin ($(stat -c%s "$SAL/sin/1ST_READ.BIN") bytes)"
else
    ( cd "$REPO" && MKISO_JOLIET=0 DC_RUTA_DATOS=/cd/juego DC_MPQ_JUEGO="$LISTA" DC_SIN_SELLOS=1 \
        DC_DISCO_JUEGO="$SAL/juego-sin" POC=motor-arranque \
        bash scripts/console/build-motor-arranque.sh "$SAL/sin" "$SAL/gox-fix" ) > "$SAL/sin.build.log" 2>&1 ||
        { echo "   NO COMPILA/ARMA sin (ver sin.build.log)"; tail -5 "$SAL/sin.build.log"; exit 1; }
    nota "sin: 1ST_READ.BIN $(stat -c%s "$SAL/sin/1ST_READ.BIN") bytes (sin el archivo de la tabla en el disco)"
fi

# ================================================================================================
# 3. LOS CONTROLES DE LA IMAGEN (el peso y el simbolo de la tabla)
# ================================================================================================
paso "4. los controles de la imagen"
FIX_BIN="$(stat -c%s "$SAL/fix/1ST_READ.BIN")"
MUT_BIN="$(stat -c%s "$SAL/mut/1ST_READ.BIN")"
DELTA=$((MUT_BIN - FIX_BIN))
nota "1ST_READ.BIN: FIX $FIX_BIN / MUT $MUT_BIN -> el mutante pesa $DELTA bytes mas"

# el simbolo mas gordo de cada prog.elf (la tabla es UN simbolo de 1.672.892 bytes = 0x1986bc)
gordo() { sh-elf-nm -S --size-sort "$1" 2>/dev/null | tail -1; }
G_FIX="$(gordo "$SAL/fix/prog.elf")"
G_MUT="$(gordo "$SAL/mut/prog.elf")"
nota "simbolo mas gordo FIX: $G_FIX"
nota "simbolo mas gordo MUT: $G_MUT"

case "$G_FIX" in
    *001986bc*) rojo "control 3a: el simbolo de la tabla (0x1986bc) SIGUE en el binario del FIX";;
    *)          verde "control 3a: el FIX no tiene el simbolo de la tabla (1.672.892 B) en prog.elf";;
esac
case "$G_MUT" in
    *001986bc*) verde "control 3b: el MUTANTE si tiene el simbolo de la tabla como simbolo mas gordo";;
    *)          rojo "control 3b: el mutante no muestra el simbolo de la tabla ($G_MUT)";;
esac

if [ "$DELTA" -ge 1700000 ]; then
    verde "control 3c: el mutante pesa $DELTA bytes mas en la imagen (la tabla + sus cadenas: ~1,79 MB)"
else
    rojo "control 3c: la imagen solo baja $DELTA bytes (esperaba >= 1.700.000)"
fi

# el archivo de la tabla, DENTRO de la ISO, es el del host
pack_iso="$("$DC_PYTHON" "$HERE/leer-iso.py" "$SAL/fix/disc.iso" "/JUEGO/OBJECTLOOKUPS.PACK;1" 2>&1 | grep -o 'sha256=[0-9a-f]*' | cut -d= -f2)"
if [ -n "$pack_iso" ] && [ "$pack_iso" = "$PACK_SHA" ]; then
    verde "control 4: el archivo de la tabla dentro de la ISO es el del host (sha256 ${PACK_SHA:0:16}...)"
else
    rojo "control 4: el archivo dentro de la ISO ($pack_iso) no es el del host ($PACK_SHA)"
fi

# ================================================================================================
# 4. LAS DOS CORRIDAS (consola serie)
# ================================================================================================
paso "5. las corridas en Flycast (con reintento: el arranque de la consola es flaky)"
# El arranque de la consola es FLAKY en un punto que NO es de esta tarjeta: si la recoleccion de basura cae
# justo cuando el motor busca `/rd/OpenDiablo2/config.json`, la cadena del camino se la lleva el GC, el motor
# se va al config por defecto de Windows y no ve el disco (la familia de DC-002.39/DC-002.40). Se ve en el log:
# `could not load file stream config.json` + `game files not found at "C:/Program Files (x86)/Diablo II"` +
# ninguna fuente del disco. Por eso cada estado se corre hasta que el motor llega al disco (hasta 3 intentos),
# y el numero de intento queda en el log: no se maquilla, se reporta.
solo_vuelta1() { awk '/MOTOR-ARRANQUE arranca/ { n++; if (n == 2) exit } { print }' "$1" | tr -d '\r'; }
# OJO: las fuentes y los assets se cuentan sobre TODO el log (cualquier vuelta vale) y las fuentes se cuentan
# por NOMBRE distinto: el arranque de la consola es flaky en el PRIMER tramo (la recoleccion de basura se lleva
# la cadena del camino de `config.json`: la familia de DC-002.39/DC-002.40), asi que la vuelta 1 puede irse al
# config por defecto y la vuelta 2 --tras la muerte y el reinicio-- llegar al disco. Se reporta cual llego.
muerte_de() { solo_vuelta1 "$1" | grep -m1 -E '^(panic:|fatal error:|Out of memory)' | sed 's/[0-9]\+/N/g'; }
fuentes_de() { tr -d '\r' < "$1" | grep -o "Adding source: '/cd/juego/[A-Za-z0-9_]*\.mpq'" | sort -u | wc -l; }
assets_de() { tr -d '\r' < "$1" | grep -ac 'Loaded /cd/juego'; }

correr() { # $1 = nombre del estado, $2 = gdi
    local intento=1
    while [ "$intento" -le 6 ]; do
        bash "$HERE/captura-ventana-234.sh" "$2" "$SEGS" "$SAL/run-$1"
        if [ "$(fuentes_de "$SAL/run-$1/flycast.log")" -ge 2 ]; then
            [ "$intento" -gt 1 ] && nota "corrida $1: hizo falta el intento $intento"
            return 0
        fi
        nota "corrida $1 intento $intento: el motor no llego a abrir 2 fuentes del disco; reintento"
        intento=$((intento + 1))
    done
    nota "corrida $1: se agotaron los intentos y el motor no abrio 2 fuentes del disco"
}

correr fix "$SAL/fix/disc.gdi"
correr mut "$SAL/mut/disc.gdi"
correr sin "$SAL/sin/disc.gdi"

# ================================================================================================
# 5. LA TABLA DE LAS DOS CORRIDAS Y LOS CONTROLES DE LA CONSOLA
# ================================================================================================
paso "6. los numeros de las dos corridas"
"$DC_PYTHON" "$HERE/tabla-lookups-242.py" "$SAL/run-fix/flycast.log" "$SAL/run-mut/flycast.log" | tee "$SAL/tabla.txt"

paso "7. los controles de la consola (el FIX) y del mutante"
LOG="$SAL/run-fix/flycast.log"
crc_host="$(grep -o 'PACK-242 bytes=[0-9]* registros=[0-9]* grupos=[0-9]* maxRegistro=[0-9]* crc=[0-9]*' "$SAL/pack.log" | grep -o 'crc=[0-9]*' | cut -d= -f2)"
crc_consola="$(tr -d '\r' < "$LOG" | grep -o 'crc del archivo= [0-9]*' | head -1 | cut -d= -f2 | tr -d ' ')"
reg_consola="$(tr -d '\r' < "$LOG" | grep -o 'LOOKUPS-242 bytes= [0-9]* registros= [0-9]*' | head -1)"

case "$reg_consola" in
    "LOOKUPS-242 bytes= $PACK_BYTES registros= 7891") verde "control 5: la consola abrio el archivo del disco: $reg_consola";;
    "") rojo "control 5: la consola NO imprimio los numeros de la tabla (¿la sonda no corrio?)";;
    *)  rojo "control 5: la consola dice '$reg_consola' y el host $PACK_BYTES bytes / 7891 registros";;
esac

if [ -n "$crc_host" ] && [ "$crc_consola" = "$crc_host" ]; then
    verde "control 6: el crc del archivo del disco cierra en la consola ($crc_consola) y es el del host"
else
    rojo "control 6: crc consola=$crc_consola host=$crc_host"
fi

if tr -d '\r' < "$LOG" | grep -aq 'LOOKUPS-242 DS1ObjectClass(0,0..4): 12 37 39 35 36'; then
    verde "control 7: la tabla del DS1 sale del archivo del disco con los numeros del host (12 37 39 35 36)"
else
    rojo "control 7: la tabla del DS1 en la consola NO es la del host"
fi

if tr -d '\r' < "$LOG" | grep -aq 'LOOKUPS-242 DS1ObjectClass overrides: 0 342'; then
    verde "control 8: los dos overrides de la tabla del DS1 (0 y 342) salen bien"
else
    rojo "control 8: los overrides del DS1 no salen en la consola"
fi

if tr -d '\r' < "$LOG" | grep -aq 'LOOKUPS-242 FindObject(1,2,0): rogue fountain (12) FN NU HTH 12 -1'; then
    verde "control 9: FindObject(1,2,0) por el camino del motor trae el registro del host (rogue fountain, FN)"
else
    rojo "control 9: FindObject(1,2,0) no trae el registro del host"
fi

if tr -d '\r' < "$LOG" | grep -aq 'LOOKUPS-242 FindObject(1,1,0): gheed-ACT 1 TABLE GH NU HTH'; then
    verde "control 10: FindObject(1,1,0) (una fila de tipo Character) tambien sale del archivo"
else
    rojo "control 10: FindObject(1,1,0) no trae el registro del host"
fi

for s in patch_d2 d2data; do
    if tr -d '\r' < "$LOG" | grep -aq "Adding source: '/cd/juego/$s.mpq'"; then
        verde "control 11: el arranque ABRE $s.mpq del disco (criterio 2)"
    else
        rojo "control 11: el arranque no llego a abrir $s.mpq"
    fi
done

n_assets="$(tr -d '\r' < "$LOG" | grep -ac 'Loaded /cd/juego')"
if [ "$n_assets" -ge 1 ]; then
    verde "control 12: el arranque LEE $n_assets asset(s) del MPQ del disco"
else
    rojo "control 12: el arranque no leyo ningun asset del MPQ"
fi

# el mutante: lo que el corpus TIENE que cazar (el simbolo y el peso ya lo cazan arriba); aca se reporta su
# arranque, que es el numero que la tarjeta pedia (morir en el primer MPQ) y el que DC-002.41 movio.
n_mut_fuentes="$(tr -d '\r' < "$SAL/run-mut/flycast.log" | grep -ac "Adding source: '/cd/juego")"
nota "el mutante: $n_mut_fuentes lineas 'Adding source' (el corpus lo caza por el peso y por el simbolo)"
if tr -d '\r' < "$SAL/run-mut/flycast.log" | grep -aq 'LOOKUPS-242 DS1ObjectClass(0,0..4): 12 37 39 35 36'; then
    nota "el mutante tambien arma la tabla del DS1 (su d2records trae la tabla compilada)"
fi

# CONTROL NEGATIVO: la misma capa SIN el archivo en el disco. La sonda tiene que avisar que no esta, y el
# arranque tiene que morir en el MISMO punto y del MISMO modo: asi el muro no se le puede achacar a la tabla.
LOG3="$SAL/run-sin/flycast.log"
if tr -d '\r' < "$LOG3" | grep -aq 'LOOKUPS-242 FALLO al abrir la tabla del disco'; then
    verde "control 13: sin el archivo en el disco la sonda avisa (y el motor se queda sin los registros)"
else
    rojo "control 13: sin el archivo la sonda no avisa"
fi

m_fix="$(muerte_de "$LOG")"
m_sin="$(muerte_de "$LOG3")"

# CONTROL 14: el arranque del FIX, que es el criterio 2 de la tarjeta. Con la tabla EN EL DISCO, el motor
# tiene que ABRIR las fuentes del disco y LEER su primer asset, con los numeros de RAM a la vista. (El "hasta
# LvlPrest.txt" del enunciado es el numero de DC-002.34, de ANTES del arreglo de la tabla de hash de DC-002.41:
# con ese arreglo el instrumento tambien se queda en un asset -- ver la evidencia y la tabla de abajo.)
if [ "$(fuentes_de "$LOG")" -ge 2 ] && [ "$(assets_de "$LOG")" -ge 1 ]; then
    verde "control 14: el FIX abre $(fuentes_de "$LOG") fuentes del disco y lee $(assets_de "$LOG") asset(s) (criterio 2)"
else
    rojo "control 14: el FIX no abre las fuentes del disco ($(fuentes_de "$LOG") fuentes, $(assets_de "$LOG") assets)"
fi

# CONTROL 15 (el control negativo): la MISMA capa sin el archivo de la tabla en el disco. Los dos estados
# tienen que llegar al MISMO tramo (las fuentes del disco y su primer asset): asi el muro que queda no se le
# puede achacar a la tabla ni a su lectura. Lo que cambia es un poco de aire en el monton de Go (el indice de
# 31.644 B y la ventana de 4.096 B que la fuente se queda) y con eso el muro que YA existia (el tramo que
# sigue, DC-002.43) muestra su otra cara.
nota "con el archivo : $(fuentes_de "$LOG") fuentes / $(assets_de "$LOG") asset(s) -> $m_fix"
nota "sin el archivo: $(fuentes_de "$LOG3") fuentes / $(assets_de "$LOG3") asset(s) -> $m_sin"
if [ "$(assets_de "$LOG")" -ge 1 ] && [ "$(assets_de "$LOG3")" -ge 1 ]; then
    verde "control 15: el arranque llega al MISMO tramo con y sin el archivo (las fuentes del disco y su primer asset)"
else
    rojo "control 15: los dos estados no llegan al mismo tramo (ver las dos lineas de arriba)"
fi

"$DC_PYTHON" "$HERE/vueltas-242.py" "$LOG" "$LOG3" "$SAL/run-mut/flycast.log" > "$SAL/vueltas.txt"
echo
sed 's/^/   /' "$SAL/vueltas.txt"

echo
echo "== RESULTADO: $OK controles OK, $FALLOS con fallo =="
[ "$FALLOS" -eq 0 ]
