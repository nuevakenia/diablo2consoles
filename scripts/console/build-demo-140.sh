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
#   DEMO_MOOR=si|no    si cruzar el borde del campamento compone el nivel de AFUERA (nivel nuevo, el orden de
#                      DC-002.109). Por omision NO, y asi queda: DC-002.144 lo corrio y no entra por RAM (los
#                      `.dt1` del afuera piden 4.368.642 B de arenas de bloques contra 2.995.872 B libres ->
#                      `fatal error: gc_external_alloc: OOM`). Ver README-140.md y
#                      console/dc/evidencia/dc-002.144-el-blood-moor.txt.
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
#   bash scripts/console/build-demo-140.sh /tmp/dcbuild/demo140
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
# DC-002.144b: el orden de los dos pedidos grandes al cruzar (el buffer del `.dt1` mas grande primero, "si").
export DEMO_MOOR_SCRATCH="${DEMO_MOOR_SCRATCH:-si}"
# DC-002.145: DEMO_MOOR_DRLG=si = el afuera es una REGION del Blood Moor DE VERDAD (el plan del DRLG) y no la
# pieza de 17 x 17. Por omision "no": el binario de la demo sigue siendo el de DC-002.144b.
export DEMO_MOOR_DRLG="${DEMO_MOOR_DRLG:-no}"
# DC-002.147: DEMO_MOOR_STREAM=si = el afuera son las SEIS regiones del nivel y se CAMBIA DE REGION caminando,
# en caliente y sin recompilar. Por omision "no": el binario es el de DC-002.145 (una region, sin borde).
export DEMO_MOOR_STREAM="${DEMO_MOOR_STREAM:-no}"

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
# DC-002.141: el golpe va al bicho VIVO MAS CERCANO. Sin esto la demo con MONSTRUOS=2 solo puede matar el primero
# (`bichoDePelea` devuelve siempre bichos[0]) y la Den no queda limpia: medido en la corrida de DC-002.140.
# Se enciende por omision y se puede apagar con `COMBATE_CERCA=no ... build-demo-140.sh` para el A/B de la tarjeta.
export COMBATE_CERCA="${COMBATE_CERCA:-si}"
export DROP=si
export INVENTARIO=si
export ALIJO=si
export MONSTRUOS="${MONSTRUOS:-2}"
export VISTA_X="${VISTA_X:-37.0}"
export VISTA_Y="${VISTA_Y:-6.0}"

# ---- LO QUE SE MIDE ----
# MEDIR_CUADRO=si deja el instrumento por cuadro (16 crc32 de 1,2 MB): es el que mide como se midio siempre, y
# cuesta decenas de ms. Con MEDIR_CUADRO=no sale el ritmo de la corrida REAL. Se corren las dos.
export MEDIR_CUADRO="${MEDIR_CUADRO:-si}"
export MEDIR_VIVO="${MEDIR_VIVO:-si}"
export VERBOSO="${VERBOSO:-no}"

echo "== DEMO DC-002.140: el build de la demo jugable (sin guion: el control es del jugador)"
echo "   DEMO demo=$(echo si) moor=${DEMO_MOOR} drlg=${DEMO_MOOR_DRLG} stream=${DEMO_MOOR_STREAM} jugadores=${JUGADORES} monstruos=${MONSTRUOS} vista=${VISTA_X},${VISTA_Y}"
echo "   DEMO guiones: mvp=no fase=apagado guionPoc=nada inyectarP1=0 golpeAuto=no hablarAuto=no recogerAuto=no"
echo "   DEMO instrumentos: medirCuadro=${MEDIR_CUADRO} medirVivo=${MEDIR_VIVO} verboso=${VERBOSO}"

exec bash "$HERE/build-juego-131.sh" "$OUT"
