#!/usr/bin/env bash
# DC-002.49 -- el corpus completo, de una, con la salida en un archivo para adjuntar.
#
# Uso: bash scripts/console/checar-anim-249-corrida.sh <build> <build-mutante> <corrida> <corrida-mutante> <salida>
set -u
source "$(dirname "$0")/env.sh" 2>/dev/null || true

BUILD=${1:?falta el build}
BUILD_MUDO=${2:?falta el build del mutante}
CORRIDA=${3:?falta la corrida}
CORRIDA_MUDA=${4:?falta la corrida del mutante}
SALIDA=${5:-/tmp/dc249-corpus.txt}

# la ventana del emulador la reporta la corrida en sus lineas GEO (cliente=L,T,W,H). Desde DC-002.49b se busca
# primero en el geo.txt de la corrida (ahi queda siempre) y despues en flycast.log, que es donde la dejaban las
# corridas viejas. El area cliente NO se puede suponer: medido en este host, la pantalla es 3584x1080 y la
# ventana del emulador 320x240, asi que el 667x442 de antes recortaba una region que no era la del juego.
VENTANA=""
for fuente in "$CORRIDA/geo.txt" "$CORRIDA/flycast.log"; do
    [ -s "$fuente" ] || continue
    VENTANA=$(grep -a -o "cliente=[0-9,]*" "$fuente" 2>/dev/null | head -1 | cut -d= -f2)
    [ -n "$VENTANA" ] && break
done
VENTANA=${VENTANA:-1400,120,320,240}

python3 scripts/console/checar-anim-249.py \
    --host "$BUILD/host" \
    --run-normal "$CORRIDA" \
    --run-mutante "$CORRIDA_MUDA" \
    --ventana "$VENTANA" 2>&1 | tee "$SALIDA"

echo
echo "corpus en $SALIDA"
