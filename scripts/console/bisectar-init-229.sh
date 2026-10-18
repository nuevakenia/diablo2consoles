#!/bin/bash
# DC-002.29: BISECCION del `init` DE PAQUETE QUE MATA EL ARRANQUE REARMADO (sin rearmar el binario).
#
# QUE HACE
#   Repunta UNA llamada de `___go_init_main` (parchar-init-229.py: 4 bytes en su lugar, el binario queda del
#   MISMO tamano) a una direccion NO mapeada de la zona 3 del SH-4 (0x0FF00000+4k) y corre Flycast.
#     - si la corrida LLEGA a esa llamada, el PC se va a la zona basura y la sonda de DC-002.24b lo grita
#       (`SONDA-PC FUERA pc=0ff0...`): el arranque paso por los `init` 1..k-1;
#     - si NO llega, la corrida muere con el defecto de la tarjeta antes (0 cuadros, reinicios del bus).
#   Con eso se BUSCA EL ULTIMO k ALCANZADO: el `init` que mata es el k+1.
#
# Uso: bash scripts/console/bisectar-init-229.sh <prog.elf> [segundos] [sal]
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=env.sh
. "$HERE/env.sh"

ELF="${1:?uso: bisectar-init-229.sh <prog.elf> [segundos] [sal]}"
SEGS="${2:-15}"
SAL="${3:-/tmp/dc229-biseccion}"
mkdir -p "$SAL"
N=$(python3 "$HERE/parchar-init-229.py" --lista "$ELF" | head -1 | awk '{print $1}')

alcanza() {   # $1 = k -> 0/1 ; deja el log en $SAL/k$1.log
    local k="$1"
    python3 "$HERE/parchar-init-229.py" "$ELF" "$SAL/k$k.elf" "$k" > "$SAL/k$k.parche.log" 2>&1 || return 2
    sh-elf-objcopy -R .go_export -R .comment --strip-debug "$SAL/k$k.elf" "$SAL/k$k-emu.elf" || return 2
    sh-elf-strip -s "$SAL/k$k-emu.elf"
    CAPTURE=none bash "$HERE/run-flycast.sh" "$SAL/k$k-emu.elf" "$SEGS" "$SAL/run-k$k" > "$SAL/k$k.run.log" 2>&1
    tr -d '\r' < "$SAL/run-k$k/flycast.log" > "$SAL/k$k.log"
    local f
    f=$(grep -ac 'SONDA-PC FUERA' "$SAL/k$k.log")
    local fin
    fin=$(grep -a 'SONDA-PC FUERA' "$SAL/k$k.log" | head -1)
    echo "   k=$k  alcanzado=$f  $fin"
    [ "$f" -gt 0 ]
}

echo "== biseccion del arranque: $ELF ($N llamadas de init, $SEGS s por corrida)"
BAJO=0; ALTO=$N
# primer tanteo: la ultima llamada. Si el arranque llega AHI, no hay nada que buscar (el defecto no esta en los init)
if ! alcanza "$N"; then
    ALTO=$N
else
    echo "   el arranque llega al ULTIMO init: el defecto NO esta en los init de paquete"
    exit 0
fi
if alcanza 1; then BAJO=1; else echo "   ni la primera llamada se alcanza: el defecto es ANTERIOR a los init"; exit 0; fi

while [ $((ALTO - BAJO)) -gt 1 ]; do
    MEDIO=$(((BAJO + ALTO) / 2))
    if alcanza "$MEDIO"; then BAJO=$MEDIO; else ALTO=$MEDIO; fi
done
echo
echo "== RESULTADO: los init 1..$BAJO se alcanzan (y vuelven); el que mata es el $ALTO de $N:"
python3 "$HERE/parchar-init-229.py" --lista "$ELF" | sed -n "$((ALTO + 1))p"
echo "== logs: $SAL"
