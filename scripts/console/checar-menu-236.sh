#!/bin/bash
# CORPUS DE DC-002.36: EL MENU PRINCIPAL EN LA DREAMCAST (dibujado y navegable), con la pantalla CONTADA.
#
# QUE MIDE (los tres criterios de la tarjeta, con numeros)
#   1. la pantalla del menu capturada, con sus elementos contados: los pixeles que NO son el fondo (el PVR
#      pinta el fondo de negro) y los colores dominantes, mas la coincidencia contra la referencia que arma
#      el HOST con el CODIGO DEL MOTOR (referencia-foco-1-640.png, 640x480).
#   2. la navegacion con el control: se manda 'abajo' por el TECLADO VIRTUAL de /dev/uinput (en KDE/Wayland
#      no hay xdotool) y se cuentan los pixeles que CAMBIAN en la pantalla al mover la seleccion, con la
#      caja donde cambian.
#   3. el MUTANTE: sin el plan del menu no hay nada que dibujar y la pantalla tiene que quedar como estaba
#      (negra): si el corpus sigue viendo elementos, el que dibuja no es el que dice la tarjeta.
#
# EL FONDO, DECLARADO (defecto medido, no escondido)
#   Con los 11 assets el POC MUERE al subir la textura 0 (el FONDO) a la VRAM: el plan esta bien leido
#   (`PLAN-LEIDO id= 0 ... ph= 1024 banco= 0`) y al salir de `armarDC6` la MISMA textura tiene `ph` y
#   `banco` con basura con pinta de puntero (`ph= -2147482624 banco= -1942452840`): algo escribe fuera de su
#   buffer al decodificar los frames del fondo (el tope explicito que se le puso al pegado de la grilla NO
#   se dispara: no es el pegado). Por eso la corrida "el menu se ve" va SIN el fondo (SIN_ASSET=FONDO.DC6):
#   los otros 10 elementos (4 logos, 4 fuentes, 2 botones) se cargan, se suben y se dibujan -- 281 quads por
#   cuadro, medido. Es el mismo sintoma de fondo que la tarjeta DC-002.39 (la consola corrompe un dato vivo).
#
# Uso:  bash scripts/console/checar-menu-236.sh [segundos]
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

SEGS="${1:-45}"
REPO="${REPO:-$(cd "$HERE/../.." && pwd)}"
SAL="${SAL:-/home/dgalli/.hermes/cache/scratch/corpus-menu-236}"
BUILD="${BUILD:-$HOME/.hermes/cache/scratch/dc36/build}"
REF="$BUILD/MENU/referencias/referencia-foco-1-640.png"
MEDIR="$HERE/medir-menu-236.py"
GEOS=""

OK=0
FALLOS=0
verde() { OK=$((OK + 1)); echo "   OK: $*"; }
rojo() { FALLOS=$((FALLOS + 1)); echo "   FALLO: $*"; }
nota() { echo "   $*"; }

mkdir -p "$SAL"

# corre el ELF en el emulador con la ventana identificada y capturas; deja el log en $SAL/<nombre>/flycast.log
corre() { # $1 nombre  $2 TECLAS
    local nom="$1"; local teclas="${2:-}"
    local d="$SAL/$nom"
    mkdir -p "$d"
    TECLAS="$teclas" bash "$HERE/run-flycast-236.sh" "$BUILD/prog.elf" "$SEGS" "$d" > "$d/run.log" 2>&1
    GEOS="$(grep -a 'geo=' "$d/run.log" | tail -1 | sed 's/.*geo=//')"
    nota "$nom: ventana (marco) $GEOS"
}

# los pixeles que NO son el fondo de la pantalla de la consola, del log del instrumento
pixeles_no_fondo() { # $1 log del instrumento
    tr -d '\r' < "$1" | grep -a 'CAPTURA: pixeles que NO son el fondo' | tail -1 |
        sed 's/.*: \([0-9]*\) de \([0-9]*\) (\(.*\)).*/\1 de \2 (\3)/'
}

cuadros_dibujados() { # $1 flycast.log
    tr -d '\r' < "$1" | grep -a 'MENU236: CUADROS' | tail -1 | sed 's/.*dibujados= *\([0-9]*\).*/\1/'
}

# ==============================================================================================
echo "== 0. el instrumento y la referencia"
if [ -f "$REF" ]; then
    verde "la referencia del host esta: $(basename "$REF") ($(stat -c%s "$REF") bytes)"
else
    rojo "no esta la referencia $REF (corre build-menu-236.sh)"
fi

# ==============================================================================================
echo "== 1. EL MENU SE VE (sin el fondo, que es el defecto declarado) y se NAVEGA"
SIN_ASSET=FONDO.DC6 SALTAR_HOST=1 bash "$HERE/build-menu-236.sh" > "$SAL/build-menuseve.log" 2>&1
if grep -qa 'romdisk: 19 archivos' "$SAL/build-menuseve.log"; then
    verde "el disco/romdisk del menu se armo con los otros 19 archivos (falta FONDO.DC6, el mutante del fondo)"
else
    rojo "el build no armo el romdisk esperado (ver $SAL/build-menuseve.log)"
fi

corre menuseve "15:abajo 25:abajo"
L="$SAL/menuseve/flycast.log"

TEXTURAS="$(tr -d '\r' < "$L" | grep -a 'MENU236: TEXTURAS subidas=' | tail -1 | sed 's/.*subidas= *\([0-9]*\).*/\1/')"
FALTAN="$(tr -d '\r' < "$L" | grep -a 'MENU236: LIENZOS listos' | tail -1 | sed 's/.*faltan= *\([0-9]*\).*/\1/')"
if [ "${TEXTURAS:-0}" -ge 10 ]; then
    verde "control 1: las texturas subidas a la VRAM son ${TEXTURAS} (faltan ${FALTAN}: el fondo)"
else
    rojo "control 1: solo ${TEXTURAS:-0} texturas subidas (ver $L)"
fi

DIB="$(cuadros_dibujados "$L")"
if [ "${DIB:-0}" -gt 0 ]; then
    verde "control 1b: el POC dibuja ${DIB} quads por cuadro (el menu esta en pantalla)"
else
    rojo "control 1b: no dibujo ningun quad (ver $L)"
fi

FOCO="$(tr -d '\r' < "$L" | grep -ac 'MENU236: FOCO')"
nota "las marcas de FOCO en la consola serie: $(tr -d '\r' < "$L" | grep -a 'MENU236: FOCO' | tail -2 | tr '\n' ' ')"

if [ -n "$GEOS" ]; then
    for t in 6 10 15 25; do
        [ -f "$SAL/menuseve/f${t}s.png" ] || continue
        python3 "$MEDIR" pinta "$SAL/menuseve/f${t}s.png" --referencia "$REF" --ventana "$GEOS" \
            > "$SAL/menuseve/pinta-${t}s.txt" 2>&1
    done

    NOF="$(pixeles_no_fondo "$SAL/menuseve/pinta-10s.txt")"
    if [ -n "$NOF" ]; then
        verde "criterio 1 (captura de los 10 s, foco -1): pixeles que NO son el fondo = $NOF"
    else
        rojo "criterio 1: el instrumento no midio la captura (ver $SAL/menuseve/pinta-10s.txt)"
    fi
    nota "colores dominantes: $(tr -d '\r' < "$SAL/menuseve/pinta-10s.txt" | grep -a -A4 'colores dominantes' | tail -4 | tr '\n' ' ')"
    nota "contra la referencia del host: $(tr -d '\r' < "$SAL/menuseve/pinta-10s.txt" | grep -a 'contra la referencia del host' | head -1)"

    # Dos pares: el primero (10s->15s) es INFORMATIVO porque la tecla de /dev/uinput va y vuelve (apretar y
    # soltar) y el POC ve DOS bordes: si la seleccion entra y sale de un boton, la pantalla del primero
    # puede quedar igual que la del inicio. El par que manda es el segundo (15s->25s): ahi la seleccion
    # queda puesta en un boton y el cuadro del boton cambia.
    for par in "10 15 sin_foco_a_foco0 blando" "15 25 foco0_a_foco1 duro"; do
        set -- $par
        A="$SAL/menuseve/f${1}s.png"; B="$SAL/menuseve/f${2}s.png"
        [ -f "$A" ] && [ -f "$B" ] || continue
        python3 "$MEDIR" navega "$A" "$B" --ventana "$GEOS" > "$SAL/menuseve/navega-${3}.txt" 2>&1
        N="$(tr -d '\r' < "$SAL/menuseve/navega-${3}.txt" | grep -a 'pixeles que CAMBIAN' | sed 's/.*canal): \([0-9]*\) de \([0-9]*\) (\(.*\)).*/\1 de \2 (\3)/')"
        CAJA="$(tr -d '\r' < "$SAL/menuseve/navega-${3}.txt" | grep -a 'la caja donde cambian:' | head -1)"
        if [ -n "$N" ] && [ "${N%% *}" -gt 0 ]; then
            verde "criterio 2 ($3, entre los ${1}s y los ${2}s): pixeles que cambian = $N"
            nota "$CAJA"
        elif [ "${4:-duro}" = "blando" ]; then
            nota "criterio 2 ($3): 0 pixeles (la tecla fue y volvio: la seleccion quedo como al principio; el par que manda es el otro)"
        else
            rojo "criterio 2 ($3): el cambio en pantalla es '${N:-sin dato}': la tecla NO movio la seleccion (ver $SAL/menuseve/run.log y navega-${3}.txt)"
        fi
    done
else
    rojo "no pude leer la geometria de la ventana (ver $SAL/menuseve/run.log)"
fi

# ==============================================================================================
echo "== 2. EL MUTANTE: sin el plan del menu no hay nada que dibujar"
SIN_ASSET=PLAN.TXT SALTAR_HOST=1 bash "$HERE/build-menu-236.sh" > "$SAL/build-mutante.log" 2>&1
corre mutante ""
LM="$SAL/mutante/flycast.log"
SUBM="$(tr -d '\r' < "$LM" | grep -a 'MENU236: TEXTURAS subidas=' | tail -1 | sed 's/.*subidas= *\([0-9]*\).*/\1/')"
DIBM="$(cuadros_dibujados "$LM")"
PLANF="$(tr -d '\r' < "$LM" | grep -ac 'MENU236: PLAN FALLO')"

# El POC se va ANTES de imprimir las texturas y los cuadros cuando no encuentra el plan: que las dos lineas
# NO esten es parte del mutante, no un dato que falte.
if [ "${PLANF:-0}" -ge 1 ] && [ -z "${SUBM:-}" ] && [ -z "${DIBM:-}" ]; then
    verde "criterio 3: con el mutante (sin PLAN.TXT) el POC no lee el plan (PLAN FALLO=$PLANF), no sube ninguna textura y no dibuja ningun cuadro: la pantalla NO es el menu"
elif [ "${PLANF:-0}" -ge 1 ] && [ "${SUBM:-9}" = "0" ] && [ "${DIBM:-9}" = "0" ]; then
    verde "criterio 3: con el mutante el POC no lee el plan, sube 0 texturas y dibuja 0 quads: la pantalla queda como estaba"
else
    rojo "criterio 3: con el mutante PLAN FALLO=$PLANF subidas=${SUBM:-?} dibujados=${DIBM:-?} (tenia que quedar en 0)"
fi

if [ -n "$GEOS" ] && [ -f "$SAL/mutante/f10s.png" ]; then
    python3 "$MEDIR" pinta "$SAL/mutante/f10s.png" --ventana "$GEOS" > "$SAL/mutante/pinta.txt" 2>&1
    NOFM="$(pixeles_no_fondo "$SAL/mutante/pinta.txt")"
    nota "el mutante en pantalla: pixeles que NO son el fondo = ${NOFM:-?} (el fondo del PVR es negro)"
fi

# ==============================================================================================
echo
echo "== RESUMEN: el MENU PRINCIPAL en la Dreamcast"
echo "   texturas subidas (el menu se ve): ${TEXTURAS:-?}  de 11 (falta el FONDO, el defecto declarado)"
echo "   quads por cuadro: ${DIB:-?}   |   marcas de FOCO en la consola serie: ${FOCO:-?}"
echo "   la captura de los 10 s (foco -1), pixeles que no son el fondo: $(pixeles_no_fondo "$SAL/menuseve/pinta-10s.txt")"
echo "   navegacion: $(tr -d '\r' < "$SAL/menuseve/navega-sin_foco_a_foco0.txt" 2>/dev/null | grep -a 'pixeles que CAMBIAN' | sed 's/^ *//')"
echo "               $(tr -d '\r' < "$SAL/menuseve/navega-foco0_a_foco1.txt" 2>/dev/null | grep -a 'pixeles que CAMBIAN' | sed 's/^ *//')"
echo "   el mutante (sin el plan): subidas=${SUBM:-?} quads=${DIBM:-?}"
echo "   el instrumento: $MEDIR   |   las capturas: $SAL/menuseve/"
echo
echo "== RESULTADO: $OK controles OK, $FALLOS con fallo"
[ "$FALLOS" -eq 0 ]
