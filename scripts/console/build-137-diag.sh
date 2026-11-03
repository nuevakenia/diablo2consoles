#!/bin/bash
# DC-002.137: los builds de las corridas de medir del minimapa (una por modo de MINIMAPA_DIAG).
set -uo pipefail
cd /home/dgalli/Proyectos/diablo2consoles || exit 1

for modo in "$@"; do
  out="/tmp/dcbuild/anim137-$modo"
  echo "== build $modo"
  CUADROS=60 JUGADORES=1 MONSTRUOS=1 COMBATE=si GOLPE_AUTO=si HUD=si MINIMAPA=si NPC=no QUEST=no \
    INVENTARIO=no DROP=no BLIT_RAPIDO=si MEDIR_CUADRO=no ETAPAS=si VERBOSO=no BLIT_PALABRAS=si \
    PINTAR_UNA_VEZ=si MINIMAPA_RAPIDO=si MINIMAPA_DIAG="$modo" \
    bash scripts/console/build-anim-132.sh "$out" > "/tmp/dcbuild/build-137-$modo.out" 2>&1
  echo "   exit=$?"
done
