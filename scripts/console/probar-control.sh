#!/bin/bash
# La prueba del CONTROL de la Dreamcast, de punta a punta y sin que nadie apriete nada.
#
# QUE PRUEBA: que el control se lee del bus maple, que los botones llegan con los bits correctos, que la
# cruceta mueve el cursor del motor, y que el logo del POC sigue al cursor. Lo ultimo se mide con el CENTRO
# DE MASA del contenido de la pantalla (dos numeros), no mirando una captura: "el logo se movio" tiene que
# poder compararse entre corridas.
#
# COMO LE MANDA LAS TECLAS AL EMULADOR, que es la parte que costo: en Wayland no hay xdotool, y `wtype` no
# sirve porque el compositor de KDE no tiene el protocolo de teclado virtual ("Compositor does not support the
# virtual keyboard protocol"). Pero /dev/uinput es del KERNEL: lo que se escribe ahi lo ve cualquier cosa que
# lea el teclado. Ver teclas-virtuales.py.
#
# USO:  bash scripts/console/probar-control.sh [segundos]
set -u

HERE="$(cd "$(dirname "$0")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)"
SEGS="${1:-40}"
SALIDA="${SALIDA:-/tmp/dcframes-control}"
# la imagen ya compilada del POC (build-dc.sh la deja aca). Si no esta, se dice y se sale: correr el emulador
# con una imagen vieja probaria el codigo de antes, que es la forma mas silenciosa de dar por bueno un cambio.
DISCO="${DISCO:-/tmp/dcbuild/motor-pantalla/disc.gdi}"

echo "== la prueba del control (el POC: motor-pantalla, ${SEGS}s) =="

if [ ! -f "$DISCO" ]; then
  echo "  no esta la imagen del POC: $DISCO"
  echo "  compilala primero:  bash scripts/console/build-dc.sh console/dc/poc/motor-pantalla /tmp/dcbuild/motor-pantalla"
  exit 1
fi

mkdir -p "$SALIDA"

(
  cd "$REPO"
  # shellcheck disable=SC1091
  source scripts/console/env.sh
  bash scripts/console/run-flycast.sh "$DISCO" "$SEGS" "$SALIDA"
) &
LA_CORRIDA=$!

# el emulador tarda unos segundos en arrancar: se espera a que el POC avise que hay control
ESPERA=0
while [ "$ESPERA" -lt 30 ]; do
  sleep 2
  ESPERA=$(( ESPERA + 2 ))
  grep -qa 'hay control' "$SALIDA/flycast.log" 2>/dev/null && break
done

echo "  el POC arranco (o se espero lo suficiente); mando las teclas"
python3 "$HERE/teclas-virtuales.py" $(for _ in $(seq 1 18); do echo derecha; done) $(for _ in $(seq 1 10); do echo abajo; done)

wait "$LA_CORRIDA" 2>/dev/null

echo
echo "== lo que dijo la consola serie =="
grep -a 'hay control\|MOTOR-CONTROL\|MOTOR-PANTALLA cuadros' "$SALIDA/flycast.log" 2>/dev/null | head -30
echo
echo "== el veredicto =="
grep -a 'MOTOR-PANTALLA cuadros' "$SALIDA/flycast.log" 2>/dev/null | awk '
  { for (i = 1; i <= NF; i++) if ($i == "centro") { cx = $(i+1); cy = $(i+2) } }
  { print "  cuadro", $3, "centro del contenido", cx, cy }
'
