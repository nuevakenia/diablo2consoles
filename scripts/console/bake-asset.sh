#!/bin/bash
# Hornea los assets de un POC desde los MPQ de Diablo II, EN TIEMPO DE BUILD.
#
# Por que asi: los datos del juego no pueden vivir en el repo (scripts/check_repo_hygiene.sh
# prohibe .dc6/.dt1/.dcc/.ds1/... y archivos grandes). El .h horneado queda en el directorio
# de build, que es descartable.
#
# Uso:  scripts/console/bake-asset.sh <dir_del_poc> <dir_de_salida>
# Lee <dir_del_poc>/bake.conf (lineas clave=valor):
#   mpq=d2data.mpq            archivo del MPQ, relativo a la carpeta de Diablo II
#   dc6=...\carpeta\x.dc6     ruta del asset dentro del MPQ (barras invertidas)
#   pal=...\pal.pl2           paleta (PL2) dentro del MPQ
#   frame=0                   frame del DC6
#   name=d2logo               nombre base de los archivos generados
#
# Salida: <dir_de_salida>/gen/<name>.h y <name>.png (referencia del host)
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)"

SRC="${1:?uso: bake-asset.sh <dir_del_poc> <dir_de_salida>}"
OUT="${2:?falta el directorio de salida}"
CONF="$SRC/bake.conf"

if [ ! -f "$CONF" ]; then
    echo "== sin bake.conf en $SRC: no hay assets que hornear =="
    exit 0
fi

get() { sed -n "s/^$1=//p" "$CONF" | head -1; }

MPQNAME="$(get mpq)"
DC6="$(get dc6)"
PAL="$(get pal)"
FRAME="$(get frame)"
NAME="$(get name)"

: "${MPQNAME:?falta mpq= en bake.conf}"
: "${DC6:?falta dc6= en bake.conf}"
: "${NAME:=asset}"
: "${FRAME:=0}"

# --- carpeta de Diablo II: $D2_DIR, o los caminos tipicos de Wine ---
D2DIR="${D2_DIR:-}"
if [ -z "$D2DIR" ]; then
    for c in "$HOME/.wine-d2/drive_c/Games/Diablo II" \
             "$HOME/.wine/drive_c/Games/Diablo II" \
             "$HOME/Games/Diablo II"; do
        if [ -f "$c/$MPQNAME" ]; then D2DIR="$c"; break; fi
    done
fi
if [ -z "$D2DIR" ] || [ ! -f "$D2DIR/$MPQNAME" ]; then
    echo "ERROR: no encuentro $MPQNAME. Pasa la carpeta: D2_DIR='/ruta/Diablo II' $0 $*" >&2
    exit 1
fi

GEN="$OUT/gen"
mkdir -p "$GEN"

if [ "$FRAME" = compuesto ]; then
  echo "== hornendo asset: $DC6 (TODOS los frames compuestos) desde $MPQNAME =="
else
  echo "== hornendo asset: $DC6 (frame $FRAME) desde $MPQNAME =="
fi
cd "$REPO"
# el horneador es una herramienta del host: se compila con el Go normal, no con el cruzado
go build -o "$OUT/.bake" ./console/dc/bake
# frame=compuesto: los DC6 de la interfaz son celdas sueltas que hay que pegar con sus offsets; el
# horneador las compone en un lienzo y ESO es lo que se sube como textura. Con un frame suelto sale
# una pieza (el frame 0 del fondo del menu, por ejemplo, es solo la "D" del logo).
if [ "$FRAME" = compuesto ]; then
  "$OUT/.bake" -mpq "$D2DIR/$MPQNAME" -dc6 "$DC6" -pal "$PAL" -compuesto -out "$GEN" -name "$NAME"
else
  "$OUT/.bake" -mpq "$D2DIR/$MPQNAME" -dc6 "$DC6" -pal "$PAL" -frame "$FRAME" -out "$GEN" -name "$NAME"
fi
