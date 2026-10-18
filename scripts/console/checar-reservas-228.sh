#!/bin/bash
# CORPUS DE DC-002.28: LAS TABLAS DEL MPQ POR TROZOS.
#
# QUE MIDE, EN LOS TRES NIVELES QUE HACEN FALTA (host, consola y el motor):
#   HOST:  la tabla se descifra POR TROZOS dando palabra por palabra lo mismo que de un solo tiron, y
#          NINGUN pedido de lectura pasa de 64 KiB (el umbral de "objeto grande" de libgodc: mas que eso se
#          va al monton de C de la consola, que es donde el motor muere). Ademas deja los numeros de
#          referencia de las cabeceras de los MPQ del juego.
#   CONSOLA (POC motor-reservas): los dos numeros de cada tabla (entradas y bytes) leidos del disco, y el
#          cargador del motor (`mpq.NewSource`) abriendo los MPQ en el ORDEN del motor, con el estado del
#          monton de C en cada paso (arena, libre, el bloque CONTIGUO mas grande y el `sbrk` contra el
#          techo de KOS). Lo que se exige es que el `gc_external_alloc: OOM` de DC-002.27 NO este.
#   MOTOR (POC motor-disco): el arranque del motor entero. Se mide hasta donde llega y se declara.
#
# CONTROLES (la imagen CON los MPQ):
#  1. el host: `go test ./d2common/d2fileformats/d2mpq/` verde (incluye los casos por trozos y el de las dos
#     tablas de un MPQ de verdad)
#  2. las cabeceras de los 3 MPQ dicen lo mismo en el host y en la consola (misma demanda de tabla)
#  3. la consola abre el MPQ chico por trozos: `mpq.NewSource OK` y NO hay `gc_external_alloc: OOM`
#  4. el motor ya no muere en el `gc_external_alloc: OOM` de las tablas (y se declara hasta donde llega)
#
# MUTANTES (tienen que CAZARSE):
#  A. MUTANTE_RESERVA=1 (o el script suelto): la tabla vuelve a pedirse ENTERA de un tiron ->
#     en el HOST el control 1 se pone ROJO (el pedido pasa los 64 KiB) y en la CONSOLA vuelve el
#     `gc_external_alloc: OOM` con el MPQ grande. Toca el repo (no el vendor): va con bandera para que la
#     corrida corta siga siendo corta.
#  B. el trozo de 128 KiB (`mutar-reservas-228.py --trozo-de-128k`): en el HOST el control 1 se pone ROJO.
#     Se corre siempre (cuesta segundos) y se restaura: el corpus exige que el archivo quede con el mismo
#     sha256 con el que empezo.
#
# Uso: bash scripts/console/checar-reservas-228.sh [dir_gox] [dir_mpq] [segundos_motor]
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

GEN="${1:-/tmp/gox-228}"
MPQ="${2:-$HOME/.wine-d2/drive_c/Games/Diablo II}"
SEGS="${3:-75}"
REPO="${REPO:-$(cd "$HERE/../.." && pwd)}"
SAL="${SAL:-/home/dgalli/.hermes/cache/scratch/corpus-228}"
MUT="$HERE/mutar-reservas-228.py"
CRYPTO="$REPO/d2common/d2fileformats/d2mpq/crypto.go"

export GODC_GOX="$GEN"
mkdir -p "$SAL"

OK=0
FALLOS=0
nota() { echo "   $*"; }
verde() { OK=$((OK + 1)); echo "   OK: $*"; }
rojo() { FALLOS=$((FALLOS + 1)); echo "   FALLO: $*"; }

# ================================================================================================
# 0. LA CARPETA QUE SE HORNEA Y EL ESTADO DE LOS PARCHES DE LOS QUE DEPENDE
# ================================================================================================
echo "== 0. la carpeta del juego (enlaces duros: no copia 515 MB) y el parche de la sesion de KOS =="
mkdir -p "$SAL/juego"

# El orden es el del motor (d2setup.CoreMPQs, "-mpq" del build): patch_d2 primero.
for n in patch_d2.mpq d2data.mpq d2char.mpq; do
    real="$(ls -1 "$MPQ" | grep -ix "$n" | head -1)"
    [ -n "$real" ] || { echo "   no encuentro $n en $MPQ"; exit 1; }

    ln -f "$MPQ/$real" "$SAL/juego/$n" 2>/dev/null || cp "$MPQ/$real" "$SAL/juego/$n"
done

nota "MPQ horneados: $(ls "$SAL/juego" | tr '\n' ' ') ($(du -sb "$SAL/juego" | cut -f1) bytes)"

if [ "$("$DC_PYTHON" "$HERE/fix-kos-iso9660-sesion.py" --ver | grep -c PARCHEADO)" = "1" ]; then
    verde "control 0: el driver iso9660 de KOS tiene el parche de la sesion de alta densidad"
else
    rojo "control 0: falta el parche de KOS (scripts/console/fix-kos-iso9660-sesion.py + make -C \$KOS_BASE)"
fi

# ================================================================================================
# 1. EL HOST: el descifrado por trozos y el tope del pedido
# ================================================================================================
echo "== 1. el host: la tabla por trozos, palabra por palabra igual y sin pedir mas de 64 KiB =="
if (cd "$REPO" && go test -count=1 ./d2common/d2fileformats/d2mpq/) > "$SAL/host-test.log" 2>&1; then
    verde "control 1: go test de d2mpq verde ($(grep -c '^ok' "$SAL/host-test.log") paquete(s))"
else
    rojo "control 1: go test de d2mpq en ROJO (ver $SAL/host-test.log)"
fi

nota "los casos por trozos: $(cd "$REPO" && go test -count=1 -v -run 'TestDecryptTablePorTrozosLeeDeAPoco|TestTablasPorTrozosUnMPQDeVerdad|TestDecryptTableSigueSiendoLaReferencia' ./d2common/d2fileformats/d2mpq/ 2>&1 | grep -c '^    --- PASS') subcasos verdes"

# los numeros de referencia del host: la cabecera de cada MPQ y lo que pide cada tabla (entradas * 16)
HOST_CAB="$SAL/cabeceras-host.txt"
"$DC_PYTHON" "$HERE/cabeceras-mpq.py" "$MPQ" > "$HOST_CAB"
nota "cabeceras del host (la demanda de cada tabla es entradas * 16):"
sed 's/^/     /' "$HOST_CAB"

# ================================================================================================
# 2. LA CONSOLA: el POC de las reservas (que pide cada tabla y con cuanta RAM libre)
# ================================================================================================
echo "== 2. la consola: el POC motor-reservas sobre el MISMO disco =="
A="$SAL/reservas"
MKISO_JOLIET=0 DC_RUTA_DATOS=/cd/juego DC_MPQ_JUEGO=patch_d2.mpq,d2data.mpq,d2char.mpq \
    DC_SIN_SELLOS=1 DC_DISCO_JUEGO="$SAL/juego" POC=motor-reservas \
    bash "$HERE/build-motor-arranque.sh" "$A" "$GEN" > "$A.build.log" 2>&1 ||
    { echo "   el POC motor-reservas NO COMPILA/ARMA (ver $A.build.log)"; exit 1; }

CAPTURE=none bash "$HERE/run-flycast.sh" "$A/disc.gdi" 45 "$SAL/run-reservas" >/dev/null 2>&1
LR="$SAL/run-reservas/flycast.log"

# control 2: los dos numeros de cada tabla, los mismos que dice el host
CAB_OK=0
for caso in "patch_d2.mpq 16384 262144 219 3504" "d2data.mpq 65536 1048576 11083 177328" "d2char.mpq 16384 262144 10010 160160"; do
    set -- $caso
    nombre="$1"; he="$2"; hb="$3"; be="$4"; bb="$5"

    if grep -aq "RESERVA-CONSOLA   1 $nombre hashEntradas= $he hashBytes= $hb blockEntradas= $be blockBytes= $bb" "$LR" ||
       grep -aq "RESERVA-CONSOLA   2 $nombre hashEntradas= $he hashBytes= $hb blockEntradas= $be blockBytes= $bb" "$LR" ||
       grep -aq "RESERVA-CONSOLA   3 $nombre hashEntradas= $he hashBytes= $hb blockEntradas= $be blockBytes= $bb" "$LR"; then
        CAB_OK=$((CAB_OK + 1))
        nota "$nombre: consola = host (hash $he entradas / $hb bytes, bloques $be / $bb)"
    else
        nota "$nombre: la consola NO dijo los numeros del host (ver $LR)"
    fi
done

if [ "$CAB_OK" = 3 ]; then
    verde "control 2: las cabeceras de los 3 MPQ coinciden en el host y en la consola"
else
    rojo "control 2: solo $CAB_OK de 3 cabeceras coinciden"
fi

# control 3: el cargador del motor abre el MPQ por trozos y NO hay OOM del monton de C
if grep -aq 'CARGADOR-CONSOLA  mpq.NewSource OK (las dos tablas)' "$LR" &&
   [ "$(grep -ac 'gc_external_alloc: OOM' "$LR")" = "0" ] &&
   [ "$(grep -ac 'encoding/binary: Read no soporta' "$LR")" = "0" ]; then
    verde "control 3: el MPQ chico se abre por trozos y NO hay gc_external_alloc: OOM"
else
    rojo "control 3: el cargador del motor no abre por trozos o vuelve el OOM (ver $LR)"
fi

nota "el monton de la consola en cada paso:"
tr -d '\r' < "$LR" | grep -a "CARGADOR-CONSOLA  monton\|RESERVA-CONSOLA 2/2" | sed 's/^/     /' | head -8
nota "la pared que queda (declarada, no es un control): $(tr -d '\r' < "$LR" | grep -a 'fatal error' | tail -1)"
nota "las marcas del runtime: $(tr -d '\r' < "$LR" | grep -a 'Memory: arena' | tail -1)"

# ================================================================================================
# 3. EL MOTOR: el arranque de verdad (hasta donde llega)
# ================================================================================================
echo "== 3. el motor entero sobre el mismo disco =="
M="$SAL/motor"
MKISO_JOLIET=0 DC_RUTA_DATOS=/cd/juego DC_MPQ_JUEGO=patch_d2.mpq,d2data.mpq,d2char.mpq \
    DC_SIN_SELLOS=1 DC_DISCO_JUEGO="$SAL/juego" POC=motor-disco \
    bash "$HERE/build-motor-arranque.sh" "$M" "$GEN" > "$M.build.log" 2>&1 ||
    { echo "   el POC motor-disco NO COMPILA/ARMA (ver $M.build.log)"; exit 1; }

CAPTURE=none bash "$HERE/run-flycast.sh" "$M/disc.gdi" "$SEGS" "$SAL/run-motor" >/dev/null 2>&1
LM="$SAL/run-motor/flycast.log"

# control 4: el motor YA NO muere en el OOM de las tablas (el de DC-002.27)
if [ "$(grep -ac 'gc_external_alloc: OOM' "$LM")" = "0" ] &&
   [ "$(grep -ac 'C-CONSTRUCTOR\|MOTOR-DISCO' "$LM")" -ge 1 ]; then
    verde "control 4: el motor pasa el OOM de las tablas (la pared de ahora NO es el monton de C)"
else
    rojo "control 4: el motor ni arranca o vuelve el OOM (ver $LM)"
fi

# y el hallazgo de la tarjeta, declarado tal cual (no es un control): hasta donde llega el motor hoy
nota "hasta donde llego el motor:"
tr -d '\r' < "$LM" | grep -a "C-CONSTRUCTOR\|MOTOR-DISCO\|CARGADOR-CONSOLA\|File Loader\|App\]\|fatal\|out of memory\|Memory: arena" |
    sed 's/^/     /' | tail -12
if [ "$(grep -ac 'MOTOR-DISCO arranca' "$LM")" = "0" ]; then
    nota "OJO (hallazgo DC-002.28, no es un control): el binario del motor NO llego a su primera marca: cuelgue"
    nota "  mudo adentro del arranque del runtime de Go. Medido: con los gox de DC-002.27 el mismo build corre"
    nota "  hasta el cuadro 2. Ver evidencia/dc-002.28 (seccion 4) y la tarjeta hija."
fi

# ================================================================================================
# 4. LOS MUTANTES
# ================================================================================================
echo "== 4. mutante B: el trozo de 128 KiB (mas que el umbral de 64 KiB) =="
SHA_ANTES="$(sha256sum "$CRYPTO" | awk '{print $1}')"
"$DC_PYTHON" "$MUT" --trozo-de-128k > "$SAL/mutante-b.log" 2>&1

if (cd "$REPO" && go test -count=1 ./d2common/d2fileformats/d2mpq/) > "$SAL/mutante-b-test.log" 2>&1; then
    rojo "mutante B NO cazado: el host lo aprueba con el trozo de 128 KiB"
else
    verde "mutante B cazado en el host: $(grep -a 'el tope es' "$SAL/mutante-b-test.log" | head -1 | sed 's/^ *//')"
fi

"$DC_PYTHON" "$MUT" --restaurar > /dev/null 2>&1
SHA_DESPUES="$(sha256sum "$CRYPTO" | awk '{print $1}')"
[ "$SHA_ANTES" = "$SHA_DESPUES" ] && verde "el archivo quedo con el mismo sha256 (sin mutante pegado)" ||
    rojo "el archivo NO quedo como estaba ($SHA_ANTES contra $SHA_DESPUES)"

echo "== 4b. mutante A: la tabla entera de un tiron (host siempre; consola con MUTANTE_RESERVA=1) =="
"$DC_PYTHON" "$MUT" --una-sola-reserva > "$SAL/mutante-a.log" 2>&1

if (cd "$REPO" && go test -count=1 ./d2common/d2fileformats/d2mpq/) > "$SAL/mutante-a-test.log" 2>&1; then
    rojo "mutante A NO cazado en el host: la reserva unica pasa el control"
else
    verde "mutante A cazado en el host: $(grep -aE 'el tope es|trozos, esperaba' "$SAL/mutante-a-test.log" | head -1 | sed 's/^ *//')"
fi

if [ "${MUTANTE_RESERVA:-0}" = "1" ]; then
    # la consola: se recompila d2mpq CON el mutante y se vuelve a correr el POC de las reservas.
    #
    # OJO -- MEDIDO EN DC-002.28: el mutante NO se caza por el OOM en este POC. El POC tiene 12,8 MB
    # contiguos (bloqueMax) y la reserva unica de d2data (2 MB) entra; el unico binario con el monton
    # apretado es el MOTOR (16 MB de RAM, 14 MB de ELF) y hoy no arranca (cuelgue del layout, ver la
    # evidencia seccion 4 y la tarjeta hija). Lo que SI se mide aca es la HUELLA del mutante en el monton
    # de C: con la reserva unica el `sbrk` se mueve al abrir el MPQ chico (la peticion grande existe),
    # y por el camino por trozos el `sbrk` NO se mueve (medido: 0x8c3b7000 antes y despues).
    bash "$HERE/recompilar-paquete-motor.sh" d2common/d2fileformats/d2mpq "$GEN" > "$SAL/mutante-a-gox.log" 2>&1

    B="$SAL/reservas-mutante-a"
    MKISO_JOLIET=0 DC_RUTA_DATOS=/cd/juego DC_MPQ_JUEGO=patch_d2.mpq,d2data.mpq,d2char.mpq \
        DC_SIN_SELLOS=1 DC_DISCO_JUEGO="$SAL/juego" POC=motor-reservas \
        bash "$HERE/build-motor-arranque.sh" "$B" "$GEN" > "$B.build.log" 2>&1

    CAPTURE=none bash "$HERE/run-flycast.sh" "$B/disc.gdi" 45 "$SAL/run-mutante-a" >/dev/null 2>&1
    LB="$SAL/run-mutante-a/flycast.log"

    SB_SANO="$(tr -d '\r' < "$LR" | grep -a 'CARGADOR-CONSOLA  monton tras las tablas' | head -1 | sed 's/.*sbrk=\(0x[0-9a-f]*\).*/\1/')"
    SB_MUT="$(tr -d '\r' < "$LB" | grep -a 'CARGADOR-CONSOLA  monton tras las tablas' | head -1 | sed 's/.*sbrk=\(0x[0-9a-f]*\).*/\1/')"
    OOM_MUT="$(grep -ac 'gc_external_alloc: OOM' "$LB")"

    nota "sbrk al abrir el MPQ chico: por trozos=$SB_SANO  con la reserva unica=$SB_MUT  (OOM del mutante: $OOM_MUT)"

    if [ -n "$SB_MUT" ] && [ "$SB_MUT" != "$SB_SANO" ]; then
        verde "mutante A cazado en la CONSOLA: con la reserva unica el monton de C se movio ($SB_SANO -> $SB_MUT)"
    else
        rojo "mutante A NO cazado en la consola: el monton de C se movio igual con y sin el mutante (ver $LB)"
    fi

    [ "$OOM_MUT" != "0" ] && verde "de yapa: con la reserva unica TAMBIEN vuelve el gc_external_alloc: OOM" ||
        nota "declarado: en este POC la reserva unica NO da OOM (tiene 12,8 MB contiguos); el OOM lo dio el motor (DC-002.27/29)"

    # y se deja el gox y el archivo como estaban
    "$DC_PYTHON" "$MUT" --restaurar > /dev/null 2>&1
    bash "$HERE/recompilar-paquete-motor.sh" d2common/d2fileformats/d2mpq "$GEN" > "$SAL/mutante-a-gox-restaurar.log" 2>&1
    SHA_FIN="$(sha256sum "$CRYPTO" | awk '{print $1}')"
    [ "$SHA_ANTES" = "$SHA_FIN" ] && verde "el archivo quedo con el mismo sha256 (sin mutante pegado)" ||
        rojo "el archivo NO quedo como estaba ($SHA_ANTES contra $SHA_FIN)"
else
    nota "el mutante A en la consola no se corrio (MUTANTE_RESERVA=1 lo corre: recompila d2mpq con la reserva unica y exige que el monton de C se mueva)"
fi

"$DC_PYTHON" "$MUT" --estado

echo
echo "== RESULTADO: $OK controles OK, $FALLOS con fallo =="
[ "$FALLOS" -eq 0 ]
