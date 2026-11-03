#!/bin/bash
# DC-002.133 -- EL SEGUNDO MVP: los dos personajes animados y los controles andando bien.
#
# QUE SALE DE ACA. El POC de DC-002.135 (el pueblo con el mapa del motor, la amazona, el Returned, el golpe, el HUD,
# el minimapa y la presentacion partida en tres) MAS lo de esta tarjeta:
#
#   animacion_133.go   el medidor de la animacion: por personaje y por cuadro del POC anota el MODO que declara el
#                      COF (NU, TW, A1, GH, DT, DD), la DIRECCION, el CUADRO del ciclo y la FASE (cuantos cuadros de
#                      animacion pasaron desde que empezo el modo). Cada cambio de direccion queda con sus dos fases:
#                      si el reloj se reiniciara al girar, la fase retrocederia y el giro saldria `salto=true`.
#                      El resumen del cierre da, por personaje: modos jugados, cuadros DISTINTOS de cada animacion,
#                      cambios de direccion, saltos de fase, volteos, direcciones del COF dibujadas, segundos y
#                      cuadros de animacion por segundo.
#
#   movimiento.go      EL RELOJ DEL QUIETO. El ciclo NU de la amazona ya no es un cuadro tieso: corre a 128,00 ms por
#                      cuadro (8 cuadros, ciclo 1.024 ms) mientras la cruceta no pide nada, y se pone en cero al
#                      empezar a caminar (la animacion nueva arranca en su cuadro 0, la regla del motor). El cuadro 1
#                      sigue siendo el de siempre porque el reloj arranca en 0.
#
#   coop.go            EL GUION "ocho": las 8 direcciones de la cruceta, 2 s cada una con 1 s de pausa entre tramos,
#                      que es lo que la tarjeta pide ver (y el Returned patrullando al lado, con la vida multiplicada
#                      -- VIDA_X -- para que no se muera en los primeros golpes y siga respondiendo todo el guion).
#
# LAS PALANCAS DE ESTA TARJETA (todas se pueden pisar desde el entorno):
#   GUION_POC=ocho        las 8 direcciones (el guion de la prueba)
#   VIDA_X=8              la vida del Returned x8: aguanta el guion entero de golpes sin morirse
#   JUGADORES=1           una amazona + el Returned: los DOS personajes de la tarjeta
#   CUADROS=60            el guion de 8 tramos de 2 s + 1 s de pausa son 24 s; a ~2 cuadros/s son ~50 cuadros
#
# Uso:
#   bash scripts/console/build-anim-133.sh [dir_salida]
#   CAPTURAS=6 INTERVALO=8 bash scripts/console/run-anim-132.sh <dir_salida>/disc.gdi /tmp/capturas-133 900 120
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

OUT="${1:-${DC133_OUT:-/tmp/dcbuild/anim133}}"

# El guion de la tarjeta: las ocho direcciones, el golpe automatico (el emulador no tiene teclado) y el bicho vivo.
export GUION_POC="${GUION_POC:-ocho}"
export VIDA_X="${VIDA_X:-8}"
export JUGADORES="${JUGADORES:-1}"
export CUADROS="${CUADROS:-60}"
export MONSTRUOS="${MONSTRUOS:-1}"
export COMBATE="${COMBATE:-si}"
export GOLPE_AUTO="${GOLPE_AUTO:-si}"
export CAMARA="${CAMARA:-siguiendo}"

# El cuadro barato y el instrumento, como en DC-002.134/135: el mundo se sigue midiendo en los tres primeros cuadros.
export BLIT_RAPIDO="${BLIT_RAPIDO:-si}"
export MEDIR_CUADRO="${MEDIR_CUADRO:-no}"
export PINTAR_UNA_VEZ="${PINTAR_UNA_VEZ:-si}"
export BLIT_PALABRAS="${BLIT_PALABRAS:-si}"
export PRES_CONV_RAPIDA="${PRES_CONV_RAPIDA:-si}"
export PRES_VRAM_AGRUPADA="${PRES_VRAM_AGRUPADA:-si}"
export ETAPAS="${ETAPAS:-si}"
export VERBOSO="${VERBOSO:-no}"

# Las piezas que no entran en los 16 MB (el techo de RAM medido en DC-002.132) y que esta tarjeta no mira.
export HUD="${HUD:-si}"
export MINIMAPA="${MINIMAPA:-si}"
export NPC="${NPC:-no}"
export QUEST="${QUEST:-no}"
export INVENTARIO="${INVENTARIO:-no}"
export DROP="${DROP:-no}"

# FASE: EL CICLO DE NIVELES (DC-002.120) SE DEJA COMO VIENE (`ciclo`), aunque no sea de esta tarjeta. Parece al
# reves, pero es lo medido en las corridas de esta tarjeta: la palanca `faseKnob` tiene tres valores y solo `ciclo`
# enciende el ciclo; con `no` -- y con `si`, que NO es lo mismo -- el ciclo queda apagado y entonces la politica de
# reusar las casillas de nivel tambien queda apagada (main.go: "con el ciclo apagado la politica queda apagada y esta
# corrida es la de DC-002.117 byte a byte"). Con la cache de tiles cargada por un guion que camina, la composicion del
# pueblo se queda sin monton: medido, `ARQ-CARGA lee-volvio TownN1.ds1` y de ahi no sale mas (8 minutos, 15 % de CPU,
# sin primer cuadro). Con `ciclo` compone en 71 s.
# El resumen de la animacion se imprime ANTES de que el ciclo se lleve el cuadro (`resumenDeLaAnimacionHecho`), porque
# con el ciclo encendido el cierre de siempre (`cuadro == cuadros+3`) no llega nunca.
export FASE="${FASE:-ciclo}"

echo "== DC-002.133: los DOS personajes animados con los controles (guion=$GUION_POC vidaX=$VIDA_X)"
exec bash "$HERE/build-juego-131.sh" "$OUT"
