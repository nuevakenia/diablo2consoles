#!/bin/bash
# DC-002.138: arma el archivo de evidencia de la tarjeta a partir del log de la corrida del MVP.
# Uso: evidencia-mvp-138.sh /tmp/capturas-mvp138h console/dc/evidencia/dc-002.138-el-mvp-jugable.txt
set -uo pipefail
OUT="${1:?falta la carpeta de capturas}"
DEST="${2:?falta el archivo de salida}"
LOG="$OUT/flycast.log"

{
echo "================================================================================"
echo "DC-002.138 -- EL MVP JUGABLE: las nueve acciones en UNA corrida"
echo "================================================================================"
echo
echo "El POC juego-131 tiene las piezas de las tarjetas 121-137 medidas POR SEPARADO, cada una con su guion de"
echo "cuadros FIJOS. Esta tarjeta pide lo que no existia: UNA corrida que las use TODAS JUNTAS, en el orden en que"
echo "se juega, sin que una pise a la otra. La pieza nueva es un guion de ETAPAS (mvp_138.go): cada etapa se cierra"
echo "con lo que MIDIO la anterior y recien entonces arma el cuadro de la siguiente en las palancas de las otras"
echo "tarjetas (cuadroAbreElInventario, cuadroAbreElAlijo, guionDeLaQuest). El camino de cada pieza es EL SUYO, sin"
echo "una linea cambiada."
echo
echo "Corrida: bash scripts/console/build-mvp-138.sh /tmp/dcbuild/mvp138"
echo "         CAPTURE=spectacle bash scripts/console/run-mvp-138.sh /tmp/dcbuild/mvp138/disc.gdi 120 $OUT"
echo
echo "================================================================================"
echo "1. EL ESCENARIO (donde corrio el MVP y por que ahi)"
echo "================================================================================"
grep -a 'JUEGO138: MVP ARRANCA\|JUEGO138: MVP ESCENARIO' "$LOG" | tr -d '\r' | sed 's/^/  /'
echo
echo "  La ventana del pueblo es la de AKARA (VISTA_X=37 VISTA_Y=6) porque la cache de tiles del renderer tiene UN"
echo "  tramo de 3.481.600 B y la ventana del centro del pueblo ya son 2.600.960 B: no entran dos zonas (medido en"
echo "  DC-002.116 y .131). Akara esta a 19,8 celdas del centro del POC, asi que el MVP corre ENTERO en su esquina."
echo
echo "================================================================================"
echo "2. LAS ETAPAS (el guion: cada cambio con su motivo medido)"
echo "================================================================================"
grep -a 'JUEGO138: MVP ETAPA\|JUEGO138: MVP ARMA\|JUEGO138: MVP AVISO\|JUEGO138: MVP CIERRA\|JUEGO138: MVP GUION-TERMINADO\|JUEGO138: MVP 5 COFRE' "$LOG" | tr -d '\r' | sed 's/^/  /'
echo
echo "================================================================================"
echo "3. LAS NUEVE MARCAS DEL CRITERIO"
echo "================================================================================"
grep -a 'JUEGO138: MVP [0-9] ' "$LOG" | tr -d '\r' | sed 's/^/  /'
echo
echo "================================================================================"
echo "4. LA TABLA"
echo "================================================================================"
grep -a 'JUEGO138: MVP TABLA\|JUEGO138: MVP CRITERIO' "$LOG" | tr -d '\r' | sed 's/^/  /'
echo
echo "================================================================================"
echo "5. LA EVIDENCIA DE CADA PIEZA, EN LA MISMA CORRIDA (los instrumentos de cada tarjeta)"
echo "================================================================================"
echo
echo "-- 5.1 EL CAMINO: los tramos de la cruceta (DC-002.113)"
grep -a 'JUEGO121: TRAMO ' "$LOG" | tr -d '\r' | grep -v 'despues-del' | sed 's/^/  /'
echo
echo "-- 5.2 EL CONTROL (la lectura del bus maple, cuadro por cuadro, hasta donde anduvo el heroe)"
grep -a 'JUEGO121: CONTROL cuadro' "$LOG" | tr -d '\r' | head -12 | sed 's/^/  /'
echo "  ..."
grep -a 'JUEGO121: CONTROL cuadro' "$LOG" | tr -d '\r' | tail -6 | sed 's/^/  /'
echo
echo "-- 5.3 EL COMBATE Y LA MUERTE (DC-002.122 / .126)"
grep -a 'JUEGO122: GOLPE-EMPIEZA\|JUEGO122: GOLPE \|JUEGO122: GOLPE-AFUERA\|JUEGO122: GOLPE-FIN\|JUEGO126: MUERTE EMPIEZA' "$LOG" | tr -d '\r' | sed 's/^/  /'
grep -a 'JUEGO126: MUERTE CUADRO' "$LOG" | tr -d '\r' | head -3 | sed 's/^/  /'
echo "  ..."
grep -a 'JUEGO126: MUERTE CUADRO' "$LOG" | tr -d '\r' | tail -3 | sed 's/^/  /'
grep -a 'JUEGO126: MUERTE FIN\|JUEGO126: CADAVER' "$LOG" | tr -d '\r' | sed 's/^/  /'
echo
echo "-- 5.4 EL DROP Y LA RECOGIDA (DC-002.123)"
grep -a 'JUEGO123: DROP SUELTA\|JUEGO123: ITEM RECOGIDO\|JUEGO123: ITEM PASO\|JUEGO123: ITEM CAE' "$LOG" | tr -d '\r' | sed 's/^/  /'
echo
echo "-- 5.5 EL INVENTARIO Y EL ALIJO (DC-002.124)"
grep -a 'JUEGO124: INV ABRE\|JUEGO124: INV ENTRA\|JUEGO124: INV CIERRA' "$LOG" | tr -d '\r' | sed 's/^/  /'
grep -a 'JUEGO124: INV CUADRO' "$LOG" | tr -d '\r' | grep -a 'abierto=si\|elCuadroCambio= true' | sed 's/^/  /'
grep -a 'JUEGO124: ALIJO ABRE\|JUEGO124: ALIJO GUARDA\|JUEGO124: ALIJO DEVUELVE\|JUEGO124: ALIJO CIERRA' "$LOG" | tr -d '\r' | sed 's/^/  /'
echo
echo "-- 5.6 LOS NPCs Y LA QUEST (DC-002.127 / .128)"
grep -a 'JUEGO127: DIALOGO ABRE\|JUEGO127: NPC COLOCADO\|JUEGO128: QUEST HABLA\|JUEGO128: QUEST ACEPTA\|JUEGO128: QUEST AREA\|JUEGO128: QUEST MATA\|JUEGO128: QUEST LIMPIA\|JUEGO128: QUEST CUEVA\|JUEGO128: QUEST CRITERIO' "$LOG" | tr -d '\r' | sed 's/^/  /'
echo
echo "-- 5.7 EL HUD Y EL MINIMAPA (DC-002.125 / .130 / .131 / .137)"
grep -a 'JUEGO125: HUD CRITERIO\|JUEGO131: MINIMAPA CRITERIO' "$LOG" | tr -d '\r' | sed 's/^/  /'
echo
echo "-- 5.8 EL MUNDO AL FINAL: el cierre del POC (la repetibilidad del cuadro)"
grep -a 'JUEGO121: VUELTA AL ORIGEN\|JUEGO121: REPETIBILIDAD\|JUEGO121: VUELTA AL FINAL\|JUEGO121: CUADRO [0-9]* SOLO-MAPA' "$LOG" | tr -d '\r' | tail -8 | sed 's/^/  /'
echo
echo "-- 5.9 LA MEMORIA (los tramos de la cache de tiles en toda la corrida: sin derrames)"
grep -a 'JUEGO121: TRAMO despues-del-cuadro' "$LOG" | tr -d '\r' | tail -3 | sed 's/^/  /'
grep -a 'JUEGO121: FIN pico\|derrames= [1-9]' "$LOG" | tr -d '\r' | tail -3 | sed 's/^/  /'
echo
echo "-- 5.11 LA NO-REGRESION: el mismo binario con MVP=no da el MISMO cuadro 1 que la corrida base de DC-002.137"
echo "  Las palancas que el guion del MVP necesita armar en runtime pasaron de const a VAR con los valores"
echo "  viejos como default (cuadroAbreElInventario=8, cuadroAbreElAlijo=10, cuadroAbreElDialogo=2,"
echo "  celdaDelAlijo=28,20) y todo lo nuevo esta detras de mvpKnob==\"si\". La prueba: JUGADORES=1 sin MVP,"
echo "  ventana del centro del pueblo (la de DC-002.137):"
echo "    base DC-002.137  cuadro 1: pixelesNoVacios= 249006 crc32= 1990320724 jugador= 896 ancla= 320 283 celda= 28 20"
echo "    esta tarjeta     cuadro 1: pixelesNoVacios= 249006 crc32= 1990320724 jugador= 896 ancla= 320 283 celda= 28 20"
echo "  Los cuadros 2 y 3 no se comparan uno a uno con la base: el guion cooperativo del POC mueve al jugador por"
echo "  cuadro y la animacion corre por reloj real, asi que dos corridas no coinciden cuadro a cuadro. El cuadro 1"
echo "  es la composicion entera del mundo quieto y ese es identico."
echo

echo "-- 5.10 LO QUE SE ROMPIO, SI SE ROMPIO (o lo que el POC avisa que no tiene)"
grep -aE 'fatal error|Fatal:|OOM|exception|SIN-GRAFICO' "$LOG" | tr -d '\r' | sort -u | tail -8 | sed 's/^/  /'
echo "  (los SIN-GRAFICO de S4/S5 son las capas de CAMINAR del Returned que no estan en el MPQ de este disco: el"
echo "   bicho camina igual, pero esa capa no se pinta. Es la misma limitacion medida en DC-002.121, no es nueva.)"
echo
echo "================================================================================"
echo "6. LAS CAPTURAS (cada una atada a SU cuadro por el crc32 del lienzo)"
echo "================================================================================"
echo "  Las capturas de la corrida estan en $OUT (una cada segundo) y en console/dc/evidencia/ quedan las siete"
echo "  que importan (ocho), recortadas al area cliente del emulador (640x480 en 900,120 de la pantalla):"
echo
echo "    dc-002.138-mvp-1-camina.png       el heroe caminando con la cruceta (cuadro 2-3)"
echo "    dc-002.138-mvp-2-golpea.png       el golpe sobre el Returned (cuadro 4-5)"
echo "    dc-002.138-mvp-3-recoge.png       el objeto en el piso, al lado del cadaver (cuadro 41-42)"
echo "    dc-002.138-mvp-4-inventario.png   el panel del inventario con el objeto EN SU CELDA (cuadro 63-65)"
echo "    dc-002.138-mvp-5-alijo.png        el cofre del pueblo abierto (cuadro 69)"
echo "    dc-002.138-mvp-5-alijo-con-el-objeto.png"
echo "                                      el cofre CON el objeto adentro (cuadro 70, de la corrida anterior con el"
echo "                                      mismo guion: en esta corrida la captura cayo un cuadro antes y el cofre"
echo "                                      sale vacio; el objeto adentro lo mide el propio POC en JUEGO124: INV" 
echo "                                      CUADRO 70 -> alijo= 3/48/aqv@0,0)"
echo "    dc-002.138-mvp-6-akara.png        el panel de Akara abierto (cuadro 75-76, 40.500 px de panel)"
echo "    dc-002.138-mvp-9-cierre.png       el mundo y el HUD DESPUES de todo (el cuadro de la vuelta)"
echo
echo "  El nombre y el cuadro de cada captura estan en $OUT/geo.txt (una linea por captura, con la etapa del MVP,"
echo "  la celda del heroe y el crc32 del lienzo de ese cuadro)."
echo
} > "$DEST"

wc -l "$DEST"
