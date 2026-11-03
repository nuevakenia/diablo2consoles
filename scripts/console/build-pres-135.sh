#!/bin/bash
# DC-002.135 -- LA PRESENTACION: los 36,8 ms fijos que ponen el techo en 27 cuadros por segundo.
#
# QUE SALE DE ACA. El POC de DC-002.131 (el pueblo con el mapa del motor, la amazona animada, el Returned,
# el golpe, el HUD y el minimapa) con los instrumentos de DC-002.132/.134 MAS las dos palancas de esta
# tarjeta, que son dos cuellos DISTINTOS de la presentacion y se prueban por separado:
#
#   PRES_CONV_RAPIDA=si|no     el bucle de conversion RGBA -> RGB565: UNA carga de 32 bits por pixel (en vez de
#                              tres de 8 bits contra la VRAM), el color con mascaras y corrimientos (sin tablas
#                              de color) y DOS pixeles por deposito de 32 bits, recorriendo la textura en orden
#                              con una tabla de desplazamientos armada una sola vez por ancho de lienzo.
#                              "no" es el bucle de DC-002.132 TAL CUAL, en el mismo binario: ese es el A/B.
#
#   PRES_VRAM_AGRUPADA=si|no   las copias a la VRAM: los cuatro bloques de un grupo de 2x2 (64x64 px = 8.192 B)
#                              son CONTIGUOS en la textura twiddleada, asi que van en UNA copia. 90 copias por
#                              cuadro en vez de 300 (70 grupos de 2x2 = 280 bloques + los 20 bloques sueltos de la
#                              franja de abajo: 480 = 7x64 + 32, sin par vertical ni horizontal que agrupar).
#                              "no" es una copia de 2.048 B por bloque, como siempre.
#
# LOS NUMEROS QUE ESTE BUILD DEJA EN EL LOG DE LA CORRIDA (y que son el criterio):
#
#   PVR135: PRESENTAR conv_us= vram_us= pintar_us= total_us= bloques= subidas= convRapida= vramAgrupada=
#       la presentacion partida en sus TRES pedazos, promediada cada 30 cuadros. `total_us` es el mismo
#       presentar_us de la linea de abajo (medido por el ciclo), asi que la suma de los tres tiene que dar eso.
#   PVR132: CUADRO ... presentar_us= ...     el desglose del cuadro de DC-002.132, sin tocar (para comparar).
#   JUEGO135: RELOJ costo_de_1000_llamadas_us=  el costo del instrumento, para poder descontarlo.
#   JUEGO121: CUADRO 1 SOLO-MAPA ... crc32= 1437022778 ... igualAlHost= true    que el mundo se dibuja igual.
#
# Uso:
#   bash scripts/console/build-pres-135.sh [dir_salida]
#       PRES_CONV_RAPIDA=si|no  PRES_VRAM_AGRUPADA=si|no  (mas las palancas de siempre: CUADROS, JUGADORES, ...)
#
#   CAPTURAS=4 INTERVALO=10 bash scripts/console/run-anim-132.sh /tmp/dcbuild/pres135/disc.gdi /tmp/capturas-135
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

OUT="${1:-${DC135_OUT:-/tmp/dcbuild/pres135}}"

# Las palancas de esta tarjeta, con el valor con el que se mide. Se pueden pisar desde el entorno.
export PRES_CONV_RAPIDA="${PRES_CONV_RAPIDA:-si}"
export PRES_VRAM_AGRUPADA="${PRES_VRAM_AGRUPADA:-si}"

# El mismo cuadro que midio DC-002.132: un jugador (la amazona), su bicho (el Returned), el golpe automatico para
# que haya animacion sin teclado, el HUD y el minimapa. NPC/QUEST/INVENTARIO/DROP apagados para entrar en los
# 16 MB (el OOM de arranque es otra tarjeta). 60 cuadros = dos tramos del informe de 30.
export JUGADORES="${JUGADORES:-1}"
export CUADROS="${CUADROS:-60}"
export MONSTRUOS="${MONSTRUOS:-1}"
export COMBATE="${COMBATE:-si}"
export GOLPE_AUTO="${GOLPE_AUTO:-si}"
export HUD="${HUD:-si}"
export MINIMAPA="${MINIMAPA:-si}"
export NPC="${NPC:-no}"
export QUEST="${QUEST:-no}"
export INVENTARIO="${INVENTARIO:-no}"
export DROP="${DROP:-no}"
export MEDIR_CUADRO="${MEDIR_CUADRO:-no}"
export BLIT_RAPIDO="${BLIT_RAPIDO:-si}"
export CAMARA="${CAMARA:-siguiendo}"

echo "== DC-002.135: el POC de DC-002.131 con la presentacion partida (convRapida=$PRES_CONV_RAPIDA vramAgrupada=$PRES_VRAM_AGRUPADA)"
exec bash "$HERE/build-juego-131.sh" "$OUT"
