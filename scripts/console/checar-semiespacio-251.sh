#!/bin/bash
# DC-002.51: CONTROL DEL SEMIESPACIO DEL GC (opcion A: 2048 KB por semiespacio).
#
# Verifica tres cosas, en orden:
#   1. el #define que esta en el repo de libgodc,
#   2. el que quedo INSTALADO en el KOS (que es el que usa el enlazador; si difieren, el build miente),
#   3. si se le pasa un log de Flycast, que la separacion entre los dos semiespacios del arranque sea
#      la esperada (2 MB = 2.097.152 B, con el redondeo del runtime).
#
# Uso:  bash scripts/console/checar-semiespacio-251.sh [log-de-flycast]
# Sale 0 si todo cuadra, 1 si algo no.
set -uo pipefail

KB_ESPERADO=2048
LIBGODC="${LIBGODC:-$HOME/dreamcast/libgodc}"
KOS_BASE="${KOS_BASE:-$HOME/dreamcast/kos}"
fallos=0

decir() { printf '%s\n' "$*"; }

# 1) repo
h="$LIBGODC/runtime/godc_config.h"
if [ -f "$h" ]; then
  kb=$(grep -E '^#define[[:space:]]+GC_SEMISPACE_SIZE_KB' "$h" | awk '{print $3}')
  decir "repo libgodc : GC_SEMISPACE_SIZE_KB = ${kb:-?}  ($h)"
  [ "${kb:-0}" = "$KB_ESPERADO" ] || { decir "  FALLA: se esperaba $KB_ESPERADO"; fallos=$((fallos+1)); }
else
  decir "repo libgodc : no encuentro $h"; fallos=$((fallos+1))
fi

# 2) instalado
hi="$KOS_BASE/include/godc/godc_config.h"
if [ -f "$hi" ]; then
  kbi=$(grep -E '^#define[[:space:]]+GC_SEMISPACE_SIZE_KB' "$hi" | awk '{print $3}')
  decir "KOS instalado: GC_SEMISPACE_SIZE_KB = ${kbi:-?}  ($hi)"
  [ "${kbi:-0}" = "$KB_ESPERADO" ] || { decir "  FALLA: el instalado no coincide (correr make install en libgodc)"; fallos=$((fallos+1)); }
else
  decir "KOS instalado: no encuentro $hi"; fallos=$((fallos+1))
fi

# 3) log de arranque (opcional)
log="${1:-}"
if [ -n "$log" ]; then
  if [ -f "$log" ]; then
    s0=$(grep -a "gc_init-semiespacio0 v=" "$log" | head -1 | sed 's/.*v=//' | tr -d '\r')
    s1=$(grep -a "gc_init-semiespacio1 v=" "$log" | head -1 | sed 's/.*v=//' | tr -d '\r')
    if [ -n "$s0" ] && [ -n "$s1" ]; then
      dif=$(( s1 - s0 ))
      decir "arranque      : semiespacio0=$s0 semiespacio1=$s1 separacion=$dif"
      esperado=$(( KB_ESPERADO * 1024 ))
      # el runtime redondea a pagina: se acepta +-4 KB
      if [ "$dif" -ge $(( esperado - 4096 )) ] && [ "$dif" -le $(( esperado + 4096 )) ]; then
        decir "  OK: la separacion corresponde a ${KB_ESPERADO} KB"
      else
        decir "  FALLA: se esperaban ~${esperado} B"; fallos=$((fallos+1))
      fi
    else
      decir "arranque      : el log no trae las marcas gc_init-semiespacio0/1"; fallos=$((fallos+1))
    fi
  else
    decir "arranque      : no encuentro el log $log"; fallos=$((fallos+1))
  fi
fi

if [ "$fallos" -eq 0 ]; then
  decir "CONTROL OK"
  exit 0
fi
decir "CONTROL EN ROJO ($fallos fallas)"
exit 1
