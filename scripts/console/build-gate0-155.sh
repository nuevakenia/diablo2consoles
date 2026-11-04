#!/bin/bash
# DC-002.155: EL BUILD DEL GATE 0 -- el pueblo del acto 1 por el arranque del MOTOR en la Dreamcast.
#
# Es `build-motor-arranque.sh` (el que enlaza los 42 paquetes del motor + la stdlib + los terceros) con el
# POC de esta tarjeta y las palancas de la consola puestas en UN solo lugar, para que la corrida se pueda
# repetir sin recordar las seis variables:
#
#   POC=gate0-pueblo          el main que arranca d2app y carga la pantalla del pueblo
#   DC_RUTA_DATOS=/cd/juego    los MPQ del juego se leen del DISCO (no del romdisk de 16 MB)
#   DC_MPQ_JUEGO=...           los tres que el pueblo necesita (d2data, d2char, patch_D2)
#   DC_SIN_SELLOS=1            no se crean los archivos vacios: los MPQ van de verdad en la ISO
#   DC_DISCO_JUEGO=<dir>       los MPQ que se hornean en la imagen (symlinks, ver abajo)
#   DC_EH_FINAL=1              el `.eh_frame` al final del mapa y fuera de la imagen plana: el monton de C
#                              gana sus 1.076.512 B (DC-002.47) -- sin eso el presupuesto de DC-002.93 no es
#                              el que se esta midiendo
#   DC_LDEXTRA="-Wl,--wrap=malloc"
#                              el `malloc` interpuesto (DC-002.136): cada pedido de 64 KB o mas queda en el log
#                              con su tamano y su direccion de retorno. Es lo que atribuye el OOM a un pedido
#                              concreto en vez de a "el motor".
#
# LOS MPQ DEL DISCO: `mkdisco.sh` hornea TODOS los `*.mpq` del directorio que se le pasa, y el directorio de
# la instalacion tiene 1,8 GB (musica, videos, voces) que este POC no abre. Por eso se arma un directorio de
# trabajo con TRES symlinks y se hornea ese: la ISO queda en ~542 MB, como las otras tarjetas del disco.
#
# Uso:  scripts/console/build-gate0-155.sh [dir_salida] [dir_gox]
#       Antes: bash scripts/console/build-std-gox.sh /tmp/dc155/gox
#              bash scripts/console/build-motor-all.sh /tmp/dc155/gox /tmp/dc155/log-motor
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

REPO="$(cd "$HERE/../.." && pwd)"
SAL="${1:-/tmp/dcbuild/gate0-pueblo}"
GOX="${2:-${GODC_GOX:-/tmp/dc155/gox}}"

D2="${D2_DIR:-$HOME/.wine-d2/drive_c/Games/Diablo II}"
[ -f "$D2/d2data.mpq" ] || { echo "no encuentro los datos del juego en $D2 (usa D2_DIR=...)" >&2; exit 1; }

# el directorio de horneado: dos symlinks, no una copia de 540 MB
JUEGO="$SAL/juego"
mkdir -p "$JUEGO" "$SAL"
for mpq in d2data.mpq patch_D2.mpq; do
    [ -f "$D2/$mpq" ] || { echo "falta $D2/$mpq" >&2; exit 1; }
    ln -sf "$D2/$mpq" "$JUEGO/$mpq"
done

echo "== el pueblo por el motor (DC-002.155) =="
echo "   POC=gate0-pueblo  gox=$GOX  salida=$SAL"
echo "   datos del juego: $D2 -> /cd/juego (2 MPQ en la ISO: d2data + patch_D2)"
echo "   palancas: eh-final=1 wrap-malloc=1 sin-sellos=1 mpq=d2data,patch_D2 (sin d2char: el pueblo no tiene"
echo "             ni un grafico de personaje, y sus dos tablas de MPQ son ~196 KB del monton de C)"

# shellcheck disable=SC2086
POC=gate0-pueblo \
DC_RUTA_DATOS=/cd/juego \
DC_MPQ_JUEGO="d2data.mpq,patch_d2.mpq" \
DC_SIN_SELLOS=1 \
DC_DISCO_JUEGO="$JUEGO" \
DC_EH_FINAL=1 \
DC_LDEXTRA="-Wl,--wrap=malloc" \
    bash "$HERE/build-motor-arranque.sh" "$SAL" "$GOX"

echo
echo "== listo: $SAL/disc.gdi (Flycast) y $SAL/prog-emu.elf (ciclo rapido) =="
