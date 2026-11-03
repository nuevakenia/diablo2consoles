#!/bin/bash
# DC-002.137 -- las corridas de la evidencia del minimapa, de una: build + corrida por combinacion.
#
# Uso:  bash scripts/console/corridas-137-minimapa.sh <etiqueta>:<RAPIDO>:<VERBOSO>:<DIAG> ...
#   bash scripts/console/corridas-137-minimapa.sh evidencia:si:no:no viejo:no:no:no
#
# Cada combinacion deja el build en /tmp/dcbuild/anim137-<etiqueta>, el log de la corrida en
# /tmp/capturas-137-<etiqueta>/flycast.log y las capturas en ese mismo directorio. Las palancas de la tarjeta
# (MINIMAPA_RAPIDO, MINIMAPA_DIAG) se pisan en la copia del POC del build, no en el arbol.
set -uo pipefail
cd /home/dgalli/Proyectos/diablo2consoles || exit 1

for combo in "$@"; do
  IFS=: read -r etiqueta rapido verboso diag <<< "$combo"

  echo "== $etiqueta (rapido=$rapido verboso=$verboso diag=$diag)"

  CUADROS=60 JUGADORES=1 MONSTRUOS=1 COMBATE=si GOLPE_AUTO=si HUD=si MINIMAPA=si NPC=no QUEST=no \
    INVENTARIO=no DROP=no BLIT_RAPIDO=si MEDIR_CUADRO=si ETAPAS=si VERBOSO="$verboso" BLIT_PALABRAS=si \
    PINTAR_UNA_VEZ=si MINIMAPA_RAPIDO="$rapido" MINIMAPA_DIAG="$diag" \
    bash scripts/console/build-anim-132.sh "/tmp/dcbuild/anim137-$etiqueta" \
    > "/tmp/dcbuild/build-137-$etiqueta.out" 2>&1

  if [ ! -f "/tmp/dcbuild/anim137-$etiqueta/disc.gdi" ]; then
    echo "   BUILD FALLO: ver /tmp/dcbuild/build-137-$etiqueta.out"
    continue
  fi

  CAPTURAS=2 INTERVALO=10 ESPERA_CIERRE=300 \
    bash scripts/console/run-anim-132.sh "/tmp/dcbuild/anim137-$etiqueta/disc.gdi" "/tmp/capturas-137-$etiqueta" \
    900 120 > "/tmp/run-137-$etiqueta.out" 2>&1

  echo "   corrida exit=$? lineas=$(wc -l < "/tmp/capturas-137-$etiqueta/flycast.log" 2>/dev/null)"
done
