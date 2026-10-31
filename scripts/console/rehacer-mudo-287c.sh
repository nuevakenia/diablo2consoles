#!/bin/bash
# DC-002.87c -- rearmar un build MUDO de antes con la sonda ARREGLADA (SCIF a mano) y correrlo con capturas.
#
# Uso: rehacer.sh <etiqueta> <dir_src_original> <gox_enlace> [segundos]
#   <dir_src_original>  el src/ del intento viejo (/tmp/t8coop/H/src): el codigo EXACTO que moria
#   <gox_enlace>        el arbol de enlace de ese mismo intento (sus .gox del motor NO se rehacen)
# Deja en /tmp/dc287c/<etiqueta>: build.log, prog.elf, 1ST_READ.BIN, disc.gdi, run/flycast.log,
# run/fN.png (capturas a 3,6,10,15,25,40,60 s) y resumen.txt.
set -uo pipefail
X="${1:?etiqueta}"; SRC0="${2:?src original}"; GOX="${3:?gox del intento}"; SECS="${4:-70}"
REPO=/home/dgalli/Proyectos/diablo2consoles
OUT=/tmp/dc287c/$X
PY="$HOME/dreamcast/venv/bin/python"; [ -x "$PY" ] || PY=python3
D2DIR="${D2_DIR:-$HOME/.wine-d2/drive_c/Games/Diablo II}"

echo "== 1/4 compilando el src ORIGINAL contra el vendor con la sonda arreglada (/tmp/dc287/dcroot)"
mkdir -p "$OUT"; rm -rf "$OUT/src"; cp -r "$SRC0" "$OUT/src"
rm -f "$OUT/prog.elf" "$OUT/1ST_READ.BIN" "$OUT/disc.iso" "$OUT"/track0*.iso "$OUT/track02.raw" "$OUT/disc.gdi"
DC_ROOT=/tmp/dc287/dcroot GODC_GOX="$GOX" bash "$REPO/scripts/console/build-dc.sh" "$OUT/src" "$OUT" \
    > "$OUT/build.log" 2>&1 || { echo "   FALLO el build"; tail -20 "$OUT/build.log"; exit 1; }
printf '   1ST_READ.BIN %s B   prog.elf %s B\n' "$(stat -c%s "$OUT/1ST_READ.BIN")" "$(stat -c%s "$OUT/prog.elf")"
# grep -c (no -q): con pipefail, -q corta `strings` con SIGPIPE y el pipeline "falla" aunque haya match
[ "$(strings "$OUT/prog.elf" | grep -c 'chars=%')" -ge 1 ] || { echo "   FALLO: el binario no trae la sonda arreglada"; exit 1; }

echo "== 2/4 la imagen con los DOS MPQ en /JUEGO"
LISTA="$OUT/iso-lista.txt"
printf '%s:/JUEGO/D2DATA.MPQ;1\n%s:/JUEGO/D2CHAR.MPQ;1\n' \
    "$D2DIR/$(ls "$D2DIR" | grep -ix 'd2data.mpq' | head -1)" \
    "$D2DIR/$(ls "$D2DIR" | grep -ix 'd2char.mpq' | head -1)" > "$LISTA"
MKISO_JOLIET=0 "$PY" "$REPO/scripts/console/mkiso.py" "$OUT/disc.iso" "$OUT/IP.BIN" \
    "$OUT/1ST_READ.BIN" "@$LISTA" | tail -1
"$PY" "$REPO/scripts/console/mkgdi.py" "$OUT" | tail -1

if [ "$SECS" = "0" ]; then exit 0; fi
echo "== 3/4 la corrida (${SECS}s, con capturas)"
CAPTURE=spectacle bash "$REPO/scripts/console/run-flycast.sh" "$OUT/disc.gdi" "$SECS" "$OUT/run" \
    > "$OUT/run-script.log" 2>&1
L="$OUT/run/flycast.log"; [ -f "$L" ] || { echo "   no hay $L"; exit 1; }
T="$OUT/resumen.txt"; tr -d '\r' < "$L" > "$T"

echo "== 4/4 lo que dijeron los DOS testigos"
echo "--- pantalla: md5 de cada captura"
for f in "$OUT"/run/f*.png; do [ -f "$f" ] && printf '   %-8s %s\n' "$(basename "$f")" "$(md5sum "$f" | cut -c1-16)"; done
echo "--- log"
echo "   lineas=$(wc -l < "$T")  REIOS-boot=$(grep -ac 'REIOS: Booting up' "$T")  COOP=$(grep -ac 'COOP276' "$T")"
echo "   CUADRO:    $(grep -a 'COOP276: CUADRO' "$T" | tail -1)"
echo "   ultimo t:  $(grep -a 'SONDA-PC t=' "$T" | tail -1)"
echo "   FUERA=$(grep -ac 'SONDA-PC FUERA' "$T")  RACHA=$(grep -ac 'SONDA-PC RACHA' "$T")"
echo "   SONDA-REG FALLO=$(grep -ac 'SONDA-REG FALLO' "$T")"
echo "   ultima COOP: $(grep -a 'COOP276' "$T" | tail -1)"
{ echo "## $X"; echo "--- pantalla"; for f in "$OUT"/run/f*.png; do [ -f "$f" ] && printf '%s %s\n' "$(basename "$f")" "$(md5sum "$f" | cut -c1-16)"; done;
  echo "--- log"; grep -a 'COOP276: CUADRO' "$T" | tail -1; grep -a 'SONDA-PC t=' "$T" | tail -1;
  echo "FUERA=$(grep -ac 'SONDA-PC FUERA' "$T") RACHA=$(grep -ac 'SONDA-PC RACHA' "$T")";
  grep -a 'SONDA-REG' "$T" | head -40; } >> "$OUT/resumen-largo.txt"
