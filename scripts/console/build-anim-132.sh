#!/bin/bash
# DC-002.132 -- LAS ANIMACIONES A ALTOS FPS: la amazona y el Returned al limite de la consola.
#
# QUE SALE DE ACA. El POC de DC-002.131 (el pueblo con el mapa del motor, la amazona animada, el Returned,
# el golpe, el HUD y el minimapa) MAS las dos palancas de esta tarjeta:
#
#   BLIT_RAPIDO=si   el dibujado del tile deja de pagar por pixel lo que se puede pagar una vez por tile:
#                    el RECORTE (dos comparaciones por pixel, y el tile se recorria entero aunque estuviera
#                    medio fuera del lienzo) y la CUENTA DEL TINTE (3 multiplicaciones y 3 divisiones por 255,
#                    que con el estado neutro -- sin color puesto, el caso del mapa -- no cambia nada).
#                    Con BLIT_RAPIDO=no corre el bucle VIEJO, tal cual, en el MISMO binario: ese es el A/B.
#
#   MEDIR_CUADRO=no  apaga el instrumento POR CUADRO del POC: 16 crc32 del lienzo de 1.228.800 B mas el conteo
#                    de pixeles, que son decenas de ms por cuadro y no son parte del juego. Los TRES PRIMEROS
#                    cuadros se miden igual (ahi vive la comparacion del cuadro 1 contra el host, crc32
#                    1437022778): con MEDIR_CUADRO=no el mundo sigue probado y el numero de cuadros por
#                    segundo es el de DIBUJAR, no el de medirse.
#
# LOS NUMEROS QUE ESTE BUILD DEJA EN EL LOG DE LA CORRIDA (y que son el criterio):
#
#   PVR132: FPS seg_fpsX100=...            los cuadros por segundo, medidos por el ciclo del PVR (ventana de 1 s)
#   PVR132: CUADRO ... espera_us= trabajo_us= presentar_us= cuadro_us=   el desglose del cuadro
#   JUEGO132: MAPA cuadro=.. ms=.. stats=floors=.. walls=.. upperwalls=.. roofs=..   lo que el mapa dibujo
#   JUEGO132: FPS cuadros=.. msPorCuadro=.. fps=.. animsHeroePorSeg=.. animsBichoPorSeg=..
#            los cuadros por segundo del tramo y los cuadros de ANIMACION que llegaron al lienzo
#
# Uso:
#   bash scripts/console/build-anim-132.sh [dir_salida]
#       MEDIR_CUADRO=si|no  BLIT_RAPIDO=si|no  (mas las palancas de siempre: JUGADORES, CUADROS, HUD, ...)
#
#   CAPTURAS=8 INTERVALO=4 bash scripts/console/run-flycast-131.sh /tmp/dcbuild/anim132/disc.gdi /tmp/capturas-132
#
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

OUT="${1:-${DC132_OUT:-/tmp/dcbuild/anim132}}"

# Las palancas de esta tarjeta, con el valor con el que se mide. Se pueden pisar desde el entorno.
export BLIT_RAPIDO="${BLIT_RAPIDO:-si}"
export MEDIR_CUADRO="${MEDIR_CUADRO:-no}"

# El guion de la corrida: un jugador (la amazona) con su bicho (el Returned), el golpe automatico encendido para
# que la animacion de atacar y la de morir tengan algo que mostrar en el emulador (no hay teclado), y el HUD y el
# minimapa como estan: son parte del cuadro que se quiere ver rapido.
export JUGADORES="${JUGADORES:-1}"
export CUADROS="${CUADROS:-30}"
export MONSTRUOS="${MONSTRUOS:-1}"
export COMBATE="${COMBATE:-si}"
export GOLPE_AUTO="${GOLPE_AUTO:-si}"
export CAMARA="${CAMARA:-siguiendo}"

echo "== DC-002.132: el POC de DC-002.131 con el cuadro barato (BLIT_RAPIDO=$BLIT_RAPIDO MEDIR_CUADRO=$MEDIR_CUADRO)"
exec bash "$HERE/build-juego-131.sh" "$OUT"
