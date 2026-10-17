#!/bin/bash
# Build del POC de DC-002.9 (motor-pvr): el mismo flujo que el POC de DC-002.3 pero horneando TAMBIEN
# la paleta (pal=... en el asset.conf del POC), porque lo que se mide es el COLOR: el motor decodifica
# el DC6 y la paleta, y el PVR pinta el resultado como textura de 1 byte por pixel.
#
# Uso:  scripts/console/build-motor-pvr.sh [dir_salida]     (por defecto /tmp/dcbuild/motor-pvr)
#       D2_DIR='/ruta/Diablo II' para decir donde esta el juego si no esta en los caminos tipicos.
#
# Despues:  scripts/console/compare-motor-pvr.sh
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

POC=motor-pvr bash "$HERE/build-motor-dc6.sh" "${1:-/tmp/dcbuild/motor-pvr}"
