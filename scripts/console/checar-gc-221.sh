#!/bin/bash
# CORPUS DE DC-002.21: EL MONTON DEL MOTOR -- el semiespacio del GC contra las paredes de RAM.
# ALINEADO EN DC-002.22: el motor ya no pide su pantalla al monton (vive en la VRAM), asi que la pared de
# 1.916.928 bytes DEJO DE EXISTIR y el arranque ENTRA AL LOOP. Los controles 5 y 6, y las marcas por
# arranque (10 -> 11), se movieron con la medicion. Ademas `GEN` pasa a ser el gox de la rama: el POC de
# hoy usa `ebiten.MemLibreDelMonton`/`VramDelFramebuffer`, que NO estan en el gox de DC-002.20 y el
# reenlace no enlazaba (medido: "reference to undefined identifier 'ebiten.MemLibreDelMonton'").
#
# El arranque del motor, con el gzip ya arreglado (DC-002.20), moria pidiendo 98.304 bytes contiguos para
# la imagen del atlas de texto, con 3.096 bytes libres en el monton de C (newlib: arena 2.579.164). Esta
# tarjeta mide el semiespacio del GC (2 x GC_SEMISPACE_SIZE_KB, que sale del MISMO monton) y las paredes que
# quedan en cada valor:
#
#   KB=512  (el valor que dejo DC-002.14)  ->  Out of memory ... diff 98304    (la imagen del atlas)
#   KB=256  (esta tarjeta)                 ->  Out of memory ... diff 1916928  (el atlas ya entra, y lo que
#                                              no entra es la pantalla del motor -- MEDIDO en DC-002.22:
#                                              800x600x4 = 1.920.000 bytes; hoy esa pantalla vive en la
#                                              VRAM y el OOM no esta)
#
# El "mutante" de esta tarjeta es el valor VIEJO: volver a 512 tiene que hacer reaparecer la pared de 98.304
# bytes. Es el unico mutante honesto aca, porque el defecto no es una linea de codigo sino un PRESUPUESTO:
# el que lo cambia es un numero del vendor y se aplica con su script idempotente.
#
# CONTROLES (sobre la corrida con KB=256):
#  1. las 11 marcas MOTOR-ARRANQUE por arranque (el arranque no cambio de forma)
#  2. la seleccion de archivos sigue verde (Complete y Validate.OK con 7 vistos)
#  3. el config lo lee el motor (loaded configuration file from ...)
#  4. NO esta la pared de 98.304 bytes: el atlas de texto ENTRA
#  5. el motor ENTRA AL LOOP (DC-002.22): pide el cuadro 0 al PVR (la pared de 1.916.928 ya no esta)
#  6. el monton de C medido: 4.128 libres antes del motor y 40.936 al crear la pantalla (el pedido grande
#     ya no sale de ahi)
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
# OJO (medido): despues del OOM la consola REARRANCA y el programa vuelve a correr, asi que el log repite la
# secuencia (medido: 50 marcas = 5 arranques). Y el texto del OOM NO sale en todos los arranques: en 4 de 5
# solo quedo la secuencia de marcas (medido). Por eso la marca no se cuenta "hasta el primer OOM" sino por
# ARRANQUE COMPLETO: cada arranque imprime sus 10 marcas y su linea "MOTOR-ARRANQUE arranca", asi que se
# exige que las dos cuentas cierren (total = 10 x arranques).
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
c_monton()     { grep -qaF 'monton de C: libre 40936 bytes' "$1" \
                   && grep -qaF 'monton de C antes del motor: 4128 bytes libres' "$1"; }

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

# ---------------------------------------------------------------- 1. la pared de 512 KB (el valor viejo)
echo "== 1. semiespacio 512 KB (el valor de DC-002.14: la pared de la imagen del atlas)"
python3 "$HERE/fix-libgodc-gc.py" 512 --build 2>&1 | sed 's/^/   /' | tail -4
compilar /tmp/dcbuild/gc221-kb512 || exit 1
correr /tmp/dcbuild/gc221-kb512 /tmp/gc221-kb512 || exit 1
if grep -qaF 'diff 98304' /tmp/gc221-kb512/flycast.log; then
    echo "   el MUTANTE cae: con 512 KB vuelve la pared de 98.304 bytes (el atlas de texto no entra)"
    CAZADOS=$((CAZADOS + 1))
else
    CUADROS512="$(grep -ac 'EBITEN-LOOP cuadro [0-9]* listo' /tmp/gc221-kb512/flycast.log)"
    echo "   NO LO CAZO, Y LA RAZON ES BUENA (medida en DC-002.22): con el framebuffer del motor YA FUERA"
    echo "   del monton, 512 KB no lo ahoga -- completa $CUADROS512 cuadros del loop (el control sano se cae en"
    echo "   el cuadro 0). O sea: el valor viejo del semiespacio dejo de ser un defecto. El presupuesto del GC"
    echo "   lo sigue vigilando el comodin de checar-recorte-214.sh (el GC en 2048 KB muere en gc_init)."
    echo "   Se declara (no se esconde): un mutante que no cae es informacion, no un verde que se aprueba."
fi
mostrar /tmp/gc221-kb512/flycast.log

# ---------------------------------------------------------------- 2. el valor de esta tarjeta
echo
echo "== 2. semiespacio 256 KB (esta tarjeta)"
python3 "$HERE/fix-libgodc-gc.py" 256 --build 2>&1 | sed 's/^/   /' | tail -4
compilar /tmp/dcbuild/gc221-kb256 || exit 1
correr /tmp/dcbuild/gc221-kb256 /tmp/gc221-kb256 || exit 1
BIEN="$(controles /tmp/gc221-kb256/flycast.log | sed -n '$p')"
controles /tmp/gc221-kb256/flycast.log | sed -n '1,6p'
mostrar /tmp/gc221-kb256/flycast.log
if [ "$BIEN" -ne "$TOTAL_CTRL" ]; then
    echo "   EL CONTROL SANO NO DA ($BIEN/$TOTAL_CTRL)" >&2
    exit 1
fi
echo "   OK: $BIEN/$TOTAL_CTRL controles verdes"

# ---------------------------------------------------------------- 3. el arbol queda en 256
echo
echo "== 3. el vendor queda en 256 KB (idempotente) y se verifica"
python3 "$HERE/fix-libgodc-gc.py" 256 2>&1 | grep -a 'GC_SEMISPACE\|semiespacio' | sed 's/^/   /'

echo
echo "== $CAZADOS/$TOTAL_MUT mutantes cazados, $BIEN/$TOTAL_CTRL controles verdes =="
echo "   (el mutante del semiespacio queda declarado NO cazado con su razon medida: DC-002.22 lo dejo sin causa)"
[ "$BIEN" -eq "$TOTAL_CTRL" ]
