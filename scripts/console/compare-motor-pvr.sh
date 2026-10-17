#!/bin/bash
# Corre el POC de DC-002.9 (motor-pvr) en Flycast y compara su resumen "MOTOR-PVR rgba frame ..." contra
# el que da el mismo codigo del motor en el host, sobre los MISMOS bytes del DC6 y de la paleta.
#
# Lo que se compara son DOS cosas por frame:
#   - los indices decodificados (MOTOR-DC6 frame ...), que ya medía DC-002.3
#   - los RGBA finales (MOTOR-PVR rgba frame ...): el COLOR, que es lo nuevo
# El hash FNV-1a de los dos lados es lo que hace que "se ve igual" sea un numero y no una impresion.
#
# Uso:  scripts/console/compare-motor-pvr.sh [dir_de_build] [nombre_asset]
#       CAPTURE=spectacle para que ademas saque capturas de pantalla (se ven en <dir>/run/f*.png)
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

POC=motor-pvr bash "$HERE/compare-motor-dc6.sh" "${1:-/tmp/dcbuild/motor-pvr}" "${2:-d2logo}"
