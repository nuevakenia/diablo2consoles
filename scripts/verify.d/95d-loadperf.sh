scenario_realtime=1
scenario_timeout=240
scenario_name="level load and first-sighting costs on a populated real level (Frigid Highlands with its natural population; PERF spans populate/generate, PERF load = animation mode builds on the frame thread; OD2_PREFETCH=0 for the A/B)"
scenario_env() {
  echo 'export OD2_AUTOLEVEL=111 OD2_POPULATE=1 OD2_AUTOPERF=1 OD2_AUTOPERF_SECONDS=8 OD2_AUTOPERF_WARMUP=4'
}
scenario_check() {
  grep -E "PERF (mark|span|update|render|frame|load)|POPULATE level" $log.txt | cut -c1-220
  grep -qE "PERF span populate level=111" $log.txt || { echo "FAIL: no populate span (level not populated)"; fail=1; }
  grep -qE "PERF load modeloads=" $log.txt || { echo "FAIL: no PERF load line"; fail=1; }
}
