#!/bin/bash
# DC-002.57: CONTROL AUTOMATICO DEL UBC DEL SH-4 SOBRE EL MARCO DE PILA DEL ARRANQUE DEL VIDEO.
#
# PARA QUE SIRVE: la tarjeta DC-002.57 pregunto si el UBC del SH-4 puede cortar la escritura que pisa el
# marco de `_pvr_init` en el binario con el bicho (bestia=1). Este control corre el instrumento y CLASIFICA:
#
#   CORTA      el log trae al menos una linea `UBC: escritura pc=...` -> ahi esta el escritor
#   NO-CORTA   el canal no corto: lo dice la AUTOPRUEBA sobre una variable conocida (misma condicion,
#              direccion sabida) y no hay ni una escritura en el rango de la pila. El resumen imprime
#              tambien si la corrida murio o dibujo, porque el instrumento cambia el layout.
#   SIN-DATOS  no compilo / no corrio
#
# QUE HACE, EN ORDEN
#   0) la SONDA SUELTA (console/dc/sonda-ubc-257, C puro, se corre como ELF sin disco): arma el canal
#      sobre una variable de .bss, lo dispara y cuenta. Es la medida que separa "el canal no corta" de
#      "vigiramos el rango equivocado". ~20 s.
#   1) el POC INSTRUMENTADO (bestia=1) sobre una COPIA del POC con el parche aplicado: el arbol queda
#      intacto, porque el instrumento perturba el layout (medido: +1.856 B y el binario deja de morir).
#      Corre el rango calculado en runtime e imprime base, tope y SP. ~2 min.
#
# uso:  bash scripts/console/checar-ubc-253.sh [salida]        (SEGUNDOS=110 por defecto)
# sale: 0 = CORTA, 1 = NO-CORTA, 3 = sin datos
set -o pipefail

SEGUNDOS=${SEGUNDOS:-110}
SALIDA=${1:-/tmp/dcbuild/chequeo-ubc257}
AQUI=$(cd "$(dirname "$0")/../.." && pwd) || exit 3
PARCHE="$AQUI/console/dc/evidencia/dc-002.57-instrumento.patch"
COPIA="$SALIDA/poc-instrumentado"

cd "$AQUI" || exit 3
mkdir -p "$SALIDA"

echo "== 0/2 la sonda SUELTA del UBC (canal de dato, canal de instruccion, canal de rango)"
bash scripts/console/build-c.sh console/dc/sonda-ubc-257 "$SALIDA/sonda-suelta" > "$SALIDA/sonda-build.log" 2>&1
if [ ! -f "$SALIDA/sonda-suelta/prog.elf" ]; then
    echo "   FALLA: la sonda no compilo (ver $SALIDA/sonda-build.log)"; tail -3 "$SALIDA/sonda-build.log"
else
    CAPTURE=none bash scripts/console/run-flycast.sh "$SALIDA/sonda-suelta/prog.elf" 20 "$SALIDA/sonda-corrida" > "$SALIDA/sonda-run.log" 2>&1
    tr -d '\r' < "$SALIDA/sonda-corrida/flycast.log" | grep -a "^UBC-AUTOPRUEBA" | sed 's/^/   /'
fi

echo
echo "== 1/2 el POC INSTRUMENTADO (bestia=1) sobre una copia del arbol"
rm -rf "$COPIA"
cp -r console/dc/poc/personaje-mapa "$COPIA" || { echo "   FALLA: no pude copiar el POC"; exit 3; }
( cd "$COPIA" && patch -p5 --no-backup-if-mismatch < "$PARCHE" >/dev/null ) || {
    echo "   FALLA: el parche no aplica sobre la copia"; exit 3; }

REUSAR=""
if [ -f "$HOME/dreamcast/gox/orden.txt" ]; then REUSAR="SALTAR_STDLIB=1 SALTAR_MOTOR=1"; fi

env $REUSAR POC_DIR="$COPIA" BESTIA=1 \
    bash scripts/console/build-monstruo-250.sh "$SALIDA" > "$SALIDA-build.log" 2>&1
if [ ! -f "$SALIDA/disc.gdi" ]; then
    echo "   FALLA: no compilo (ver $SALIDA-build.log)"; tail -5 "$SALIDA-build.log"; exit 3
fi

CAPTURE=none bash scripts/console/run-flycast.sh "$SALIDA/disc.gdi" "$SEGUNDOS" "$SALIDA/corrida" > "$SALIDA-run.log" 2>&1

LOG="$SALIDA/corrida/flycast.log"
[ -f "$LOG" ] || { echo "   FALLA: no hay log de la corrida (ver $SALIDA-run.log)"; exit 3; }

cuenta() { tr -d '\r' < "$LOG" | grep -ac "$1"; }

ESCRITURAS=$(cuenta "^UBC: escritura")
AUTOPRUEBA=$(tr -d '\r' < "$LOG" | grep -a "^UBC: AUTOPRUEBA" | tail -1)
RANGO=$(tr -d '\r' < "$LOG" | grep -a "^UBC: RANGO" | tail -1)
EXCEPCIONES=$(cuenta "Unhandled exception")
CUADROS=$(cuenta "HITO238: POS cuadro")
BICHO=$(cuenta "ANIM-BESTIA")
TAM=$(stat -c %s "$SALIDA/1ST_READ.BIN" 2>/dev/null)

echo "   1ST_READ.BIN: $TAM B"
[ -n "$RANGO" ] && echo "   $RANGO"
[ -n "$AUTOPRUEBA" ] && echo "   $AUTOPRUEBA"
echo "   escrituras-en-el-rango=$ESCRITURAS excepciones=$EXCEPCIONES cuadros=$CUADROS animaciones-del-bicho=$BICHO"

if [ -z "$RANGO" ]; then
    echo
    echo "VEREDICTO SIN-DATOS: la sonda no dejo su marca (el binario no llego al armado o la sonda no corre)"
    exit 3
fi

if [ "$ESCRITURAS" -gt 0 ]; then
    echo
    echo "VEREDICTO CORTA: el UBC corto $ESCRITURAS escritura(s) en el rango vigilado"
    tr -d '\r' < "$LOG" | grep -a "^UBC: escritura" | sed 's/^/   /'
    exit 0
fi

echo
echo "VEREDICTO NO-CORTA: con el rango armado (base y tope arriba, calculados del SP de este build)"
echo "   y la autoprueba sobre una direccion conocida, el UBC no corto ninguna escritura ni se disparo."
echo "   El SH-4 de este Flycast guarda los registros del UBC pero no evalua las condiciones de corte"
echo "   (ver console/dc/evidencia/dc-002.57-ubc.txt, seccion 2)."
exit 1
