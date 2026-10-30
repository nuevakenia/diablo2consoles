#!/bin/bash
# DC-002.69 -- compilar y enlazar la PRUEBA DE HUMO del backend de render de la consola (d2render/pvr).
#
# QUE HACE, y por que no alcanza build-dc.sh solo: build-dc.sh compila un POC contra los .gox de la STDLIB
# (orden.txt). d2render/pvr no es stdlib: es un paquete del MOTOR, y sus .gox los produce
# build-motor-all.sh. Asi que este script arma, igual que build-enlace-267.sh, un arbol de ENLACE con
# enlaces simbolicos a la gox del motor y una orden.txt = stdlib + SOLO los paquetes del motor que estan en
# el cierre de d2render/pvr. El cierre NO se escribe a mano: sale de `go list -deps` (medido: 6 paquetes).
#
# Uso:  scripts/console/build-pvr-surface.sh [dir_salida]
#       BASE_GOX=...   arbol de .gox con el motor compilado (por defecto /tmp/gox-std)
#       SALTAR_GEN=1   reusar el arbol de enlace ya armado
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
SRC="$REPO/console/dc/poc/pvr-surface"
BASE_GOX="${BASE_GOX:-/tmp/gox-std}"
OUT="${1:-/tmp/dcbuild/pvr-surface}"
GEN="$OUT/gox-enlace"
PREFIJO="github.com/OpenDiablo2/OpenDiablo2"

# el cierre del paquete, con `go list -deps` y el tag dreamcast (el paquete tiene //go:build dreamcast, asi
# que sin el tag go list no lo ve; con -e los paquetes que el host no puede resolver --kos-- no cortan la
# lista).
PAQUETES="$(cd "$REPO" && go list -deps -e -tags dreamcast ./d2core/d2render/pvr 2>/dev/null |
    grep "$PREFIJO" | sed "s|.*$PREFIJO/||" | sort | tr '\n' ' ')"
[ -n "${PAQUETES// /}" ] || { echo "la lista de paquetes quedo vacia (go list -deps fallo?)" >&2; exit 1; }

[ -f "$BASE_GOX/orden.txt" ] || { echo "falta el arbol del motor en $BASE_GOX (corre build-motor-all.sh)" >&2; exit 1; }
mkdir -p "$OUT"

echo "== 1/4 el arbol de enlace ($GEN)"
if [ "${SALTAR_GEN:-0}" != "1" ]; then
    rm -rf "$GEN"
    mkdir -p "$GEN"
    # enlaces simbolicos, no copias: los .gox del motor son decenas de MB y no se tocan desde aca
    cp -rs "$BASE_GOX/." "$GEN"/
fi
rm -f "$GEN/orden.txt"
cp "$BASE_GOX/orden.txt" "$GEN/orden.txt"
for p in $PAQUETES; do
    f="$GEN/$PREFIJO/$p.gox"
    [ -f "$f" ] || { echo "   FALTA $p.gox en el arbol del motor: no se puede enlazar lo que no existe" >&2; exit 1; }
    grep -qxF "$f" "$GEN/orden.txt" || echo "$f" >> "$GEN/orden.txt"
done
echo "   paquetes del motor en el enlace: $(printf '%s\n' $PAQUETES | wc -l) -> $PAQUETES"
echo "   objetos en orden.txt: $(wc -l < "$GEN/orden.txt")"

echo "== 2/4 compilando el POC contra los .gox del motor"
GODC_GOX="$GEN" bash "$HERE/build-dc.sh" "$SRC" "$OUT" > "$OUT/build.log" 2>&1 || {
    echo "   FALLO el build (ver $OUT/build.log)"; tail -25 "$OUT/build.log"; exit 1; }

echo "== 3/4 lo que entro al enlace"
for p in $PAQUETES; do
    printf '   %-46s %10s B\n' "$p" "$(stat -L -c%s "$GEN/$PREFIJO/$p.gox")"
done

echo "== 4/4 lo que salio"
for f in prog.elf 1ST_READ.BIN disc.iso; do
    [ -f "$OUT/$f" ] && printf '   %-14s %10s B\n' "$f" "$(stat -c%s "$OUT/$f")"
done
printf '   codigo (.text)  %10s B\n' "$(sh-elf-size -A "$OUT/prog.elf" | awk '$1==".text"{print $2}')"
printf '   data+bss        %10s B\n' "$(sh-elf-size -A "$OUT/prog.elf" | awk '$1==".data"||$1==".bss"{s+=$2} END{print s}')"
[ -f "$OUT/disc.gdi" ] && echo "   imagen:        $OUT/disc.gdi"
