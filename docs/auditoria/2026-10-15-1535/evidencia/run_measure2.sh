#!/bin/bash
# Mide working set real del motor (fork diablo2consoles) en Linux x86-64.
# Uso: run_measure2.sh <nivel> <segundos> [etiqueta]
# Mide VmHWM del proceso od2 (no del timeout) y guarda screenshot + perf.
set -u
S=/home/dgalli/.hermes/cache/scratch
REPO=/home/dgalli/Proyectos/diablo2consoles
LEVEL="${1:-1}"; SECS="${2:-10}"; TAG="${3:-l$LEVEL}"
LOG="$S/run-$TAG.log"; SHOT="$S/shot-$TAG.png"
cd "$REPO" || exit 1
export DISPLAY="${DISPLAY:-:0}"
export OD2_CONFIG_DIR=$S/od2cfg
export OD2_NO_SETUP=1 OD2_AUTOTEST_MUTE=1 OD2_AUTOEXIT=1
export OD2_AUTOGAME="$S/od2cfg/Saves/0.od2"
export OD2_AUTOLEVEL="$LEVEL"
export OD2_AUTOPERF=1 OD2_AUTOPERF_SECONDS="$SECS" OD2_AUTOPERF_WARMUP=4
export OD2_PPROF_HEAP="$S/heap-$TAG.pprof"
export OD2_AUTOSHOT="$SHOT" OD2_AUTOSHOT_SECONDS=$((SECS-2))
timeout 200 "$S/od2" > "$LOG" 2>&1 &
PID=$!
MAX=0
while kill -0 "$PID" 2>/dev/null; do
  for p in $(pgrep -x od2 2>/dev/null); do
    h=$(awk '/VmHWM/{print $2}' "/proc/$p/status" 2>/dev/null)
    if [ -n "${h:-}" ] && [ "$h" -gt "$MAX" ]; then MAX=$h; fi
  done
  sleep 0.25
done
wait "$PID"; RC=$?
SHOTSZ=$(stat -c%s "$SHOT" 2>/dev/null || echo 0)
echo "RESULT tag=$TAG level=$LEVEL exit=$RC VmHWM_MAX=${MAX}kB ($((MAX/1024))MB) shot=${SHOTSZ}B"
grep -E "PERF runtime|PERF frame |PERF load|PERF cpu|Perf mark" "$LOG" | tail -12
echo "--- lineas con error/panic/fail ---"
grep -icE "error|panic|failed" "$LOG"
