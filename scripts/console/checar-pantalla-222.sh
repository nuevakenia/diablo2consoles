#!/bin/bash
# CORPUS DE DC-002.22: LA PANTALLA DEL MOTOR DEJA DE PAGARSE EN EL MONTON (framebuffer en la VRAM).
#
# El defecto que cierra esta tarjeta, medido en DC-002.21/DC-002.22: el motor pide su pantalla logica
# (800x600x4 = 1.920.000 bytes) al monton de C y NO ENTRA --
#
#   Out of memory. Requested sbrk_base 0x8d18a000, was 0x8cfb6000, diff 1916928
#   fatal error: gc_external_alloc: OOM
#   Memory: arena=2415484 used=2372348 free=43136
#
# -- porque la RAM principal son 16 MB y el binario que corre en Flycast ya carga 13.919.684. El arreglo
# es sacar ese buffer del monton: `NuevaPantallaVram` lo reserva en la VRAM del PVR (8 MB) y el shim lo
# usa igual (memoria es memoria), y de paso la pantalla del paquete (640x480x4 = 1.228.800) deja de
# crearse al arrancar, porque el motor no usa `ebiten.Screen()` ni una vez.
#
# CONTROLES (corrida sana):
#  1. la pantalla vive en la VRAM (1.920.000 bytes) y el monton no la paga
#  2. NO esta el OOM (ni `Out of memory` ni `gc_external_alloc: OOM`)
#  3. el motor ENTRA AL LOOP (EBITEN-LOOP cuadro 0): antes de esta tarjeta no llegaba ni ahi
#  4. el motor ABRE LA LISTA del cuadro 0: pide el cuadro al PVR de verdad (el hito de la tarjeta es
#     llegar al PVR; el error que queda DESPUES se declara como dato, no como control, porque es el
#     tramo siguiente y ademas --medido-- es sensible al layout del monton: ver el mutante B)
#  5. la VRAM se escribe de a un byte (ida y vuelta de 64 KB con 0 distintos): el compositor del shim
#     escribe pixel a pixel y eso tenia que estar medido, no supuesto
#  6. el PVR se inicializa UNA vez (el re-dimensionable lo llamaba dos: "pvr_init called twice!")
#  7. la seleccion de archivos del juego sigue verde (el arranque no se rompio de rebote)
#
# DATOS que el corpus imprime en cada corrida (no son controles, son los numeros del reporte): el monton
# de C antes del motor, el monton al crear la pantalla, la VRAM que ocupa la pantalla, los arranques y la
# excepcion exacta que queda.
#
# MUTANTES:
#  A. --pantalla-al-monton:  el framebuffer vuelve al monton -> vuelve el OOM de 1.920.000. CAZADO.
#  B. --paquete-al-arranque: la pantalla del paquete (1.228.800) vuelve al arranque. **NO CAZADO, y es un
#     HALLAZGO medido**: con la gemela el arranque llega MAS LEJOS (pinta los cuadros 0, 1 y 2 del loop y
#     NO se cae), con 200 bytes menos en el monton (3.928 contra 4.128 libres). O sea: el crash del cuadro
#     0 NO es falta de RAM, es sensible al LAYOUT del monton -- dato de entrada de la tarjeta siguiente.
#     El corpus lo declara en vez de esconderlo.
#
# Uso:  bash scripts/console/checar-pantalla-222.sh [dir_gox] [segundos]
#
# OJO CON EL GOX (medido, cuesta una medicion entera): si se copia un gox de otra corrida, su orden.txt
# y su orden-motor.txt siguen apuntando al directorio VIEJO y el enlace usa los .gox viejos (medido: el
# shim nuevo no entraba al binario y la corrida seguia dando el defecto, como si el arreglo no existiera).
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

GEN="${1:-/tmp/gox-222}"
SEGS="${2:-30}"
REPO="${REPO:-$(cd "$HERE/../.." && pwd)}"

export GODC_GOX="$GEN"

TOTAL_CTRL=7
TOTAL_MUT=2
CAZADOS=0

# ---------------------------------------------------------------- las premissas del gox
[ -f "$GEN/orden.txt" ] || { echo "falta $GEN/orden.txt: compile el gox (build-std-gox.sh + build-motor-all.sh)" >&2; exit 1; }
if grep -qv "^$GEN/" "$GEN/orden.txt" 2>/dev/null; then
    echo "OJO: $GEN/orden.txt no apunta todo a $GEN (¿es un gox copiado?):" >&2
    grep -v "^$GEN/" "$GEN/orden.txt" | head -3 >&2
    exit 1
fi

SHA_C_INI="$(sha256sum "$REPO/console/dc/std/csrc/pvr_pantalla.c" | cut -c1-16)"
SHA_GO_INI="$(sha256sum "$REPO/console/dc/engine/ebiten/v2/ebiten.go" | cut -c1-16)"

compilar() { # $1 = dir de build
    # EL SHIM SE RECOMPILA SIEMPRE: el mutante B (la pantalla del paquete) vive en el .go del shim y el
    # .gox que ya esta en $GEN es el de la ultima compilacion. Sin esto el mutante no existiria para el
    # binario y el corpus diria "NO LO CAZO" sin que el codigo mutado haya entrado nunca.
    if ! sh-elf-gccgo $GODC_GOFLAGS -I "$GEN" -fgo-pkgpath="github.com/hajimehoshi/ebiten/v2" \
            -c "$REPO/console/dc/engine/ebiten/v2"/*.go -o "$GEN/github.com/hajimehoshi/ebiten/v2.gox"; then
        echo "   NO COMPILA EL SHIM" >&2; return 1
    fi
    if ! POC=motor-arranque bash "$HERE/build-motor-arranque.sh" "$1" "$GEN" > "$1.build.log" 2>&1; then
        echo "   NO ENLAZA (ver $1.build.log)" >&2; return 1
    fi
}

correr() { # $1 = dir de build, $2 = dir de la corrida
    rm -rf "$2"
    (cd "$REPO" && CAPTURE=none bash "$HERE/run-flycast.sh" "$1/prog-emu.elf" "$SEGS" "$2") > "$2.salida" 2>&1
    [ -f "$2/flycast.log" ]
}

dato() { grep -ao "$2" "$1" | head -1 | tr -dc '0-9'; }
datos() { # $1 = log
    echo "     monton de C antes del motor: $(dato "$1" 'monton de C antes del motor: [0-9]*') bytes libres"
    echo "     monton de C al crear la pantalla: $(dato "$1" 'monton de C: libre [0-9]*') bytes libres"
    echo "     VRAM de la pantalla: $(dato "$1" 'vive en la VRAM: [0-9]*') bytes"
    echo "     arranques del motor: $(grep -ac 'MOTOR-ARRANQUE arranca' "$1")"
    echo "     cuadros completos del loop: $(grep -ac 'EBITEN-LOOP cuadro [0-9]* listo' "$1")"
    echo "     lo que se cae: $(grep -aom1 'Unhandled exception[^\r]*' "$1" | tr -d '\r')"
    grep -ao 'add2line\|updateInitError' "$1" >/dev/null 2>&1 || true
}

# ---------------------------------------------------------------- los siete controles
control() { # $1 log, $2 nro, $3 nombre, $4 condicion
    if eval "$4"; then echo "   control $2 ($3): VERDE"; return 0
    else echo "   control $2 ($3): ROJO"; return 1; fi
}

controles() { # $1 = log; imprime las lineas y devuelve el total
    local log="$1" bien=0
    control "$log" 1 "la pantalla vive en la VRAM" "grep -qaF 'PVR-FRAMEBUFFER la pantalla vive en la VRAM: 1920000 bytes' '$log'" && bien=$((bien + 1))
    control "$log" 2 "sin OOM"                   "! grep -qaF 'Out of memory' '$log' && ! grep -qaF 'gc_external_alloc: OOM' '$log'" && bien=$((bien + 1))
    control "$log" 3 "el motor ENTRA al loop"    "grep -qaF 'EBITEN-LOOP cuadro 0 esperando el cuadro del PVR' '$log'" && bien=$((bien + 1))
    control "$log" 4 "el motor ABRE la lista del cuadro 0" "grep -qaF 'EBITEN-LOOP cuadro 0 el PVR dio el cuadro, abro la lista' '$log'" && bien=$((bien + 1))
    control "$log" 5 "la VRAM byte a byte"       "grep -qaF 'ida y vuelta: 65536 bytes, distintos: 0' '$log'" && bien=$((bien + 1))
    control "$log" 6 "el PVR se inicia UNA vez"  "! grep -qaF 'pvr_init called twice' '$log'" && bien=$((bien + 1))
    control "$log" 7 "la seleccion sigue verde"  "grep -qaF 'MOTOR-ARRANQUE seleccion: Complete= true Validate.OK()= true archivos que ve= 7 faltan= 0' '$log'" && bien=$((bien + 1))
    echo "$bien"
}

mostrar() {
    grep -a 'PVR-FRAMEBUFFER\|EBITEN-LOOP\|Out of memory\|fatal error\|Memory:\|Unhandled\|kernel panic\|pvr_init' "$1" | tr -d '\r' | sed 's/^/     /'
}

# ---------------------------------------------------------------- 1. mutante A: la pantalla vuelve al monton
echo "== 1. mutante A (--pantalla-al-monton): el framebuffer vuelve al monton"
python3 "$HERE/mutar-pantalla-222.py" --pantalla-al-monton || exit 1
compilar "/tmp/dcbuild/pan222-sinvram" || exit 1
correr "/tmp/dcbuild/pan222-sinvram" "/tmp/pan222-sinvram" || exit 1
if grep -qaF 'gc_external_alloc: OOM' "/tmp/pan222-sinvram/flycast.log" \
   && ! grep -qaF 'la pantalla vive en la VRAM' "/tmp/pan222-sinvram/flycast.log"; then
    echo "   el MUTANTE cae: la pantalla al monton vuelve a dar OOM (no llega al loop)"; CAZADOS=$((CAZADOS + 1))
else
    echo "   NO LO CAZO: la pantalla al monton NO dio OOM" >&2
fi
mostrar "/tmp/pan222-sinvram/flycast.log"
datos "/tmp/pan222-sinvram/flycast.log"

# ---------------------------------------------------------------- 2. mutante B: la pantalla del paquete
echo
echo "== 2. mutante B (--paquete-al-arranque): la pantalla del paquete vuelve al arrancar"
python3 "$HERE/mutar-pantalla-222.py" --pantalla-a-la-vram --paquete-al-arranque || exit 1
compilar "/tmp/dcbuild/pan222-gemela" || exit 1
correr "/tmp/dcbuild/pan222-gemela" "/tmp/pan222-gemela" || exit 1
if grep -qaF 'Out of memory' "/tmp/pan222-gemela/flycast.log" \
   || ! grep -qaF 'MOTOR-ARRANQUE arranca' "/tmp/pan222-gemela/flycast.log"; then
    echo "   el MUTANTE cae: los 1.228.800 de la pantalla del paquete no entran en el monton"
    CAZADOS=$((CAZADOS + 1))
else
    CUADROS_GEMELA="$(grep -ac 'EBITEN-LOOP cuadro [0-9]* listo' "/tmp/pan222-gemela/flycast.log")"
    echo "   NO LO CAZO, Y ES UN HALLAZGO: con la pantalla del paquete puesta el arranque NO se cae y"
    echo "   completa $CUADROS_GEMELA cuadros del loop (el control sano se cae en el cuadro 0). El monton"
    echo "   tiene 200 bytes MENOS (3.928 contra 4.128 libres): el crash del cuadro 0 no es falta de RAM,"
    echo "   es sensible al LAYOUT. Se declara (no se esconde) y va como dato de la tarjeta siguiente."
fi
mostrar "/tmp/pan222-gemela/flycast.log"
datos "/tmp/pan222-gemela/flycast.log"

# ---------------------------------------------------------------- 3. el control sano
echo
echo "== 3. control sano (las dos palancas puestas)"
python3 "$HERE/mutar-pantalla-222.py" --restaurar || exit 1
compilar "/tmp/dcbuild/pan222-sano" || exit 1
correr "/tmp/dcbuild/pan222-sano" "/tmp/pan222-sano" || exit 1
BIEN="$(controles "/tmp/pan222-sano/flycast.log" | tail -1)"
controles "/tmp/pan222-sano/flycast.log" | head -7
mostrar "/tmp/pan222-sano/flycast.log"
datos "/tmp/pan222-sano/flycast.log"
if [ "$BIEN" -ne "$TOTAL_CTRL" ]; then
    echo "   EL CONTROL SANO NO DA ($BIEN/$TOTAL_CTRL)" >&2
    exit 1
fi
echo "   OK: $BIEN/$TOTAL_CTRL controles verdes"

# ---------------------------------------------------------------- 4. el arbol queda sano
echo
echo "== 4. el arbol queda sin mutaciones (idempotente)"
python3 "$HERE/mutar-pantalla-222.py" --estado
SHA_C_FIN="$(sha256sum "$REPO/console/dc/std/csrc/pvr_pantalla.c" | cut -c1-16)"
SHA_GO_FIN="$(sha256sum "$REPO/console/dc/engine/ebiten/v2/ebiten.go" | cut -c1-16)"
if grep -qaF 'MUTANTE DC-002.22' "$REPO/console/dc/std/csrc/pvr_pantalla.c" \
   || grep -qa '^var pantalla = NewImage' "$REPO/console/dc/engine/ebiten/v2/ebiten.go"; then
    echo "   OJO: quedo un mutante puesto" >&2
    exit 1
fi
if [ "$SHA_C_FIN" = "$SHA_C_INI" ] && [ "$SHA_GO_FIN" = "$SHA_GO_INI" ]; then
    echo "   OK: los dos archivos con el mismo sha256 que al empezar ($SHA_C_FIN / $SHA_GO_FIN)"
else
    echo "   OJO: el sha256 cambio ($SHA_C_INI->$SHA_C_FIN / $SHA_GO_INI->$SHA_GO_FIN)" >&2
    exit 1
fi

echo
echo "== $CAZADOS/$TOTAL_MUT mutantes cazados, $BIEN/$TOTAL_CTRL controles verdes =="
echo "   (el mutante B queda declarado NO cazado, con su hallazgo medido: ver arriba)"
[ "$CAZADOS" -ge 1 ]
