#!/bin/bash
# DC-002.140 -- EL BUILD DE LA DEMO JUGABLE (el binario que Daniel abre en Flycast y maneja el).
#
# QUE SALE DE ACA. El MISMO POC de juego-131 (no una copia: `POC_SRC` sin definir, o sea el arbol del repo)
# compilado con dos palancas nuevas y TODOS LOS INSTRUMENTOS DE ENTRADA APAGADOS, que es lo que lo vuelve
# jugable:
#
#   DEMO=si            `demo_140.go`: el POC no se corta en cuadros+3 (corre hasta que se sale del emulador), la
#                      Den of Evil avanza con lo que hace el jugador (X cerca de Akara para tomarla, Y para
#                      matar, X otra vez para cobrar) y cada 30 cuadros deja el ritmo real (`DEMO140: FPS`).
#   DEMO_MOOR=si|no    si cruzar el borde del campamento compone el BLOOD MOOR (nivel nuevo, el orden de
#                      DC-002.109). Por omision NO: el nivel de afuera pide RAM y ese numero se mide antes.
#
# Y LO QUE SE APAGA, que es la mitad de la tarjeta (todo esto aprieta solo y hace que el binario no se pueda
# jugar):
#   GUION_POC=nada / INYECTAR_P1=0   la cruceta la pone el CONTROL, no el shim
#   GOLPE_AUTO=no                    el boton Y no se aprieta solo
#   HABLAR_AUTO=no / CERRAR_DIALOGO=no   el panel de dialogo se abre y se cierra con X
#   RECOGER_AUTO=no / INVENTARIO_AUTO=no / ALIJO_AUTO=no / CERRAR_PANELES=no   los paneles, con sus teclas
#   MUERTE_POR_CUADRO=no             el cadaver se anima con el reloj del motor (49 ms por cuadro), no uno por
#                                    cuadro del POC
#   MVP=no / FASE=apagado            sin guiones de tarjeta
#
# LO QUE QUEDA ENCENDIDO: el pueblo con sus NPCs animados, la quest, los bichos, el combate, el drop, el
# inventario, el alijo, el HUD y el minimapa. Es todo el MVP de DC-002.138, pero en las manos del jugador.
#
# Uso:
#   bash scripts/console/build-fps-139.sh /tmp/dcbuild/fps139
#   MONSTRUOS=3 DEMO_MOOR=si bash scripts/console/build-demo-140.sh /tmp/dcbuild/demo140-moor
#
# Despues:
#   CAPTURAS=40 bash scripts/console/run-demo-140.sh /tmp/dcbuild/demo140/disc.gdi /tmp/capturas-demo140
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

OUT="${1:-/tmp/dcbuild/demo140}"

# ---- LA DEMO ----
export DEMO=si
export DEMO_MOOR="${DEMO_MOOR:-no}"

# ---- SIN GUIONES: todo lo que aprieta solo, apagado ----
export MVP=no
export FASE=apagado
export GUION_POC=nada
export INYECTAR_P1=0
export GOLPE_AUTO=no
export HABLAR_AUTO=no
export CERRAR_DIALOGO=no
export RECOGER_AUTO=no
export INVENTARIO_AUTO=no
export ALIJO_AUTO=no
export CERRAR_PANELES=no
export MUERTE_POR_CUADRO=no

# ---- EL MUNDO DE LA DEMO ----
# VISTA_X/VISTA_Y = la ventana de Akara: el jugador arranca a ~3 celdas de ella (la misma eleccion del MVP de
# DC-002.138, por la cache de tiles: dos zonas no entran en la RAM de la consola).
export JUGADORES="${JUGADORES:-1}"
export NPC=si
export NPC_LISTA="${NPC_LISTA:-akara,warriv1,charsi,kashya}"
export QUEST=si
export HUD=si
export MINIMAPA=si
export COMBATE=si
export DROP=si
export INVENTARIO=si
export ALIJO=si
export MONSTRUOS="${MONSTRUOS:-2}"
export VISTA_X="${VISTA_X:-37.0}"
export VISTA_Y="${VISTA_Y:-6.0}"

# ---- LO QUE SE MIDE ----
# MEDIR_CUADRO=si deja el instrumento por cuadro (16 crc32 de 1,2 MB): es el que mide como se midio siempre, y
# cuesta decenas de ms. Con MEDIR_CUADRO=no sale el ritmo de la corrida REAL. Se corren las dos.
export MEDIR_CUADRO="${MEDIR_CUADRO:-no}"
export MEDIR_VIVO="${MEDIR_VIVO:-si}"

# ---- DC-002.139: EL MAPA ESTATICO ----
# El mapa no se repinta si la camara no se movio, y la caja de cada ente se guarda/devuelve para que el mapa
# vuelva debajo de los sprites. Con MAPA_ESTATICO=no el binario es el de DC-002.140, bit a bit.
export MAPA_ESTATICO="${MAPA_ESTATICO:-si}"
export CAJAS_139="${CAJAS_139:-si}"
export CONV_BLOQUES="${CONV_BLOQUES:-si}"
export MEDIR_CARGA="${MEDIR_CARGA:-no}"
export VERBOSO="${VERBOSO:-no}"

echo "== DEMO DC-002.140: el build de la demo jugable (sin guion: el control es del jugador)"
echo "   DEMO demo=$(echo si) moor=${DEMO_MOOR} jugadores=${JUGADORES} monstruos=${MONSTRUOS} vista=${VISTA_X},${VISTA_Y}"
echo "   DEMO guiones: mvp=no fase=apagado guionPoc=nada inyectarP1=0 golpeAuto=no hablarAuto=no recogerAuto=no"
echo "   DEMO instrumentos: medirCuadro=${MEDIR_CUADRO} medirVivo=${MEDIR_VIVO} verboso=${VERBOSO}"

exec bash "$HERE/build-juego-131.sh" "$OUT"
