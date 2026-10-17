#!/bin/bash
# CORPUS DE DC-002.26: EL CUADRO DEL MOTOR SALIA ROTADO 90 GRADOS.
#
# SINTOMA (captura de Papu, 2026-10-17 18:56): el arranque del motor llega al loop y dibuja su cuadro de
# error (el texto blanco sobre rojo de `updateInitError`), pero las lineas de texto corren de ARRIBA ABAJO.
#
# LOS DOS DEFECTOS, LOS DOS EN `console/dc/std/csrc/pvr_pantalla.c` (el twiddle de la pantalla del PVR):
#
#  A. EL LUGAR DE LOS BITS. La convencion del PVR es la del propio KOS (`pvr_texture.c:34`):
#
#         TWIDOUT(x, y) = TWIDTAB(y) | (TWIDTAB(x) << 1)      <- los bits de X van a los IMPARES
#
#     y en `pvr_pantalla.c` estaban al reves (la Y a los impares): eso no es un detalle de estilo, es escribir
#     la textura TRASPUESTA -- el hardware lee el pixel que espera en (x, y) y encuentra el de (y, x). El
#     cuadro del motor se ve ROTADO 90 grados. (La formula se escribio en DC-002.10 y nunca se comparo contra
#     una referencia en la orientacion: las comparaciones que si dieron identidad 100% --DC-001.3, DC-002.12--
#     son de SPRITES, que no pasan por esta funcion. Y `medir-twiddle-217.py` compara la conversion del cuadro
#     entero contra la conversion POR BLOQUES: las dos con la MISMA convencion, asi que una traspuesta no la
#     ve.)
#
#  B. LA TABLA DEL TWIDDLE ERA DE 512 Y EL LADO MENOR PUEDE SER 1024. El indice sale de `x0 & (lado_menor-1)`
#     y `y0 & (lado_menor-1)`, asi que la tabla necesita `lado_menor` entradas. Con el POC de 640x480 la
#     textura es 1024x512 (lado menor 512) y la tabla alcanzaba justo; con el MOTOR (800x600) la textura es
#     1024x1024, `twid_mask` vale 1023 y las lecturas con x0 o y0 >= 512 SALIAN DE LA TABLA: medido, 219 de
#     los 475 bloques de 32x32 del cuadro del motor, escritos donde no van.
#
# CONTROLES (corrida sana):
#  1. el instrumento de host: el indice de CADA pixel visible (800x600 y 640x480) es el que pide el PVR (la
#     convencion de KOS); ademas comprueba que la variante vieja difiere en el 99,9%, o sea que discrimina
#  2. el instrumento de host: la tabla del twiddle cubre el lado menor mas grande (1024 >= 1024)
#  3. la captura del cuadro del motor sale DERECHA: el perfil de tinta por filas varia mas que el de columnas
#     (`medir-volteo-226.py`, que ubica la pantalla por el rojo del cuadro de error y cuenta la tinta)
#  4. hay tinta (no se esta midiendo una pantalla vacia: si no, la varianza da 0 y el control no prueba nada)
#  5. el motor sigue llegando al loop y dando sus cuadros (el arreglo no rompio la conversion)
#
# MUTANTES (los dos tienen que CAZARSE):
#  A. --al-reves:  el lugar de los bits vuelve atras -> el instrumento de host dice NO y la captura vuelve a
#     salir ROTADA (las dos puntas: el indice y el emulador)
#  B. --tabla-512: la tabla vuelve a 512 entradas -> el instrumento de host dice NO (con 512 no cubre el lado
#     menor de 1024). DECLARADO: la captura SOLA no lo caza como rotacion -- con la tabla corta el texto sale
#     DERECHO pero con ARTEFACTOS (medido: tinta 5831 px contra 3196 del sano y la pantalla roja que detecta
#     el instrumento mide 483 px de ancho contra 604: los bloques de x0/y0 >= 512 se escriben donde no van).
#     El mutante se caza por el instrumento que mide justo lo que el defecto rompe.
#
# Uso:  bash scripts/console/checar-twiddle-226.sh [dir_gox] [segundos]
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

GEN="${1:-/tmp/gox-222}"
SEGS="${2:-15}"
REPO="${REPO:-$(cd "$HERE/../.." && pwd)}"
MUT="$HERE/mutar-twiddle-226.py"
MEDIR="$HERE/medir-volteo-226.py"
VERIF="$HERE/verificar-twiddle-226.py"
PAV="$REPO/console/dc/std/csrc/pvr_pantalla.c"

export GODC_GOX="$GEN"

TOTAL_CTRL=5
TOTAL_MUT=2
CAZADOS=0
BIEN=0

SHA_INI="$(sha256sum "$PAV" | cut -c1-16)"

compilar() { # $1 = dir de build
    if ! POC=motor-arranque bash "$HERE/build-motor-arranque.sh" "$1" "$GEN" > "$1.build.log" 2>&1; then
        echo "   NO ENLAZA (ver $1.build.log)" >&2
        return 1
    fi
}

correr() { # $1 = dir de build, $2 = dir de la corrida
    rm -rf "$2"
    (cd "$REPO" && CAPTURE=spectacle bash "$HERE/run-flycast.sh" "$1/prog-emu.elf" "$SEGS" "$2") \
        > "$2.salida" 2>&1
    [ -f "$2/flycast.log" ]
}

# la captura que se mide: f6s (los cuadros del motor ya estan dibujados y es de las primeras que se sacan)
captura() { echo "$1/f6s.png"; }

medir() { # $1 = dir de la corrida -> el veredicto por stdout, los numeros del instrumento por stderr
    local cap out
    cap="$(captura "$1")"
    [ -f "$cap" ] || { echo "sin-captura"; return; }
    out="$(python3 "$MEDIR" "$cap")"
    echo "$out" >&2
    echo "$out" | grep -ao 'DERECHO\|ROTADO 90 GRADOS' | head -1
}

verificar() { # el instrumento de host: imprime su salida y devuelve 0 si esta bien
    local out
    out="$(python3 "$VERIF" 2>&1)"
    echo "$out" | sed 's/^/     /'
    echo "$out" | grep -qa 'RESULTADO: OK'
}

cuadros() { grep -ac 'EBITEN-LOOP cuadro [0-9]* listo' "$1/flycast.log" 2>/dev/null; }

# ---------------------------------------------------------------- 1. mutante A: el lugar de los bits
echo "== 1. mutante A (--al-reves): el lugar de los bits del twiddle vuelve atras"
python3 "$MUT" --al-reves || exit 1
VH="NO"; verificar && VH="SI"
compilar "/tmp/dcbuild/tw226-reves" || exit 1
correr "/tmp/dcbuild/tw226-reves" "/tmp/tw226-reves" || exit 1
V="$(medir "/tmp/tw226-reves")"
echo "   cuadros del loop: $(cuadros "/tmp/tw226-reves")"
if [ "$VH" = "NO" ] && echo "$V" | grep -qa 'ROTADO'; then
    echo "   CAZADO: el instrumento de host dice NO y la captura vuelve a salir ROTADA"
    CAZADOS=$((CAZADOS + 1))
else
    echo "   NO LO CAZO (host=$VH, captura=$V)" >&2
fi

# ---------------------------------------------------------------- 2. mutante B: la tabla de 512
echo
echo "== 2. mutante B (--tabla-512): la tabla del twiddle vuelve a 512 entradas"
python3 "$MUT" --restaurar || exit 1
python3 "$MUT" --tabla-512 || exit 1
VH="NO"; verificar && VH="SI"
compilar "/tmp/dcbuild/tw226-t512" || exit 1
correr "/tmp/dcbuild/tw226-t512" "/tmp/tw226-t512" || exit 1
V="$(medir "/tmp/tw226-t512")"
ANCHO="$(python3 "$MEDIR" "$(captura "/tmp/tw226-t512")" | grep -ao 'pantalla de la consola: [0-9]*' | grep -ao '[0-9]*$')"
echo "   cuadros del loop: $(cuadros "/tmp/tw226-t512")"
if [ "$VH" = "NO" ]; then
    echo "   CAZADO por el instrumento de host (la tabla no cubre el lado menor del motor)"
    echo "   DECLARADO: la captura sola sale '$V' -- con la tabla corta el texto queda DERECHO pero con"
    echo "   ARTEFACTOS (los bloques de x0/y0 >= 512 se escriben donde no van); la pantalla roja mide"
    echo "   $ANCHO px de ancho"
    CAZADOS=$((CAZADOS + 1))
else
    echo "   NO LO CAZO: el instrumento de host no vio la tabla corta (captura: $V)" >&2
fi

# ---------------------------------------------------------------- 3. el control sano
echo
echo "== 3. control sano (el twiddle arreglado: X a los bits impares y la tabla de 1024)"
python3 "$MUT" --restaurar || exit 1
if verificar; then echo "   control 1 (el indice es el del PVR):        VERDE"; BIEN=$((BIEN + 1))
else echo "   control 1 (el indice es el del PVR):        ROJO"; fi
if python3 "$VERIF" 2>&1 | grep -qa '1024 >= 1024: OK'; then
    echo "   control 2 (la tabla cubre el lado menor):   VERDE"; BIEN=$((BIEN + 1))
else echo "   control 2 (la tabla cubre el lado menor):   ROJO"; fi
compilar "/tmp/dcbuild/tw226-sano" || exit 1
correr "/tmp/dcbuild/tw226-sano" "/tmp/tw226-sano" || exit 1
V="$(medir "/tmp/tw226-sano")"
CUADROS="$(cuadros "/tmp/tw226-sano")"
TINTA="$(python3 "$MEDIR" "$(captura "/tmp/tw226-sano")" | grep -ao 'tinta (no son el fondo): [0-9]*' | grep -ao '[0-9]*$')"
if echo "$V" | grep -qa 'DERECHO'; then echo "   control 3 (la captura sale DERECHA):        VERDE"; BIEN=$((BIEN + 1))
else echo "   control 3 (la captura sale DERECHA):        ROJO"; fi
if [ "${TINTA:-0}" -gt 200 ]; then echo "   control 4 (hay tinta, $TINTA px):           VERDE"; BIEN=$((BIEN + 1))
else echo "   control 4 (hay tinta):                      ROJO ($TINTA px)"; fi
if [ "${CUADROS:-0}" -ge 1 ]; then echo "   control 5 (el motor llega al loop: $CUADROS cuadros): VERDE"; BIEN=$((BIEN + 1))
else echo "   control 5 (el motor llega al loop):         ROJO"; fi

# ---------------------------------------------------------------- 4. el arbol queda sano
echo
echo "== 4. el arbol queda sin mutaciones (idempotente)"
python3 "$MUT" --estado
SHA_FIN="$(sha256sum "$PAV" | cut -c1-16)"
if [ "$SHA_FIN" = "$SHA_INI" ]; then
    echo "   OK: pvr_pantalla.c con el mismo sha256 que al empezar ($SHA_FIN)"
else
    echo "   OJO: el sha256 cambio ($SHA_INI -> $SHA_FIN)" >&2
    exit 1
fi

echo
echo "== $CAZADOS/$TOTAL_MUT mutantes cazados, $BIEN/$TOTAL_CTRL controles verdes =="
[ "$CAZADOS" -eq "$TOTAL_MUT" ] && [ "$BIEN" -eq "$TOTAL_CTRL" ]
