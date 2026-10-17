#!/bin/bash
# CORPUS DEL SEMIESPACIO DEL GC -- RE-MEDIDO EN DC-002.25 CON EL GC SANO.
#
# HISTORIA (se corrige la medicion, no se borra): DC-002.21 midio esta escalera con el defecto de la copia
# por Store Queue PUESTO (las escrituras de la copia se descartaban: el area salia siempre 0 = la BIOS, ver
# scripts/console/fix-libgodc-sq.py). Con la copia rota el conjunto vivo del GC era PRACTICAMENTE VACIO (no
# se copiaba nada), asi que los numeros de entonces -- "256 KB alcanza", "512 KB muere en la imagen del
# atlas (diff 98304)" -- NO valen para un GC sano. DC-002.24 arreglo la copia y DC-002.25 re-midio la
# escalera: con el GC sano la escalera se DA VUELTA.
#
# LO MEDIDO EN DC-002.25 (mismo POC `motor-arranque`, mismo gox, 30 s; solo cambia GC_SEMISPACE_SIZE_KB):
#
#   KB=512  cuadros 3, 11 marcas / 1 arranque, SIN OOM, monton de C antes del motor 3968 libres,
#           PVR-FRAMEBUFFER 40776 libres   <- EL VALOR (deja el conjunto vivo al 51%, DC-002.24)
#   KB=384  cuadros 3, 11 marcas, SIN OOM, mismos 3968/40776  (arranca, pero sin margen medido)
#   KB=256  cuadros 0, 10 marcas: `fatal error: out of memory`
#           (Memory: arena=688700 used=684732 free=3968)     <- EL VALOR VIEJO (DC-002.21): ya NO alcanza
#
# CONTROLES (sobre la corrida con KB=512):
#  1. las 11 marcas MOTOR-ARRANQUE por arranque (total = 11 x arranques)
#  2. la seleccion de archivos sigue verde (Complete= true Validate.OK()= true con 7 vistos)
#  3. el config lo lee el motor (loaded configuration file from /rd/OpenDiablo2/config.json)
#  4. NO esta la pared de 98.304 bytes: el atlas de texto ENTRA
#  5. el motor ENTRA AL LOOP (pide el cuadro 0 al PVR; DC-002.22 saco la pantalla del monton)
#  6. el monton de C medido: 3968 libres antes del motor y 40.776 al crear la pantalla
#
# MUTANTE (1): el valor VIEJO del vendor, `fix-libgodc-gc.py 256 --build`. Tiene que CAER (fatal error:
# out of memory, sin llegar al loop). Es el unico mutante honesto aca porque el defecto no es una linea de
# codigo sino un PRESUPUESTO: el que lo cambia es un numero del vendor y se aplica con su script
# idempotente.
#
#   OJO, ESTE CORPUS ES ADEMAS EL VIGIA DEL GC: el par 512-arranca / 256-no-alcanza solo se sostiene con la
#   copia por Store Queue ARREGLADA. Si alguien la vuelve a romper, el conjunto vivo se vacia, 256 KB
#   vuelve a "alcanzar" y el mutante queda VERDE -> el corpus sale ROJO por mutante no cazado. El hueco no
#   se tapa: se declara (el corpus no puede pasar por verde con un GC que no copia).
#
# Uso:  bash scripts/console/checar-gc-221.sh [dir_gox] [segundos]
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

GEN="${1:-/tmp/gox-222}"
SEGS="${2:-30}"
REPO="${REPO:-$(cd "$HERE/../.." && pwd)}"
LIB=/home/dgalli/dreamcast/libgodc

export GODC_GOX="$GEN"

TOTAL_CTRL=6
CAZADOS=0
TOTAL_MUT=1

compilar() { # $1 = dir de build (relinkea el arranque con el libgodc que este)
    if ! POC=motor-arranque bash "$HERE/build-motor-arranque.sh" "$1" "$GEN" > "$1.build.log" 2>&1; then
        echo "   NO ENLAZA (ver $1.build.log)"; return 1
    fi
}

correr() { # $1 = dir de build, $2 = dir de la corrida
    rm -rf "$2"
    (cd "$REPO" && CAPTURE=none bash "$HERE/run-flycast.sh" "$1/prog-emu.elf" "$SEGS" "$2") > "$2.salida" 2>&1
    [ -f "$2/flycast.log" ]
}

# ---------------------------------------------------------------- los seis controles
# OJO (medido en DC-002.21): despues de un OOM la consola REARRANCA y el programa vuelve a correr, asi que
# el log puede repetir la secuencia. Por eso la marca no se cuenta "hasta el primer OOM" sino por ARRANQUE
# COMPLETO: cada arranque que llega al motor imprime sus 11 marcas y su linea "MOTOR-ARRANQUE arranca", asi
# que se exige que las dos cuentas cierren (total = 11 x arranques).
c_marcas() {
    local total arranques
    total=$(grep -ac 'MOTOR-ARRANQUE' "$1")
    arranques=$(grep -ac 'MOTOR-ARRANQUE arranca' "$1")
    [ "$arranques" -ge 1 ] && [ "$total" = "$((11 * arranques))" ]
}
c_seleccion()  { grep -qaF 'MOTOR-ARRANQUE seleccion: Complete= true Validate.OK()= true archivos que ve= 7 faltan= 0' "$1"; }
c_config()     { grep -qaF 'loaded configuration file from /rd/OpenDiablo2/config.json' "$1"; }
c_sin_98304()  { ! grep -qaF 'diff 98304' "$1"; }
c_loop()       { grep -qaF 'EBITEN-LOOP cuadro 0 esperando el cuadro del PVR' "$1" \
                   && grep -qaF 'EBITEN-LOOP cuadro 0 el PVR dio el cuadro, abro la lista' "$1"; }
# DC-002.25: los dos numeros son la re-medicion con el GC sano (con el defecto eran 4128 y 40936).
c_monton()     { grep -qaF 'monton de C: libre 40776 bytes' "$1" \
                   && grep -qaF 'monton de C antes del motor: 3968 bytes libres' "$1"; }

controles() { # $1 = log; imprime los seis y devuelve cuantos dan bien (en la ultima linea)
    local log="$1" bien=0
    if c_marcas "$log";      then echo "   control 1 (las 11 marcas):              VERDE"; bien=$((bien + 1)); else echo "   control 1 (las 11 marcas):              ROJO ($(grep -ac 'MOTOR-ARRANQUE' "$log"))"; fi
    if c_seleccion "$log";   then echo "   control 2 (la seleccion):               VERDE"; bien=$((bien + 1)); else echo "   control 2 (la seleccion):               ROJO"; fi
    if c_config "$log";      then echo "   control 3 (el config):                  VERDE"; bien=$((bien + 1)); else echo "   control 3 (el config):                  ROJO"; fi
    if c_sin_98304 "$log";   then echo "   control 4 (el atlas de texto ENTRA):    VERDE"; bien=$((bien + 1)); else echo "   control 4 (el atlas de texto ENTRA):    ROJO"; fi
    if c_loop "$log";        then echo "   control 5 (el motor ENTRA al loop):     VERDE"; bien=$((bien + 1)); else echo "   control 5 (el motor ENTRA al loop):     ROJO"; fi
    if c_monton "$log";      then echo "   control 6 (el monton de C medido):      VERDE"; bien=$((bien + 1)); else echo "   control 6 (el monton de C medido):      ROJO"; fi
    echo "$bien"
}

mostrar() {
    grep -a 'MOTOR-ARRANQUE seleccion\|Out of memory\|fatal error\|Memory:\|loaded config' "$1" | tr -d '\r' | sed 's/^/     /'
}

cuadros() { grep -ac 'EBITEN-LOOP cuadro [0-9]* listo' "$1"; }

# el valor viejo NO alcanza: OOM + sin loop + las marcas no cierran (10 en vez de 11)
mutante_cae() { # $1 = log
    grep -qaF 'fatal error: out of memory' "$1" || return 1
    grep -qaF 'EBITEN-LOOP cuadro 0 esperando el cuadro del PVR' "$1" && return 1
    local total arranques
    total=$(grep -ac 'MOTOR-ARRANQUE' "$1")
    arranques=$(grep -ac 'MOTOR-ARRANQUE arranca' "$1")
    [ "$arranques" -ge 1 ] && [ "$total" != "$((11 * arranques))" ]
}

# ---------------------------------------------------------------- 1. el control sano, con el valor que anda
echo "== 1. semiespacio 512 KB (el valor que deja fix-libgodc-sq.py: el conjunto vivo al 51%)"
python3 "$HERE/fix-libgodc-gc.py" 512 --build 2>&1 | sed 's/^/   /' | tail -4
compilar /tmp/dcbuild/gc221-kb512 || exit 1
correr /tmp/dcbuild/gc221-kb512 /tmp/gc221-kb512 || exit 1
BIEN="$(controles /tmp/gc221-kb512/flycast.log | sed -n '$p')"
controles /tmp/gc221-kb512/flycast.log | sed -n '1,6p'
echo "   cuadros del loop completos: $(cuadros /tmp/gc221-kb512/flycast.log)"
mostrar /tmp/gc221-kb512/flycast.log
if [ "$BIEN" -ne "$TOTAL_CTRL" ]; then
    echo "   EL CONTROL SANO NO DA ($BIEN/$TOTAL_CTRL)" >&2
    exit 1
fi
echo "   OK: $BIEN/$TOTAL_CTRL controles verdes"

# ---------------------------------------------------------------- 2. el mutante: el valor VIEJO (256 KB)
echo
echo "== 2. mutante: semiespacio 256 KB (el valor de DC-002.21, medido con la copia rota)"
python3 "$HERE/fix-libgodc-gc.py" 256 --build 2>&1 | sed 's/^/   /' | tail -4
compilar /tmp/dcbuild/gc221-kb256 || exit 1
correr /tmp/dcbuild/gc221-kb256 /tmp/gc221-kb256 || exit 1
if mutante_cae /tmp/gc221-kb256/flycast.log; then
    echo "   el MUTANTE cae: con 256 KB el arranque muere en 'fatal error: out of memory' y no llega al loop"
    echo "   ($(cuadros /tmp/gc221-kb256/flycast.log) cuadros, $(grep -ac 'MOTOR-ARRANQUE' /tmp/gc221-kb256/flycast.log) marcas de 11)"
    CAZADOS=$((CAZADOS + 1))
else
    echo "   NO LO CAZO: con 256 KB el arranque ENTRA igual -- el GC no esta copiando como debe" >&2
    echo "   (mirar scripts/console/fix-libgodc-sq.py: con la copia por Store Queue rota el conjunto vivo" >&2
    echo "    del GC queda vacio y cualquier semiespacio 'alcanza')" >&2
fi
mostrar /tmp/gc221-kb256/flycast.log

# ---------------------------------------------------------------- 3. el arbol queda en 512
echo
echo "== 3. el vendor queda en 512 KB (idempotente) y se rehace libgodc"
python3 "$HERE/fix-libgodc-gc.py" 512 --build 2>&1 | grep -a 'GC_SEMISPACE\|libgodc.a:' | sed 's/^/   /'

echo
echo "== $CAZADOS/$TOTAL_MUT mutantes cazados, $BIEN/$TOTAL_CTRL controles verdes =="
echo "   (un mutante verde = el valor viejo dejo de ser defecto O el GC dejo de copiar: las dos cosas se miran)"
[ "$BIEN" -eq "$TOTAL_CTRL" ] && [ "$CAZADOS" -eq "$TOTAL_MUT" ]
