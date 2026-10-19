#!/bin/bash
# CORPUS DE DC-002.45: EL LISTADO DE UN DIRECTORIO DEL DISCO POR EL `fs_readdir` DEL DRIVER.
#
# QUE MIDE (los cinco criterios de la tarjeta, con la salida a la vista):
#
#  1. el PUENTE pide el listado del ISO9660 por el driver: `os.ReadDir` llama a `d2c_iso_listar`
#     (`fs_open(ruta, O_RDONLY|O_DIR)` + `fs_readdir` en bucle) y el manifiesto del disco
#     (`/JUEGO/MANIFIESTO.TXT`) queda de RESPALDO. El control de la capa exige que el `os.gox` que se
#     enlaza tenga el simbolo nuevo (con la capa vieja el binario lleva el puente anterior y el numero sale
#     como el defecto: es el defecto mas facil de leer mal, ya medido en DC-002.44).
#  2. el numero del MOTOR sigue siendo el de DC-002.44 con los 3 MPQ en /JUEGO:
#       Complete= true Validate.OK()= false archivos que ve= 3 faltan= 4
#     y el mutante sin /JUEGO sigue en 0 / faltan= 7.
#  3. EL CONTROL QUE EL MANIFIESTO NO DABA: un directorio del disco con un archivo que el HORNEADOR NO
#     CONOCE (`/JUEGO/MARCA_SUELTA.DAT`, 78 bytes, horneado por `mkdisco.sh` con `DC_MARCA_SUELTA=` y
#     deliberadamente FUERA del manifiesto) tiene que SALIR en el listado. Se mide tres veces: que la marca
#     este en la ISO, que NO este en el manifiesto (si estuviera, el control no probaria nada: los dos
#     caminos darian lo mismo) y que SI este en el listado de os.ReadDir, con sus 78 bytes.
#  4. lo que ya andaba sigue andando: el overlay de la RAM (`/rd/prueba-seleccion`) sigue listando sus
#     entradas en el motor, y el romdisk se sigue listando por su `.manifiesto` (el FS ROM de KOS no deja
#     abrir un directorio: eso lo mide el POC `vfs`, que este corpus corre como control).
#  5. LA MEMORIA: el listado se arma con buffer LOCAL (pila de tamano fijo), no del monton. Se mide con 40
#     vueltas del listado sobre el mismo directorio: el monton de C (`cfree` de mallinfo) tiene que quedar
#     IGUAL antes y despues. El semiespacio del GC si se mueve, y se declara: lo que sube es el RESULTADO
#     (el []DirEntry con sus cadenas), que paga igual el camino del manifiesto.
#
# Uso:  bash scripts/console/checar-lista-disco-245.sh [--refrescar] [segundos_motor]
#   --refrescar  rehace el POC del listado y los dos discos desde el arbol (la capa de .gox NO se rehace:
#                se le recompila el paquete os con compilar-paquete-std.sh, que es lo que cambia).
# Variables:  SAL=<dir de trabajo>  GEN=<capa de .gox>  (por defecto ~/.hermes/cache/scratch/c245)
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
SAL="${SAL:-/home/dgalli/.hermes/cache/scratch/c245}"
GEN="${GEN:-$SAL/gox}"
SEGS="${2:-45}"
REFRESCAR=0
[ "${1:-}" = "--refrescar" ] && REFRESCAR=1

OK=0
FALLOS=0
verde() { OK=$((OK + 1)); echo "   OK: $*"; }
rojo() { FALLOS=$((FALLOS + 1)); echo "   FALLO: $*"; }
paso() { printf '\n== %s\n' "$*"; }
linea() { grep -aF "$2" "$SAL/run-$1/flycast.log" 2>/dev/null | head -1 | tr -d '\r'; }

mkdir -p "$SAL"

# ================================================================================================
# 0. LOS MPQ (los mismos del PC), LA MARCA SUELTA Y LOS DOS DISCOS
# ================================================================================================
paso "0. los MPQ, la marca suelta y los dos discos"
MPQ="$HOME/.wine-d2/drive_c/Games/Diablo II"
mkdir -p "$SAL/juego3"
for n in patch_d2.mpq d2data.mpq d2char.mpq; do
    real="$(ls -1 "$MPQ" | grep -ix "$n" | head -1)"
    [ -n "$real" ] || { echo "no encuentro $n en $MPQ"; exit 1; }
    ln -f "$MPQ/$real" "$SAL/juego3/$n" 2>/dev/null || cp "$MPQ/$real" "$SAL/juego3/$n"
    if [ "$(sha256sum "$SAL/juego3/$n" | cut -d' ' -f1)" = "$(sha256sum "$MPQ/$real" | cut -d' ' -f1)" ]; then
        verde "MPQ $n es el archivo del PC byte a byte ($(stat -c%s "$SAL/juego3/$n") bytes)"
    else
        rojo "MPQ $n NO coincide con el del PC"
    fi
done

mkdir -p "$SAL/marca"
printf 'DC-002.45 marca suelta: este archivo esta en la ISO y NO en el MANIFIESTO.TXT\n' > "$SAL/marca/MARCA_SUELTA.DAT"
MARCA_BYTES="$(stat -c%s "$SAL/marca/MARCA_SUELTA.DAT")"
nota_marca="la marca suelta son $MARCA_BYTES bytes (sha256 $(sha256sum "$SAL/marca/MARCA_SUELTA.DAT" | cut -c1-16))"
echo "   $nota_marca"

# ================================================================================================
# 1. LA CAPA DE .GOX (el control de que el binario que se mide lleva el puente nuevo)
# ================================================================================================
paso "1. la capa de .gox: el paquete os con el listado por el driver"
[ -d "$GEN" ] || { echo "no existe la capa $GEN (correr una vez con la capa de DC-002.44)"; exit 1; }
if [ "$REFRESCAR" = 1 ] || [ ! -f "$GEN/os.gox" ] || [ "$(strings "$GEN/os.gox" | grep -c d2c_iso_listar)" = "0" ]; then
    ( cd "$REPO" && bash scripts/console/compilar-paquete-std.sh os "$GEN" ) > "$SAL/os-paquete.log" 2>&1 \
        || { echo "no compilo el paquete os en la capa (ver $SAL/os-paquete.log)"; exit 1; }
fi
if [ "$(strings "$GEN/os.gox" | grep -c d2c_iso_listar)" -ge 1 ]; then
    verde "control de la CAPA: el os.gox ($(stat -c%s "$GEN/os.gox") bytes) llama a d2c_iso_listar"
else
    rojo "control de la CAPA: el os.gox NO tiene el puente nuevo: el numero saldria como el defecto"
fi

# ================================================================================================
# 2. EL BINARIO DEL MOTOR Y LOS DOS DISCOS
# ================================================================================================
paso "2. el binario del motor y los dos discos (con marca suelta / mutante sin /JUEGO)"
if [ "$REFRESCAR" = 1 ] || [ ! -f "$SAL/marranque/1ST_READ.BIN" ] || [ "$SAL/marranque/os.gox.sha" != "$(sha256sum "$GEN/os.gox" | cut -d' ' -f1)" ]; then
    mkdir -p "$SAL/marranque"
    ( cd "$REPO" && POC=motor-arranque DC_RUTA_DATOS=/cd/juego DC_MPQ_JUEGO="patch_d2.mpq,d2data.mpq,d2char.mpq" \
        DC_SIN_SELLOS=1 bash scripts/console/build-motor-arranque.sh "$SAL/marranque" "$GEN" ) \
        > "$SAL/marranque.build.log" 2>&1 || { echo "no se armo el binario (ver $SAL/marranque.build.log)"; exit 1; }
    sha256sum "$GEN/os.gox" | cut -d' ' -f1 > "$SAL/marranque/os.gox.sha"
fi
verde "binario: 1ST_READ.BIN $(stat -c%s "$SAL/marranque/1ST_READ.BIN") bytes sha256 $(sha256sum "$SAL/marranque/1ST_READ.BIN" | cut -c1-16)"

if [ "$REFRESCAR" = 1 ] || [ ! -f "$SAL/l245-suelta/disc.gdi" ] || [ ! -f "$SAL/l245-mut/disc.gdi" ]; then
    bash "$SAL/arma-discos.sh" > "$SAL/discos.log" 2>&1 || { echo "no se armaron los discos (ver $SAL/discos.log)"; exit 1; }
fi

# el control del criterio 3, del lado de la IMAGEN: la marca esta adentro de la ISO...
DENTRO="$("$DC_PYTHON" "$HERE/leer-iso.py" "$SAL/l245-suelta/disc.iso" "/JUEGO/MARCA_SUELTA.DAT;1" 2>/dev/null | grep -o 'sha256=[0-9a-f]*' | cut -d= -f2)"
FUERA="$(sha256sum "$SAL/marca/MARCA_SUELTA.DAT" | cut -d' ' -f1)"
if [ "$DENTRO" = "$FUERA" ]; then
    verde "la marca suelta esta en la ISO, byte a byte (sha256 ${DENTRO:0:16}, $MARCA_BYTES bytes)"
else
    rojo "la marca suelta NO esta en la ISO (dentro='$DENTRO' fuera='$FUERA')"
fi
# ...y NO esta en el manifiesto del horneador (si estuviera, el criterio 3 no probaria nada)
if grep -qi 'marca_suelta' "$SAL/l245-suelta/MANIFIESTO.TXT" 2>/dev/null; then
    rojo "el MANIFIESTO.TXT nombra la marca suelta: el control del criterio 3 no probaria nada"
else
    verde "el MANIFIESTO.TXT del horneador NO nombra la marca suelta ($(wc -l < "$SAL/l245-suelta/MANIFIESTO.TXT") lineas, solo los 3 MPQ)"
fi
for d in l245-suelta l245-mut; do
    echo "   $d: disc.iso $(stat -c%s "$SAL/$d/disc.iso") bytes sha256 $(sha256sum "$SAL/$d/disc.iso" | cut -c1-16)"
done

# ================================================================================================
# 3. EL POC DEL LISTADO (sin el motor arriba): los criterios 3 y 5, con los numeros a la vista
# ================================================================================================
paso "3. el POC lista-disco sobre el disco con la marca suelta"
if [ "$REFRESCAR" = 1 ] || [ ! -f "$SAL/poc-lista/1ST_READ.BIN" ] \
   || [ "$SAL/poc-lista/1ST_READ.BIN" -ot "$REPO/console/dc/poc/lista-disco/main.go" ] \
   || [ "$SAL/poc-lista/1ST_READ.BIN" -ot "$REPO/console/dc/std/csrc/os_dc.c" ]; then
    ( cd "$REPO" && GODC_GOX="$GEN" bash scripts/console/build-dc.sh console/dc/poc/lista-disco "$SAL/poc-lista" ) \
        > "$SAL/poc-lista.build.log" 2>&1 || { echo "no compilo el POC (ver $SAL/poc-lista.build.log)"; exit 1; }
    ( cd "$REPO" && DC_MARCA_SUELTA="$SAL/marca/MARCA_SUELTA.DAT" \
        bash scripts/console/mkdisco.sh "$SAL/poc-lista" "$SAL/juego3" ) > "$SAL/poc-lista.disco.log" 2>&1
fi

export FLYCAST="${FLYCAST:-$HOME/dreamcast/dl/squashfs-root/AppRun}"
export CAPTURE=none
# el run se rehace si el BINARIO cambio: un log viejo mide otro binario y da un verde (o un rojo) que no es
POC_SHA="$(sha256sum "$SAL/poc-lista/1ST_READ.BIN" | cut -d' ' -f1)"
if [ "$REFRESCAR" = 1 ] || [ ! -s "$SAL/run-poc/flycast.log" ] || [ "$(cat "$SAL/run-poc/bin.sha" 2>/dev/null)" != "$POC_SHA" ]; then
    bash "$HERE/run-flycast.sh" "$SAL/poc-lista/disc.gdi" 25 "$SAL/run-poc" > "$SAL/run-poc.salida" 2>&1
    mkdir -p "$SAL/run-poc"; echo "$POC_SHA" > "$SAL/run-poc/bin.sha"
fi
echo "   run-poc: POC sha256 ${POC_SHA:0:16} ($(stat -c%s "$SAL/poc-lista/1ST_READ.BIN") bytes)"

L="$(linea poc '/cd/juego entradas=')"
case "$L" in
    *"entradas= 4"*) verde "criterio 3: el driver lista 4 entradas en /cd/juego (los 3 MPQ + la marca suelta)" ;;
    *) rojo "criterio 3: esperaba 4 entradas en /cd/juego y salio: $L" ;;
esac
L="$(linea poc 'marca_suelta.dat')"
case "$L" in
    *"bytes= $MARCA_BYTES"*) verde "criterio 3: la marca suelta SALE del listado con sus $MARCA_BYTES bytes: $L" ;;
    *) rojo "criterio 3: la marca suelta no sale (o sale sin sus $MARCA_BYTES bytes): $L" ;;
esac
# y los 3 MPQ con sus tamanos EXACTOS (el driver los trae de de->size, no del manifiesto)
for par in "d2char.mpq:267704443" "d2data.mpq:267330019" "patch_d2.mpq:5022588"; do
    nom="${par%%:*}"; byt="${par##*:}"
    L="$(linea poc "] $nom ")"
    case "$L" in
        *"bytes= $byt"*) verde "el listado trae $nom con sus $byt bytes" ;;
        *) rojo "el listado trae $nom mal: $L" ;;
    esac
done
# el indice NO se lista (es el indice, no una entrada del usuario)
if grep -aq 'manifiesto.txt' "$SAL/run-poc/flycast.log"; then
    rojo "el MANIFIESTO.TXT sale como entrada del listado (es el indice, no una entrada del usuario)"
else
    verde "el MANIFIESTO.TXT no se lista a si mismo"
fi
# y el directorio del disco se lista como DIRECTORIO (el attr del driver es O_DIR)
L="$(linea poc '] juego dir=')"
case "$L" in
    *"dir= true"*) verde "el subdirectorio 'juego' de /cd sale como directorio: $L" ;;
    *) rojo "el subdirectorio 'juego' de /cd no sale como directorio: $L" ;;
esac

# criterio 5: la memoria del listado (el buffer es LOCAL, el monton de C no se mueve)
CF_ANTES="$(linea poc 'antes-de-40-listados' | sed -n 's/.*cfree=\([0-9-]*\).*/\1/p')"
CF_DESPUES="$(linea poc 'despues-de-40-listados' | sed -n 's/.*cfree=\([0-9-]*\).*/\1/p')"
if [ -n "$CF_ANTES" ] && [ "$CF_ANTES" = "$CF_DESPUES" ]; then
    verde "criterio 5: el monton de C NO se mueve en 40 listados (cfree= $CF_ANTES antes y despues)"
else
    rojo "criterio 5: el monton de C cambia con el listado (antes= '$CF_ANTES' despues= '$CF_DESPUES')"
fi
echo "   $(linea poc 'antes-de-40-listados')"
echo "   $(linea poc 'despues-de-40-listados')"

# ================================================================================================
# 4. EL MOTOR ENTERO: el numero del criterio 2 y lo que ya andaba (criterio 4)
# ================================================================================================
paso "4. el motor entero sobre los dos discos ($SEGS s cada uno)"
BIN_SHA="$(sha256sum "$SAL/marranque/1ST_READ.BIN" | cut -d' ' -f1)"
for par in "suelta:l245-suelta" "mut:l245-mut"; do
    et="${par%%:*}"; disco="${par##*:}"
    if [ "$REFRESCAR" = 1 ] || [ ! -s "$SAL/run-$et/flycast.log" ] || [ "$(cat "$SAL/run-$et/bin.sha" 2>/dev/null)" != "$BIN_SHA" ]; then
        bash "$HERE/run-flycast.sh" "$SAL/$disco/disc.gdi" "$SEGS" "$SAL/run-$et" > "$SAL/run-$et.salida" 2>&1
        mkdir -p "$SAL/run-$et"; echo "$BIN_SHA" > "$SAL/run-$et/bin.sha"
    fi
    echo "   run-$et: $(linea "$et" 'MOTOR-ARRANQUE seleccion:')"
done

L="$(linea suelta 'MOTOR-ARRANQUE seleccion:')"
case "$L" in
    *"Complete= true"*"archivos que ve= 3"*"faltan= 4"*)
        verde "criterio 2: el numero del motor es el de DC-002.44: $L" ;;
    *) rojo "criterio 2: esperaba 'Complete= true ... ve= 3 ... faltan= 4' y salio: $L" ;;
esac
L="$(linea mut 'MOTOR-ARRANQUE seleccion:')"
case "$L" in
    *"Complete= false"*"archivos que ve= 0"*"faltan= 7"*)
        verde "criterio 2 (mutante): el disco sin /JUEGO sigue en 0 / faltan= 7: $L" ;;
    *) rojo "criterio 2 (mutante): esperaba 've= 0 ... faltan= 7' y salio: $L" ;;
esac

for et in suelta mut; do
    L="$(linea "$et" 'MOTOR-ARRANQUE seleccion overlay:')"
    case "$L" in
        *"-> 1 entradas: uno.txt"*) verde "criterio 4 ($et): el overlay de la RAM sigue listando: $L" ;;
        *) rojo "criterio 4 ($et): el overlay no listo: $L" ;;
    esac
done

# ================================================================================================
# 5. EL ROMDISK SIGUE LISTANDO POR SU .manifiesto (criterio 4, la otra mitad)
# ================================================================================================
paso "5. el romdisk por su .manifiesto (POC vfs: el driver NO puede listar /rd)"
if [ "$REFRESCAR" = 1 ] || [ ! -f "$SAL/poc-vfs/prog.elf" ] \
   || [ "$SAL/poc-vfs/prog.elf" -ot "$REPO/console/dc/std/os/os.go" ]; then
    ( cd "$REPO" && GODC_GOX="$GEN" bash scripts/console/build-dc.sh console/dc/poc/vfs "$SAL/poc-vfs" ) \
        > "$SAL/poc-vfs.build.log" 2>&1
    VFS_SHA="$(sha256sum "$SAL/poc-vfs/prog.elf" | cut -d' ' -f1)"
else
    VFS_SHA="$(sha256sum "$SAL/poc-vfs/prog.elf" | cut -d' ' -f1)"
fi
if [ "$REFRESCAR" = 1 ] || [ ! -s "$SAL/run-vfs/flycast.log" ] || [ "$(cat "$SAL/run-vfs/bin.sha" 2>/dev/null)" != "$VFS_SHA" ]; then
    bash "$HERE/run-flycast.sh" "$SAL/poc-vfs/prog.elf" 30 "$SAL/run-vfs" > "$SAL/run-vfs.salida" 2>&1
    mkdir -p "$SAL/run-vfs"; echo "$VFS_SHA" > "$SAL/run-vfs/bin.sha"
fi
L="$(linea vfs 'VFS: casos')"
case "$L" in
    *"mal 0"*) verde "criterio 4: el corpus de os en la consola sigue verde (romdisk incluido): $L" ;;
    *) rojo "criterio 4: el corpus de os en la consola se rompio: $L" ;;
esac

paso "VEREDICTO"
echo "   controles OK: $OK, fallos: $FALLOS"
[ "$FALLOS" = "0" ] || exit 1
