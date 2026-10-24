#!/bin/bash
# DC-002.53: CONTROL AUTOMATICO DEL ARRANQUE DEL POC EN LA CONSOLA.
#
# PARA QUE SIRVE: el sintoma de esta tarjeta (bestia=1 muere adentro del arranque de video, y el punto de
# muerte se mueve con cualquier cambio de codigo) no lo caza ninguna suite: se caza arrancando. Este
# script compila, corre 90 s en Flycast con la consola serie y CLASIFICA la corrida en tres estados
# medibles, para que la falla no vuelva sin que nos enteremos:
#
#   ANDA              llego al bucle de cuadros  -> hay "HITO238: POS cuadro" (o "HITO249: ANIM")
#   MUERE-EXCEPCION   salto una excepcion del SH-4 (la sonda imprime ARQ-PILA253-REAL)
#   COLGADO           empezo el video ("HITO250: PVR antes-de-pvr_init_defaults") y no hay ni cuadros
#                     ni excepcion: se quedo adentro de pvr_init (medido: pasa, ver la evidencia)
#
# uso:  bash scripts/console/checar-arranque-253.sh [salida]        (BESTIA=0|1, SEGUNDOS=90)
# sale: 0 = ANDA, 1 = MUERE-EXCEPCION, 2 = COLGADO, 3 = no compilo / no corrio
#
# OJO: el estado depende del BINARIO, no del codigo: un cambio de instrumentacion puede mover el punto de
# muerte (esta medido en console/dc/evidencia/dc-002.53-arranque-bestia.txt). Por eso el script imprime
# tambien el tamano de 1ST_READ.BIN, que es la huella del layout que se esta juzgando.
set -o pipefail

BESTIA=${BESTIA:-1}
SEGUNDOS=${SEGUNDOS:-90}
SALIDA=${1:-/tmp/dcbuild/chequeo-253}
AQUI=$(cd "$(dirname "$0")/../.." && pwd)

cd "$AQUI" || exit 3

echo "== build bestia=$BESTIA -> $SALIDA"
BESTIA=$BESTIA SALTAR_STDLIB=1 SALTAR_MOTOR=1 bash scripts/console/build-monstruo-250.sh "$SALIDA" > "$SALIDA-build.log" 2>&1

if [ $? -ne 0 ]; then
    echo "FALLA: no compilo (ver $SALIDA-build.log)"
    tail -5 "$SALIDA-build.log"
    exit 3
fi

echo "== corrida $SEGUNDOS s"
CAPTURE=none bash scripts/console/run-flycast.sh "$SALIDA/disc.gdi" "$SEGUNDOS" "$SALIDA/corrida" > "$SALIDA-run.log" 2>&1

if [ $? -ne 0 ]; then
    echo "FALLA: no corrio (ver $SALIDA-run.log)"
    exit 3
fi

LOG="$SALIDA/corrida/flycast.log"
TAM=$(stat -c %s "$SALIDA/1ST_READ.BIN" 2>/dev/null)

cuenta() { tr -d '\r' < "$LOG" | grep -ac "$1"; }

CUADROS=$(cuenta "HITO238: POS cuadro")
EXCEPCIONES=$(cuenta "ARQ-PILA253-REAL")
VIDEO=$(cuenta "HITO250: PVR antes-de-pvr_init_defaults")
UBC=$(cuenta "ARQ-UBC253 pc")

echo "-- 1ST_READ.BIN: $TAM B"
echo "-- marcas: cuadros=$CUADROS video=$VIDEO excepciones=$EXCEPCIONES cortes-del-UBC=$UBC"

if [ "$EXCEPCIONES" -gt 0 ]; then
    echo "== estado: MUERE-EXCEPCION"
    tr -d '\r' < "$LOG" | grep -a -m1 -A 3 "ARQ-PILA253-REAL" | head -4
    echo "   (resolver el pc/pr con: python3 scripts/console/resolver-pc.py $SALIDA/prog.elf <pc> <pr>)"
    exit 1
fi

if [ "$CUADROS" -gt 0 ]; then
    echo "== estado: ANDA"
    tr -d '\r' < "$LOG" | grep -a -m1 "HITO238: POS cuadro"
    exit 0
fi

if [ "$VIDEO" -gt 0 ]; then
    echo "== estado: COLGADO (empezo el video y no llego al primer cuadro)"
    exit 2
fi

echo "== estado: COLGADO (no llego ni al video)"
tr -d '\r' < "$LOG" | grep -a "HITO\|SONDA253" | tail -3
exit 2
