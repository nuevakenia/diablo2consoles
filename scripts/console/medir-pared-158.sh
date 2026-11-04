#!/bin/bash
# DC-002.158: el reloj de la consola contra el RELOJ DE PARED DEL HOST.
#
# El POC del reloj (console/dc/poc/reloj-158) imprime una linea por vuelta y cierra con RESUMEN. Entre la
# linea de la VUELTA 1 y el RESUMEN el programa duerme 1000+300+150+700+1000+2000 = 5150 ms (las esperas 2 a
# 7; la 1 ya paso cuando imprime su linea). Aca se mide ese mismo intervalo con el reloj del host, mirando el
# log, para tener las TRES lecturas del mismo tramo: el timer crudo de KOS, Now() y el reloj de pared.
set -uo pipefail
cd "$(dirname "$0")/../.." || exit 1
OUT=/tmp/reloj158-pared
rm -rf "$OUT"
mkdir -p "$OUT"

CAPTURE=none bash scripts/console/run-flycast.sh /tmp/dcbuild/reloj-158/prog.elf 25 "$OUT" >/dev/null 2>&1 &
P=$!
t1=""
t2=""
while kill -0 "$P" 2>/dev/null; do
    log="$OUT/flycast.log"
    if [ -z "$t1" ] && grep -aq "VUELTA 1 " "$log" 2>/dev/null; then
        t1="$(date +%s%3N)"
    fi
    if grep -aq "RESUMEN" "$log" 2>/dev/null; then
        t2="$(date +%s%3N)"
        break
    fi
    sleep 0.02
done
wait "$P" 2>/dev/null

if [ -z "$t1" ] || [ -z "$t2" ]; then
    echo "no pude medir (el POC no imprimio las dos marcas)"
    exit 1
fi
pared=$((t2 - t1))
echo "pared_ms=$pared esperado_ms=5150"
echo "desvio_mil=$(( (pared - 5150) * 1000 / 5150 ))"
