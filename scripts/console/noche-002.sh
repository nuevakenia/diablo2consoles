#!/bin/bash
# Corrida autonoma del port de Diablo II a la Dreamcast (DC-002).
#
# Lanza UNA sesion de Hermes que sigue con la tarjeta abierta del tablero, con el metodo del
# proyecto. Pensado para reanudar el trabajo cuando el agente principal se corta (contexto, cierre de
# sesion, un turno que no se puede continuar): lo llama el cron cada 45 minutos.
#
# Toma un lock para que NO haya dos agentes sobre el mismo repo (regla del proyecto: una rama, un
# agente, una carpeta). Si ya hay una corrida en curso, sale sin hacer nada.
#
# Uso:  scripts/console/noche-002.sh            (una corrida)
#       scripts/console/noche-002.sh --ver      (muestra el estado del lock y el log)
set -uo pipefail

REPO=/home/dgalli/Proyectos/diablo2consoles
LOCK=/tmp/dc-002-noche.lock
LOG=/home/dgalli/.hermes/cache/scratch/noche-002.log
PROMPT="$REPO/scripts/console/prompt-noche.txt"
TOPE_MIN=150   # tope de la corrida (minutos); Hermes corta solo si se pasa

if [ "${1:-}" = "--ver" ]; then
    echo "lock: $( [ -f "$LOCK" ] && cat "$LOCK" || echo 'libre' )"
    echo "log ($LOG): $( [ -f "$LOG" ] && wc -l < "$LOG" || echo 0 ) lineas"
    tail -20 "$LOG" 2>/dev/null
    exit 0
fi

exec 9>"$LOCK"
if ! flock -n 9; then
    echo "$(date -Is) ya hay una corrida en curso (lock $LOCK): no arranco otra"
    exit 0
fi

mkdir -p "$(dirname "$LOG")"
{
    echo
    echo "================================================================"
    echo "corrida autonoma: $(date -Is)"
    echo "================================================================"
} >> "$LOG"

cd "$REPO" || exit 1
# la rama tiene que ser DC-002: si alguien la movio, esto no corre a ciegas
RAMA="$(git rev-parse --abbrev-ref HEAD)"
if [ "$RAMA" != "DC-002" ]; then
    echo "$(date -Is) la rama es $RAMA y no DC-002: no arranco" >> "$LOG"
    exit 0
fi
if [ -n "$(git status --porcelain)" ]; then
    echo "$(date -Is) el arbol tiene cambios sin commitear: los miro antes de lanzar al agente" >> "$LOG"
    git status --short >> "$LOG"
fi

# NOCHE_SECO=1 prueba toda la mecanica (lock, rama, prompt) sin lanzar al agente: sirve para
# verificar el script mientras ya hay una corrida (esta!) trabajando sobre el repo.
if [ "${NOCHE_SECO:-0}" = "1" ]; then
    echo "modo seco: lock tomado, rama $RAMA, prompt de $(wc -c < "$PROMPT") bytes. No lanzo al agente."
    exit 0
fi

# --continue NO: una sesion nueva lee el tablero y el README, que es la fuente de verdad. Asi la
# corrida no depende de que la sesion anterior haya quedado bien cerrada.
timeout "${TOPE_MIN}m" hermes chat -q "$(cat "$PROMPT")" >> "$LOG" 2>&1
echo "$(date -Is) la corrida termino con codigo $?" >> "$LOG"
