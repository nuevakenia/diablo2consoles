#!/usr/bin/env bash
# DC-002.49b -- el corpus del GIRO en una sola corrida: la tabla del host, el log del POC y el mutante.
#
# Uso: bash scripts/console/checar-giro-249b-corrida.sh <build> <corrida> [<corrida-mutante>] [salida]
#
#   <build>            el directorio del build con el giro (deja la tabla en <build>/host/crc249b.txt)
#   <corrida>          la corrida de Flycast (o el flycast.log) del build CON el giro
#   <corrida-mutante>  opcional: la corrida del build con giroSordo=true (el mutante del criterio 4)
#   <salida>           archivo donde queda el informe (por defecto /tmp/dc249b-corpus.txt)
#
# QUE HACE
#   1. la TABLA del host: `verif-personaje-238 -censo-dirs TN,TW -crc-dirs` da el crc32 de CADA cuadro de CADA
#      direccion, compuesto con el mismo compositor que la consola. Medido: TN 256 compuestos, 256 crc32
#      DISTINTOS (0 colisiones) -> el crc32 de una linea `HITO249: ANIM` dice que direccion se dibujo.
#      Se guarda en <build>/host/ para que la corrida quede reproducible sin volver a censar.
#   2. la corrida NORMAL: composicion (las 8 direcciones del control), los crc32 conocidos, el GIRO que llega
#      al dibujo y la cadencia.
#   3. la corrida del MUTANTE: con giroSordo=true NO puede haber GIRO ni otra direccion que la 0.
#   4. los 11 de DC-002.49 (checar-anim-249.py), que NO se pueden romper: se corren si estan las dos corridas.
#
# Exit: 0 solo si TODO da verde.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)"
PY="${DC_PYTHON:-python3}"

BUILD=${1:?uso: checar-giro-249b-corrida.sh <build> <corrida> [<corrida-mutante>] [salida]}
CORRIDA=${2:?falta la corrida}
CORRIDA_MUDA=${3:-}
SALIDA=${4:-/tmp/dc249b-corpus.txt}

mkdir -p "$BUILD/host"
TABLA="$BUILD/host/crc249b.txt"

: > "$SALIDA"
fallas=0

log() { echo "$@" | tee -a "$SALIDA"; }

if [ ! -s "$TABLA" ]; then
    log "== 0. la tabla del host (crc32 por direccion y cuadro): no estaba, se censa"
    ( cd "$REPO" && go run ./console/dc/verif-personaje-238 -censo-dirs TN,TW -crc-dirs ) > "$TABLA" 2>"$TABLA.err" \
        || { log "   FALLA el censo del host (ver $TABLA.err)"; exit 1; }
fi
log "   tabla: $TABLA ($(grep -c 'CRC249B' "$TABLA") compuestos)"

log ""
log "== 1. la corrida CON el giro: $CORRIDA"
"$PY" "$HERE/checar-giro-249b.py" --run "$CORRIDA" --tabla "$TABLA" 2>&1 | tee -a "$SALIDA"
[ "${PIPESTATUS[0]}" -eq 0 ] || fallas=$((fallas + 1))

if [ -n "$CORRIDA_MUDA" ]; then
    log ""
    log "== 2. la corrida del MUTANTE (giroSordo): $CORRIDA_MUDA"
    "$PY" "$HERE/checar-giro-249b.py" --run "$CORRIDA_MUDA" --tabla "$TABLA" --mutante 2>&1 | tee -a "$SALIDA"
    [ "${PIPESTATUS[0]}" -eq 0 ] || fallas=$((fallas + 1))

    log ""
    log "== 3. los 11 de DC-002.49 no se rompen (el corpus de la animacion)"
    if [ -f "$HERE/checar-anim-249.py" ]; then
        VENTANA=$(grep -a -o "cliente=[0-9,]*" "$CORRIDA/flycast.log" 2>/dev/null | head -1 | cut -d= -f2)
        "$PY" "$HERE/checar-anim-249.py" --host "$BUILD/host" --run-normal "$CORRIDA" \
            --run-mutante "$CORRIDA_MUDA" --ventana "${VENTANA:-1400,120,667,442}" 2>&1 | tee -a "$SALIDA" | tail -6
        [ "${PIPESTATUS[0]}" -eq 0 ] || fallas=$((fallas + 1))
    else
        log "   (no esta checar-anim-249.py: se saltea)"
    fi
fi

log ""
if [ "$fallas" -eq 0 ]; then
    log "VERDE: el corpus del giro paso entero. Informe: $SALIDA"
    exit 0
fi
log "ROJO: $fallas bloque(s) fallaron. Informe: $SALIDA"
exit 1
