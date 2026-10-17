#!/bin/bash
# CORPUS DE DC-002.24b: EL "CORTE DESPUES DE LOS CUADROS" ERA EL INSTRUMENTO, NO EL MOTOR.
#
# SINTOMA (heredado de DC-002.24, la tarjeta anterior)
#   Con la copia por Store Queue del GC arreglada el motor entra al loop, imprime los cuadros 0/1/2, imprime
#   el PERFIL del cuadro 11 y DESPUES el flycast.log no dice mas nada. La tarjeta abrio este hilo como "otro
#   defecto": (a) ese corte silencioso y (b) la excepcion intermitente
#
#       Unhandled exception: PC 64800008, code 1
#
# LO QUE MIDE ESTE CORPUS (con la sonda del PC puesta: runtime/sonda_pc.c, fix-libgodc-sonda-pc.py)
#   1. EL CORTE NO ES UN CORTE: el loop sigue dando cuadros. El loop imprime su heartbeat cada 300 cuadros
#      (`EBITEN-LOOP, cuadros 300 huella <h> dibujos <d>`) y con ~0,31 s por cuadro eso cae en t~95 s de
#      corrida: entre el cuadro 11 (el PERFIL) y el 300 el loop NO imprime nada, asi que "el log se corto"
#      era el instrumento, no el motor. Medido: cuadro_us 309326 (3,23 cuadros/s) y el heartbeat aparece
#      entre las ventanas t=94 y t=95 de la sonda (300 cuadros / 94,4 s = 3,18 cuadros/s).
#   2. LA SONDA DEL PC dice DONDE esta el CPU en esa mudez: 118 ventanas de 1 s, el 99,7% de las muestras en
#      el dibujado del cuadro de error del motor (`Image.SetRGBA` 50,4%, `pvr_pantalla_convertir` 28,4%,
#      `Image.Fill`/`LlenarDeRGBA` 20,9%) y CERO muestras fuera de la RAM y CERO rachas clavadas. O sea: el
#      motor no esta colgado, esta dibujando a 3 cuadros por segundo (pixel por pixel en Go).
#   3. LA EXCEPCION INTERMITENTE no aparece en las corridas de este corpus (ver la evidencia para la lista
#      completa): este corpus la vigila (control 3) pero DECLARA que un corpus de corridas fijas no puede
#      probar que un defecto intermitente no existe.
#
# CONTROLES (corrida sana)
#   1. la corrida larga LLEGA AL CUADRO 300 (el heartbeat del loop): el corte silencioso no es un corte
#   2. la sonda esta puesta y MUESTREA (>= 90 ventanas de 1 s) y no grita: 0 'SONDA-PC FUERA', 0 'RACHA'
#   3. el CPU vive en el dibujado del cuadro (el censo: SetRGBA/Fill/convertir se llevan el top de la ventana)
#   4. 0 'Unhandled exception' (y el control 3 de la evidencia lo declara: es intermitente)
#   5. la cadena de la sonda es idempotente (dos corridas dejan el MISMO sha256 en sonda_pc.c y go-main.c)
#
# MUTANTE (tiene que CAZARSE)
#   El TOPE VIEJO de run-flycast.sh: la misma corrida pedida con 40 s se corta a los ~41 s (el bucle de
#   capturas terminaba en t=40) y NO llega al cuadro 300 -> el control 1 se pone ROJO. Ese es el defecto de
#   instrumento que esta tarjeta midio: con el tope viejo, "la corrida se acaba a los 30 s" y el heartbeat
#   del cuadro 300 no entraba nunca.
#
# Uso:  bash scripts/console/checar-cuadros-24b.sh [dir_gox] [segundos_largos]
#       (deja la sonda puesta en ~/dreamcast/libgodc, como el arreglo de DC-002.24: es el instrumento de la
#        tarjeta. Para volver al runtime pelado: fix-libgodc-sonda-pc.py --quitar --build y reenlazar.)
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

GEN="${1:-/tmp/gox-222}"
LARGO="${2:-140}"
REPO="${REPO:-$(cd "$HERE/../.." && pwd)}"
SONDA="$HERE/fix-libgodc-sonda-pc.py"

export GODC_GOX="$GEN"

TOTAL_CTRL=6
TOTAL_MUT=1
CAZADOS=0
BIEN=0

compilar() { # $1 = dir de build
    if ! POC=motor-arranque bash "$HERE/build-motor-arranque.sh" "$1" "$GEN" > "$1.build.log" 2>&1; then
        echo "   NO ENLAZA (ver $1.build.log)" >&2
        return 1
    fi
}

correr() { # $1 = dir de build, $2 = dir de la corrida, $3 = segundos
    rm -rf "$2"
    (cd "$REPO" && CAPTURE=none bash "$HERE/run-flycast.sh" "$1/prog-emu.elf" "$3" "$2") \
        > "$2.salida" 2>&1
    [ -f "$2/flycast.log" ]
}

cuadro300()   { grep -ac 'EBITEN-LOOP, cuadros [0-9]* ' "$1/flycast.log" 2>/dev/null; }
ventanas()    { grep -a 'SONDA-PC t=' "$1/flycast.log" 2>/dev/null | grep -ac . ; }
alarmas()     { grep -a 'SONDA-PC' "$1/flycast.log" 2>/dev/null | grep -ac 'FUERA\|RACHA'; }
excepciones() { grep -ac 'Unhandled exception' "$1/flycast.log" 2>/dev/null; }
top_dibujo()  { grep -a 'SONDA-PC t=' "$1/flycast.log" 2>/dev/null | grep -ac '8c013348\|8c03f680\|8c03db'; }

# ---------------------------------------------------------------- 0. la sonda puesta (instrumento)
echo "== 0. la sonda del PC (runtime/sonda_pc.c) + la cadena idempotente"
# el estado inicial del instrumento se registra para dejarlo IGUAL al final (el .bss de la sonda, 552 bytes,
# mueve el monton de C un pelo: otras tarjetas comparan numeros exactos, asi que no se deja puesta de arriba)
if [ -f "$LIBGODC/runtime/sonda_pc.c" ]; then SONDA_ANTES=1; else SONDA_ANTES=0; fi
echo "   la sonda estaba puesta al empezar: $SONDA_ANTES"
python3 "$SONDA" | sed 's/^/   /'
SHA_A="$(sha256sum "$LIBGODC/runtime/sonda_pc.c" "$LIBGODC/runtime/go-main.c" | cut -c1-16 | tr '\n' ' ')"
python3 "$SONDA" >/dev/null
SHA_B="$(sha256sum "$LIBGODC/runtime/sonda_pc.c" "$LIBGODC/runtime/go-main.c" | cut -c1-16 | tr '\n' ' ')"
if [ "$SHA_A" = "$SHA_B" ]; then echo "   control 5 (la cadena es idempotente):       VERDE"; BIEN=$((BIEN + 1))
else echo "   control 5 (la cadena es idempotente):       ROJO ($SHA_A -> $SHA_B)"; fi

python3 "$SONDA" --build > /tmp/dc24b-sonda.build.log 2>&1 || { echo "la sonda no compila" >&2; exit 1; }
compilar "/tmp/dcbuild/dc24b-corpus" || exit 1

# ---------------------------------------------------------------- 1. mutante: el tope viejo (40 s)
echo
echo "== 1. mutante: la MISMA corrida con el tope viejo (40 s) -- no puede llegar al cuadro 300"
correr "/tmp/dcbuild/dc24b-corpus" "/tmp/dc24b-corpus-corto" 40 || exit 1
C300C="$(cuadro300 /tmp/dc24b-corpus-corto)"
echo "   heartbeat del cuadro 300 en la corrida de 40 s: ${C300C:-0}"
if [ "${C300C:-0}" -eq 0 ]; then
    echo "   CAZADO: la corrida corta se queda sin el heartbeat (el tope del instrumento manda)"
    CAZADOS=$((CAZADOS + 1))
else
    echo "   NO LO CAZO: la corrida de 40 s igual llego al cuadro 300" >&2
fi

# ---------------------------------------------------------------- 2. el control sano: la corrida larga
echo
echo "== 2. control: la corrida larga (${LARGO} s) -- el loop sigue dando cuadros"
correr "/tmp/dcbuild/dc24b-corpus" "/tmp/dc24b-corpus-largo" "$LARGO" || exit 1
L="/tmp/dc24b-corpus-largo"
C300="$(cuadro300 "$L")"; VENT="$(ventanas "$L")"; ALAR="$(alarmas "$L")"; EXC="$(excepciones "$L")"
TOP="$(top_dibujo "$L")"
if [ "${C300:-0}" -ge 1 ]; then echo "   control 1 (el loop llega al cuadro 300):    VERDE"; BIEN=$((BIEN + 1))
else echo "   control 1 (el loop llega al cuadro 300):    ROJO (heartbeat: ${C300:-0})"; fi
if [ "${VENT:-0}" -ge 90 ]; then echo "   control 2 (la sonda muestrea: $VENT ventanas): VERDE"; BIEN=$((BIEN + 1))
else echo "   control 2 (la sonda muestrea):              ROJO ($VENT ventanas)"; fi
if [ "${ALAR:-0}" -eq 0 ]; then echo "   control 2b (la sonda no grita FUERA/RACHA): VERDE"; BIEN=$((BIEN + 1))
else echo "   control 2b (la sonda no grita):             ROJO ($ALAR alarmas)"; fi
if [ "${TOP:-0}" -ge 60 ]; then echo "   control 3 (el CPU vive en el dibujado: $TOP ventanas): VERDE"; BIEN=$((BIEN + 1))
else echo "   control 3 (el CPU vive en el dibujado):     ROJO ($TOP ventanas)"; fi
if [ "${EXC:-0}" -eq 0 ]; then echo "   control 4 (0 excepciones del SH-4):         VERDE"; BIEN=$((BIEN + 1))
else echo "   control 4 (0 excepciones del SH-4):         ROJO ($EXC)"; fi

# el censo, para la evidencia
echo
echo "== 3. el censo de la sonda (que funcion se lleva el tiempo del SH-4)"
python3 "$HERE/resolver-pc.py" "/tmp/dcbuild/dc24b-corpus/prog.elf" "$L/flycast.log" --censo 2>/dev/null | head -12 | sed 's/^/   /'

# el instrumento se deja como estaba: si no estaba puesto al empezar, se saca y se rehace libgodc.a
echo
echo "== 4. el instrumento queda como estaba al empezar"
if [ "$SONDA_ANTES" -eq 0 ]; then
    python3 "$SONDA" --quitar --build | sed 's/^/   /'
    echo "   la sonda NO estaba al empezar: se saco y libgodc.a quedo pelado (el arbol sano)"
else
    echo "   la sonda YA estaba al empezar: se deja puesta"
    python3 "$SONDA" | sed 's/^/   /'
fi

echo
echo "== $CAZADOS/$TOTAL_MUT mutantes cazados, $BIEN/$TOTAL_CTRL controles verdes =="
[ "$CAZADOS" -eq "$TOTAL_MUT" ] && [ "$BIEN" -eq "$TOTAL_CTRL" ]
