#!/bin/bash
# Estado estacionario: deja el heroe quieto en un nivel, muestrea VmRSS/VmHWM cada 0.5s
# y deja que autoperf escriba el heap profile. Uso: run_steady.sh <nivel> [segundos_totales]
set -u
S=/home/dgalli/.hermes/cache/scratch
REPO=/home/dgalli/Proyectos/diablo2consoles
LEVEL="${1:-1}"; TOT="${2:-35}"
TAG="steady$LEVEL"; LOG="$S/run-$TAG.log"
cd "$REPO" || exit 1
export DISPLAY="${DISPLAY:-:0}"
export OD2_CONFIG_DIR=$S/od2cfg
export OD2_NO_SETUP=1 OD2_AUTOTEST_MUTE=1
export OD2_AUTOGAME="$S/od2cfg/Saves/0.od2"
export OD2_AUTOLEVEL="$LEVEL"
export OD2_AUTOSCRIPT="wait:$TOT"
export OD2_AUTOPERF=1 OD2_AUTOPERF_SECONDS=10 OD2_AUTOPERF_WARMUP=3
export OD2_PPROF_HEAP="$S/heap-$TAG.pprof"
timeout "$TOT" "$S/od2" > "$LOG" 2>&1 &
PID=$!
START=$(date +%s); MAXRSS=0; MAXHWM=0; LAST=0; STEADY=0
while kill -0 "$PID" 2>/dev/null; do
  for p in $(pgrep -x od2 2>/dev/null); do
    read -r rss hwm < <(awk '/VmRSS|VmHWM/{printf "%s ", $2}' "/proc/$p/status" 2>/dev/null)
    T=$(( $(date +%s) - START ))
    [ -n "${rss:-}" ] && [ "$rss" -gt "$MAXRSS" ] && MAXRSS=$rss
    [ -n "${hwm:-}" ] && [ "$hwm" -gt "$MAXHWM" ] && MAXHWM=$hwm
    [ -n "${rss:-}" ] && LAST=$rss
    [ "$T" -ge 15 ] && [ "${rss:-0}" -gt "$STEADY" ] && STEADY=$rss
    printf "%s %s %s\n" "$T" "${rss:-0}" "${hwm:-0}" >> "$S/rss-$TAG.txt"
  done
  sleep 0.5
done
wait "$PID"; RC=$?
echo "RESULT tag=$TAG level=$LEVEL exit=$RC"
echo "  RSS_max=$((MAXRSS/1024))MB  RSS_final=$((LAST/1024))MB  RSS_estable(t>=15s)=$((STEADY/1024))MB  VmHWM=$((MAXHWM/1024))MB"
echo "--- PERF ---"; grep -E "PERF runtime|PERF frame |PERF cpu" "$LOG" | tail -5
ls -la "$S/heap-$TAG.pprof" 2>/dev/null || echo "(sin heap profile)"
