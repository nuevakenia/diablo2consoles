#!/bin/bash
# CORPUS DE DC-002.29: QUE MATA EL ARRANQUE DEL MOTOR REARMADO.
#
# QUE MIDE (host + consola). El defecto de la tarjeta era: "recompilar un paquete del motor y
# reenlazar deja un binario que muere MUDO adentro de los `init`". Lo que este corpus exige:
#
#  1. HOST: recompilar un paquete cuya fuente NO cambio reproduce su `.text` BYTE A BYTE (4 paquetes de
#     tamanos distintos). (No el sha256 del `.gox`: el sha siempre cambia, porque los caminos del debug
#     cambian de forma --medido--; lo que importa es el CODIGO.) Si el `.text` cambia, la camada ya no
#     es coherente con el arbol y cualquier medicion de arriba es humo.
#  2. HOST: el compilador es determinista: dos rearmes de d2mpq dan el mismo `.gox` byte a byte.
#  3. CONSOLA (control sano): la camada de referencia (la de DC-002.20), reenlazada tal cual, ARRANCA y
#     llega al cuadro.
#  4. CONSOLA (el control que ordena todo): la MISMA camada con d2mpq compilado de SU PROPIA fuente (la
#     del commit con el que se armo la camada, 6e85010b) TAMBIEN ARRANCA. O sea: rearmar no rompe nada.
#  5. CONSOLA (el DEFECTO, vigilado): la camada VIEJA + d2mpq de la fuente del arbol de trabajo MUERE
#     adentro de los `init` (0 main.main, 0 cuadros, ultima marca `main_wrapper-entra`). Es el LIMITE
#     medido: MEZCLAR camadas mata el arranque. Si un dia deja de morir, el corpus se pone ROJO para que
#     se revise el limite, no para taparlo.
#  6. CONSOLA: la lista de los `init` de `___go_init_main` es la MISMA en el binario que anda y en el
#     que muere (119 entradas, mismos simbolos y mismo orden) -> no es la lista ni el orden. Las
#     DIRECCIONES se corren con el layout (el d2mpq nuevo agranda el binario) y no cuentan.
#  7. CONSOLA (EL ARREGLO, DC-002.29): la camada COHERENTE del arbol de trabajo (armada con
#     build-std-gox.sh + build-motor-all.sh; ver $COHERENTE): (a) rearmar d2mpq NO cambia su `.text` y
#     (b) el motor ARRANCA (main.main=1). Sobre la camada del MISMO estado del arbol el rearme es un
#     no-op y el arranque es sano.
#
# LA CAUSA (medida en DC-002.29): el `.gox` de gccgo NO es autocontenido (lleva los cuerpos inlineados y
# los numeros de tipo de sus imports, con la camada con la que se compilo), asi que el enlace solo es
# sano con la camada del MISMO estado del arbol. El arreglo es de PROCEDIMIENTO: la camada se rehace
# cuando cambia el codigo de un paquete que se va a medir; el rearme corto solo vale para un paquete
# cuya fuente NO cambio (y ahi el `.text` sale identico: control 1).
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
SALTADOS=0
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

# ================================================================================================
# 4. CONSOLA: la camada + d2mpq de SU PROPIA fuente (el control que tira la hipotesis "rearmar rompe")
# ================================================================================================
echo "== 4. consola: la camada + d2mpq de SU PROPIA fuente ($COMMIT_VIEJO)"
rm -rf "$FUENTE_VIEJA"; mkdir -p "$FUENTE_VIEJA"
# OJO (medido, y arreglado en DC-002.29): los miembros del archivo son
# `d2common/d2fileformats/d2mpq/<archivo>.go` => hay que sacar TRES componentes. Con `--strip-components=4`
# el tar no extrae NADA (directorio vacio) y el control 4 moria en el compilador. Y los `_test.go` se
# filtran, igual que en recompilar-paquete-motor.sh: gccgo no filtra por sufijo y piden testing/testify.
( cd "$REPO" && git archive "$COMMIT_VIEJO" "$P" | tar -x -C "$FUENTE_VIEJA" --strip-components=3 ) 2>"$SAL/archivo-viejo.log"
SRCS_VIEJA=$(ls "$FUENTE_VIEJA"/*.go 2>/dev/null | grep -v _test.go | grep -vE '_(darwin|windows|js|wasm|plan9|freebsd|netbsd|openbsd|linux|android|ios|solaris|aix|386|amd64|arm|arm64|ppc64|ppc64le|mips|mipsle|mips64|mips64le|riscv64|s390x|loong64)\.go$')
nota "fuentes: $(echo "$SRCS_VIEJA" | grep -c .) archivos de $COMMIT_VIEJO (sin _test.go ni otras plataformas)"
copia "$SAL/gox-ctl4"
# shellcheck disable=SC2086
if sh-elf-gccgo $GODC_GOFLAGS -I "$SAL/gox-ctl4" -fgo-pkgpath="$PREFIJO/$P" -c $SRCS_VIEJA \
        -o "$SAL/gox-ctl4/$PREFIJO/$P.gox" > "$SAL/gox-ctl4.log" 2>&1; then
    nota "d2mpq.gox puesto: $(stat -c%s "$SAL/gox-ctl4/$PREFIJO/$P.gox") bytes (.text $(sh-elf-objdump -h "$SAL/gox-ctl4/$PREFIJO/$P.gox" | awk '$2==".text"{print "0x"$3}'))"
    corre c4-vieja "$SAL/gox-ctl4" "$SEGS"
    mide c4-vieja
    nota "marcas=$MARCA_  main.main=$MAIN_  cuadros=$CUADROS_  arranques=$ARRANQUES_  gc_collect=$GC_"
    nota "ultima marca: $UELTIMA_"
    [ "$MAIN_" -ge 1 ] && [ "$CUADROS_" -ge 1 ] &&
        verde "control 4: rearmar con la fuente de la camada ARRANCA ($CUADROS_ cuadros) -> rearmar no rompe nada" ||
        rojo "control 4: rearmar con la fuente de la camada NO arranca (main.main=$MAIN_ cuadros=$CUADROS_)"
else
    # sin el .gox compilado el control NO se puede correr: la camada copiada trae el d2mpq de la
    # referencia, asi que correrla daria un VERDE falso. Se marca rojo y se sigue con el resto.
    rojo "control 4: la fuente de $COMMIT_VIEJO no compila (ver $SAL/gox-ctl4.log) -- control 4 NO medido"
fi

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
# La comparacion es por (indice -> SIMBOLO), NO por direccion: las direcciones se CORREN con el layout
# (el d2mpq nuevo hace el binario mas grande: medido, la primera pasa de 8c45992c a 8c459bd8) y eso no es
# lo que este control mide. Lo que mide es que la secuencia de `init` que arma el enlazador sea la misma.
awk '{print $1, $3}' "$SAL/inits-ref.txt" > "$SAL/inits-ref.norm.txt"
awk '{print $1, $3}' "$SAL/inits-trabajo.txt" > "$SAL/inits-trabajo.norm.txt"
if diff -q "$SAL/inits-ref.norm.txt" "$SAL/inits-trabajo.norm.txt" > /dev/null 2>&1; then
    verde "control 6: la lista de \`init\` es la MISMA ($(wc -l < "$SAL/inits-ref.norm.txt") entradas, mismos simbolos y mismo orden; lo unico que cambia son las DIRECCIONES, que se corren con el layout)"
else
    rojo "control 6: la lista de \`init\` CAMBIA entre los dos binarios (ver $SAL/inits-*.norm.txt)"
fi

if [ "$MAIN_" = 0 ] && [ "$CUADROS_" = 0 ]; then
    verde "control 5 (el DEFECTO, VIGILADO): el rearme corto muere adentro de los \`init\` (main.main=0, cuadros=0, arranques=$ARRANQUES_, gc_collect=0, ultima=$(echo "$UELTIMA_" | cut -c1-40))"
else
    rojo "control 5: el rearme corto YA NO muere (main.main=$MAIN_ cuadros=$CUADROS_) -- el limite cambio: hay que REVISARLO, no taparlo"
fi

# ================================================================================================
# 7. CONSOLA: EL ARREGLO -- la camada COHERENTE del arbol de trabajo
# ================================================================================================
echo "== 7. consola: la camada COHERENTE del arbol (${COHERENTE:-/tmp/gox-330})"
COH="${COHERENTE:-/tmp/gox-330}"
if [ -f "$COH/orden-motor.txt" ] && [ -f "$COH/$PREFIJO/$P.gox" ]; then
    nota "la camada tiene $(find "$COH" -name '*.gox' | wc -l) .gox y $(wc -l < "$COH/orden-motor.txt") paquetes del motor"
    nota "d2mpq.gox: $(stat -c%s "$COH/$PREFIJO/$P.gox") B (.text $(sh-elf-objdump -h "$COH/$PREFIJO/$P.gox" | awk '$2==".text"{print "0x"$3}'))"
    T7A=$(sh-elf-objdump -h "$COH/$PREFIJO/$P.gox" | awk '$2==".text"{print $3}')
    bash "$HERE/recompilar-paquete-motor.sh" "$P" "$COH" > "$SAL/rearme-coherente.log" 2>&1
    T7B=$(sh-elf-objdump -h "$COH/$PREFIJO/$P.gox" | awk '$2==".text"{print $3}')
    [ "$T7A" = "$T7B" ] && verde "control 7a (EL ARREGLO): sobre la camada del arbol el rearme NO cambia el .text ($T7B)" ||
        rojo "control 7a: el rearme cambia el .text sobre la camada del arbol ($T7A -> $T7B)"
    corre c7-coherente "$COH" "$SEGS"
    mide c7-coherente
    nota "marcas=$MARCA_  main.main=$MAIN_  cuadros=$CUADROS_  arranques=$ARRANQUES_  gc_collect=$GC_"
    nota "ultima marca: $UELTIMA_"
    [ "$MAIN_" -ge 1 ] && verde "control 7b: con la camada del arbol el motor ARRANCA (main.main=1, $GC_ recolecciones del GC)" ||
        rojo "control 7b: con la camada del arbol el motor NO arranca (main.main=$MAIN_)"
else
    SALTADOS=$((SALTADOS + 1))
    nota "SALTADO: no hay camada coherente en $COH. Se arma con: build-std-gox.sh \$COH && build-motor-all.sh \$COH \$SAL/motor"
fi

echo
echo "== RESULTADO: $OK controles OK, $FALLOS con fallo, $SALTADOS saltados (el 5 es el que VIGILA el defecto de la tarjeta)"
echo "== el discriminador: rearmar es inocuo (control 4); el codigo nuevo contra la camada vieja muere (control 5)."
echo "== evidencia: console/dc/evidencia/dc-002.29-arranque-rearmado.txt"
[ "$FALLOS" -eq 0 ]
