#!/bin/bash
# DC-002.67 -- ENLAZAR el motor compilado (los .gox de DC-002.66) en un binario de Dreamcast.
#
# DC-002.66 probo que los 139 paquetes del motor COMPILAN a .gox para sh-elf. Compilar no es enlazar: este
# script arma el binario de la consola con ESOS MISMOS .gox, KOS y libgodc, y mide cuanto sale en cada paso.
#
# COMO SE ARMA EL ARBOL DE ENLACE
#   El arbol de DC-002.66 (/tmp/gox-std por defecto) tiene dos listas: orden.txt (la stdlib minima + los
#   terceros, 63 objetos) y orden-motor.txt (los 139 del motor). build-dc.sh enlaza UNA sola lista:
#   orden.txt. Asi que aca se arma un arbol de ENLACE con enlaces simbolicos a los .gox de DC-002.66 (no se
#   copian 67 MB) y una orden.txt propia = la stdlib + SOLO los paquetes del motor que este paso necesita.
#   Los paquetes se pasan en PAQUETES, asi que el paso 2 y el paso 3 se diferencian solo en esa lista.
#
# Uso:  scripts/console/build-enlace-267.sh [dir_salida]
#       PASO3=1          subir la apuesta: importar d2ui + d2gamescreen (125 paquetes) en vez de los 4 del paso 2
#       PAQUETES="..."   paquetes del motor a enlazar (por defecto lo que diga el paso)
#       BASE_GOX=...     arbol de .gox de DC-002.66            (por defecto /tmp/gox-std)
#       SALTAR_GEN=1     reusar el arbol de enlace ya armado   (para no repetir el cp -rs)
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
SRC_BASE="$REPO/console/dc/poc/enlace-267"
BASE_GOX="${BASE_GOX:-/tmp/gox-std}"
OUT="${1:-/tmp/dcbuild/enlace-267}"
GEN="$OUT/gox-enlace"
PREFIJO="github.com/OpenDiablo2/OpenDiablo2"

# PASO 2: el cierre de d2common/d2act3 medido con `go list -deps` sobre el repo = 4 paquetes.
# PASO 3: el cierre de d2core/d2ui + d2game/d2gamescreen = 125 paquetes, calculado con la misma herramienta
#         (no se escribe a mano: si el motor cambia, el numero cambia solo).
if [ "${PASO3:-0}" = "1" ]; then
    PAQUETES="${PAQUETES:-$(cd "$REPO" && go list -deps ./d2core/d2ui ./d2game/d2gamescreen 2>/dev/null |
        grep "$PREFIJO" | sed "s|.*$PREFIJO/||" | sort | tr '\n' ' ')}"
else
    PAQUETES="${PAQUETES:-d2common/d2act3 d2common/d2quest d2common/d2level d2common/d2fileformats/d2s}"
fi
[ -n "${PAQUETES// /}" ] || { echo "la lista de paquetes quedo vacia (go list -deps fallo?)" >&2; exit 1; }

[ -f "$BASE_GOX/orden.txt" ] || { echo "falta el arbol de DC-002.66 en $BASE_GOX (corre build-motor-all.sh)" >&2; exit 1; }
mkdir -p "$OUT"

# PASO 3: el POC del paso 2 + referencias.go, copiados a un directorio de build. El POC del repo no se toca:
# el paso 2 tiene que seguir midiendo SOLO los 4 paquetes de d2act3.
SRC="$SRC_BASE"
if [ "${PASO3:-0}" = "1" ]; then
    SRC="$OUT/poc-paso3"
    rm -rf "$SRC"
    mkdir -p "$SRC"
    cp "$SRC_BASE"/*.go "$SRC"/
    cp "$SRC_BASE/paso3/referencias.go" "$SRC"/
    cp -r "$SRC_BASE/csrc" "$SRC"/
    echo "   paso 3: el POC + referencias.go en $SRC"
fi

echo "== 1/4 el arbol de enlace ($GEN)"
if [ "${SALTAR_GEN:-0}" != "1" ]; then
    # arbol limpio: si quedo a medias de una corrida anterior, `cp -rs` se queja de "son el mismo fichero"
    # (los enlaces ya estan) y el arbol queda mezclado
    rm -rf "$GEN"
    mkdir -p "$GEN"
    # enlaces simbolicos, no copias: los .gox son 67 MB y no se tocan
    cp -rs "$BASE_GOX/." "$GEN"/
    : > "$GEN/.armado"
fi
# la orden de enlace: la stdlib/terceros de DC-002.66 + los paquetes del motor de este paso
# (el orden.txt del arbol quedo como ENLACE SIMBOLICO al de origen: hay que sacarlo antes de escribir,
#  si no `cp` copia el archivo sobre si mismo)
rm -f "$GEN/orden.txt"
cp "$BASE_GOX/orden.txt" "$GEN/orden.txt"
for p in $PAQUETES; do
    f="$GEN/$PREFIJO/$p.gox"
    [ -f "$f" ] || { echo "   FALTA $p.gox en el arbol de DC-002.66: no se puede enlazar lo que no existe" >&2; exit 1; }
    grep -qxF "$f" "$GEN/orden.txt" || echo "$f" >> "$GEN/orden.txt"
done
echo "   objetos en orden.txt: $(wc -l < "$GEN/orden.txt") (stdlib+terceros $(wc -l < "$BASE_GOX/orden.txt") + motor $(printf '%s\n' $PAQUETES | wc -l))"

echo "== 2/4 compilando el main contra los .gox del motor"
GODC_GOX="$GEN" bash "$HERE/build-dc.sh" "$SRC" "$OUT" > "$OUT/build.log" 2>&1 || {
    echo "   FALLO el build (ver $OUT/build.log)"; tail -20 "$OUT/build.log"; exit 1; }
echo "   main.o  $(stat -c%s "$OUT/main.o") B"

echo "== 3/4 lo que entro al enlace"
MOTOR_B=0
: > "$OUT/objetos-motor.txt"
for p in $PAQUETES; do
    f="$GEN/$PREFIJO/$p.gox"
    # stat -L: en el arbol de enlace los .gox son ENLACES SIMBOLICOS y sin -L se mide el enlace, no el .gox
    b=$(stat -L -c%s "$f")
    MOTOR_B=$((MOTOR_B + b))
    printf '   %-46s %10s B\n' "$p" "$b" >> "$OUT/objetos-motor.txt"
done
cat "$OUT/objetos-motor.txt"
STD_B=0
while read -r f; do
    [ -f "$f" ] && STD_B=$((STD_B + $(stat -L -c%s "$f")))
done < <(head -n "$(wc -l < "$BASE_GOX/orden.txt")" "$GEN/orden.txt")
echo "   motor  $(printf '%s\n' $PAQUETES | wc -l) paquetes  $MOTOR_B B"
echo "   stdlib+terceros                        $STD_B B"
echo "   suma de .gox enlazados                 $((MOTOR_B + STD_B)) B"

echo "== 4/4 lo que salio"
for f in prog.elf 1ST_READ.BIN disc.iso; do
    [ -f "$OUT/$f" ] && printf '   %-14s %10s B\n' "$f" "$(stat -c%s "$OUT/$f")"
done
# EL TAMANO QUE IMPORTA NO ES EL DEL ELF: los .gox traen DWARF y el ELF son 4,8 MB de los cuales la mitad
# es depuracion. Lo que ocupa RAM en la consola es lo que se copia al arrancar = 1ST_READ.BIN, y el codigo
# y los datos reales se leen por seccion.
printf '   codigo (.text)  %10s B\n' "$(sh-elf-size -A "$OUT/prog.elf" | awk '$1==".text"{print $2}')"
printf '   rodata          %10s B\n' "$(sh-elf-size -A "$OUT/prog.elf" | awk '$1==".rodata"{print $2}')"
printf '   data+bss        %10s B\n' "$(sh-elf-size -A "$OUT/prog.elf" | awk '$1==".data"||$1==".bss"{s+=$2} END{print s}')"
printf '   depuracion      %10s B\n' "$(sh-elf-size -A "$OUT/prog.elf" | awk '$1 ~ /^\.debug/ {s+=$2} END{print s}')"
[ -f "$OUT/disc.gdi" ] && echo "   imagen:        $OUT/disc.gdi"
grep -E '^(ELF|1ST_READ|imagen):' "$OUT/build.log" || true
