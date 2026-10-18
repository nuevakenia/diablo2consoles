#!/bin/bash
# DC-002.32: arma la evidencia del peso (capas, enlaces, imagenes planas y corridas).
# Idempotente: lo que ya esta en disco no se rehace. Con --refrescar rehace las capas desde cero.
#
# Los tres binarios y las cuatro corridas:
#   base      el motor de HEAD tal cual (con la tabla adentro)          -> la linea base
#   variante  el motor con `objectLookups` vaciada (la tabla afuera)    -> la palanca 1
#   ehfin     la base reenlazada con el .eh_frame AL FINAL del mapa     -> la palanca 2 (imagen plana)
#   panic     un panic de verdad (con pila de 4 niveles)                -> la palanca 2 no cuesta traza
set -uo pipefail

AQUI="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="${DC232_REPO:-$(cd "$AQUI/../.." && pwd)}"
SRC="${DC232_SRC:-$REPO}"

GOX_BASE="${DC232_GOX_BASE:-/tmp/gox-232p}"
GOX_VAR="${DC232_GOX_VAR:-/tmp/gox-232v}"
OUT="${DC232_OUT:-/tmp/dcbuild}"
BASE="$OUT/dc232-base"; VAR="$OUT/dc232-var"; PANIC="$OUT/dc232-panic"
POC="${DC232_POC:-sonda-peso-232}"
REFRESCAR=0
[ "${1:-}" = "--refrescar" ] && REFRESCAR=1

. "$SRC/scripts/console/env.sh"
lsalida() { printf '\n== %s\n' "$*"; }

# --- la copia de fuentes propia (el repo lo comparte otro agente: DC-002.33) ---
if [ "$REFRESCAR" = 1 ] || [ ! -d "$SRC/d2core" ]; then
    lsalida "copia de fuentes: $REPO -> $SRC"
    mkdir -p "$SRC"
    rsync -a --exclude '.git' "$REPO/" "$SRC/"
fi

# --- el script del linker con el .eh_frame al final ---
python3 "$AQUI/gen-ldscript-eh-232.py" "$OUT/dc232-ld/shlelf-eh-final.xc"

# --- las capas de paquetes ---
if [ "$REFRESCAR" = 1 ] || [ ! -f "$GOX_BASE/orden-motor.txt" ]; then
    lsalida "capa base: $GOX_BASE"
    ( cd "$SRC" && GODC_GOX="$GOX_BASE" bash scripts/console/build-std-gox.sh \
        && GODC_GOX="$GOX_BASE" bash scripts/console/build-motor-all.sh "$GOX_BASE" "$OUT/dc232-capa-base.log" ) || exit 1
fi

# --- el binario de la base (y su corrida) ---
if [ "$REFRESCAR" = 1 ] || [ ! -f "$BASE/prog.elf" ]; then
    lsalida "binario de la base"
    ( cd "$SRC" && POC="$POC" bash scripts/console/build-motor-arranque.sh "$BASE" "$GOX_BASE" ) || exit 1
fi
if [ "$REFRESCAR" = 1 ] || [ ! -f "$OUT/dc232-base-run/flycast.log" ]; then
    lsalida "corrida de la base"
    CAPTURE=none bash "$AQUI/run-flycast.sh" "$BASE/prog-emu.elf" 30 "$OUT/dc232-base-run" >/dev/null 2>&1
fi

# --- LA PALANCA 1: la tabla afuera (y la fuente de vuelta adentro, siempre) ---
if [ "$REFRESCAR" = 1 ] || [ ! -f "$VAR/prog.elf" ]; then
    lsalida "variante: la tabla fuera del binario"
    python3 "$AQUI/fix-d2records-tabla-232.py" --fuera || exit 1
    ( cd "$SRC" && GODC_GOX="$GOX_VAR" bash scripts/console/build-std-gox.sh \
        && GODC_GOX="$GOX_VAR" bash scripts/console/build-motor-all.sh "$GOX_VAR" "$OUT/dc232-capa-var.log" ) || {
            python3 "$AQUI/fix-d2records-tabla-232.py" --dentro; exit 1; }
    ( cd "$SRC" && POC="$POC" bash scripts/console/build-motor-arranque.sh "$VAR" "$GOX_VAR" ) || exit 1
    python3 "$AQUI/fix-d2records-tabla-232.py" --dentro || exit 1
fi
if [ "$REFRESCAR" = 1 ] || [ ! -f "$OUT/dc232-var-run/flycast.log" ]; then
    lsalida "corrida de la variante"
    CAPTURE=none bash "$AQUI/run-flycast.sh" "$VAR/prog-emu.elf" 30 "$OUT/dc232-var-run" >/dev/null 2>&1
fi

# --- LA PALANCA 2: el .eh_frame al final, y sus dos imagenes planas ---
lsalida "el .eh_frame al final del mapa"
reenlazar() {   # $1 build, $2 gox, $3 salida, $4 script.xc
    local LDF="${KOS_LDFLAGS//-T$KOS_BASE\/utils\/ldscripts\/shlelf.xc/-T$4}"
    ( cd "$SRC" && KOS_LDFLAGS="$LDF" kos-cc -o "$3" "$1"/*.o \
        $(tr '\n' ' ' < "$1/orden-std.txt") $(tr '\n' ' ' < "$2/orden-motor.txt") \
        $DC_LDPATHS $DC_LIBS_PRE $DC_LIBS_POST ) >/dev/null 2>&1 || { echo "fallo el reenlace" >&2; exit 1; }
}
XC="$OUT/dc232-ld/shlelf-eh-final.xc"
[ "$REFRESCAR" = 1 ] || [ ! -f "$OUT/232-ehfin.elf" ] && reenlazar "$BASE" "$GOX_BASE" "$OUT/232-ehfin.elf" "$XC"
sh-elf-objcopy -R .stack -O binary "$OUT/232-ehfin.elf" "$OUT/232-ehfin-completa.bin"
sh-elf-objcopy -R .stack -R .eh_frame -O binary "$OUT/232-ehfin.elf" "$OUT/232-ehfin-sin.bin"
# y el control del hueco: el recorte sobre el enlace de KOS (no baja nada)
sh-elf-objcopy -R .stack -R .eh_frame -O binary "$BASE/prog.elf" "$OUT/232-kos-sin.bin"

# --- el panic de verdad, por el camino del disco (donde el .eh_frame de verdad falta) ---
if [ "$REFRESCAR" = 1 ] || [ ! -f "$OUT/232-panic-disco/run-sin/flycast.log" ]; then
    lsalida "el panic con y sin .eh_frame"
    if [ ! -f "$PANIC/prog.elf" ]; then
        ( cd "$SRC" && POC=sonda-panic-232 bash scripts/console/build-motor-arranque.sh "$PANIC" "$GOX_BASE" ) || exit 1
    fi
    DISCO="$OUT/232-panic-disco"
    mkdir -p "$DISCO/con-eh" "$DISCO/sin-eh"
    reenlazar "$PANIC" "$GOX_BASE" "$DISCO/con-eh/prog.elf" "$XC"
    sh-elf-objcopy -R .stack -O binary "$DISCO/con-eh/prog.elf" "$DISCO/con-eh/1ST_READ.BIN"
    sh-elf-objcopy -R .stack -R .eh_frame -O binary "$DISCO/con-eh/prog.elf" "$DISCO/sin-eh/1ST_READ.BIN"
    for d in con-eh sin-eh; do
        cp "$PANIC/IP.BIN" "$DISCO/$d/IP.BIN"
        python3 "$AQUI/mkiso.py" "$DISCO/$d/disc.iso" "$DISCO/$d/IP.BIN" "$DISCO/$d/1ST_READ.BIN" >/dev/null
        python3 "$AQUI/mkgdi.py" "$DISCO/$d" >/dev/null
    done
    CAPTURE=none bash "$AQUI/run-flycast.sh" "$PANIC/prog-emu.elf" 12 "$DISCO/run-elf" >/dev/null 2>&1
    CAPTURE=none bash "$AQUI/run-flycast.sh" "$DISCO/con-eh/disc.gdi" 25 "$DISCO/run-con" >/dev/null 2>&1
    CAPTURE=none bash "$AQUI/run-flycast.sh" "$DISCO/sin-eh/disc.gdi" 25 "$DISCO/run-sin" >/dev/null 2>&1
fi

lsalida "evidencia lista en $OUT"
