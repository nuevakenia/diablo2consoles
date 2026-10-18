#!/bin/bash
# DC-002.32: EL PESO DEL BINARIO DEL MOTOR EN LA RAM DE LA CONSOLA (16.777.216 bytes), MEDIDO POR SECCION
# Y POR SIMBOLO, con cada palanca y su mutante.
#
# QUE MIDE (y por que cada cosa tiene DOS caminos que tienen que coincidir)
#   1. El peso cargado: la suma de TODAS las secciones ALLOC del ELF (no el tamano del archivo, que
#      incluye .go_export y el .debug). Total contra los 16.777.216 bytes de la consola.
#   2. La seccion mas gorda, una por una (.text/.rodata/.eh_frame/.data/.bss).
#   3. Dentro del .data, el simbolo mas gordo: `nm -S` (la verdad del ELF) contra la cuenta de la fuente
#      (7.891 elementos x 212 bytes, con el 212 medido ADENTRO de la consola por el POC).
#   4. LA PALANCA 1 (la tabla fuera del binario): se arma una capa propia con la tabla vaciada, se enlaza,
#      y se miden las tres cosas que importan: lo que baja la imagen, lo que sube el bloque contiguo libre
#      del monton de C ANTES del motor, y lo que costaria devolver la tabla a la RAM (reserva medida).
#   5. LA PALANCA 2 (el .eh_frame): 1.074.504 bytes de tablas de desenrollado. Aca esta el hallazgo: el
#      recorte del objcopy NO baja la imagen plana porque el linker lo deja EN MEDIO del mapa y el hueco se
#      rellena. La palanca no es "sacarlo", es "que el linker lo ponga AL FINAL" (script propio, generado
#      por gen-ldscript-eh-232.py). Y no cuesta traza: el panic de verdad imprime lo mismo sin el.
#   6. Mutantes: cada palanca se mide con su mutante (una mentira que la prueba TIENE que cazar).
#
# USO
#   bash scripts/console/checar-peso-232.sh              # completo: arma capas, enlaza y corre (~10 min)
#   bash scripts/console/checar-peso-232.sh --rapido      # solo vuelve a medir lo que ya esta en disco
#   bash scripts/console/checar-peso-232.sh --mutar=X     # corre con el mutante X (tiene que salir ROJO)
set -uo pipefail

AQUI="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="${DC232_REPO:-$(cd "$AQUI/../.." && pwd)}"
# por defecto se trabaja SOBRE EL PROPIO repo (la copia de la tarjeta, /tmp/dc232-src, se pide con DC232_SRC)
SRC="${DC232_SRC:-$REPO}"
GOX_BASE="${DC232_GOX_BASE:-/tmp/gox-232p}"
GOX_VAR="${DC232_GOX_VAR:-/tmp/gox-232v}"
OUT="${DC232_OUT:-/tmp/dcbuild}"
BASE="$OUT/dc232-base"
VAR="$OUT/dc232-var"
PANIC="$OUT/dc232-panic"
DISCO="$OUT/232-panic-disco"
POC="${DC232_POC:-sonda-peso-232}"
RAPIDO=0
MUTAR=""

for a in "$@"; do
    case "$a" in
        --rapido) RAPIDO=1 ;;
        --mutar=*) MUTAR="${a#--mutar=}" ;;
        *) echo "opcion que no conozco: $a" >&2; exit 2 ;;
    esac
done

. "$SRC/scripts/console/env.sh"

verde=0; rojo=0; total=0
ok()   { total=$((total+1)); verde=$((verde+1)); printf '  [OK]   %-46s %s\n' "$1" "$2"; }
mal()  { total=$((total+1)); rojo=$((rojo+1));  printf '  [ROJO] %-46s %s\n' "$1" "$2"; }
chk()  { if [ "$2" = "$3" ]; then ok "$1" "$2 = $3"; else mal "$1" "esperaba $3, medi $2"; fi; }
# dos caminos que miden lo mismo no dan el mismo byte: se acepta la holgura de alineacion (y se dice)
cerca() {   # nombre, medido, esperado, holgura
    local d=$(( $2 - $3 )); [ "$d" -lt 0 ] && d=$(( -d ))
    if [ "$d" -le "$4" ]; then ok "$1" "$2 ~ $3 (dif $d, tope $4)"; else mal "$1" "esperaba ~$3 +-$4, medi $2"; fi
}
# el mutante que "miente" en un camino de medicion: si la prueba NO lo caza, la prueba esta de adorno
mentir() { [ "${MUTAR}" = "$2" ] && echo "$(( $1 + ${3:-1} ))" || echo "$1"; }

# --- herramientas de medicion -------------------------------------------------
seccion() { sh-elf-size -A "$1" | awk -v s="$2" '$1==s {print $2; exit}'; }
cargado() { python3 "$AQUI/medir-cargado.py" "$1" | sed -n 's/^.*: \([0-9]*\) bytes.*$/\1/p' | head -1; }
imagen()  { sh-elf-objcopy -R .stack ${3:-} -O binary "$1" "$2" >/dev/null 2>&1; stat -c%s "$2"; }
log()     { grep -a "$2" "$1" 2>/dev/null | tr -d '\r' | sed -n "s/.*$3\([0-9]*\).*/\1/p" | head -1; }

# reenlazar con un script de linker propio (o sin --gc-sections): el ultimo -T manda
reenlazar() {   # $1 dir del build, $2 dir gox, $3 salida, $4 script.xc (opcional), $5 "sin-gc" (opcional)
    local LDF="$KOS_LDFLAGS"
    [ -n "${4:-}" ] && LDF="${LDF//-T$KOS_BASE\/utils\/ldscripts\/shlelf.xc/-T$4}"
    [ "${5:-}" = "sin-gc" ] && LDF="${LDF//-Wl,--gc-sections/}"
    ( cd "$SRC" && KOS_LDFLAGS="$LDF" kos-cc -o "$3" "$1"/*.o \
        $(tr '\n' ' ' < "$1/orden-std.txt") $(tr '\n' ' ' < "$2/orden-motor.txt") \
        $DC_LDPATHS $DC_LIBS_PRE $DC_LIBS_POST ) >/dev/null 2>&1
    [ -f "$3" ] && echo si || echo no
}

# --- armar la evidencia -------------------------------------------------------
if [ "$RAPIDO" = 0 ]; then
    echo "== armando la evidencia (capas, enlaces y corridas)"
    DC232_SRC="$SRC" bash "$AQUI/armar-peso-232.sh" || { echo "no pude armar la evidencia" >&2; exit 1; }
fi

for f in "$BASE/prog.elf" "$VAR/prog.elf" "$BASE/1ST_READ.BIN" "$OUT/dc232-base-run/flycast.log" \
         "$OUT/dc232-var-run/flycast.log" "$OUT/232-ehfin-sin.bin" "$DISCO/run-sin/flycast.log"; do
    [ -f "$f" ] || { echo "falta $f (correlo sin --rapido)" >&2; exit 1; }
done

echo
echo "== DC-002.32: el peso del motor en los 16.777.216 bytes de la consola"
echo

# 1. el total cargado, y que las secciones cuadren con el (el mutante del camino de las secciones)
CARGADO=$(cargado "$BASE/prog.elf")
SUMA=0
for s in .init .fini .text .rodata .eh_frame .gcc_except_table .ctors .dtors .data .got .bss; do
    n=$(seccion "$BASE/prog.elf" "$s"); SUMA=$((SUMA + ${n:-0}))
done
echo "  el ELF cargado pesa $CARGADO bytes = $(( CARGADO * 10000 / 16777216 )) centesimas de % de la RAM"
chk "secciones ALLOC suman el total cargado" "$(mentir "$SUMA" secciones-suma 4096)" "$CARGADO"

# 2. las secciones, una por una (las dos gordas y el .eh_frame)
TEXT=$(seccion "$BASE/prog.elf" .text); ROD=$(seccion "$BASE/prog.elf" .rodata)
EH=$(seccion "$BASE/prog.elf" .eh_frame); DATA=$(seccion "$BASE/prog.elf" .data); BSS=$(seccion "$BASE/prog.elf" .bss)
echo "  .text $TEXT | .rodata $ROD | .eh_frame $EH | .data $DATA | .bss $BSS"

# 3. la tabla por DOS caminos: el simbolo del ELF y la cuenta de la fuente
SIMB_LINEA=$(sh-elf-nm -S --size-sort "$BASE/prog.elf" | tail -1)
SIMB=$(( 0x$(awk '{print $2}' <<<"$SIMB_LINEA") )); NOMBRE=$(awk '{print $4}' <<<"$SIMB_LINEA")
ELEM=$(grep -c '^[[:space:]]*{Act:' "$AQUI/../../d2core/d2records/object_lookup_record_data.go")
SIZE=$(log "$OUT/dc232-var-run/flycast.log" "PESO-232 ObjectLookupRecord" "mide \([0-9]*\) bytes")
[ -z "${SIZE:-}" ] && SIZE=$(log "$OUT/dc232-base-run/flycast.log" "PESO-232 ObjectLookupRecord" "mide \([0-9]*\) bytes")
echo "  el simbolo mas gordo ($NOMBRE) pesa $SIMB | la fuente tiene $ELEM elementos | el registro mide $SIZE"
chk "la tabla por el ELF contra la fuente" "$(mentir "$SIMB" tabla-simbolo 7)" "$(( ELEM * SIZE ))"
chk "y el simbolo mas gordo es el de la tabla" "$(grep -c C3111 <<<"$NOMBRE")" "1"

# 4. LA PALANCA 1: la tabla fuera del binario
CARGADO_VAR=$(cargado "$VAR/prog.elf")
IMAGEN=$(stat -c%s "$BASE/1ST_READ.BIN"); IMAGEN_VAR=$(stat -c%s "$VAR/1ST_READ.BIN")
DATA_VAR=$(seccion "$VAR/prog.elf" .data); ROD_VAR=$(seccion "$VAR/prog.elf" .rodata)
SBRK_BASE=$(log "$OUT/dc232-base-run/flycast.log" "sbrk=" "bloqueMax=\([0-9]*\)")
SBRK_VAR=$(log "$OUT/dc232-var-run/flycast.log" "sbrk=" "bloqueMax=\([0-9]*\)")
LOSA=$(log "$OUT/dc232-var-run/flycast.log" "monton con la tabla adentro" "arena=\([0-9]*\)")
LOSA0=$(log "$OUT/dc232-var-run/flycast.log" "monton antes de la reserva" "arena=\([0-9]*\)")
echo "  la tabla: baja el .data $(( DATA - DATA_VAR )) y el .rodata $(( ROD - ROD_VAR )); el total cargado cae $(( CARGADO - CARGADO_VAR ))"
echo "  la imagen plana: $IMAGEN -> $IMAGEN_VAR ($(( IMAGEN - IMAGEN_VAR )) bytes menos)"
echo "  el monton de C antes del motor: bloqueMax $SBRK_BASE -> $SBRK_VAR ($(( SBRK_VAR - SBRK_BASE )) mas)"
chk "la tabla vive en el .data, no en el .rodata" "$(( (DATA - DATA_VAR) > (ROD - ROD_VAR) ? 1 : 0 ))" "1"
cerca "el .data baja exactamente la tabla" "$(( DATA - DATA_VAR ))" "$(( ELEM * SIZE ))" 16
cerca "la palanca baja la imagen lo que baja el ELF" "$(mentir "$(( IMAGEN - IMAGEN_VAR ))" palanca-imagen 4096)" "$(( CARGADO - CARGADO_VAR ))" 32
cerca "el aire del monton crece lo que baja la imagen" "$(mentir "$(( SBRK_VAR - SBRK_BASE ))" palanca-aire 4096)" "$(( IMAGEN - IMAGEN_VAR ))" 32
cerca "el precio: devolver la tabla al monton cuesta su tamano" "$(mentir "$(( LOSA - LOSA0 ))" palanca-precio 100000)" "$(( ELEM * SIZE ))" 4096

# 5. LA PALANCA 2: el .eh_frame (y el hallazgo del hueco)
EH_SIN_MOVER=$(imagen "$BASE/prog.elf" "$OUT/232-kos-sin.bin" "-R .eh_frame")
IMAGEN_CON=$(stat -c%s "$BASE/1ST_READ.BIN")
IMAGEN_EHFIN=$(imagen "$OUT/232-ehfin.elf" "$OUT/232-ehfin-sin.bin" "-R .eh_frame")
EHFIN_CON=$(imagen "$OUT/232-ehfin.elf" "$OUT/232-ehfin-completa.bin")
echo "  con el script de KOS: imagen $IMAGEN_CON y quitando el .eh_frame $EH_SIN_MOVER"
echo "  con el .eh_frame AL FINAL: imagen $EHFIN_CON y quitandolo $IMAGEN_EHFIN"
chk "el hueco: sacarlo SIN moverlo NO baja nada" "$(mentir "$(( IMAGEN_CON - EH_SIN_MOVER ))" eh-hueco 1)" "0"
chk "moviendolo al final, sacarlo baja su tamano" "$(mentir "$(( EHFIN_CON - IMAGEN_EHFIN ))" eh-movido 1)" "$EH"
chk "y la traza del panic NO depende de el" "$(grep -ac 'Stack Trace' "$DISCO/run-sin/flycast.log")" "$(grep -ac 'Stack Trace' "$DISCO/run-elf/flycast.log")"
chk "el control: la traza CON el .eh_frame por disco es la misma" "$(grep -ac 'goroutine 1 \[running\]' "$DISCO/run-con/flycast.log")" "1"

# 6. las palancas que NO cuestan (medidas, no supuestas)
reenlazar "$BASE" "$GOX_BASE" "$OUT/232-singc.elf" "" "sin-gc" >/dev/null
imagen "$OUT/232-singc.elf" "$OUT/232-singc.bin" >/dev/null
SIN_GC=$(stat -c%s "$OUT/232-singc.bin")
GCT=$(seccion "$BASE/prog.elf" .gcc_except_table)
echo "  sin --gc-sections la imagen crece $(( SIN_GC - IMAGEN_CON )) (el .gcc_except_table mide $GCT)"
salvo=$(( SIN_GC - IMAGEN_CON ))
if [ "$salvo" -ge "$GCT" ] && [ "$salvo" -gt 0 ]; then
    ok "el --gc-sections que KOS ya pasa vale imagen" "$salvo bytes (>= el .gcc_except_table)"
else
    mal "el --gc-sections que KOS ya pasa vale imagen" "esperaba >= $GCT, medi $salvo"
fi

echo
echo "== 7. la palanca se puede poner de vuelta (criterio 4)"
VUELTA=$(python3 "$AQUI/fix-d2records-tabla-232.py" --dentro | sed -n 's/.* sha256=\([0-9a-f]\{64\}\).*/\1/p')
ORIG=$(sha256sum "$OUT/dc232-tabla/object_lookup_record_data.go.original" | awk '{print $1}')
echo "  la fuente volvio a $VUELTA y el original guardado es $ORIG"
chk "poner la tabla de vuelta devuelve la fuente identica" "$VUELTA" "$ORIG"
# con la fuente de vuelta, se REENLAZA el motor (la capa de la tabla ya esta): el peso tiene que volver
if [ "$RAPIDO" = 0 ] || [ ! -f "$OUT/dc232-vuelta/prog.elf" ]; then
    ( cd "$SRC" && POC="$POC" bash "$AQUI/build-motor-arranque.sh" "$OUT/dc232-vuelta" "$GOX_BASE" ) >/dev/null 2>&1
fi
VUELTA_PESO=$(cargado "$OUT/dc232-vuelta/prog.elf")
echo "  con la tabla de vuelta el ELF cargado pesa $VUELTA_PESO (la base pesa $CARGADO)"
chk "y el peso vuelve al de la base" "$(mentir "$VUELTA_PESO" vuelta 4096)" "$CARGADO"

echo
echo "== $verde de $total verdes$( [ -n "$MUTAR" ] && echo "  (con el mutante '$MUTAR')" )"
[ "$rojo" = 0 ] && echo "== PESO-232: el peso y las palancas quedan medidos y con mutante" || echo "== PESO-232: HAY $rojo MEDICIONES QUE NO CIERRAN"
[ "$rojo" = 0 ]
