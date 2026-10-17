#!/bin/bash
# Compila un programa de Go (libgodc) para Dreamcast y arma la imagen arrancable.
#
# Uso:  scripts/console/build-dc.sh console/dc/poc/hello [salida_dir]
#
# Deja en el directorio de salida:  prog.elf, 1ST_READ.BIN, IP.BIN, disc.iso
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

SRC="${1:?uso: build-dc.sh <dir_con_main.go> [salida_dir]}"
OUT="${2:-/tmp/dcbuild/$(basename "$SRC")}"
mkdir -p "$OUT"

# Un POC con asset.conf (los bytes CRUDOS del MPQ como var de Go, los hornea console/dc/bake-dc6) NO se
# arma con este script: sus assets no salen de un .h y ademas necesita los .gox del MOTOR, que este
# script no compila. Sin este aviso el error que sale es "reference to undefined name 'assetDC6'", que
# manda a buscar el problema donde no esta (medido: DC-002.10 lo diagnostico como "el horneador no
# genera los bytes" cuando el camino correcto ya existia). Ese camino se declara con POC_ASSET_CRUDO=1,
# que es lo que pone build-motor-dc6.sh cuando llama aca en su paso 4/4.
if [ -f "$SRC/asset.conf" ] && [ "${POC_ASSET_CRUDO:-0}" != "1" ]; then
    echo "ERROR: $SRC es un POC de asset CRUDO (trae asset.conf), no de un .h horneado." >&2
    echo "       Se arma con:  scripts/console/build-motor-dc6.sh [dir_salida]" >&2
    echo "                     scripts/console/build-motor-pvr.sh [dir_salida]   (para motor-pvr)" >&2
    echo "       build-dc.sh solo compila y enlaza: los bytes crudos y los .gox del motor los pone ese." >&2
    exit 2
fi

# Assets de Diablo II (si el POC trae bake.conf): se hornean al directorio de build, no al repo.
bash "$HERE/bake-asset.sh" "$SRC" "$OUT"

# Si el POC trae el archivo "genera-casos", se le hornea la tabla de casos de la stdlib: las
# respuestas las calcula la biblioteca estandar real de Go en el host (ver gen-std-casos.sh).
if [ -f "$SRC/genera-casos" ]; then
    bash "$HERE/gen-std-casos.sh" "$OUT"
fi

# romdisk: los archivos que viajan ADENTRO del ELF. Es el sistema de archivos de la consola (KOS lo
# monta en /rd al arrancar con KOS_INIT_ROMDISK, que llama el csrc del POC). Se arma con el genromfs de
# KOS y se convierte a objeto con bin2o. Medido: genromfs deja una imagen de 1024 bytes para un
# archivo chico, y bin2o genera los simbolos romdisk / romdisk_end.
ROMDISK_OBJ=""
if [ -d "$SRC/romdisk" ]; then
    ARRIBA="$OUT/romdisk-arriba"
    rm -rf "$ARRIBA"
    mkdir -p "$ARRIBA"
    cp -r "$SRC/romdisk/." "$ARRIBA/"
    # El .manifiesto: el FS ROM de KOS no deja ABRIR un directorio (medido: fs_open("/rd") da INVALID y
    # fs_readdir no llega), asi que os.ReadDir no puede listar el sistema de archivos. El listado se
    # arma con el MISMO build que hornea el romdisk, asi que no se puede desincronizar de su contenido.
    # Formato: nombre TAB bytes TAB tipo, una linea por entrada.
    : > "$ARRIBA/.manifiesto"
    for f in "$SRC/romdisk"/*; do
        # Un DIRECTORIO se marca "directorio" y su tamano va 0: el tamano de un directorio lo pone el
        # sistema de archivos que lo hospeda (medido: 4096 con el romdisk del POC armado en /tmp), no es
        # un dato del contenido, y os.ReadDir no lo usa. Se declara 0 en vez de inventar un numero.
        if [ -d "$f" ]; then
            printf '%s\t0\tdirectorio\n' "$(basename "$f")" >> "$ARRIBA/.manifiesto"
        else
            printf '%s\t%s\tarchivo\n' "$(basename "$f")" "$(stat -c%s "$f")" >> "$ARRIBA/.manifiesto"
        fi
    done
    "$KOS_BASE/utils/genromfs/genromfs" -f "$OUT/romdisk.img" -d "$ARRIBA" -V d2romdisk >/dev/null
    "$KOS_BASE/utils/bin2o/bin2o" "$OUT/romdisk.img" romdisk "$OUT/romdisk.o"
    ROMDISK_OBJ="$OUT/romdisk.o"
    echo "   romdisk: $(stat -c%s "$OUT/romdisk.img") bytes, $(ls "$SRC/romdisk" | wc -l) archivos + manifiesto -> /rd"
fi

echo "== 1/4 compilando Go ($SRC) =="
# si existe el arbol .gox de la stdlib minima (scripts/console/build-std-gox.sh), se usa para
# resolver "fmt", "strings", etc. en la consola
GOX="${GODC_GOX:-$DC_ROOT/gox}"
GOXI=""
if [ -d "$GOX" ]; then
    GOXI="-I $GOX"
fi
# Todo lo que haya en gen/ es generado en el build (la tabla de casos de la stdlib, los bytes de un
# asset horneado): se compila junto con el POC. Se listan UNA sola vez: antes casos_gen.go se pasaba
# ademas por separado y gccgo lo veia dos veces (redefinition).
GEN_GO=""
for g in "$OUT"/gen/*.go; do
    if [ -f "$g" ]; then GEN_GO="$GEN_GO $g"; fi
done
sh-elf-gccgo $GODC_GOFLAGS $GOXI -c "$SRC"/*.go $GEN_GO -o "$OUT/main.o"

# Puentes a C (para funciones de KOS que el binding de Go no expone). Pueden vivir en
# $SRC/*.c o en $SRC/csrc/*.c. Se enlazan aparte: gccgo los llama con el comentario //extern.
# El C de la STDLIB (console/dc/std/csrc/*.c): los shims que no pueden ser Go (p.ej. los atomicos
# del SH-4, que necesitan irq_disable/irq_restore de KOS). Se compilan siempre: son pocas funciones y
# el enlace solo se queda con las que se usan.
# OJO: DC_ROOT es el arbol del toolchain (~/dreamcast), no el repo. La raiz del repo es $HERE/../..
STD_C_OBJS=""
for f in "$HERE/../../console/dc/std/csrc"/*.c; do
  [ -f "$f" ] || continue
  o="$OUT/stdlib-$(basename "${f%.c}").o"
  # -I del zlib de KOS: el puente de compress/zlib lo necesita (kos-ports/zlib)
  ZLIB_INC="${ZLIB_INC:-$DC_ROOT/kos-ports/zlib/inst/include}"
  kos-cc -O2 -ml -m4-single -I"$ZLIB_INC" -c "$f" -o "$o"
  STD_C_OBJS="$STD_C_OBJS $o"
done

EXTRA_OBJS=""
for f in "$SRC"/*.c "$SRC"/csrc/*.c; do
  [ -f "$f" ] || continue
  o="$OUT/$(basename "${f%.c}").o"
  kos-cc -O2 -ml -m4-single -I"$SRC" -I"$SRC/csrc" -I"$OUT/gen" -c "$f" -o "$o"
  EXTRA_OBJS="$EXTRA_OBJS $o"
done

echo "== 2/4 enlazando ELF =="
# la stdlib minima: sus .gox son objetos de verdad y van al enlace. Solo se enlazan si el POC los
# usa (si no, engordarian el binario al pedo).
STD_OBJS=""
if [ -f "$GOX/orden.txt" ]; then
    # se enlazan los .gox que el POC importe de verdad (los nombres salen del propio arbol)
    IMPORTS="$(sed "s|^$GOX/||; s|\.gox$||" "$GOX/orden.txt" | tr '\n' '|' | sed 's/|$//')"
    if grep -qE "\"($IMPORTS)\"" "$SRC"/*.go 2>/dev/null; then
        STD_OBJS="$(tr '\n' ' ' < "$GOX/orden.txt")"
    fi
fi
kos-cc -o "$OUT/prog.elf" "$OUT/main.o" $ROMDISK_OBJ $EXTRA_OBJS $STD_C_OBJS $STD_OBJS $DC_LDPATHS $DC_LIBS_PRE $DC_LIBS_POST

echo "== 3/4 binario de arranque (1ST_READ.BIN) + IP.BIN =="
sh-elf-objcopy -R .stack -O binary "$OUT/prog.elf" "$OUT/1ST_READ.BIN"
"$DC_MAKEIP" -f "$OUT/IP.BIN" >/dev/null
# el IP.BIN generado debe apuntar al binario de arranque por defecto (1ST_READ.BIN)

echo "== 4/4 imagen =="
"$DC_PYTHON" "$HERE/mkiso.py" "$OUT/disc.iso" "$OUT/IP.BIN" "$OUT/1ST_READ.BIN"
# El .cue (CD-R) hace que el emulador descramble el binario: para arrancar de verdad, GDI.
"$DC_PYTHON" "$HERE/mkgdi.py" "$OUT"

echo
echo "ELF:        $OUT/prog.elf   ($(stat -c%s "$OUT/prog.elf") bytes)"
echo "1ST_READ:   $OUT/1ST_READ.BIN   ($(stat -c%s "$OUT/1ST_READ.BIN") bytes)"
echo "imagen:     $OUT/disc.iso"
