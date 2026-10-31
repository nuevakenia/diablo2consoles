#!/bin/bash
# DC-002.91 -- CONTROL: EL ARBOL DE .gox VIEJO AVISA ANTES DE COMPILAR, Y CON EL ARBOL FRESCO NO MOLESTA.
#
# POR QUE EXISTE. En DC-002.90 (commit a02487b8) el build de las tablas murio por
#   error: reference to undefined identifier 'zlib.NuevoInflador'
# apuntando a `d2common/d2fileformats/d2mpq/prestamo_de_zlib_consola.go`, o sea al CODIGO. El simbolo
# existia: lo que estaba viejo era /tmp/gox-med (30/10 13:22) contra la stdlib del repo (30/10 19:24,
# DC-002.80 commit 97a5c183). Rehacerlo cuesta ~6 min. `revisar-arbol-gox.sh` es el chequeo que lo dice
# ANTES de compilar; esta es su prueba, con un arbol envejecido A PROPOSITO (un .gox tocado hacia atras).
#
# LO QUE EXIGE
#   1. arbol FRESCO: exit 0, una linea, sin recuadro, y rapido (es un chequeo de fechas, no un build)
#   2. stdlib envejecida: exit 1 y el recuadro dice el PAQUETE (compress/zlib), las DOS fechas y la RECETA
#   3. GOX_VIEJO_SEGUIR=1: avisa lo mismo y NO aborta (para medir a proposito con un arbol viejo)
#   4. un paquete del motor en RECOMPILA: no aborta, se informa como "se recompila aca"
#   5. el mismo paquete fuera de RECOMPILA: aborta y lo nombra
#   6. un tercero (ebiten): aborta con su RUTA DE IMPORTACION (la tabla sale de build-std-gox.sh)
#   7. un .gox que no existe: se informa como tal
#   8. GOX_CHEQUEO=0: se salta
#   9. el build de verdad (build-tablas-282.sh) ABORTA antes de compilar: no hay build.log ni paso 1/6
#  10. EN CONTRA: con TODOS los .gox envejecidos el chequeo tiene que gritar por ~200 paquetes (si se
#      rompiera y dijera "todo fresco", esta prueba se pone roja)
#
# Uso:  scripts/console/checar-arbol-gox-291.sh [arbol_fresco] [dir_salida]
#       arbol fresco por defecto: /tmp/gox-290 (el que dejo DC-002.90: stdlib 63 + motor 145/145)
# sale: 0 = todo en verde; 1 = alguna en rojo; 3 = no se pudo correr
set -o pipefail

REPO="$(cd "$(dirname "$0")/../.." && pwd)"
HERRAMIENTA="$REPO/scripts/console/revisar-arbol-gox.sh"
BUILD="$REPO/scripts/console/build-tablas-282.sh"
BASE="${1:-/tmp/gox-290}"
SALIDA="${2:-/tmp/dcbuild/chequeo-291}"
MOTOR="github.com/OpenDiablo2/OpenDiablo2"

verdes=0
rojas=0
ok()  { echo "   OK    $*"; verdes=$((verdes + 1)); }
rojo() { echo "   ROJO  $*"; rojas=$((rojas + 1)); }

[ -f "$HERRAMIENTA" ] || { echo "FALLA: no esta $HERRAMIENTA"; exit 3; }
[ -f "$BASE/orden.txt" ] || {
    echo "FALLA: no hay arbol de .gox en $BASE"
    echo "       arma uno: bash scripts/console/build-std-gox.sh $BASE"
    echo "                 bash scripts/console/build-motor-all.sh $BASE $BASE-log"
    exit 3
}

# el clon es una COPIA de verdad (68 MB, menos de un segundo): con enlaces duros, tocar un .gox del clon
# le cambia la fecha al inodo COMPARTIDO y envenena el arbol de verdad -- medido en la primera corrida de
# esta prueba, que dejo /tmp/gox-290 con todo en 01/01/00:00. Copia, no enlace.
CLON="$(mktemp -d /tmp/gox-clon-291.XXXXXX)"
rm -rf "$CLON"
cp -a --reflink=auto "$BASE" "$CLON"
trap 'rm -rf "$CLON"' EXIT

envejecer() {  # $1 = archivo del clon, [$2 = fecha]. Copia + mv = inodo nuevo; el original no se toca.
    local f="$1" cuando="${2:-2020-01-01 00:00}"
    [ -f "$f" ] || { rojo "no existe $f (¿el arbol es el de DC-002.90?)"; return 1; }
    cp -p "$f" "$f.nuevo" && mv -f "$f.nuevo" "$f"
    touch -d "$cuando" "$f"
}

corre() {  # $1 = BASE_GOX ; el resto de las variables ya vienen del entorno de la llamada
    local dir="$1"
    shift
    SAL="$(env "$@" BASE_GOX="$dir" "$HERRAMIENTA" 2>&1)"
    RC=$?
}

echo "== 1. el arbol FRESCO ($BASE) no molesta"
T0="$(date +%s%N)"
corre "$BASE"
MS=$(( ($(date +%s%N) - T0) / 1000000 ))
[ "$RC" -eq 0 ] && ok "fresco: exit 0" || rojo "fresco: exit $RC (esperaba 0)"
printf '%s\n' "$SAL" | grep -q "ninguno mas viejo que su fuente" &&
    ok "fresco: dice que esta al dia" || rojo "fresco: no dice que esta al dia: $SAL"
printf '%s\n' "$SAL" | grep -q "OJO" &&
    rojo "fresco: saco un recuadro de aviso (no deberia)" || ok "fresco: sin recuadro"
[ "$MS" -lt 2000 ] && ok "fresco: tarda ${MS} ms" || rojo "fresco: tarda ${MS} ms (es un chequeo de fechas)"

echo "== 2. la stdlib envejecida a proposito: paquete, dos fechas y receta"
envejecer "$CLON/compress/zlib.gox"
corre "$CLON"
[ "$RC" -eq 1 ] && ok "zlib viejo: aborta (exit 1)" || rojo "zlib viejo: exit $RC (esperaba 1)"
for t in "compress/zlib" "01/01 00:00" "build-std-gox.sh" "build-motor-all.sh" "ABORTA"; do
    printf '%s\n' "$SAL" | grep -qF "$t" && ok "zlib viejo: el aviso dice '$t'" \
        || rojo "zlib viejo: el aviso NO dice '$t'"
done
printf '%s\n' "$SAL" | grep -q "zlib.go" && ok "zlib viejo: nombra la fuente mas nueva (zlib.go)" \
    || rojo "zlib viejo: no nombra la fuente"

echo "== 3. GOX_VIEJO_SEGUIR=1 avisa igual pero no aborta"
corre "$CLON" GOX_VIEJO_SEGUIR=1
[ "$RC" -eq 0 ] && ok "seguir: exit 0 (no aborta)" || rojo "seguir: exit $RC (esperaba 0)"
printf '%s\n' "$SAL" | grep -q "compress/zlib" && ok "seguir: el aviso sigue saliendo" \
    || rojo "seguir: se perdio el aviso"
printf '%s\n' "$SAL" | grep -q "se SIGUE igual" && ok "seguir: dice que sigue igual" \
    || rojo "seguir: no dice que sigue"

echo "== 4. un paquete del motor que ESTE build recompila: informa, no aborta"
touch "$CLON/compress/zlib.gox"   # el de la prueba 2 vuelve a estar al dia: aca el unico viejo es pvr
envejecer "$CLON/$MOTOR/d2core/d2render/pvr.gox"
corre "$CLON" RECOMPILA="d2common/d2cache d2core/d2render/pvr"
[ "$RC" -eq 0 ] && ok "pvr en RECOMPILA: exit 0" || rojo "pvr en RECOMPILA: exit $RC (esperaba 0)"
printf '%s\n' "$SAL" | grep -q "se recompila aca" && ok "pvr en RECOMPILA: lo dice" \
    || rojo "pvr en RECOMPILA: no lo dice"

echo "== 5. el mismo .gox viejo FUERA de RECOMPILA: aborta y lo nombra"
corre "$CLON"
[ "$RC" -eq 1 ] && ok "pvr fuera de RECOMPILA: aborta" || rojo "pvr fuera de RECOMPILA: exit $RC (esperaba 1)"
printf '%s\n' "$SAL" | grep -q "d2core/d2render/pvr" && ok "pvr fuera de RECOMPILA: lo nombra" \
    || rojo "pvr fuera de RECOMPILA: no lo nombra"

echo "== 6. un tercero reescrito: se busca por su RUTA DE IMPORTACION, no por la carpeta"
envejecer "$CLON/github.com/hajimehoshi/ebiten/v2.gox"
corre "$CLON"
[ "$RC" -eq 1 ] && ok "ebiten viejo: aborta" || rojo "ebiten viejo: exit $RC (esperaba 1)"
printf '%s\n' "$SAL" | grep -qF "github.com/hajimehoshi/ebiten/v2" &&
    ok "ebiten viejo: el aviso usa la ruta de importacion" || rojo "ebiten viejo: no aparece la ruta de importacion"

echo "== 7. un .gox que no existe en el arbol"
rm -f "$CLON/bytes.gox"
corre "$CLON"
[ "$RC" -eq 1 ] && ok "sin objeto: aborta" || rojo "sin objeto: exit $RC (esperaba 1)"
printf '%s\n' "$SAL" | grep -q "bytes *(" && ok "sin objeto: lo dice como '(no existe)'" \
    || rojo "sin objeto: no aparece 'bytes (no existe)'"

echo "== 8. GOX_CHEQUEO=0 se lo salta"
corre "$CLON" GOX_CHEQUEO=0
[ "$RC" -eq 0 ] && ok "saltado: exit 0" || rojo "saltado: exit $RC (esperaba 0)"
printf '%s\n' "$SAL" | grep -q "chequeo saltado" && ok "saltado: lo dice" || rojo "saltado: no lo dice"

echo "== 9. el build de verdad ABORTA ANTES de compilar (build-tablas-282.sh con el arbol viejo)"
SALIDA_BUILD="$SALIDA/tablas"
SALB="$(BASE_GOX="$CLON" bash "$BUILD" "$SALIDA_BUILD" 2>&1)"
RCB=$?
[ "$RCB" -ne 0 ] && ok "build: termina en rojo (exit $RCB)" || rojo "build: exit 0 con el arbol viejo"
printf '%s\n' "$SALB" | grep -q "OJO: EL ARBOL DE .gox ES MAS VIEJO" &&
    ok "build: el recuadro sale en el build" || rojo "build: el build no mostro el recuadro"
printf '%s\n' "$SALB" | grep -q "== 1/6" &&
    rojo "build: ALCANZO A COMPILAR (paso 1/6) antes de avisar" || ok "build: no llego a compilar"
[ -f "$SALIDA_BUILD/build.log" ] && rojo "build: dejo build.log (compilo)" || ok "build: sin build.log"

echo "== 10. EN CONTRA: con TODOS los .gox envejecidos tiene que gritar, no quedarse callado"
find "$CLON" -name '*.gox' -exec touch -d '2020-01-01 00:00' {} +
corre "$CLON"
[ "$RC" -eq 1 ] && ok "todo viejo: aborta" || rojo "todo viejo: exit $RC (esperaba 1)"
CUENTA="$(printf '%s\n' "$SAL" | grep -c '^   ')"
[ "$CUENTA" -ge 100 ] && ok "todo viejo: nombra $CUENTA paquetes (si dijera 'fresco', el chequeo estaria roto)" \
    || rojo "todo viejo: solo nombra $CUENTA paquetes (el chequeo no esta mirando el arbol)"

echo
echo "== RESULTADO: $verdes en verde, $rojas en rojo"
[ "$rojas" -eq 0 ] || exit 1
exit 0
