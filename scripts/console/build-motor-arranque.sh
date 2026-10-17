#!/bin/bash
# DC-002.13: enlaza EL MOTOR ENTERO y arranca su entry point de verdad en la consola.
#
# Por que un script propio y no build-dc.sh: build-dc.sh enlaza la stdlib minima y los .gox de los paquetes
# que el POC importe. Este enlaza los 142 paquetes del MOTOR (los que compila build-motor-all.sh, listados
# en orden-motor.txt) MAS la stdlib minima y los terceros (orden.txt): los 205 .gox. Es el unico binario del
# repo que lleva el motor entero adentro, y por eso tambien el que mide CUANTO PESA el motor.
#
# Uso:  scripts/console/build-motor-arranque.sh [dir_salida] [dir_gox]
#       D2 de Diablo II no hace falta: este arranque no hornea assets, justamente porque el motor los busca
#       en disco y ese es el punto que se mide.
#
# Antes:  bash scripts/console/build-std-gox.sh        (la stdlib minima + terceros)
#         bash scripts/console/build-motor-all.sh      (los 142 del motor -> orden-motor.txt)
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
POC="${POC:-motor-arranque}"
SRC="$REPO/console/dc/poc/$POC"
OUT="${1:-/tmp/dcbuild/$POC}"
GEN="${2:-${GODC_GOX:-$DC_ROOT/gox}}"

[ -f "$GEN/orden.txt" ] || {
    echo "falta $GEN/orden.txt (la stdlib minima): corre scripts/console/build-std-gox.sh" >&2
    exit 1
}
[ -f "$GEN/orden-motor.txt" ] || {
    echo "falta $GEN/orden-motor.txt (el motor): corre scripts/console/build-motor-all.sh '$GEN'" >&2
    exit 1
}
[ -f "$SRC/main.go" ] || { echo "no encuentro $SRC/main.go" >&2; exit 1; }

mkdir -p "$OUT"

# ================================================================================================
# DC-002.19: EL ROMDISK DEL ARRANQUE -- el config del motor con los DATOS DEL JUEGO y el directorio
# donde el arranque los busca (/rd/juego), los dos dentro del binario.
#
# POR QUE: el arranque se frenaba en `d2setup.EnsureGameFiles` con "no Diablo II game files were selected",
# porque en la consola no hay quien elija la carpeta (los dos caminos del motor que no preguntan salen de
# `os.Getenv`, que en la consola es un stub, y sin env queda `OsaUI`, que es osascript). El unico camino
# que no pregunta es que la CONFIGURACION ya este completa: `Complete(MpqPath, MpqLoadOrder)` = true.
#
# La configuracion la escribe el MOTOR (`d2config.Save`, el `MarshalJSON` de DC-002.18) con la tabla de
# archivos del motor (`d2setup.CoreMPQs`), y los archivos se crean VACIOS: son el instrumento que corre la
# frontera hacia adelante (los MPQ de verdad no entran en los 16 MB del romdisk y el camino de lectura es
# el disco, tarjeta siguiente). Eso lo dice el generador en su salida, no se esconde.
#
# NADA DE ESTO ENTRA AL REPO: todo se arma en el directorio de build (descartable).
#
# MUTANTES (los usa scripts/console/medir-seleccion-219.sh y el corpus):
#   DC_SIN_ROMDISK=1  no se hornea ningun romdisk -> /rd no existe: el estado del arranque ANTES del arreglo
#   DC_SIN_CONFIG=1   romdisk con el directorio de datos pero SIN config -> el motor arma su default
#   DC_SIN_SELLOS=1   config escrita pero SIN los archivos del juego -> la seleccion se frena otra vez
ROMSRC="$OUT/romdisk"
ROMDISK_OBJ=""

if [ "${DC_SIN_ROMDISK:-0}" != "1" ]; then
    rm -rf "$ROMSRC"
    mkdir -p "$ROMSRC/OpenDiablo2" "$ROMSRC/juego"

    if [ "${DC_SIN_CONFIG:-0}" != "1" ]; then
        echo "== 1/5 el config del arranque (lo escribe el motor, con los datos del juego puestos) =="
        # DC-002.27: los dos parametros del DISCO, apagados por defecto (sin las tres variables, la linea
        # de siempre: la ruta del romdisk, los 7 CoreMPQs y los sellos vacios de DC-002.19, byte por byte).
        #   DC_RUTA_DATOS=/cd/juego  -> el arranque busca los datos en el disco (GD-ROM, ISO9660)
        #   DC_MPQ_JUEGO=a,b,c       -> los MPQ de la config, de la tabla de d2setup (el orden lo pone el motor)
        #   DC_SIN_SELLOS=1          -> no crea los archivos vacios (con los MPQ de verdad en el disco no hay
        #                               nada que sellar: el sello era el instrumento cuando no habia disco)
        #   DC_DISCO_JUEGO=<dir>     -> ademas hornea esos MPQ en la ISO, en /JUEGO (paso 5b)
        ARGS_CONFIG=(-datos "$ROMSRC/juego" -config "$ROMSRC/OpenDiablo2/config.json"
            -ruta-consola "${DC_RUTA_DATOS:-/rd/juego}")
        [ -n "${DC_MPQ_JUEGO:-}" ] && ARGS_CONFIG+=(-mpq "$DC_MPQ_JUEGO")
        [ "${DC_SIN_SELLOS:-0}" = "1" ] && ARGS_CONFIG+=(-sin-sellos)
        if ! (cd "$REPO" && go run ./console/dc/gen-config-consola "${ARGS_CONFIG[@]}") > "$OUT/config-consola.log" 2>&1; then
            echo "   EL GENERADOR DE LA CONFIGURACION FALLO (ver $OUT/config-consola.log)" >&2
            tail -5 "$OUT/config-consola.log" >&2
            exit 1
        fi
        sed 's/^/   /' "$OUT/config-consola.log"
    else
        echo "== 1/5 SIN config del arranque (mutante DC_SIN_CONFIG=1) =="
    fi

    echo "== y los manifiestos del romdisk (uno por directorio: el FS ROM de KOS no deja listar) =="
    "$DC_PYTHON" "$HERE/manifiesto-romdisk.py" "$ROMSRC" | sed 's/^/   /'
    "$KOS_BASE/utils/genromfs/genromfs" -f "$OUT/romdisk.img" -d "$ROMSRC" -V d2romdisk >/dev/null
    "$KOS_BASE/utils/bin2o/bin2o" "$OUT/romdisk.img" romdisk "$OUT/romdisk.o"
    ROMDISK_OBJ="$OUT/romdisk.o"
    echo "   romdisk: $(stat -c%s "$OUT/romdisk.img") bytes -> /rd"
else
    echo "== 1/5 SIN romdisk (mutante DC_SIN_ROMDISK=1) =="
fi

N_MOTOR="$(wc -l < "$GEN/orden-motor.txt")"
N_STD="$(wc -l < "$GEN/orden.txt")"

echo "== 2/5 compilando el arranque (POC $POC) =="
sh-elf-gccgo $GODC_GOFLAGS -I "$GEN" -c "$SRC"/*.go -o "$OUT/main.o"

echo "== 3/5 el C: los puentes de la stdlib minima + los del POC =="
STD_C_OBJS=""
for f in "$REPO/console/dc/std/csrc"/*.c; do
    [ -f "$f" ] || continue
    o="$OUT/stdlib-$(basename "${f%.c}").o"
    ZLIB_INC="${ZLIB_INC:-$DC_ROOT/kos-ports/zlib/inst/include}"
    kos-cc -O2 -ml -m4-single -I"$ZLIB_INC" -c "$f" -o "$o"
    STD_C_OBJS="$STD_C_OBJS $o"
done

EXTRA_OBJS=""
for f in "$SRC"/*.c "$SRC"/csrc/*.c; do
    [ -f "$f" ] || continue
    o="$OUT/$(basename "${f%.c}").o"
    kos-cc -O2 -ml -m4-single -I"$SRC" -I"$SRC/csrc" -c "$f" -o "$o"
    EXTRA_OBJS="$EXTRA_OBJS $o"
done

echo "== 4/5 enlazando el MOTOR ENTERO ($N_MOTOR paquetes del motor + $N_STD de stdlib/terceros) =="
# DC-002.17: orden.txt puede traer los paquetes del MOTOR adentro si antes se corrio build-motor-gox.sh
# (ese los anota a proposito: los POC del motor se enlazan con un solo -I y esa lista). Como el enlace de
# aca suma orden.txt + orden-motor.txt, el mismo .gox entraba DOS veces y el linker moria con
#   multiple definition of `..._d2fileformats_1d2dat.Load..f'
# apuntando a d2dat.gox como "first defined here". Medido: orden.txt con 74 lineas (63 de la stdlib + 11
# del motor) y el enlace del arranque sin poder salir. Se filtra orden.txt contra orden-motor.txt: mismo
# orden, y cuando orden.txt esta limpio es un no-op (el binario sale byte por byte igual).
ORDEN_STD="$OUT/orden-std.txt"
grep -vxF -f "$GEN/orden-motor.txt" "$GEN/orden.txt" > "$ORDEN_STD" || true
N_REPETIDOS="$((N_STD - $(wc -l < "$ORDEN_STD")))"
[ "$N_REPETIDOS" -eq 0 ] || echo "   (orden.txt traia $N_REPETIDOS paquetes del motor ya listados: filtrados)"
# DC-002.14: DC_LDEXTRA existe para medir el RECORTE del binario (-Wl,--gc-sections, que necesita los .gox
# compilados con -ffunction-sections -fdata-sections) sin duplicar este script. Vacio por defecto: el
# binario de siempre (el que mide la tarjeta DC-002.13) sale con la MISMA linea de enlace que antes.
# shellcheck disable=SC2046,SC2086
kos-cc -o "$OUT/prog.elf" "$OUT/main.o" $EXTRA_OBJS $STD_C_OBJS $ROMDISK_OBJ \
    $(tr '\n' ' ' < "$ORDEN_STD") $(tr '\n' ' ' < "$GEN/orden-motor.txt") \
    ${DC_LDEXTRA:-} $DC_LDPATHS $DC_LIBS_PRE $DC_LIBS_POST

echo "== 5/5 binario de arranque (1ST_READ.BIN) + IP.BIN + imagen =="
sh-elf-objcopy -R .stack -O binary "$OUT/prog.elf" "$OUT/1ST_READ.BIN"
"$DC_MAKEIP" -f "$OUT/IP.BIN" >/dev/null
"$DC_PYTHON" "$HERE/mkiso.py" "$OUT/disc.iso" "$OUT/IP.BIN" "$OUT/1ST_READ.BIN"
"$DC_PYTHON" "$HERE/mkgdi.py" "$OUT"

# DC-002.27: la imagen del disco CON los assets del juego adentro (/JUEGO). Va despues del ISO del paso
# 5/5 a proposito: la imagen se rearma con los MPQ, y el ISO del paso 5 (el binario solo) queda como el
# control negativo de la tarjeta -- el mismo POC, con y sin la carpeta del juego.
if [ -n "${DC_DISCO_JUEGO:-}" ]; then
    echo "== 5b/5 la imagen del disco con los MPQ del juego (DC-002.27) =="
    bash "$HERE/mkdisco.sh" "$OUT" "$DC_DISCO_JUEGO" | sed 's/^/   /'
fi

# prog-emu.elf: el MISMO binario sin el debug ni el .go_export, para correrlo en Flycast.
#
# POR QUE (medido, no supuesto): el cargador de ELF de Flycast RECHAZA cualquier archivo de mas de 16 MB
# -- `if (size == 0 || size > 16_MB) return false;` en core/reios/reios_elf.cpp:18 -- y lo unico que
# contesta es "Flycast has stopped: Failed to open ELF". Con 205 .gox adentro el ELF sale de 43.876.988
# bytes (el debug son 14 de esos y el .go_export otros 11,9, ninguno de los dos se carga en la consola).
# Sacandolos queda en 13.707.552 bytes y el emulador lo carga. El que se carga de verdad son las
# secciones ALLOC: eso lo informa el `sh-elf-size -A` de abajo.
sh-elf-objcopy -R .go_export -R .comment --strip-debug "$OUT/prog.elf" "$OUT/prog-emu.elf"
sh-elf-strip -s "$OUT/prog-emu.elf"

echo
echo "ELF:        $OUT/prog.elf   ($(stat -c%s "$OUT/prog.elf") bytes, con debug: para el GDB y el 1ST_READ)"
echo "ELF (emu):  $OUT/prog-emu.elf   ($(stat -c%s "$OUT/prog-emu.elf") bytes: el que abre Flycast)"
echo "1ST_READ:   $OUT/1ST_READ.BIN   ($(stat -c%s "$OUT/1ST_READ.BIN") bytes)"
echo "secciones que SE CARGAN en los 16 MB:"
sh-elf-size -A "$OUT/prog.elf" | sed 's/^/   /'
