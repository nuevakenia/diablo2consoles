#!/bin/bash
# DC-002.138 -- EL MVP JUGABLE: una corrida con las nueve acciones juntas, de punta a punta.
#
# QUE SALE DE ACA. El POC de siempre (`console/dc/poc/juego-131`) con el guion del MVP prendido (`MVP=si`,
# `mvp_138.go`): caminar al bicho, matarlo, recoger el drop, abrir el inventario, guardarlo en el alijo, hablar con
# Akara, aceptar la Den of Evil, avanzarla y volver a caminar. Es el MISMO binario de las otras tarjetas: lo unico
# que cambia son las palancas, y la unica pieza nueva es el guion que las encadena.
#
# LAS PALANCAS DEL MVP, Y POR QUE CADA UNA.
#
#   MVP=si           el guion (`mvp_138.go`). Sin esto, la corrida es la de la tarjeta que se pida.
#   VISTA_X=37 VISTA_Y=6
#                    LA VENTANA. La cache de tiles del renderer tiene UN tramo de 3.481.600 B y la ventana del
#                    centro del pueblo ya son 2.600.960 B: no entran dos zonas (medido en DC-002.116/.131). Akara
#                    esta en (38.4,3.2) y el centro del POC en (28,20), 19,8 celdas: caminar hasta ella pide las dos
#                    ventanas. Por eso el MVP corre ENTERO en la ventana de Akara: el jugador arranca en (37,6), a
#                    3,1 celdas de ella, y el bicho nace una celda al lado del jugador, como en todas las corridas.
#   MONSTRUOS=2      uno para el combate del MVP (el bicho 1, el que se mata a golpes en el pueblo) y otro para la
#                    CUEVA de la quest: la Den of Evil cuenta bajas del area 8, asi que el bicho que la hace
#                    avanzar tiene que morir alla (lo mata el guion de DC-002.128, con su mismo camino).
#   NPC_LISTA=warriv1,charsi,kashya,akara
#                    Akara TIENE que estar en la lista o no hay marca 6 (hablar) ni marca 7 (aceptar). Los otros
#                    tres van porque son los del pueblo y no cuestan un pixel fuera de su celda.
#   JUGADORES=1      el MVP es de un jugador: el split es otra tarjeta y con dos lienzos no entran ni el mapa.
#   FASE=apagado     sin el ciclo de niveles. Los tres cuadros del cierre (el origen, la repetibilidad y la vuelta)
#                    son la prueba de la marca 9: el mundo y el HUD DESPUES de todo. Con el ciclo encendido esos
#                    cuadros se los lleva la transicion y la corrida muere por memoria (medido en DC-002.128/.137).
#   CUADROS=120      el guion del MVP NO tiene cuadros fijos (los arma etapa por etapa), asi que el presupuesto es
#                    el techo. Y no se pierde: cuando las nueve acciones terminan, el guion CIERRA la corrida el
#                    mismo (`cuadro = cuadros-1`) y los tres cuadros del cierre del POC corren detras. Lo que el
#                    guion uso de verdad lo dice su tabla (`cuadrosDeLaCorrida`).
#
# Uso:
#   bash scripts/console/build-mvp-138.sh /tmp/dcbuild/mvp138
#   CAPTURE=spectacle bash scripts/console/run-mvp-138.sh /tmp/dcbuild/mvp138/disc.gdi 90 /tmp/capturas-mvp138
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT="${1:-/tmp/dcbuild/mvp138}"

export MVP="${MVP:-si}"
# OJO CON LOS DECIMALES: `vistaX`/`vistaY` son constantes SIN TIPO en knobs.go y el `sed` del build escribe el
# texto tal cual; con `37` la constante pasa a ser INT y `celdaX`/`celdaY` (que se derivan de ella) dejan de ser
# float64 y no compilan. Se pasan con decimales.
export VISTA_X="${VISTA_X:-37.0}"
export VISTA_Y="${VISTA_Y:-6.0}"
export MONSTRUOS="${MONSTRUOS:-2}"
# CELDA_P1="dx dy": donde arranca el JUGADOR 1 respecto de la celda de la camara (VISTA). Con "0 -3" el heroe
# arranca 3 celdas al norte de la camara y a 4,1 celdas del bicho (`celdaBicho = vista+1,+1`): el primer tramo del
# MVP es un CAMINO de 4 celdas con la cruceta, no un paso de 1,4. Con decimales por el mismo motivo que VISTA_X.
export CELDA_P1="${CELDA_P1:-0.0 -3.0}"
export NPC_LISTA="${NPC_LISTA:-warriv1,charsi,kashya,akara}"
export JUGADORES="${JUGADORES:-1}"
export FASE="${FASE:-apagado}"
export CUADROS="${CUADROS:-120}"
# el golpe, la recogida, el inventario, el alijo, los NPCs, la quest, el HUD y el minimapa van PRENDIDOS: son las
# piezas que el MVP tiene que usar todas juntas. Si alguna se apaga a proposito (A/B), se dice en el log del build.
export GOLPE_AUTO="${GOLPE_AUTO:-si}"
export RECOGER_AUTO="${RECOGER_AUTO:-si}"
export INVENTARIO="${INVENTARIO:-si}"
export ALIJO="${ALIJO:-si}"
export INVENTARIO_AUTO="${INVENTARIO_AUTO:-si}"
export ALIJO_AUTO="${ALIJO_AUTO:-si}"
export NPC="${NPC:-si}"
export QUEST="${QUEST:-si}"
export HUD="${HUD:-si}"
export MINIMAPA="${MINIMAPA:-si}"
export MUERTE_POR_CUADRO="${MUERTE_POR_CUADRO:-si}"
export CADAVER="${CADAVER:-si}"

exec bash "$HERE/build-juego-131.sh" "$OUT"
