#!/bin/bash
# DC-002.158: arma el log de evidencia que se guarda en console/dc/evidencia/ a partir de las corridas de
# Flycast (los logs enteros quedan en /tmp, que se borra: esto es el extracto que la tarjeta mide).
set -uo pipefail
cd "$(dirname "$0")/../.." || exit 1
DEST=console/dc/evidencia/dc-002.158-reloj-flycast.log

{
    echo "=== 1. EL POC DEL RELOJ CON EL DEFECTO (console/dc/std/time/time.go antes del arreglo) ==="
    echo "    corrida: /tmp/reloj158-antes (Flycast, 20 s) -- POC console/dc/poc/reloj-158"
    tr -d '\r' < /tmp/reloj158-antes/flycast.log | grep -a "RELOJ158"
    echo
    echo "=== 2. EL POC DEL RELOJ CON EL ARREGLO (+ el reloj de pared del host, scripts/console/medir-pared-158.sh) ==="
    echo "    corrida: /tmp/reloj158-pared (Flycast, 25 s)"
    tr -d '\r' < /tmp/reloj158-pared/flycast.log | grep -a "RELOJ158"
    echo
    echo "=== 3. EL GATE 0 CON EL RELOJ ARREGLADO (corrida de 185 s, /tmp/capturas158) ==="
    echo "    binario: /tmp/dcbuild/gate0-pueblo-158 (5.910.020 B cargados, con _time.baseUnix adentro)"
    echo "    el reloj del PVR (EBITEN157) y el de la pantalla (CUADRO / TRAMO157) en la MISMA corrida:"
    tr -d '\r' < /tmp/capturas158/flycast.log \
        | grep -aE "pueblo compuesto|CUADRO [0-9]+ ms=|EBITEN-PERFIL|TRAMO157|EBITEN157: FASES" \
        | head -32
    echo "    (...) las FASES siguen iguales hasta el final: son el reloj del PVR, que ya estaba bien."
} > "$DEST"

echo "escrito: $DEST ($(wc -l < "$DEST") lineas, $(stat -c%s "$DEST") B)"
