#!/bin/bash
# CORPUS DE DC-002.29: QUE MATA EL ARRANQUE DEL MOTOR REARMADO.
#
# QUE MIDE (host + consola). El defecto de la tarjeta era: "recompilar un paquete del motor y
# reenlazar deja un binario que muere MUDO adentro de los `init`". Lo que este corpus exige:
#
#  1. HOST: recompilar un paquete cuya fuente NO cambio reproduce su `.text` BYTE A BYTE. (No el
#     sha256 del `.gox`: el sha SIEMPRE cambia, porque los caminos del debug cambian de forma
#     --medido--; lo que importa es el CODIGO.) Si el `.text` cambia, la camada ya no es coherente
#     con el arbol y cualquier medicion de arriba es humo.
#  2. HOST: el compilador es determinista: dos rearmes de d2mpq dan el mismo `.gox` byte a byte.
#  3. HOST: la lista de los `init` de `___go_init_main` es la MISMA en el binario que anda y en el
#     que muere (119 llamadas) -> no es la lista ni el orden.
#  4. CONSOLA (control sano): la camada de referencia, reenlazada tal cual, ARRANCA y llega al cuadro.
#  5. CONSOLA (el control que ordena todo): la MISMA camada con d2mpq compilado de SU PROPIA fuente
#     (la del commit con el que se armo la camada, 6e85010b) TAMBIEN ARRANCA. O sea: rearmar no rompe
#     nada; lo que rompe es el CODIGO NUEVO contra la camada vieja.
#  6. CONSOLA (el DEFECTO, vigilado): la camada + d2mpq de la fuente del arbol de trabajo MUERE
#     adentro de los `init` (0 main.main, 0 cuadros, ultima marca `main_wrapper-entra`). Este es el
#     defecto que la tarjeta vino a localizar y que NO esta arreglado: si un dia deja de morir, el
#     corpus se pone ROJO para que se revise el limite, no para taparlo.
#
# MUTANTE: scripts/console/mutar-rearme-229.py --fuente-tocada rompe la coherencia de la camada
# (compila d2mpq de una fuente con una linea de mas) y el control 1 tiene que ponerse ROJO: eso es
# exactamente el estado que mata el arranque (medido).
#
# Uso: bash scripts/console/checar-rearme-229.sh [camada_de_referencia] [segundos]
#      La camada de referencia tiene que ser una que ARRANQUE (/tmp/gox-240 o /tmp/gox-00220).
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REF="${1:-/tmp/gox-240}"
SEGS="${2:-30}"
REPO="$(cd "$HERE/../.." && pwd)"
SAL="${SAL:-/home/dgalli/.hermes/cache/scratch/corpus-229}"
JUEGO="${DC_DISCO_JUEGO:-/home/dgalli/.hermes/cache/scratch/corpus-227/juego}"
P=d2common/d2fileformats/d2mpq
PREFIJO=github.com/OpenDiablo2/OpenDiablo2
COMMIT_VIEJO="${COMMIT_VIEJO:-6e85010b}"   # el arbol con el que se armo la camada de referencia
FUENTE_VIEJA="$SAL/src-$COMMIT_VIEJO"

mkdir -p "$SAL"
OK=0
FALLOS=0
verde() { OK=$((OK + 1)); echo "   OK: $*"; }
rojo() { FALLOS=$((FALLOS + 1)); echo "   FALLO: $*"; }
nota() { echo "   $*"; }

echo "== 0. la camada de referencia ($REF)"
[ -f "$REF/orden.txt" ] && [ -f "$REF/orden-motor.txt" ] || { echo "   no encuentro la camada en $REF"; exit 1; }
nota "orden.txt $(wc -l < "$REF/orden.txt") objetos | orden-motor.txt $(wc -l < "$REF/orden-motor.txt") paquetes"
nota "d2mpq.gox de la referencia: $(stat -c%s "$REF/$PREFIJO/$P.gox") bytes (.text $(sh-elf-objdump -h "$REF/$PREFIJO/$P.gox" | awk '$2==".text"{print "0x"$3}'))"

copia() {   # $1 destino
    rm -rf "$1"; cp -a "$REF" "$1"
    sed -i "s#$REF#$1#g" "$1/orden.txt" "$1/orden-motor.txt"
}

# ================================================================================================
# 1. HOST: el rearme de un paquete sin cambios reproduce su .text
# ================================================================================================
echo "== 1. host: recompilar un paquete SIN cambios conserva el .text byte a byte"
copia "$SAL/gox-ctl1"
IGUALES=0
TOTAL=0
for p in d2core/d2hero d2common/d2loader/mpq d2networking/d2netpacket d2game/d2player; do
    TOTAL=$((TOTAL + 1))
    bash "$HERE/recompilar-paquete-motor.sh" "$p" "$SAL/gox-ctl1" > "$SAL/recompila-$(echo "$p" | tr / _).log" 2>&1
    A=$(sh-elf-objdump -h "$REF/$PREFIJO/$p.gox" | awk '$2==".text"{print $3}')
    B=$(sh-elf-objdump -h "$SAL/gox-ctl1/$PREFIJO/$p.gox" | awk '$2==".text"{print $3}')
    if [ -n "$A" ] && [ "$A" = "$B" ]; then
        IGUALES=$((IGUALES + 1))
        nota "$p: .text $A en los dos ($(stat -c%s "$REF/$PREFIJO/$p.gox") -> $(stat -c%s "$SAL/gox-ctl1/$PREFIJO/$p.gox") bytes de .gox)"
    else
        nota "$p: .text CAMBIA ($A -> $B)"
    fi
done
[ "$IGUALES" = "$TOTAL" ] && verde "control 1: los $TOTAL paquetes sin cambios conservan el .text" ||
    rojo "control 1: solo $IGUALES de $TOTAL conservan el .text: la camada NO es coherente con el arbol"

# ================================================================================================
# 2. HOST: el compilador es determinista
# ================================================================================================
echo "== 2. host: el rearme de d2mpq es determinista (dos veces, mismo .gox)"
copia "$SAL/gox-ctl2"
bash "$HERE/recompilar-paquete-motor.sh" "$P" "$SAL/gox-ctl2" > "$SAL/rearme-determinismo-1.log" 2>&1
cp "$SAL/gox-ctl2/$PREFIJO/$P.gox" "$SAL/determinismo-primero.gox"
bash "$HERE/recompilar-paquete-motor.sh" "$P" "$SAL/gox-ctl2" > "$SAL/rearme-determinismo-2.log" 2>&1
if cmp -s "$SAL/determinismo-primero.gox" "$SAL/gox-ctl2/$PREFIJO/$P.gox"; then
    verde "control 2: los dos rearmes dan el mismo .gox (sha256 $(sha256sum "$SAL/determinismo-primero.gox" | cut -c1-16))"
else
    rojo "control 2: los dos rearmes de la misma fuente dan .gox DISTINTOS"
fi

# ================================================================================================
# 3. CONSOLA: las dos corridas (la camada tal cual y la camada con la fuente vieja)
# ================================================================================================
corre() {   # $1 nombre  $2 camada  $3 segundos
    local nom="$1" gox="$2" segs="$3" out="$SAL/$1"
    MKISO_JOLIET=0 DC_RUTA_DATOS=/cd/juego DC_MPQ_JUEGO=patch_d2.mpq,d2data.mpq,d2char.mpq \
        DC_SIN_SELLOS=1 DC_DISCO_JUEGO="$JUEGO" POC=motor-disco \
        bash "$HERE/build-motor-arranque.sh" "$out" "$gox" > "$out.build.log" 2>&1
    [ $? -eq 0 ] || { echo "   el POC motor-disco no compila/enlaza (ver $out.build.log)"; return 1; }
    CAPTURE=none bash "$HERE/run-flycast.sh" "$out/disc.gdi" "$segs" "$out-run" > "$out-run.out" 2>&1
}
mide() {    # $1 nombre -> MARCA_ MAIN_ CUADROS_ ARRANQUES_ GC_ UELTIMA_
    local T; T=$(tr -d '\r' < "$SAL/$1-run/flycast.log")
    MARCA_=$(printf '%s' "$T" | grep -ac 'ARQ-MARCA')
    MAIN_=$(printf '%s' "$T" | grep -ac 'main.main-entra')
    CUADROS_=$(printf '%s' "$T" | grep -ac 'EBITEN-PERFIL')
    ARRANQUES_=$(printf '%s' "$T" | grep -ac 'KallistiOS v2.2.1')
    GC_=$(printf '%s' "$T" | grep -ac 'gc_collect')
    UELTIMA_=$(printf '%s' "$T" | grep -a 'ARQ-MARCA' | tail -1)
}

echo "== 3. consola: la camada de referencia, reenlazada tal cual"
copia "$SAL/gox-ctl3"; corre c3-ref "$SAL/gox-ctl3" "$SEGS"
mide c3-ref
nota "marcas=$MARCA_  main.main=$MAIN_  cuadros=$CUADROS_  arranques=$ARRANQUES_  gc_collect=$GC_"
nota "ultima marca: $UELTIMA_"
[ "$MAIN_" -ge 1 ] && [ "$CUADROS_" -ge 1 ] && [ "$ARRANQUES_" = 1 ] &&
    verde "control 3: la camada de referencia llega al cuadro ($CUADROS_ cuadros)" ||
    rojo "control 3: la camada de referencia NO llega al cuadro (main.main=$MAIN_ cuadros=$CUADROS_ arranques=$ARRANQUES_)"

# la fuente del commit con el que se armo la camada (via git show, sin tocar el arbol)
echo "== 4. consola: la camada + d2mpq de SU PROPIA fuente ($COMMIT_VIEJO)"
rm -rf "$FUENTE_VIEJA"; mkdir -p "$FUENTE_VIEJA"
( cd "$REPO" && git archive "$COMMIT_VIEJO" "$P" | tar -x -C "$FUENTE_VIEJA" --strip-components=4 ) 2>"$SAL/archivo-viejo.log"
nota "fuentes: $(ls "$FUENTE_VIEJA"/*.go 2>/dev/null | wc -l) archivos de $COMMIT_VIEJO"
copia "$SAL/gox-ctl4"
sh-elf-gccgo $GODC_GOFLAGS -I "$SAL/gox-ctl4" -fgo-pkgpath="$PREFIJO/$P" -c "$FUENTE_VIEJA"/*.go \
    -o "$SAL/gox-ctl4/$PREFIJO/$P.gox" > "$SAL/gox-ctl4.log" 2>&1 ||
    { echo "   la fuente de $COMMIT_VIEJO no compila (ver $SAL/gox-ctl4.log)"; exit 1; }
nota "d2mpq.gox puesto: $(stat -c%s "$SAL/gox-ctl4/$PREFIJO/$P.gox") bytes (.text $(sh-elf-objdump -h "$SAL/gox-ctl4/$PREFIJO/$P.gox" | awk '$2==".text"{print "0x"$3}'))"
corre c4-vieja "$SAL/gox-ctl4" "$SEGS"
mide c4-vieja
nota "marcas=$MARCA_  main.main=$MAIN_  cuadros=$CUADROS_  arranques=$ARRANQUES_  gc_collect=$GC_"
nota "ultima marca: $UELTIMA_"
[ "$MAIN_" -ge 1 ] && [ "$CUADROS_" -ge 1 ] &&
    verde "control 4: rearmar con la fuente de la camada ARRANCA ($CUADROS_ cuadros) -> rearmar no rompe nada" ||
    rojo "control 4: rearmar con la fuente de la camada NO arranca (main.main=$MAIN_ cuadros=$CUADROS_)"

# ================================================================================================
# 5. CONSOLA: EL DEFECTO -- el codigo del arbol de trabajo contra la camada vieja
# ================================================================================================
echo "== 5. consola: la camada + d2mpq de la fuente del ARBOL DE TRABAJO (el rearme corto)"
copia "$SAL/gox-ctl5"
bash "$HERE/recompilar-paquete-motor.sh" "$P" "$SAL/gox-ctl5" > "$SAL/rearme-d2mpq.log" 2>&1
nota "d2mpq.gox: $(stat -c%s "$REF/$PREFIJO/$P.gox") -> $(stat -c%s "$SAL/gox-ctl5/$PREFIJO/$P.gox") bytes (.text $(sh-elf-objdump -h "$SAL/gox-ctl5/$PREFIJO/$P.gox" | awk '$2==".text"{print "0x"$3}'))"
corre c5-trabajo "$SAL/gox-ctl5" "$SEGS"
mide c5-trabajo
nota "marcas=$MARCA_  main.main=$MAIN_  cuadros=$CUADROS_  arranques=$ARRANQUES_  gc_collect=$GC_"
nota "ultima marca: $UELTIMA_"

# la lista de `init` (control 6) y el defecto vigilado (control 5)
python3 "$HERE/lista-inits-229.py" "$SAL/c3-ref/prog.elf" > "$SAL/inits-ref.txt" 2>&1
python3 "$HERE/lista-inits-229.py" "$SAL/c5-trabajo/prog.elf" > "$SAL/inits-trabajo.txt" 2>&1
if diff -q "$SAL/inits-ref.txt" "$SAL/inits-trabajo.txt" > /dev/null 2>&1; then
    verde "control 6: la lista de \`init\` es la MISMA ($(head -1 "$SAL/inits-ref.txt")) en el que anda y en el que muere"
else
    rojo "control 6: la lista de \`init\` CAMBIA entre los dos binarios (ver $SAL/inits-*.txt)"
fi

if [ "$MAIN_" = 0 ] && [ "$CUADROS_" = 0 ]; then
    verde "control 5 (el DEFECTO, VIGILADO): el rearme corto muere adentro de los \`init\` (main.main=0, cuadros=0, arranques=$ARRANQUES_, gc_collect=0, ultima=$(echo "$UELTIMA_" | cut -c1-40))"
else
    rojo "control 5: el rearme corto YA NO muere (main.main=$MAIN_ cuadros=$CUADROS_) -- el limite cambio: hay que REVISARLO, no taparlo"
fi

echo
echo "== RESULTADO: $OK controles OK, $FALLOS con fallo (el 5 es el que VIGILA el defecto de la tarjeta)"
echo "== el discriminador: rearmar es inocuo (control 4); el codigo nuevo contra la camada vieja muere (control 5)."
echo "== evidencia: console/dc/evidencia/dc-002.29-arranque-rearmado.txt"
[ "$FALLOS" -eq 0 ]
