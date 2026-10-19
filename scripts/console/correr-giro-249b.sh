#!/bin/bash
# DC-002.49b -- UNA PASADA: los dos discos del giro (normal y mutante), los dos canales y los 11 de DC-002.49.
#
# POR QUE UNA PASADA Y NO COMANDOS SUELTOS
# ----------------------------------------
# La ventana de Flycast y la maquina son UN recurso: dos corridas superpuestas se pisan el keepAbove, el foco
# de las teclas virtuales y las capturas a pantalla entera (medido en DC-002.38/DC-002.49: `spectacle -f`
# devuelve la ventana que este encima). Por eso todo lo que necesita el emulador va en este script, en orden,
# con una GUARDIA al principio: si hay otro Flycast vivo o otra corrida de kanban trabajando el repo, no
# arranca y lo dice (mejor no medir que medir con otro agente encima).
#
# Uso: bash scripts/console/correr-giro-249b.sh <disc-normal> <disc-mutante> <salida>
#      (los discos los arma `build-monstruo-250.sh` con POC_DIR apuntando a la copia con el giro)
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NORMAL=${1:?falta el gdi del build normal}
MUTANTE=${2:?falta el gdi del build mutante}
SALIDA=${3:-/tmp/dc249b-pasada}

mkdir -p "$SALIDA"
LOG="$SALIDA/pasada.log"
: > "$LOG"
log() { echo "$@" | tee -a "$LOG"; }

# ---- la guardia: un solo agente y un solo emulador
otros_flycast() { pgrep -af flycast | grep -v "$$" | grep -v "$SALIDA" | wc -l; }
otros_agentes() { pgrep -af "kanban task t_" | grep -v "$$" | wc -l; }

log "== guardia: Flycast vivos=$(otros_flycast)  agentes de kanban=$(otros_agentes)"
if [ "$(otros_flycast)" -ne 0 ] || [ "$(otros_agentes)" -ne 0 ]; then
    log "   NO arranco: el emulador o el repo estan tomados por otra corrida (regla del repo: un agente a la vez)."
    log "   Volve a llamar cuando este libre; no se midio nada."
    exit 3
fi

log ""
log "== 1/3 la corrida CON el giro: $NORMAL"
bash "$HERE/run-flycast-249b.sh" "$NORMAL" "$SALIDA/normal" 2>&1 | tee -a "$LOG"

if [ "$(otros_agentes)" -ne 0 ]; then
    log "   AVISO: mientras corria, otro agente de kanban arranco (agentes=$(otros_agentes))."
fi

log ""
log "== 2/3 la corrida del MUTANTE (giroSordo=true): $MUTANTE"
bash "$HERE/run-flycast-249b.sh" "$MUTANTE" "$SALIDA/sordo" 2>&1 | tee -a "$LOG"

log ""
log "== 3/3 los dos canales + los 11 de DC-002.49"
bash "$HERE/checar-giro-249b-corrida.sh" "$(dirname "$NORMAL")" "$SALIDA/normal" "$SALIDA/sordo" "$SALIDA/corpus.txt" \
    2>&1 | tee -a "$LOG"
ESTADO=${PIPESTATUS[0]}

log ""
log "== la tabla del host para clasificar las capturas (TN, la direccion de cada PNG)"
grep -a 'CRC249B: TN' "$(dirname "$NORMAL")/host/crc249b.txt" > "$SALIDA/tabla-tn.txt" 2>/dev/null
log "   $(wc -l < "$SALIDA/tabla-tn.txt") lineas en $SALIDA/tabla-tn.txt"

log ""
if [ "$ESTADO" -eq 0 ]; then
    log "VERDE: la pasada del giro termino entera. Informe: $SALIDA/corpus.txt"
    exit 0
fi
log "ROJO: la pasada dejo fallas. Informe: $SALIDA/corpus.txt"
exit 1
